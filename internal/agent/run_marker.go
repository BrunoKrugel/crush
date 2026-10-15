package agent

import (
	"context"
	"sync/atomic"
)

// runCompleteMarkerKey is the unexported context key carrying a
// [runCompleteMarker] from the dispatch boundary (backend.runAgent)
// down into the coordinator. It lets the dispatcher learn whether the
// coordinator already published the authoritative terminal
// notify.RunComplete for the run, so a fallback terminal event is only
// emitted when one is actually missing (e.g. an error returned before
// sessionAgent.Run ever executed). It avoids a breaking change to the
// Coordinator interface.
type runCompleteMarkerKey struct{}

// runCompleteMarker records whether a terminal RunComplete has been
// published for a run. It is shared by pointer through the context so
// a publish deep in the call stack is observable by the dispatcher
// after the call returns.
type runCompleteMarker struct {
	published atomic.Bool
}

// WithRunCompleteMarker returns ctx carrying a fresh marker the
// coordinator can flag via [MarkRunCompletePublished] once it emits the
// run's terminal RunComplete. Callers read the result with
// [RunCompletePublished]. Attaching the marker is optional: code paths
// without one simply skip the dedup signal.
//
// An existing marker is kept rather than shadowed: a publish deep in the
// call stack must stay observable to every reader up the same chain, so
// attaching over one would hide it from the dispatcher that installed it.
func WithRunCompleteMarker(ctx context.Context) context.Context {
	if _, ok := ctx.Value(runCompleteMarkerKey{}).(*runCompleteMarker); ok {
		return ctx
	}
	return context.WithValue(ctx, runCompleteMarkerKey{}, &runCompleteMarker{})
}

// MarkRunCompletePublished records that the authoritative terminal
// RunComplete has been published for the run carried by ctx. It is a
// no-op when no marker is present (e.g. the in-process/local Run path,
// which is not dispatched through backend.runAgent).
func MarkRunCompletePublished(ctx context.Context) {
	if m, ok := ctx.Value(runCompleteMarkerKey{}).(*runCompleteMarker); ok {
		m.published.Store(true)
	}
}

// RunCompletePublished reports whether [MarkRunCompletePublished] was
// called on ctx's marker. It returns false when no marker is present.
func RunCompletePublished(ctx context.Context) bool {
	if m, ok := ctx.Value(runCompleteMarkerKey{}).(*runCompleteMarker); ok {
		return m.published.Load()
	}
	return false
}

// withFreshRunCompleteMarker shadows any existing marker with a new one.
// A queue drain running several turns needs the per-turn dedup to reset per
// turn: the first turn's terminal publish must not suppress a later turn's
// fallback. The dispatcher's own marker, further up the chain, is untouched.
func withFreshRunCompleteMarker(ctx context.Context) context.Context {
	return context.WithValue(ctx, runCompleteMarkerKey{}, &runCompleteMarker{})
}

type queuedDispatchKey struct{}

// WithQueuedDispatch marks ctx as carrying a run promoted off the message
// queue by runQueued. That path publishes the promoted turn's terminal
// completion (and its error notification) itself when the turn fails before
// its own terminal defer is installed, so the run's own deferred publish must
// stay out of its way.
func WithQueuedDispatch(ctx context.Context) context.Context {
	return context.WithValue(ctx, queuedDispatchKey{}, true)
}

// queuedDispatch reports whether ctx carries a queue-promoted run.
func queuedDispatch(ctx context.Context) bool {
	v, _ := ctx.Value(queuedDispatchKey{}).(bool)
	return v
}
