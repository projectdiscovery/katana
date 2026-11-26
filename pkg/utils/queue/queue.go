package queue

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Queue is a queue that implements bucket based depth-first
// or breadth-first queue.
//
// The breadth-first queues allow defining scores on whose
// basis the bucket is distributed.  Lower scores are picked up first, and
// higher scores which have a greater chance of being just random
// noise are picked up later in depth first.
//
// Depth-first queue uses a simple stack for LIFO operations and distributes
// items as they come in.
type Queue struct {
	sync.Mutex
	Strategy      Strategy
	stack         *stack
	priorityQueue *priorityQueue
	// inProgress counts popped items whose Done is pending; they may still push
	inProgress atomic.Int64
	wake       chan struct{}
}

// New creates a new queue from the type specified.
func New(strategyName string) (*Queue, error) {
	strategy, ok := strategiesMap[strategyName]
	if !ok {
		return nil, errors.New("unsupported strategy")
	}

	queue := &Queue{
		Strategy:      strategy,
		stack:         newStack(),
		priorityQueue: newPriorityQueue(),
		wake:          make(chan struct{}, 1),
	}

	return queue, nil
}

// Len returns the number of items in queue.
func (q *Queue) Len() int {
	q.Lock()
	defer q.Unlock()

	switch q.Strategy {
	case BreadthFirst, Priority:
		return q.priorityQueue.Len()
	case DepthFirst:
		return q.stack.Len()
	}

	return 0
}

// Push pushes an element with an optional priority into the queue.
func (q *Queue) Push(x interface{}, priority int) {
	q.Lock()
	switch q.Strategy {
	case BreadthFirst, Priority:
		q.priorityQueue.Push(x, priority)
	case DepthFirst:
		q.stack.Push(x)
	}
	q.Unlock()
	q.signal()
}

// Done marks an item received from Pop as finished, after it has pushed
// everything it found. Pop closes once the queue is empty and every
// received item is done.
func (q *Queue) Done() {
	q.inProgress.Add(-1)
	q.signal()
}

func (q *Queue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// Pop pops elements from the queue (backward-compatible, no context).
func (q *Queue) Pop() chan interface{} {
	return q.PopWithContext(context.Background())
}

// PopWithContext pops elements from the queue, respecting context cancellation.
// If ctx is nil, context.Background() is used. Call Done for every received item.
func (q *Queue) PopWithContext(ctx context.Context) chan interface{} {
	if ctx == nil {
		ctx = context.Background()
	}
	items := make(chan interface{})

	go func() {
		defer close(items)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// read before popping: if nothing was in progress and the queue is
			// empty, nothing can push anymore
			idle := q.inProgress.Load() == 0
			var item interface{}
			q.Lock()
			switch q.Strategy {
			case BreadthFirst, Priority:
				item = q.priorityQueue.Pop()
			case DepthFirst:
				item = q.stack.Pop()
			}
			q.Unlock()

			if item == nil {
				if idle {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-q.wake:
				}
				continue
			}

			q.inProgress.Add(1)
			// NOTE: if ctx is cancelled during this send, the popped item is
			// discarded. This is acceptable because cancellation means the crawl
			// is shutting down and no consumer will process it.
			select {
			case <-ctx.Done():
				return
			case items <- item:
			}
		}
	}()

	return items
}
