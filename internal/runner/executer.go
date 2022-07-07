package runner

import (
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/standard"
)

// ExecuteCrawling executes the crawling main loop
func (r *Runner) ExecuteCrawling() error {
	inputs := r.parseInputs()
	if len(inputs) == 0 {
		return errors.New("no input provided for crawling")
	}

	crawler, err := standard.New(r.crawlerOptions)
	if err != nil {
		return errors.Wrap(err, "could not create standard crawler")
	}
	defer crawler.Close()

	for _, input := range inputs {
		_ = crawler.Crawl(input)
	}

	// for debug purposes print out the schema
	// endpoints, _ := r.crawlerOptions.GraphDB.QueryEndpoints(context.Background())
	// for _, endpoint := range endpoints {
	// 	log.Println("endpoint: ", endpoint.URL)
	// 	connections, _ := r.crawlerOptions.GraphDB.QueryConnections(context.Background(), endpoint)
	// 	for _, connection := range connections {
	// 		log.Println("conntected to: ", connection.URL)
	// 	}
	// }

	// attempt to calculate shortest path between root and endpoint
	// start, err := r.crawlerOptions.GraphDB.QueryEndpoint(context.Background(), &ent.Endpoint{URL: "http://localhost:8000"})
	// if err != nil {
	// 	log.Fatal("start not found", err)
	// }
	// end, err := r.crawlerOptions.GraphDB.QueryEndpoint(context.Background(), &ent.Endpoint{URL: "http://localhost:8000/testutils/integration.go"})
	// if err != nil {
	// 	log.Fatal("end not found", err)
	// }
	// shortestPath, err := r.crawlerOptions.GraphDB.ShortestPath(context.Background(), start, end)
	// log.Println(shortestPath, err)

	return nil
}
