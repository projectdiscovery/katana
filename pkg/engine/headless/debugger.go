package headless

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ActiveURL represents a URL currently being processed
type ActiveURL struct {
	URL       string    `json:"url"`
	StartTime time.Time `json:"start_time"`
	Duration  string    `json:"duration"`
	Depth     int       `json:"depth"`
}

// CrawlDebugger tracks active URLs for debugging
type CrawlDebugger struct {
	mu         sync.RWMutex
	activeURLs map[string]*ActiveURL
	httpServer *http.Server
}

// NewCrawlDebugger creates a new debugger instance
func NewCrawlDebugger(httpPort int) *CrawlDebugger {
	cd := &CrawlDebugger{
		activeURLs: make(map[string]*ActiveURL),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/active-urls", cd.handleActiveURLs)
	mux.HandleFunc("/debug/health", cd.handleHealth)

	cd.httpServer = &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", httpPort),
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	go func() {
		if err := cd.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Debug HTTP server error: %v\n", err)
		}
	}()

	return cd
}

// StartURL marks a URL as being processed
func (cd *CrawlDebugger) StartURL(url string, depth int) {
	if cd == nil {
		return
	}

	cd.mu.Lock()
	cd.activeURLs[url] = &ActiveURL{
		URL:       url,
		StartTime: time.Now(),
		Depth:     depth,
	}
	cd.mu.Unlock()
}

// EndURL marks a URL as finished processing
func (cd *CrawlDebugger) EndURL(url string) {
	if cd == nil {
		return
	}

	cd.mu.Lock()
	delete(cd.activeURLs, url)
	cd.mu.Unlock()
}

// GetActiveURLs returns currently active URLs with durations
func (cd *CrawlDebugger) GetActiveURLs() []ActiveURL {
	if cd == nil {
		return nil
	}

	cd.mu.RLock()
	defer cd.mu.RUnlock()

	urls := make([]ActiveURL, 0, len(cd.activeURLs))
	now := time.Now()
	for _, au := range cd.activeURLs {
		copy := *au
		copy.Duration = now.Sub(au.StartTime).String()
		urls = append(urls, copy)
	}
	return urls
}

// HTTP handlers
func (cd *CrawlDebugger) handleActiveURLs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	active := cd.GetActiveURLs()
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"active_urls": active,
		"count":       len(active),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (cd *CrawlDebugger) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (cd *CrawlDebugger) Close() {
	if cd == nil {
		return
	}

	if cd.httpServer != nil {
		_ = cd.httpServer.Close()
	}
}
