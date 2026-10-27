package decider

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/projectdiscovery/gologger"
)

// Cache wraps a Decider with answer reuse, a per crawl call budget and
// optional record and replay through a JSON file.
type Cache struct {
	next     Decider
	path     string
	replay   bool
	maxCalls int

	mu       sync.Mutex
	calls    int
	answers  map[string]Answer
	inflight map[string]chan struct{}
	// failed stops all calls once the provider rejects the configuration
	failed error
}

// NewCache loads recorded answers from path when it exists. With replay set,
// next is never called and unrecorded questions fail with ErrNotRecorded.
// maxCalls <= 0 means unlimited.
func NewCache(next Decider, path string, replay bool, maxCalls int) (*Cache, error) {
	c := &Cache{next: next, path: path, replay: replay, maxCalls: maxCalls, answers: map[string]Answer{}, inflight: map[string]chan struct{}{}}
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

	for {
		c.mu.Lock()
		if a, ok := c.answers[key]; ok {
			c.mu.Unlock()
			// recordings are files a user can edit, so they get the same check as live answers
			return a, q.check(a)
		}
		if c.replay {
			c.mu.Unlock()
			return Answer{}, ErrNotRecorded
		}
		if c.failed != nil {
			c.mu.Unlock()
			return Answer{}, c.failed
		}
		// the same form often appears on many pages crawled at once, so
		// identical questions wait for the first one instead of using budget
		if wait, ok := c.inflight[key]; ok {
			c.mu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				return Answer{}, ctx.Err()
			}
		}
		if c.maxCalls > 0 && c.calls >= c.maxCalls {
			c.mu.Unlock()
			return Answer{}, ErrBudgetExceeded
		}
		c.calls++
		done := make(chan struct{})
		c.inflight[key] = done
		c.mu.Unlock()

		a, err := c.ask(ctx, q)

		c.mu.Lock()
		delete(c.inflight, key)
		if err == nil {
			c.answers[key] = a
		} else if errors.Is(err, ErrUnavailable) && c.failed == nil {
			c.failed = err
			gologger.Error().Msgf("ai decider disabled: %s", err)
		}
		c.mu.Unlock()
		close(done)
		return a, err
	}
}

func (c *Cache) ask(ctx context.Context, q Question) (Answer, error) {
	a, err := c.next.Decide(ctx, q)
	if err != nil {
		return Answer{}, err
	}
	return a, q.check(a)
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
