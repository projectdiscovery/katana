package queue

import (
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

// Pop pops an element from the queue. Result can be nil if no more
// elements are present in the queue.
func (q *Queue) Pop() chan interface{} {
	items := make(chan interface{})

	go func() {
		start := time.Now()
		for {
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
					time.Sleep(1 * time.Second)
					continue
				}
				close(items)
				return
			} else {
				items <- item
				start = time.Now()
			}
		}
	}()

	return items
}
