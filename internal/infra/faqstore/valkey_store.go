package faqstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/yanqian/ai-helloworld/internal/domain/faq"
)

// ValkeyStore persists FAQ entries using a Valkey-compatible database.
type ValkeyStore struct {
	client valkey.Client
	prefix string
}

// NewValkeyStore constructs a new store backed by Valkey.
func NewValkeyStore(client valkey.Client, prefix string) *ValkeyStore {
	if prefix == "" {
		prefix = "faq"
	}
	return &ValkeyStore{client: client, prefix: prefix}
}

func (s *ValkeyStore) GetAnswer(ctx context.Context, questionID int64) (faq.AnswerRecord, bool, error) {
	if questionID <= 0 {
		return faq.AnswerRecord{}, false, nil
	}
	cmd := s.client.B().Get().Key(s.entryKey(questionID)).Build()
	result := s.client.Do(ctx, cmd)
	payload, err := result.ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return faq.AnswerRecord{}, false, nil
		}
		return faq.AnswerRecord{}, false, err
	}
	var record faq.AnswerRecord
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return faq.AnswerRecord{}, false, err
	}
	return record, true, nil
}

func (s *ValkeyStore) SaveAnswer(ctx context.Context, record faq.AnswerRecord, ttl time.Duration) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.setString(ctx, s.entryKey(record.QuestionID), string(payload), ttl)
}

func (s *ValkeyStore) IncrementQuery(ctx context.Context, canonical, display string) error {
	if canonical == "" {
		return nil
	}
	if err := s.client.Do(ctx, s.client.B().Zincrby().Key(s.trendingKey()).Increment(1).Member(canonical).Build()).Error(); err != nil {
		return err
	}
	if display != "" {
		_ = s.client.Do(ctx, s.client.B().Set().Key(s.displayKey(canonical)).Value(display).Nx().Build()).Error()
	}
	return nil
}

func (s *ValkeyStore) TopQueries(ctx context.Context, limit int) ([]faq.TrendingQuery, error) {
	if limit <= 0 {
		limit = 10
	}
	resp := s.client.Do(ctx, s.client.B().Zrevrange().Key(s.trendingKey()).Start(0).Stop(int64(limit-1)).Withscores().Build())
	arr, err := resp.ToArray()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]faq.TrendingQuery, 0, len(arr))
	for i := 0; i < len(arr); {
		var (
			member string
			score  float64
		)
		if tuple, tupleErr := arr[i].ToArray(); tupleErr == nil && len(tuple) == 2 {
			// RESP3 returns [member, score] per element
			if member, err = tuple[0].ToString(); err != nil {
				if valkey.IsValkeyNil(err) {
					i++
					continue
				}
				return nil, err
			}
			if score, err = tuple[1].ToFloat64(); err != nil {
				return nil, err
			}
			i++
		} else {
			// RESP2 returns a flat alternating array.
			if i+1 >= len(arr) {
				break
			}
			if member, err = arr[i].ToString(); err != nil {
				if valkey.IsValkeyNil(err) {
					i += 2
					continue
				}
				return nil, err
			}
			if score, err = arr[i+1].ToFloat64(); err != nil {
				return nil, err
			}
			i += 2
		}
		display := s.fetchDisplay(ctx, member)
		out = append(out, faq.TrendingQuery{Query: display, Count: int64(score)})
	}
	return out, nil
}

func (s *ValkeyStore) fetchDisplay(ctx context.Context, canonical string) string {
	resp := s.client.Do(ctx, s.client.B().Get().Key(s.displayKey(canonical)).Build())
	display, err := resp.ToString()
	if err != nil || display == "" {
		return canonical
	}
	return display
}

func (s *ValkeyStore) setString(ctx context.Context, key, value string, ttl time.Duration) error {
	builder := s.client.B().Set().Key(key).Value(value)
	var cmd valkey.Completed
	if ttl > 0 {
		if ttl < time.Second {
			ttl = time.Second
		}
		cmd = builder.Ex(ttl).Build()
	} else {
		cmd = builder.Build()
	}
	return s.client.Do(ctx, cmd).Error()
}

func (s *ValkeyStore) entryKey(id int64) string {
	return fmt.Sprintf("q:%d", id)
}

func (s *ValkeyStore) trendingKey() string {
	return fmt.Sprintf("%s:trending", s.prefix)
}

func (s *ValkeyStore) displayKey(canonical string) string {
	return fmt.Sprintf("%s:display:%s", s.prefix, canonical)
}

var _ faq.Store = (*ValkeyStore)(nil)
