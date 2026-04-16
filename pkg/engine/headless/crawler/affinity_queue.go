package crawler

import (
	"sync"

	"github.com/adrianbrad/queue"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// AffinityQueue wraps a priority queue and provides origin-affinity dequeuing.
// When GetPreferring is called with an origin ID, it scans the queue for an
// action matching that origin before falling back to the highest-priority item.
// This avoids the expensive navigateBackToStateOrigin call when consecutive
// actions share the same origin page.
//
// It also supports template-aware purging: actions targeting exhausted URL
// templates are silently dropped during dequeue to reduce queue bloat.
type AffinityQueue struct {
	inner              queue.Queue[*types.Action]
	templateChecker    func(string) bool // returns true if template is exhausted
	exhaustionChecker  func(string) bool // returns true if origin is exhausted
	mu                 sync.Mutex
	buf                []*types.Action // reusable buffer for scan-and-reinsert
}

// NewAffinityQueue wraps a queue with origin-affinity and template-aware purging.
// templateChecker, if non-nil, is called for each action's target URL during dequeue;
// actions targeting exhausted templates are silently dropped.
func NewAffinityQueue(inner queue.Queue[*types.Action], templateChecker func(string) bool, exhaustionChecker func(string) bool) *AffinityQueue {
	return &AffinityQueue{inner: inner, templateChecker: templateChecker, exhaustionChecker: exhaustionChecker}
}

// isExhaustedOrigin checks if an action comes from an exhausted origin page.
// Exhausted origins are deprioritized but never dropped.
func (q *AffinityQueue) isExhaustedOrigin(action *types.Action) bool {
	if q.exhaustionChecker == nil {
		return false
	}
	return q.exhaustionChecker(action.OriginID)
}

// isExhaustedTemplate checks if an action targets an exhausted URL template.
func (q *AffinityQueue) isExhaustedTemplate(action *types.Action) bool {
	if q.templateChecker == nil {
		return false
	}
	targetURL := action.Input
	if targetURL == "" && action.Element != nil {
		targetURL = action.Element.Attributes["href"]
	}
	return targetURL != "" && q.templateChecker(targetURL)
}

// GetPreferring tries to find an action matching preferredOrigin by scanning
// up to maxScan items from the queue. Non-matching items are re-offered.
// If no match is found, returns the first (highest-priority) dequeued item.
// Actions targeting exhausted templates are silently dropped.
func (q *AffinityQueue) GetPreferring(preferredOrigin string, maxScan int) (*types.Action, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if preferredOrigin == "" || preferredOrigin == emptyPageHash {
		return q.getWithExhaustionPenalty()
	}

	// If the preferred origin itself is exhausted, don't use affinity —
	// fall through to priority-based dequeue with exhaustion awareness.
	if q.exhaustionChecker != nil && q.exhaustionChecker(preferredOrigin) {
		return q.getWithExhaustionPenalty()
	}

	q.buf = q.buf[:0]
	var first *types.Action

	for i := 0; i < maxScan; i++ {
		action, err := q.inner.Get()
		if err != nil {
			break // queue empty
		}

		// Drop exhausted template actions entirely
		if q.isExhaustedTemplate(action) {
			continue
		}

		if first == nil {
			first = action
		}
		// Don't match actions from exhausted origins during affinity scan
		if action.OriginID == preferredOrigin && !q.isExhaustedOrigin(action) {
			// Found a match — put back everything we pulled out
			for _, s := range q.buf {
				_ = q.inner.Offer(s)
			}
			if first != action {
				_ = q.inner.Offer(first)
			}
			return action, nil
		}
		if action != first {
			q.buf = append(q.buf, action)
		}
	}

	// No match — put stashed items back
	for _, s := range q.buf {
		_ = q.inner.Offer(s)
	}
	if first == nil {
		return nil, queue.ErrNoElementsAvailable
	}

	// If the highest-priority action is from an exhausted origin,
	// scan further for a non-exhausted alternative
	if q.isExhaustedOrigin(first) {
		_ = q.inner.Offer(first)
		return q.getWithExhaustionPenalty()
	}
	return first, nil
}

// getSkippingExhausted dequeues items, silently dropping exhausted templates.
func (q *AffinityQueue) getSkippingExhausted() (*types.Action, error) {
	for {
		action, err := q.inner.Get()
		if err != nil {
			return nil, err
		}
		if !q.isExhaustedTemplate(action) {
			return action, nil
		}
		// Exhausted template — drop and try next
	}
}

// getWithExhaustionPenalty scans up to 20 items preferring non-exhausted origins.
// If all scanned items are from exhausted origins, returns the best one anyway (fallback — never drop).
func (q *AffinityQueue) getWithExhaustionPenalty() (*types.Action, error) {
	const maxExhaustionScan = 20

	q.buf = q.buf[:0]
	var best *types.Action

	for i := 0; i < maxExhaustionScan; i++ {
		action, err := q.inner.Get()
		if err != nil {
			break // queue empty
		}

		// Drop exhausted template actions entirely
		if q.isExhaustedTemplate(action) {
			continue
		}

		if best == nil {
			best = action
		}

		if !q.isExhaustedOrigin(action) {
			// Found a non-exhausted action — put back everything else
			for _, s := range q.buf {
				_ = q.inner.Offer(s)
			}
			if best != action {
				_ = q.inner.Offer(best)
			}
			return action, nil
		}

		if action != best {
			q.buf = append(q.buf, action)
		}
	}

	// All scanned items are exhausted — put stashed items back, return best anyway
	for _, s := range q.buf {
		_ = q.inner.Offer(s)
	}
	if best == nil {
		return nil, queue.ErrNoElementsAvailable
	}
	return best, nil
}

// Get removes and returns the highest-priority item (no affinity).
// Skips exhausted template actions.
func (q *AffinityQueue) Get() (*types.Action, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.getSkippingExhausted()
}

// Offer adds an action to the queue.
func (q *AffinityQueue) Offer(a *types.Action) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.inner.Offer(a)
}

// Size returns the number of queued actions.
func (q *AffinityQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.inner.Size()
}
