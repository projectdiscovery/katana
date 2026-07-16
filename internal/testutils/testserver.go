package testutils

import (
	"fmt"
	"net"
	"net/http"
)

// TestServer is a local HTTP server for functional tests.
type TestServer struct {
	mux      *http.ServeMux
	listener net.Listener
	URL      string
}

// NewTestServer starts a local HTTP server on a random port with
// a small site structure suitable for crawl testing.
func NewTestServer() (*TestServer, error) {
	mux := http.NewServeMux()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start test server: %w", err)
	}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Test Home</title></head>
<body>
	<a href="%s/about">About</a>
	<a href="%s/contact">Contact</a>
	<a href="%s/blog">Blog</a>
	<script src="%s/static/app.js"></script>
</body>
</html>`, baseURL, baseURL, baseURL, baseURL)
	})

	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>About</title></head>
<body>
	<a href="%s/">Home</a>
	<a href="%s/team">Team</a>
</body>
</html>`, baseURL, baseURL)
	})

	mux.HandleFunc("/contact", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Contact</title></head>
<body>
	<a href="%s/">Home</a>
	<form action="%s/submit" method="POST">
		<input type="email" name="email">
		<input type="submit" value="Send">
	</form>
</body>
</html>`, baseURL, baseURL)
	})

	mux.HandleFunc("/blog", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Blog</title></head>
<body>
	<a href="%s/">Home</a>
	<a href="%s/blog/post-1">First Post</a>
	<a href="%s/blog/post-2">Second Post</a>
</body>
</html>`, baseURL, baseURL, baseURL)
	})

	mux.HandleFunc("/blog/post-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Post 1</title></head>
<body><a href="%s/blog">Back to Blog</a></body>
</html>`, baseURL)
	})

	mux.HandleFunc("/blog/post-2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Post 2</title></head>
<body><a href="%s/blog">Back to Blog</a></body>
</html>`, baseURL)
	})

	mux.HandleFunc("/team", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Team</title></head>
<body><a href="%s/about">About</a></body>
</html>`, baseURL)
	})

	mux.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Thanks</title></head><body>OK</body></html>`)
	})

	mux.HandleFunc("/static/app.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = fmt.Fprintf(w, `// app.js
fetch("%s/api/data").then(r => r.json());`, baseURL)
	})

	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"ok"}`)
	})

	go func() {
		_ = http.Serve(listener, mux)
	}()

	return &TestServer{
		mux:      mux,
		listener: listener,
		URL:      baseURL,
	}, nil
}

// Close shuts down the test server.
func (ts *TestServer) Close() error {
	return ts.listener.Close()
}
