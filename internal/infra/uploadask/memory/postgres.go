package memory

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
)

// PostgresMessageLog persists upload_qa_messages in Postgres.
type PostgresMessageLog struct {
	pool *pgxpool.Pool
}

// NewPostgresMessageLog constructs the adapter.
func NewPostgresMessageLog(pool *pgxpool.Pool) *PostgresMessageLog {
	return &PostgresMessageLog{pool: pool}
}

// Append inserts a conversation message.
func (l *PostgresMessageLog) Append(ctx context.Context, msg domain.ConversationMessage) error {
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	_, err := l.pool.Exec(ctx, `
		INSERT INTO upload_qa_messages (session_id, user_id, role, content, token_count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, msg.SessionID, msg.UserID, msg.Role, msg.Content, msg.TokenCount, msg.CreatedAt)
	return err
}

// ListRecent returns the newest messages that fit within the token and message budgets.
func (l *PostgresMessageLog) ListRecent(ctx context.Context, userID int64, sessionID uuid.UUID, maxTokens int, maxMessages int) ([]domain.ConversationMessage, error) {
	limit := maxMessages
	if limit <= 0 {
		limit = 200
	}
	rows, err := l.pool.Query(ctx, `
		SELECT id, session_id, user_id, role, content, token_count, created_at
		FROM upload_qa_messages
		WHERE session_id = $1 AND user_id = $2
		ORDER BY created_at DESC
		LIMIT $3
	`, sessionID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	collected := make([]domain.ConversationMessage, 0)
	totalTokens := 0
	for rows.Next() {
		var msg domain.ConversationMessage
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.UserID, &msg.Role, &msg.Content, &msg.TokenCount, &msg.CreatedAt); err != nil {
			return nil, err
		}
		tokens := msg.TokenCount
		if tokens < 0 {
			tokens = 0
		}
		if maxTokens > 0 && totalTokens+tokens > maxTokens {
			break
		}
		totalTokens += tokens
		collected = append(collected, msg)
		if maxMessages > 0 && len(collected) >= maxMessages {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// reverse to chronological order
	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}
	return collected, nil
}

var _ domain.MessageLog = (*PostgresMessageLog)(nil)

// PostgresMemoryStore persists upload_qa_memories in Postgres.
type PostgresMemoryStore struct {
	pool *pgxpool.Pool
}

// NewPostgresMemoryStore constructs the adapter.
func NewPostgresMemoryStore(pool *pgxpool.Pool) *PostgresMemoryStore {
	return &PostgresMemoryStore{pool: pool}
}

// Upsert stores or updates a memory row keyed by user/session/source/content.
func (s *PostgresMemoryStore) Upsert(ctx context.Context, mem domain.MemoryRecord) error {
	if mem.CreatedAt.IsZero() {
		mem.CreatedAt = time.Now()
	}
	var embedding any
	if len(mem.Embedding) > 0 {
		embedding = pgvector.NewVector(mem.Embedding)
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO upload_qa_memories (session_id, user_id, source, content, embedding, importance, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, session_id, source, content)
		DO UPDATE SET embedding = EXCLUDED.embedding, importance = EXCLUDED.importance, created_at = EXCLUDED.created_at
		RETURNING id, created_at
	`, mem.SessionID, mem.UserID, mem.Source, mem.Content, embedding, mem.Importance, mem.CreatedAt).Scan(&mem.ID, &mem.CreatedAt)
}

// Search returns the top-k similar memories for a session.
func (s *PostgresMemoryStore) Search(ctx context.Context, userID int64, sessionID uuid.UUID, embedding []float32, k int) ([]domain.RetrievedMemory, error) {
	if len(embedding) == 0 {
		return nil, nil
	}
	limit := k
	if limit <= 0 {
		limit = 8
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, session_id, user_id, source, content, embedding, importance, created_at,
		       (1.0 / (1.0 + (embedding <-> $3))) AS score
		FROM upload_qa_memories
		WHERE user_id = $1 AND session_id = $2 AND embedding IS NOT NULL
		ORDER BY (embedding <-> $3) ASC
		LIMIT $4
	`, userID, sessionID, pgvector.NewVector(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]domain.RetrievedMemory, 0)
	for rows.Next() {
		var (
			rec     domain.MemoryRecord
			rawEmb  any
			score   float64
			source  domain.MemorySource
			created time.Time
		)
		if err := rows.Scan(&rec.ID, &rec.SessionID, &rec.UserID, &source, &rec.Content, &rawEmb, &rec.Importance, &created, &score); err != nil {
			return nil, err
		}
		rec.Source = source
		rec.CreatedAt = created
		parsed, err := normalizeEmbedding(rawEmb)
		if err != nil {
			return nil, err
		}
		rec.Embedding = parsed
		results = append(results, domain.RetrievedMemory{
			Memory:    rec,
			Score:     score,
			CreatedAt: rec.CreatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// Prune removes older memories for a user, optionally scoped to a session.
func (s *PostgresMemoryStore) Prune(ctx context.Context, userID int64, sessionID *uuid.UUID, limit int) error {
	if limit <= 0 {
		return nil
	}
	if sessionID != nil {
		_, err := s.pool.Exec(ctx, `
			DELETE FROM upload_qa_memories
			WHERE user_id = $1 AND session_id = $2 AND id IN (
				SELECT id FROM upload_qa_memories
				WHERE user_id = $1 AND session_id = $2
				ORDER BY importance DESC, created_at DESC
				OFFSET $3
			)
		`, userID, *sessionID, limit)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM upload_qa_memories
		WHERE user_id = $1 AND id IN (
			SELECT id FROM upload_qa_memories
			WHERE user_id = $1
			ORDER BY importance DESC, created_at DESC
			OFFSET $2
		)
	`, userID, limit)
	return err
}

var _ domain.MemoryStore = (*PostgresMemoryStore)(nil)

func normalizeEmbedding(raw any) ([]float32, error) {
	switch v := raw.(type) {
	case pgvector.Vector:
		return append([]float32(nil), v.Slice()...), nil
	case []float32:
		return append([]float32(nil), v...), nil
	case []float64:
		out := make([]float32, len(v))
		for i, f := range v {
			out[i] = float32(f)
		}
		return out, nil
	case string:
		trimmed := strings.TrimSpace(v)
		trimmed = strings.TrimPrefix(trimmed, "[")
		trimmed = strings.TrimSuffix(trimmed, "]")
		if trimmed == "" {
			return nil, nil
		}
		parts := strings.Split(trimmed, ",")
		out := make([]float32, 0, len(parts))
		for _, p := range parts {
			numStr := strings.TrimSpace(p)
			if numStr == "" {
				continue
			}
			f, err := strconv.ParseFloat(numStr, 32)
			if err != nil {
				return nil, err
			}
			out = append(out, float32(f))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported embedding type %T", raw)
	}
}
