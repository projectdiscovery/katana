package queue

import (
	"sync"
)

// VarietyQueue is a queue that implements bucket based depth-first
// or breadth-first queue.
//
// The breadth-first queues allow defining scores on whose
// basis the bucket is distributed.  Lower scores are picked up first, and
// higher scores which have a greater chance of being just random
// noise are picked up later in depth first.
//
// Depth-first queue uses a simple stack for LIFO operations and distributes
// items as they come in.
type VarietyQueue struct {
	queueType     Type
	stack         *stack
	mutex         *sync.Mutex
	priorityQueue *priorityQueue
}

// Type is the type of the queue to use.
type Type int

// Types of queues available for selection.
const (
	BreadthFirst Type = iota
	DepthFirst
)

var queueTypeStringMap = map[string]Type{
	"breadth-first": BreadthFirst,
	"depth-first":   DepthFirst,
}

// New creates a new queue from the type specified.
func New(queueType string) *VarietyQueue {
	queueTypeItem, ok := queueTypeStringMap[queueType]
	varietyQueue := &VarietyQueue{queueType: queueTypeItem, mutex: &sync.Mutex{}}
	if !ok {
		varietyQueue.stack = newStack()
		varietyQueue.queueType = DepthFirst
		return varietyQueue
	}

	switch queueTypeItem {
	case BreadthFirst:
		varietyQueue.priorityQueue = newPriorityQueue()
	case DepthFirst:
		varietyQueue.stack = newStack()
	}
	return varietyQueue
}

// Len returns the number of items in queue.
func (v *VarietyQueue) Len() int {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	var x int
	if v.queueType == BreadthFirst {
		x = v.priorityQueue.Len()
	} else if v.queueType == DepthFirst {
		x = v.stack.Len()
	}
	return x
}

// Push pushes an element with an optional priority into the queue.
func (v *VarietyQueue) Push(x interface{}, priority int) {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	if v.queueType == BreadthFirst {
		v.priorityQueue.Push(x, priority)
	} else if v.queueType == DepthFirst {
		v.stack.Push(x)
	}
}

// Pop pops an element from the queue. Result can be nil if no more
// elements are present in the queue.
func (v *VarietyQueue) Pop() interface{} {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	var x interface{} = nil
	if v.queueType == BreadthFirst {
		x = v.priorityQueue.Pop()
	} else if v.queueType == DepthFirst {
		x = v.stack.Pop()
	}
	return x
}
