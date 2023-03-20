package queue

// Strategy of the queue
type Strategy int

// strategies of queues available for selection.
const (
	BreadthFirst Strategy = iota
	DepthFirst
)

func (s Strategy) String() string {
	for k, v := range strategiesMap {
		if v == s {
			return k
		}
	}
	return ""
}

var strategiesMap = map[string]Strategy{
	"breadth-first": BreadthFirst,
	"depth-first":   DepthFirst,
}
