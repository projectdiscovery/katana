package headless

import (
	"errors"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

type stubWriter struct{ err error }

func (w *stubWriter) Close() error                 { return nil }
func (w *stubWriter) Write(*output.Result) error   { return w.err }
func (w *stubWriter) WriteErr(*output.Error) error { return nil }
func (w *stubWriter) GetResultCount() int64        { return 0 }

func TestWriteResultCallsOnResult(t *testing.T) {
	result := &output.Result{Request: &navigation.Request{URL: "https://example.com/a"}}

	t.Run("written", func(t *testing.T) {
		var got []string
		h := &Headless{options: &types.CrawlerOptions{
			OutputWriter: &stubWriter{},
			Options:      &types.Options{OnResult: func(r output.Result) { got = append(got, r.Request.URL) }},
		}}
		require.NoError(t, h.writeResult(result))
		require.Equal(t, []string{"https://example.com/a"}, got)
	})

	t.Run("not_written", func(t *testing.T) {
		called := false
		h := &Headless{options: &types.CrawlerOptions{
			OutputWriter: &stubWriter{err: errors.New("filtered")},
			Options:      &types.Options{OnResult: func(output.Result) { called = true }},
		}}
		require.Error(t, h.writeResult(result))
		require.False(t, called)
	})
}
