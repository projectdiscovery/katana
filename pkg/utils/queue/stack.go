package queue

import (
	"container/list"
)

// Taken from https://stackoverflow.com/a/64641330/9546749
type stack struct {
	ll *list.List
}

func newStack() *stack {
	return &stack{ll: list.New()}
}

func (s *stack) Push(x interface{}) {
	s.ll.PushBack(x)
}

func (s *stack) Len() int {
	return s.ll.Len()
}

func (s *stack) Pop() interface{} {
	if s.ll.Len() == 0 {
		return nil
	}
	tail := s.ll.Back()
	val := tail.Value
	s.ll.Remove(tail)
	return val
}
