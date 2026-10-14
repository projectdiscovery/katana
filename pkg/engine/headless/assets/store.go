package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Response is the decoded body of a browser-observed script request.
// A non-nil Err means Chrome could not provide the complete response body.
type Response struct {
	URL             string
	Status          int
	MIME            string
	ContentLength   string
	ContentEncoding string
	Body            []byte
	Err             error
	Source          string
	ParentURL       string
	Depth           int
}

type Record struct {
	URL      string `json:"url"`
	Status   int    `json:"status"`
	MIME     string `json:"mime,omitempty"`
	Complete bool   `json:"complete"`
	Bytes    int    `json:"bytes,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
	File     string `json:"file,omitempty"`
	Error    string `json:"error,omitempty"`
	Source   string `json:"source,omitempty"`
	Parent   string `json:"parent_url,omitempty"`
}

type Discovery struct {
	URL    string
	Parent string
	Depth  int
	Source string
}

// Store writes one file per unique URL/body pair and an event manifest. It is
// safe to call from Chrome's concurrent response handlers.
type Store struct {
	mu           sync.Mutex
	dir          string
	manifest     *os.File
	seen         map[string]string
	completeURLs map[string]struct{}
	queued       map[string]struct{}
	pending      []Discovery
	maxDepth     int
}

func New(dir string) (*Store, error) {
	return NewWithDiscovery(dir, 0)
}

func NewWithDiscovery(dir string, maxDepth int) (*Store, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, err
	}
	manifest, err := os.OpenFile(filepath.Join(dir, "manifest.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &Store{dir: dir, manifest: manifest, seen: make(map[string]string), completeURLs: make(map[string]struct{}), queued: make(map[string]struct{}), maxDepth: maxDepth}, nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.manifest.Close()
}

func (s *Store) Save(resp Response) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := Record{URL: resp.URL, Status: resp.Status, MIME: resp.MIME, Source: resp.Source, Parent: resp.ParentURL}
	if record.Source == "" {
		record.Source = "observed"
	}
	record.Bytes = len(resp.Body)
	if resp.Err != nil {
		record.Error = resp.Err.Error()
	} else if resp.Status < 200 || resp.Status >= 300 {
		record.Error = fmt.Sprintf("HTTP status %d", resp.Status)
	} else if resp.ContentLength != "" && resp.ContentEncoding == "" && !matchesContentLength(resp.ContentLength, len(resp.Body)) {
		record.Error = fmt.Sprintf("response length mismatch: header=%s decoded=%d", resp.ContentLength, len(resp.Body))
	} else {
		bodyHash := sha256.Sum256(resp.Body)
		record.SHA256 = hex.EncodeToString(bodyHash[:])
		keyHash := sha256.Sum256([]byte(resp.URL + "\x00" + record.SHA256))
		key := hex.EncodeToString(keyHash[:])
		if file, ok := s.seen[key]; ok {
			record.File = file
			record.Complete = true
		} else {
			file := filepath.Join("scripts", key+".js")
			fullPath := filepath.Join(s.dir, file)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0750); err != nil {
				record.Error = err.Error()
			} else if err := writeAtomic(fullPath, resp.Body); err != nil {
				record.Error = err.Error()
			} else {
				s.seen[key] = file
				record.File = file
				record.Complete = true
			}
		}
		if record.Complete {
			s.completeURLs[resp.URL] = struct{}{}
			if resp.Depth < s.maxDepth {
				for _, child := range DiscoverImports(resp.URL, resp.Body) {
					if _, already := s.completeURLs[child]; already {
						continue
					}
					if _, already := s.queued[child]; already {
						continue
					}
					s.queued[child] = struct{}{}
					s.pending = append(s.pending, Discovery{URL: child, Parent: resp.URL, Depth: resp.Depth + 1})
				}
			}
		}
	}
	if !record.Complete && record.Source == "observed" && resp.Status >= 200 && resp.Status < 300 {
		if _, done := s.completeURLs[resp.URL]; !done {
			if _, queued := s.queued[resp.URL]; !queued {
				s.queued[resp.URL] = struct{}{}
				s.pending = append(s.pending, Discovery{URL: resp.URL, Source: "recovered"})
			}
		}
	}

	if err := json.NewEncoder(s.manifest).Encode(record); err != nil {
		return record, err
	}
	return record, nil
}

func (s *Store) PopDiscovery() (Discovery, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.pending) > 0 {
		next := s.pending[0]
		s.pending = s.pending[1:]
		if _, done := s.completeURLs[next.URL]; !done {
			return next, true
		}
	}
	return Discovery{}, false
}

var importPattern = regexp.MustCompile(`(?m)(?:\bimport\s+(?:[^'";]*?\s+from\s+)?|\bexport\s+[^'";]*?\s+from\s+)['"]([^'"]+)['"]|(?:\bimport\s*\(\s*|\bimportScripts\s*\(\s*|\bnew\s+(?:Shared)?Worker\s*\(\s*)['"]([^'"]+)['"]\s*[,)]`)

// DiscoverImports finds literal nested script references. Computed imports and
// bundler runtime URL expressions require executing the relevant UI branch.
func DiscoverImports(baseURL string, body []byte) []string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	var result []string
	for _, match := range importPattern.FindAllSubmatch(body, -1) {
		ref := string(match[1])
		if ref == "" {
			ref = string(match[2])
		}
		if !strings.HasPrefix(ref, "./") && !strings.HasPrefix(ref, "../") && !strings.HasPrefix(ref, "/") {
			continue
		}
		u, err := url.Parse(ref)
		if err != nil {
			continue
		}
		resolved := base.ResolveReference(u)
		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			continue
		}
		key := resolved.String()
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, key)
		}
	}
	return result
}

func matchesContentLength(header string, actual int) bool {
	expected, err := strconv.Atoi(strings.TrimSpace(header))
	if err != nil || expected < 0 {
		return true // Invalid/multiple headers cannot be used as a completeness test.
	}
	return expected == actual
}

func writeAtomic(path string, body []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".katana-js-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
