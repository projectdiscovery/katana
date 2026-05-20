// Fixture server for the secrets extractor. Serves a page seeded with
// well-known test credentials so katana can be smoke-tested end-to-end.
// All values below are public test fixtures from AWS / GitHub docs — they
// match detection rules but do not authorize anything.
package main

import (
	"flag"
	"log"
	"net/http"
)

const fixturePage = `<!doctype html>
<html><body>
<h1>secrets-extractor fixture</h1>
<script>
const cfg = {
  awsKey:    "AKIAIOSFODNN7EXAMPLE",
  awsSecret: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
  ghToken:   "ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
  slackBot:  "xoxb-12345-67890-abcdefghijklmnopqrstuvwx"
};
</script>
</body></html>
`

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "listen address")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fixturePage))
	})
	log.Printf("fixture server listening on http://%s/", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}
