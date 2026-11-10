package workspace

import (
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"time"
)

var (
	sseReconnectInitialBackoffNS atomic.Int64
	sseReconnectMaxBackoffNS     atomic.Int64
)

func init() {
	sseReconnectInitialBackoffNS.Store(int64(250 * time.Millisecond))
	sseReconnectMaxBackoffNS.Store(int64(10 * time.Second))
}

func SetSSEBackoffForTest(initial, maxBackoff time.Duration) (restore func()) {
	origInitial, origMax := sseReconnectInitialBackoffNS.Load(), sseReconnectMaxBackoffNS.Load()
	sseReconnectInitialBackoffNS.Store(int64(initial))
	sseReconnectMaxBackoffNS.Store(int64(maxBackoff))
	return func() {
		sseReconnectInitialBackoffNS.Store(origInitial)
		sseReconnectMaxBackoffNS.Store(origMax)
	}
}

func recoveryCreateTimeoutForTest(d time.Duration) (restore func()) {
	return func() {}
}

// ConsumeEventsForTest runs the event-handling loop on the given channel.
func (w *ClientWorkspace) ConsumeEventsForTest(evc <-chan any, send func(tea.Msg)) {}

// RunSubscriptionForTest runs the full subscribe loop.
func (w *ClientWorkspace) RunSubscriptionForTest(send func(tea.Msg)) {}

// WorkspaceIDForTest returns the cached workspace ID.
func (w *ClientWorkspace) WorkspaceIDForTest() string { return "" }
