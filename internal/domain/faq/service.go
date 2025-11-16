package faq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/yanqian/ai-helloworld/internal/infra/llm/chatgpt"
	apperrors "github.com/yanqian/ai-helloworld/pkg/errors"
)

// Service exposes smart FAQ capabilities.
type Service interface {
	Answer(ctx context.Context, req Request) (Response, error)
	Trending(ctx context.Context) ([]TrendingQuery, error)
}

type chatClient interface {
	CreateChatCompletion(ctx context.Context, req chatgpt.ChatCompletionRequest) (chatgpt.ChatCompletionResponse, error)
	CreateEmbedding(ctx context.Context, req chatgpt.EmbeddingRequest) (chatgpt.EmbeddingResponse, error)
}

type service struct {
	cfg    Config
	repo   QuestionRepository
	store  Store
	client chatClient
	logger *slog.Logger
	hasher *semanticHasher
}

// NewService wires up the FAQ domain.
func NewService(cfg Config, repo QuestionRepository, store Store, client chatClient, logger *slog.Logger) Service {
	return &service{
		cfg:    cfg,
		repo:   repo,
		store:  store,
		client: client,
		logger: logger.With("component", "faq.service"),
		hasher: newSemanticHasher(defaultSemanticHashPlanes, defaultSemanticHashSeed),
	}
}

func (s *service) Answer(ctx context.Context, req Request) (Response, error) {
	question := strings.TrimSpace(req.Question)
	if question == "" {
		return Response{}, apperrors.Wrap("invalid_input", "question cannot be empty", nil)
	}

	mode := sanitizeMode(req.Mode)
	normalized := normalizeQuestion(question)
	plan := resolveSearchPlan(mode)

	var (
		embedding       []float32
		semanticHash    uint64
		hasSemanticHash bool
	)

	var (
		record     QuestionRecord
		foundMatch bool
		actualMode = mode
	)

	for _, candidate := range plan {
		switch candidate {
		case SearchModeExact:
			rec, found, err := s.repo.FindExact(ctx, question)
			if err != nil {
				return Response{}, apperrors.Wrap("faq_error", "exact lookup failed", err)
			}
			if found {
				record = rec
				foundMatch = true
				actualMode = SearchModeExact
			}
		case SearchModeSemanticHash:
			var err error
			embedding, err = s.ensureEmbedding(ctx, embedding, question)
			if err != nil {
				return Response{}, apperrors.Wrap("faq_error", "embedding failed", err)
			}
			if !hasSemanticHash {
				semanticHash, hasSemanticHash, err = s.computeSemanticHash(embedding)
				if err != nil {
					return Response{}, apperrors.Wrap("faq_error", "semantic hash failed", err)
				}
			}
			if !hasSemanticHash {
				continue
			}
			rec, found, err := s.repo.FindBySemanticHash(ctx, semanticHash)
			if err != nil {
				return Response{}, apperrors.Wrap("faq_error", "semantic hash lookup failed", err)
			}
			if found {
				record = rec
				foundMatch = true
				actualMode = SearchModeSemanticHash
			}
		case SearchModeSimilarity:
			var err error
			embedding, err = s.ensureEmbedding(ctx, embedding, question)
			if err != nil {
				return Response{}, apperrors.Wrap("faq_error", "embedding failed", err)
			}
			match, found, err := s.repo.FindNearest(ctx, embedding)
			if err != nil {
				return Response{}, apperrors.Wrap("faq_error", "similarity lookup failed", err)
			}
			if found && match.Distance <= s.cfg.SimilarityThreshold {
				record = match.Question
				foundMatch = true
				actualMode = SearchModeSimilarity
			}
		}
		if foundMatch {
			break
		}
	}

	var (
		answer          string
		source          = "cache"
		matchedQuestion = question
		questionID      int64
	)

	if foundMatch {
		questionID = record.ID
		matchedQuestion = record.QuestionText
		cached, ok, err := s.store.GetAnswer(ctx, questionID)
		if err != nil {
			return Response{}, apperrors.Wrap("faq_error", "cache lookup failed", err)
		}
		if ok {
			answer = cached.Answer
		} else {
			source = "llm"
			var genErr error
			answer, genErr = s.generateAndCacheAnswer(ctx, questionID, matchedQuestion)
			if genErr != nil {
				return Response{}, genErr
			}
		}
	} else {
		var err error
		embedding, err = s.ensureEmbedding(ctx, embedding, question)
		if err != nil {
			return Response{}, apperrors.Wrap("faq_error", "embedding failed", err)
		}
		if !hasSemanticHash {
			semanticHash, hasSemanticHash, err = s.computeSemanticHash(embedding)
			if err != nil {
				return Response{}, apperrors.Wrap("faq_error", "semantic hash failed", err)
			}
		}
		var hashPtr *uint64
		if hasSemanticHash {
			hashCopy := semanticHash
			hashPtr = &hashCopy
		}
		rec, err := s.repo.InsertQuestion(ctx, question, embedding, hashPtr)
		if err != nil {
			return Response{}, apperrors.Wrap("faq_error", "failed to insert question", err)
		}
		record = rec
		questionID = rec.ID
		matchedQuestion = question
		source = "llm"
		answer, err = s.generateAndCacheAnswer(ctx, questionID, question)
		if err != nil {
			return Response{}, err
		}
	}

	if err := s.store.IncrementQuery(ctx, normalized, question); err != nil {
		s.logger.Warn("faq trending increment failed", "error", err)
	}

	recs, err := s.store.TopQueries(ctx, s.cfg.TopRecommendations)
	if err != nil {
		s.logger.Warn("faq trending fetch failed", "error", err)
		recs = nil
	}

	return Response{
		Question:        question,
		Answer:          answer,
		Source:          source,
		MatchedQuestion: matchedQuestion,
		Mode:            actualMode,
		Recommendations: recs,
	}, nil
}

func (s *service) Trending(ctx context.Context) ([]TrendingQuery, error) {
	recs, err := s.store.TopQueries(ctx, s.cfg.TopRecommendations)
	if err != nil {
		return nil, apperrors.Wrap("faq_error", "failed to load trending queries", err)
	}
	return recs, nil
}

func (s *service) generateAndCacheAnswer(ctx context.Context, questionID int64, question string) (string, error) {
	answer, err := s.askLLM(ctx, question)
	if err != nil {
		return "", err
	}
	record := AnswerRecord{
		QuestionID: questionID,
		Question:   question,
		Answer:     answer,
		CreatedAt:  time.Now(),
	}
	if err := s.store.SaveAnswer(ctx, record, s.cfg.CacheTTL); err != nil {
		s.logger.Warn("faq cache save failed", "error", err)
	}
	return answer, nil
}

func (s *service) ensureEmbedding(ctx context.Context, current []float32, question string) ([]float32, error) {
	if len(current) > 0 {
		return current, nil
	}
	embedding, err := s.embedQuestion(ctx, question)
	if err != nil {
		return nil, err
	}
	if len(embedding) == 0 {
		return nil, errors.New("embedding response empty")
	}
	return embedding, nil
}

func (s *service) embedQuestion(ctx context.Context, text string) ([]float32, error) {
	input := strings.TrimSpace(text)
	if input == "" {
		return nil, nil
	}
	resp, err := s.client.CreateEmbedding(ctx, chatgpt.EmbeddingRequest{
		Model: s.cfg.EmbeddingModel,
		Input: input,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 || len(resp.Data[0].Embedding) == 0 {
		return nil, errors.New("embedding response empty")
	}
	vector := make([]float32, len(resp.Data[0].Embedding))
	copy(vector, resp.Data[0].Embedding)
	return vector, nil
}

func (s *service) askLLM(ctx context.Context, question string) (string, error) {
	prompt := strings.TrimSpace(s.cfg.Prompt)
	if prompt == "" {
		prompt = "You are a helpful knowledge base assistant."
	}
	messages := []chatgpt.Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: fmt.Sprintf("Question: %s\nAnswer concisely in 3 sentences or less.", question)},
	}
	resp, err := s.client.CreateChatCompletion(ctx, chatgpt.ChatCompletionRequest{
		Model:       s.cfg.Model,
		Messages:    messages,
		Temperature: s.cfg.Temperature,
	})
	if err != nil {
		return "", apperrors.Wrap("llm_error", "chatgpt request failed", err)
	}
	if len(resp.Choices) == 0 {
		return "", apperrors.Wrap("llm_error", "chatgpt returned no choices", errors.New("empty choices"))
	}
	answer := strings.TrimSpace(resp.Choices[0].Message.Content)
	if answer == "" {
		return "", apperrors.Wrap("llm_error", "chatgpt response empty", nil)
	}
	return answer, nil
}

func (s *service) computeSemanticHash(embedding []float32) (uint64, bool, error) {
	if len(embedding) == 0 || s.hasher == nil {
		return 0, false, nil
	}
	return s.hasher.Hash(embedding)
}

func resolveSearchPlan(mode SearchMode) []SearchMode {
	switch mode {
	case SearchModeExact:
		return []SearchMode{SearchModeExact}
	case SearchModeSemanticHash:
		return []SearchMode{SearchModeSemanticHash}
	case SearchModeSimilarity:
		return []SearchMode{SearchModeSimilarity}
	default:
		return []SearchMode{SearchModeExact, SearchModeSimilarity}
	}
}

func sanitizeMode(mode SearchMode) SearchMode {
	switch mode {
	case SearchModeExact, SearchModeSemanticHash, SearchModeSimilarity, SearchModeHybrid:
		return mode
	default:
		return SearchModeHybrid
	}
}
