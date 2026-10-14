package assets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStorePreservesLargeBodiesAndReportsIncomplete(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	require.NoError(t, err)
	body := bytes.Repeat([]byte("const value = 1234567890;\n"), 250000)
	record, err := store.Save(Response{URL: "https://example.test/app.chunk.js", Status: 200, MIME: "application/javascript", Body: body, ContentLength: strconv.Itoa(len(body))})
	require.NoError(t, err)
	require.True(t, record.Complete)
	require.Equal(t, len(body), record.Bytes)
	saved, err := os.ReadFile(filepath.Join(dir, record.File))
	require.NoError(t, err)
	require.Equal(t, body, saved)

	partial, err := store.Save(Response{URL: "https://example.test/broken.js", Status: 200, Body: body[:5], ContentLength: "6000000"})
	require.NoError(t, err)
	require.False(t, partial.Complete)
	require.Contains(t, partial.Error, "length mismatch")
	retry, ok := store.PopDiscovery()
	require.True(t, ok)
	require.Equal(t, "https://example.test/broken.js", retry.URL)
	require.Equal(t, "recovered", retry.Source)
	failed, err := store.Save(Response{URL: "https://example.test/missing.js", Status: 404, Body: []byte("not found")})
	require.NoError(t, err)
	require.False(t, failed.Complete)
	require.Contains(t, failed.Error, "HTTP status 404")
	_, ok = store.PopDiscovery()
	require.False(t, ok)
	require.NoError(t, store.Close())

	manifest, err := os.ReadFile(filepath.Join(dir, "manifest.jsonl"))
	require.NoError(t, err)
	var lines []Record
	for _, line := range bytes.Split(bytes.TrimSpace(manifest), []byte("\n")) {
		var r Record
		require.NoError(t, json.Unmarshal(line, &r))
		lines = append(lines, r)
	}
	require.Len(t, lines, 3)
	require.True(t, lines[0].Complete)
	require.False(t, lines[1].Complete)
	require.False(t, lines[2].Complete)
}

func TestStoreConcurrentDeduplication(t *testing.T) {
	store, err := New(t.TempDir())
	require.NoError(t, err)
	defer store.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			record, err := store.Save(Response{URL: "https://example.test/a.js", Status: 200, Body: []byte("console.log(1)")})
			if err == nil && !record.Complete {
				err = fmt.Errorf("response marked incomplete")
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	entries, err := os.ReadDir(filepath.Join(store.dir, "scripts"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestDiscoverImportsAndDepth(t *testing.T) {
	base := "https://cdn.example.test/static/main.js"
	body := []byte(`import './a.js'; export {x} from '../b.js'; import('./c.js'); importScripts('/d.js'); new Worker('./worker.js'); import 'react'; import('/chunk' + n + '.js');`)
	require.ElementsMatch(t, []string{
		"https://cdn.example.test/static/a.js",
		"https://cdn.example.test/b.js",
		"https://cdn.example.test/static/c.js",
		"https://cdn.example.test/d.js",
		"https://cdn.example.test/static/worker.js",
	}, DiscoverImports(base, body))

	store, err := NewWithDiscovery(t.TempDir(), 1)
	require.NoError(t, err)
	defer store.Close()
	_, err = store.Save(Response{URL: base, Status: 200, Body: body})
	require.NoError(t, err)
	count := 0
	for {
		item, ok := store.PopDiscovery()
		if !ok {
			break
		}
		count++
		require.Equal(t, 1, item.Depth)
	}
	require.Equal(t, 5, count)
}
