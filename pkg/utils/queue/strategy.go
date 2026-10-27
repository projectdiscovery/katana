package queue

// Strategy of the queue
type Strategy int

// strategies of queues available for selection.
const (
	BreadthFirst Strategy = iota
	DepthFirst
	// Priority orders by a score the caller computes, lowest first.
	Priority
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
	"priority":      Priority,
}
