package queue

import (
	"container/heap"
)

type priorityQueue struct {
	itemHeap *itemHeap
}

func newPriorityQueue() *priorityQueue {
	return &priorityQueue{itemHeap: &itemHeap{}}
}

// Len returns the number of elements in the queue.
func (p *priorityQueue) Len() int {
	return p.itemHeap.Len()
}

func (p *priorityQueue) Push(v interface{}, priority int) {
	newItem := &item{
		value:    v,
		priority: priority,
	}
	heap.Push(p.itemHeap, newItem)
}

func (p *priorityQueue) Pop() interface{} {
	if len(*p.itemHeap) == 0 {
		return nil
	}
	item := heap.Pop(p.itemHeap).(*item)
	return item.value
}

type itemHeap []*item

type item struct {
	value    interface{}
	priority int
	index    int
}

func (ih *itemHeap) Len() int {
	return len(*ih)
}

func (ih *itemHeap) Less(i, j int) bool {
	// Less since we are executing lower prioritie values first and
	// higher priority values in the end.
	return (*ih)[i].priority < (*ih)[j].priority
}

func (ih *itemHeap) Swap(i, j int) {
	(*ih)[i], (*ih)[j] = (*ih)[j], (*ih)[i]
	(*ih)[i].index = i
	(*ih)[j].index = j
}

func (ih *itemHeap) Push(x interface{}) {
	it := x.(*item)
	it.index = len(*ih)
	*ih = append(*ih, it)
}

func (ih *itemHeap) Pop() interface{} {
	old := *ih
	item := old[len(old)-1]
	*ih = old[0 : len(old)-1]
	return item
}
