package queue

import (
	"context"
	"errors"
	"sync"
	"time"
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
	Timeout       time.Duration
	Strategy      Strategy
	stack         *stack
	priorityQueue *priorityQueue
}

// New creates a new queue from the type specified.
func New(strategyName string, timeout int) (*Queue, error) {
	strategy, ok := strategiesMap[strategyName]
	if !ok {
		return nil, errors.New("unsupported strategy")
	}

	queue := &Queue{
		Strategy:      strategy,
		Timeout:       time.Duration(timeout) * time.Second,
		stack:         newStack(),
		priorityQueue: newPriorityQueue(),
	}

	return queue, nil
}

// Len returns the number of items in queue.
func (q *Queue) Len() int {
	q.Lock()
	defer q.Unlock()

	switch q.Strategy {
	case BreadthFirst:
		return q.priorityQueue.Len()
	case DepthFirst:
		return q.stack.Len()
	}

	return 0
}

// Push pushes an element with an optional priority into the queue.
func (q *Queue) Push(x interface{}, priority int) {
	q.Lock()
	defer q.Unlock()

	switch q.Strategy {
	case BreadthFirst:
		q.priorityQueue.Push(x, priority)
	case DepthFirst:
		q.stack.Push(x)
	}
}

// Pop pops elements from the queue (backward-compatible, no context).
func (q *Queue) Pop() chan interface{} {
	return q.PopWithContext(context.Background())
}

// PopWithContext pops elements from the queue, respecting context cancellation.
// If ctx is nil, context.Background() is used.
func (q *Queue) PopWithContext(ctx context.Context) chan interface{} {
	if ctx == nil {
		ctx = context.Background()
	}
	items := make(chan interface{})

	go func() {
		defer close(items)
		start := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var item interface{}
			q.Lock()
			switch q.Strategy {
			case BreadthFirst:
				item = q.priorityQueue.Pop()
			case DepthFirst:
				item = q.stack.Pop()
			}
			q.Unlock()

			if item == nil {
				if !start.Add(q.Timeout).Before(time.Now()) {
					select {
					case <-ctx.Done():
						return
					case <-time.After(1 * time.Second):
					}
					continue
				}
				return
			} else {
				// NOTE: if ctx is cancelled during this send, the popped item is
				// discarded. This is acceptable because cancellation means the crawl
				// is shutting down and no consumer will process it.
				select {
				case <-ctx.Done():
					return
				case items <- item:
				}
				start = time.Now()
			}
		}
	}()

	return items
}
