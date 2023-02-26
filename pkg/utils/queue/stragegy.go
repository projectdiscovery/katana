package queue

// Strategy of the queue
type Strategy int

// strategies of queues available for selection.
const (
	BreadthFirst Strategy = iota
	DepthFirst
)

var strategiesMap = map[string]Strategy{
	"breadth-first": BreadthFirst,
	"depth-first":   DepthFirst,
}
