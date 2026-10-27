package decider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stub struct {
	answer Answer
	calls  int
}

func (s *stub) Decide(context.Context, Question) (Answer, error) {
	s.calls++
	return s.answer, nil
}

var zip = Question{Kind: Choice, Prompt: "field type?", Options: []string{"zip", "date"}, Context: `<input name="postcode">`}

func TestCacheReusesAnswers(t *testing.T) {
	s := &stub{answer: Answer{Choice: "zip"}}
	c, err := NewCache(s, "", false, 0)
	require.NoError(t, err)

	for range 3 {
		a, err := c.Decide(context.Background(), zip)
		require.NoError(t, err)
		require.Equal(t, "zip", a.Choice)
	}
	require.Equal(t, 1, s.calls)
}

func TestCacheRejectsInvalidAnswers(t *testing.T) {
	c, err := NewCache(&stub{answer: Answer{Choice: "rm -rf"}}, "", false, 0)
	require.NoError(t, err)
	_, err = c.Decide(context.Background(), zip)
	require.Error(t, err)

	c, err = NewCache(&stub{answer: Answer{Score: 2}}, "", false, 0)
	require.NoError(t, err)
	_, err = c.Decide(context.Background(), Question{Kind: Score, Prompt: "value?"})
	require.Error(t, err)

	_, err = c.Decide(context.Background(), Question{Kind: Choice, Prompt: "no options"})
	require.Error(t, err)
}

func TestCacheBudget(t *testing.T) {
	s := &stub{answer: Answer{Bool: true}}
	c, err := NewCache(s, "", false, 1)
	require.NoError(t, err)

	_, err = c.Decide(context.Background(), Question{Kind: Bool, Prompt: "a"})
	require.NoError(t, err)
	_, err = c.Decide(context.Background(), Question{Kind: Bool, Prompt: "a"})
	require.NoError(t, err, "cached answers do not count against the budget")
	_, err = c.Decide(context.Background(), Question{Kind: Bool, Prompt: "b"})
	require.ErrorIs(t, err, ErrBudgetExceeded)
	require.Equal(t, 1, s.calls)
}

func TestCacheRecordAndReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "answers.json")

	_, err := NewCache(&stub{}, path, true, 0)
	require.Error(t, err, "replay needs an existing recording")

	rec, err := NewCache(&stub{answer: Answer{Choice: "zip"}}, path, false, 0)
	require.NoError(t, err)
	_, err = rec.Decide(context.Background(), zip)
	require.NoError(t, err)
	require.NoError(t, rec.Close())

	s := &stub{}
	replay, err := NewCache(s, path, true, 0)
	require.NoError(t, err)
	a, err := replay.Decide(context.Background(), zip)
	require.NoError(t, err)
	require.Equal(t, "zip", a.Choice)
	_, err = replay.Decide(context.Background(), Question{Kind: Bool, Prompt: "new"})
	require.ErrorIs(t, err, ErrNotRecorded)
	require.Zero(t, s.calls)
}

func TestOpenAI(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer k", r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"value\":\"zip\"}"}}]}`))
	}))
	defer srv.Close()

	a, err := NewOpenAI(srv.URL+"/v1/", "m", "k", time.Second).Decide(context.Background(), zip)
	require.NoError(t, err)
	require.Equal(t, "zip", a.Choice)

	schema := got["response_format"].(map[string]any)["json_schema"].(map[string]any)["schema"].(map[string]any)
	value := schema["properties"].(map[string]any)["value"].(map[string]any)
	require.Equal(t, []any{"zip", "date"}, value["enum"])
	user := got["messages"].([]any)[1].(map[string]any)["content"].(string)
	require.Contains(t, user, "Answer with one of: zip, date")
	require.Contains(t, user, "<page>\n<input name=\"postcode\">\n</page>")
}

func TestOpenAIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "no key", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"zip"}}]}`))
	}))
	defer srv.Close()

	_, err := NewOpenAI(srv.URL, "m", "", time.Second).Decide(context.Background(), zip)
	require.ErrorContains(t, err, "401")
	_, err = NewOpenAI(srv.URL, "m", "k", time.Second).Decide(context.Background(), zip)
	require.ErrorContains(t, err, "malformed answer")
}

func TestCacheValidatesRecordedAnswers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "answers.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"`+zip.key()+`":{"choice":"rm -rf"}}`), 0o600))
	c, err := NewCache(nil, path, true, 0)
	require.NoError(t, err)
	_, err = c.Decide(context.Background(), zip)
	require.Error(t, err)
}

type slowStub struct {
	calls   atomic.Int32
	release chan struct{}
}

func (s *slowStub) Decide(context.Context, Question) (Answer, error) {
	s.calls.Add(1)
	<-s.release
	return Answer{Choice: "zip"}, nil
}

func TestCacheCoalescesConcurrentQuestions(t *testing.T) {
	s := &slowStub{release: make(chan struct{})}
	c, err := NewCache(s, "", false, 1)
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for range 5 {
		wg.Go(func() {
			_, err := c.Decide(context.Background(), zip)
			errs <- err
		})
	}
	require.Eventually(t, func() bool { return s.calls.Load() == 1 }, time.Second, 10*time.Millisecond)
	close(s.release)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err, "waiters must not spend the budget")
	}
	require.EqualValues(t, 1, s.calls.Load())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	blocked := &slowStub{release: make(chan struct{})}
	c, err = NewCache(blocked, "", false, 0)
	require.NoError(t, err)
	go func() { _, _ = c.Decide(context.Background(), zip) }()
	require.Eventually(t, func() bool { return blocked.calls.Load() == 1 }, time.Second, 10*time.Millisecond)
	_, err = c.Decide(ctx, zip)
	require.ErrorIs(t, err, context.Canceled, "a waiter keeps its own context")
	close(blocked.release)
}

func TestCacheCoalescesFailedQuestions(t *testing.T) {
	s := &errStub{release: make(chan struct{}), err: errors.New("malformed")}
	c, err := NewCache(s, "", false, 5)
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for range 5 {
		wg.Go(func() {
			_, err := c.Decide(context.Background(), zip)
			errs <- err
		})
	}
	require.Eventually(t, func() bool { return s.calls.Load() == 1 }, time.Second, 10*time.Millisecond)
	close(s.release)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.ErrorContains(t, err, "malformed")
	}
	require.EqualValues(t, 1, s.calls.Load(), "waiters must share the error")

	_, err = c.Decide(context.Background(), zip)
	require.ErrorContains(t, err, "malformed")
	require.EqualValues(t, 2, s.calls.Load(), "a later call can try again")
}

type errStub struct {
	calls   atomic.Int32
	release chan struct{}
	err     error
}

func (s *errStub) Decide(context.Context, Question) (Answer, error) {
	s.calls.Add(1)
	<-s.release
	return Answer{}, s.err
}

func TestCacheStopsAfterPermanentFailure(t *testing.T) {
	var calls atomic.Int32
	status := http.StatusServiceUnavailable
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "nope", status)
	}))
	defer srv.Close()
	c, err := NewCache(NewOpenAI(srv.URL, "m", "", time.Second), "", false, 0)
	require.NoError(t, err)

	_, err = c.Decide(context.Background(), Question{Kind: Bool, Prompt: "a"})
	require.NotErrorIs(t, err, ErrUnavailable)
	status = http.StatusUnauthorized
	_, err = c.Decide(context.Background(), Question{Kind: Bool, Prompt: "b"})
	require.ErrorIs(t, err, ErrUnavailable)
	_, err = c.Decide(context.Background(), Question{Kind: Bool, Prompt: "c"})
	require.ErrorIs(t, err, ErrUnavailable)
	require.EqualValues(t, 2, calls.Load(), "no calls after the provider rejected the configuration")
}
