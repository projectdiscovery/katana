package common

import (
	"net/http"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stretchr/testify/require"
)

func newTestBackoffCache() *lru.Cache[string, *hostBackoff] {
	c, _ := lru.New[string, *hostBackoff](100)
	return c
}

func TestIsThrottled(t *testing.T) {
	require.True(t, IsThrottled(http.StatusTooManyRequests))
	require.True(t, IsThrottled(http.StatusServiceUnavailable))
	require.False(t, IsThrottled(http.StatusOK))
	require.False(t, IsThrottled(http.StatusNotFound))
	require.False(t, IsThrottled(http.StatusInternalServerError))
}

func TestHostBackoffIncrementsOnThrottle(t *testing.T) {
	shared := &Shared{hostBackoffs: newTestBackoffCache()}

	b := shared.backoffFor("example.com")
	require.Equal(t, int32(0), b.consecutive.Load())

	b.consecutive.Add(1)
	require.Equal(t, int32(1), b.consecutive.Load())

	b.consecutive.Add(1)
	require.Equal(t, int32(2), b.consecutive.Load())
}

func TestHostBackoffDecaysOnSuccess(t *testing.T) {
	shared := &Shared{hostBackoffs: newTestBackoffCache()}

	b := shared.backoffFor("example.com")
	b.consecutive.Store(3)

	b.consecutive.Add(-1)
	require.Equal(t, int32(2), b.consecutive.Load())
}

func TestHostBackoffPerDomain(t *testing.T) {
	shared := &Shared{hostBackoffs: newTestBackoffCache()}

	a := shared.backoffFor("a.com")
	b := shared.backoffFor("b.com")

	a.consecutive.Store(5)
	require.Equal(t, int32(0), b.consecutive.Load())
}

func TestApplyBackoffNoDelayWhenClean(t *testing.T) {
	shared := &Shared{hostBackoffs: newTestBackoffCache()}

	start := time.Now()
	shared.ApplyBackoff("clean-host.com")
	elapsed := time.Since(start)

	require.Less(t, elapsed, 100*time.Millisecond)
}
