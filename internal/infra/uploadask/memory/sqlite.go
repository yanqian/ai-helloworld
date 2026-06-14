package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
)

// SQLiteMessageLog persists Upload & Ask conversation turns in SQLite.
type SQLiteMessageLog struct {
	db *sql.DB
}

// NewSQLiteMessageLog constructs a SQLite-backed message log.
func NewSQLiteMessageLog(db *sql.DB) *SQLiteMessageLog {
	return &SQLiteMessageLog{db: db}
}

func (l *SQLiteMessageLog) Append(ctx context.Context, msg domain.ConversationMessage) error {
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	_, err := l.db.ExecContext(ctx, `
		INSERT INTO upload_qa_messages (session_id, user_id, role, content, token_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, msg.SessionID.String(), msg.UserID, string(msg.Role), msg.Content, msg.TokenCount, formatSQLiteTime(msg.CreatedAt))
	return err
}

func (l *SQLiteMessageLog) ListRecent(ctx context.Context, userID int64, sessionID uuid.UUID, maxTokens int, maxMessages int) ([]domain.ConversationMessage, error) {
	limit := maxMessages
	if limit <= 0 {
		limit = 200
	}
	rows, err := l.db.QueryContext(ctx, `
		SELECT id, session_id, user_id, role, content, token_count, created_at
		FROM upload_qa_messages
		WHERE session_id = ? AND user_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, sessionID.String(), userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	collected := make([]domain.ConversationMessage, 0)
	totalTokens := 0
	for rows.Next() {
		msg, err := scanSQLiteMessage(rows)
		if err != nil {
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
	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}
	return collected, nil
}

var _ domain.MessageLog = (*SQLiteMessageLog)(nil)

// SQLiteMemoryStore persists long-term conversational memories in SQLite.
type SQLiteMemoryStore struct {
	db *sql.DB
}

// NewSQLiteMemoryStore constructs a SQLite-backed memory store.
func NewSQLiteMemoryStore(db *sql.DB) *SQLiteMemoryStore {
	return &SQLiteMemoryStore{db: db}
}

func (s *SQLiteMemoryStore) Upsert(ctx context.Context, mem domain.MemoryRecord) error {
	if mem.CreatedAt.IsZero() {
		mem.CreatedAt = time.Now().UTC()
	}
	var embedding any
	if len(mem.Embedding) > 0 {
		payload, err := json.Marshal(mem.Embedding)
		if err != nil {
			return err
		}
		embedding = string(payload)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO upload_qa_memories (session_id, user_id, source, content, embedding, importance, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, session_id, source, content) DO UPDATE SET
			embedding = excluded.embedding,
			importance = excluded.importance,
			created_at = excluded.created_at
	`, mem.SessionID.String(), mem.UserID, string(mem.Source), mem.Content, embedding, mem.Importance, formatSQLiteTime(mem.CreatedAt))
	return err
}

func (s *SQLiteMemoryStore) Search(ctx context.Context, userID int64, sessionID uuid.UUID, embedding []float32, k int) ([]domain.RetrievedMemory, error) {
	if len(embedding) == 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, session_id, user_id, source, content, embedding, importance, created_at
		FROM upload_qa_memories
		WHERE user_id = ? AND session_id = ? AND embedding IS NOT NULL
	`, userID, sessionID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]domain.RetrievedMemory, 0)
	for rows.Next() {
		mem, err := scanSQLiteMemory(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, domain.RetrievedMemory{
			Memory:    mem,
			Score:     cosineSimilarity(embedding, mem.Embedding),
			CreatedAt: mem.CreatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].CreatedAt.After(results[j].CreatedAt)
		}
		return results[i].Score > results[j].Score
	})
	if k > 0 && len(results) > k {
		results = results[:k]
	}
	return results, nil
}

func (s *SQLiteMemoryStore) Prune(ctx context.Context, userID int64, sessionID *uuid.UUID, limit int) error {
	if limit <= 0 {
		return nil
	}
	if sessionID != nil {
		_, err := s.db.ExecContext(ctx, `
			DELETE FROM upload_qa_memories
			WHERE user_id = ? AND session_id = ? AND id IN (
				SELECT id FROM upload_qa_memories
				WHERE user_id = ? AND session_id = ?
				ORDER BY importance DESC, created_at DESC
				LIMIT -1 OFFSET ?
			)
		`, userID, sessionID.String(), userID, sessionID.String(), limit)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM upload_qa_memories
		WHERE user_id = ? AND id IN (
			SELECT id FROM upload_qa_memories
			WHERE user_id = ?
			ORDER BY importance DESC, created_at DESC
			LIMIT -1 OFFSET ?
		)
	`, userID, userID, limit)
	return err
}

var _ domain.MemoryStore = (*SQLiteMemoryStore)(nil)

type sqliteScanner interface {
	Scan(dest ...any) error
}

func scanSQLiteMessage(row sqliteScanner) (domain.ConversationMessage, error) {
	var (
		msg       domain.ConversationMessage
		sessionID string
		role      string
		created   string
	)
	if err := row.Scan(&msg.ID, &sessionID, &msg.UserID, &role, &msg.Content, &msg.TokenCount, &created); err != nil {
		return domain.ConversationMessage{}, err
	}
	parsedSessionID, err := uuid.Parse(sessionID)
	if err != nil {
		return domain.ConversationMessage{}, err
	}
	createdAt, err := parseSQLiteTime(created)
	if err != nil {
		return domain.ConversationMessage{}, err
	}
	msg.SessionID = parsedSessionID
	msg.Role = domain.MessageRole(role)
	msg.CreatedAt = createdAt
	return msg, nil
}

func scanSQLiteMemory(row sqliteScanner) (domain.MemoryRecord, error) {
	var (
		mem       domain.MemoryRecord
		sessionID string
		source    string
		embedding sql.NullString
		created   string
	)
	if err := row.Scan(&mem.ID, &sessionID, &mem.UserID, &source, &mem.Content, &embedding, &mem.Importance, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.MemoryRecord{}, nil
		}
		return domain.MemoryRecord{}, err
	}
	parsedSessionID, err := uuid.Parse(sessionID)
	if err != nil {
		return domain.MemoryRecord{}, err
	}
	createdAt, err := parseSQLiteTime(created)
	if err != nil {
		return domain.MemoryRecord{}, err
	}
	if embedding.Valid && embedding.String != "" {
		var parsed []float32
		if err := json.Unmarshal([]byte(embedding.String), &parsed); err != nil {
			return domain.MemoryRecord{}, err
		}
		mem.Embedding = parsed
	}
	mem.SessionID = parsedSessionID
	mem.Source = domain.MemorySource(source)
	mem.CreatedAt = createdAt
	return mem, nil
}

func formatSQLiteTime(value time.Time) string {
	if value.IsZero() {
		value = time.Now().UTC()
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseSQLiteTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05.999999999-0700",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05-0700",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
		lastErr = err
	}
	return time.Time{}, fmt.Errorf("parse sqlite uploadask time %q: %w", value, lastErr)
}
