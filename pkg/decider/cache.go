package decider

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
)

// Cache wraps a Decider with answer reuse, a per crawl call budget and
// optional record and replay through a JSON file.
type Cache struct {
	next     Decider
	path     string
	replay   bool
	maxCalls int

	mu      sync.Mutex
	calls   int
	answers map[string]Answer
}

// NewCache loads recorded answers from path when it exists. With replay set,
// next is never called and unrecorded questions fail with ErrNotRecorded.
// maxCalls <= 0 means unlimited.
func NewCache(next Decider, path string, replay bool, maxCalls int) (*Cache, error) {
	c := &Cache{next: next, path: path, replay: replay, maxCalls: maxCalls, answers: map[string]Answer{}}
	if path == "" {
		if replay {
			return nil, errors.New("decider: replay needs a recording file")
		}
		return c, nil
	}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist) && !replay:
		return c, nil
	case err != nil:
		return nil, err
	}
	if err := json.Unmarshal(data, &c.answers); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Cache) Decide(ctx context.Context, q Question) (Answer, error) {
	if err := q.validate(); err != nil {
		return Answer{}, err
	}
	key := q.key()

	c.mu.Lock()
	if a, ok := c.answers[key]; ok {
		c.mu.Unlock()
		return a, nil
	}
	if c.replay {
		c.mu.Unlock()
		return Answer{}, ErrNotRecorded
	}
	if c.maxCalls > 0 && c.calls >= c.maxCalls {
		c.mu.Unlock()
		return Answer{}, ErrBudgetExceeded
	}
	c.calls++
	c.mu.Unlock()

	a, err := c.next.Decide(ctx, q)
	if err != nil {
		return Answer{}, err
	}
	if err := q.check(a); err != nil {
		return Answer{}, err
	}

	c.mu.Lock()
	c.answers[key] = a
	c.mu.Unlock()
	return a, nil
}

// Close writes the recording, keeping answers loaded from an earlier run.
func (c *Cache) Close() error {
	if c.path == "" || c.replay {
		return nil
	}
	c.mu.Lock()
	data, err := json.MarshalIndent(c.answers, "", "  ")
	c.mu.Unlock()
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o600)
}
