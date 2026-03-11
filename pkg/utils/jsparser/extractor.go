package jsparser

type Endpoint struct {
	Endpoint string
	Type     string
}

type Extractor interface {
	Extract(data string) []Endpoint
	Name() string
}

func Extract(data string) []Endpoint {
	return New().Extract(data)
}

// in case need name
func Backend() string {
	return New().Name()
}

