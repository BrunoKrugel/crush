package permission

import "context"

// HookDecision is the outcome of a permission-hook evaluation.
type HookDecision int

const (
	// HookDecisionNone means the hook expressed no opinion; the normal
	// prompt proceeds.
	HookDecisionNone HookDecision = iota
	// HookDecisionAllow means the hook approved the call; the prompt is
	// skipped and the call is granted.
	HookDecisionAllow
	// HookDecisionDeny means the hook blocked the call; the prompt is
	// skipped and the call is denied with the hook's reason surfacing to
	// the UI and the model.
	HookDecisionDeny
)

// PreHookResult is the outcome of a PrePermission hook evaluation.
type PreHookResult struct {
	Decision HookDecision
	Reason   string
}

// PermissionHooks lets the agent layer run external policy hooks around
// the permission prompt. Implementations are optional; the service
// behaves unchanged when none is set.
//
// PrePermission runs only when a request is about to prompt the user
// (after allowlists, session auto-approvals, and prior grants have been
// exhausted). Returning allow or deny short-circuits the prompt.
// PermissionDenied is a fire-and-forget notification that a request was
// denied; it never blocks the denial.
//
// Wiring these calls into permissionService.Request is part of the
// auto-mode permission integration; the types live here so the automode
// package can implement the interface without importing fantasy or agent
// packages.
type PermissionHooks interface {
	PrePermission(ctx context.Context, req PermissionRequest) PreHookResult
	PermissionDenied(ctx context.Context, req PermissionRequest)
}

// AutoModeToggler is implemented by hooks that can be enabled and
// disabled at runtime (the native auto mode).
type AutoModeToggler interface {
	SetAutoModeEnabled(enabled bool)
}

// AutoModeGrantObserver is implemented by hooks that track quota state
// and want to learn when the human grants a request that was escalated
// to them. A human decision restores trust, so the auto mode resets its
// consecutive-denial counter.
type AutoModeGrantObserver interface {
	OnAutoModeGrant(sessionID string)
}
