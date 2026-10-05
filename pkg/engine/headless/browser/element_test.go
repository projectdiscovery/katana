package browser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldStopLazyScroll(t *testing.T) {
	tests := []struct {
		name   string
		before pageScrollMetrics
		after  pageScrollMetrics
		stop   bool
	}{
		{
			name:   "stable bottom",
			before: pageScrollMetrics{Top: 500, Height: 1000, Viewport: 500},
			after:  pageScrollMetrics{Top: 500, Height: 1000, Viewport: 500},
			stop:   true,
		},
		{
			name:   "lazy content grew",
			before: pageScrollMetrics{Top: 500, Height: 1000, Viewport: 500},
			after:  pageScrollMetrics{Top: 500, Height: 1500, Viewport: 500},
			stop:   false,
		},
		{
			name:   "not at bottom",
			before: pageScrollMetrics{Top: 0, Height: 2000, Viewport: 500},
			after:  pageScrollMetrics{Top: 436, Height: 2000, Viewport: 500},
			stop:   false,
		},
		{
			name:   "subpixel tolerance",
			before: pageScrollMetrics{Top: 499, Height: 1000, Viewport: 500},
			after:  pageScrollMetrics{Top: 499.5, Height: 1000.5, Viewport: 500},
			stop:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.stop, shouldStopLazyScroll(test.before, test.after))
		})
	}
}
