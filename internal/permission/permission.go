package permission

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// hookApprovalKey is the unexported context key used to mark a tool call as
// pre-approved by a PreToolUse hook. The value is the tool call ID so an
// approval can't be reused across calls that happen to share a context.
type hookApprovalKey struct{}

// WithHookApproval returns a context that marks the given tool call ID as
// pre-approved by a hook. When the permission service sees a matching
// request it short-circuits the normal prompt and grants immediately.
func WithHookApproval(ctx context.Context, toolCallID string) context.Context {
	return context.WithValue(ctx, hookApprovalKey{}, toolCallID)
}

// hookApproved reports whether the context carries a hook approval for the
// given tool call ID.
func hookApproved(ctx context.Context, toolCallID string) bool {
	if toolCallID == "" {
		return false
	}
	v, _ := ctx.Value(hookApprovalKey{}).(string)
	return v == toolCallID
}

type requestPolicyKey struct{}

// RequestPolicy controls how permission requests are resolved for one agent
// turn.
type RequestPolicy uint8

const (
	RequestPolicyPrompt RequestPolicy = iota
	RequestPolicyAutoApprove
)

// WithAutoApproveRequests returns a context whose permission requests are
// automatically approved throughout the derived context tree.
func WithAutoApproveRequests(ctx context.Context) context.Context {
	return WithRequestPolicy(ctx, RequestPolicyAutoApprove)
}

// WithRequestPolicy returns a context with the given permission request
// policy. The policy replaces any inherited policy.
func WithRequestPolicy(ctx context.Context, policy RequestPolicy) context.Context {
	return context.WithValue(ctx, requestPolicyKey{}, policy)
}

// RequestPolicyFromContext returns the permission request policy carried by
// ctx. Contexts without an explicit policy prompt for permission.
func RequestPolicyFromContext(ctx context.Context) RequestPolicy {
	policy, _ := ctx.Value(requestPolicyKey{}).(RequestPolicy)
	return policy
}

type CreatePermissionRequest struct {
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

type PermissionNotification struct {
	SessionID  string `json:"session_id"`
	ToolCallID string `json:"tool_call_id"`
	Granted    bool   `json:"granted"`
	Denied     bool   `json:"denied"`
}

type PermissionRequest struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

type Service interface {
	pubsub.Subscriber[PermissionRequest]
	// GrantPersistent grants a permission request and remembers the grant
	// for the session. It returns true if this call actually resolved the
	// pending request; false if the request had already been resolved
	// (e.g., by another concurrent caller) or is unknown.
	GrantPersistent(permission PermissionRequest) bool
	// Grant grants a permission request. It returns true if this call
	// actually resolved the pending request; false if the request had
	// already been resolved or is unknown.
	Grant(permission PermissionRequest) bool
	// Deny denies a permission request. It returns true if this call
	// actually resolved the pending request; false if the request had
	// already been resolved or is unknown.
	Deny(permission PermissionRequest) bool
	Request(ctx context.Context, opts CreatePermissionRequest) (bool, error)
	// ActiveRequest returns the request that is currently awaiting a
	// decision, or false when no request is pending. A request is only
	// reported while it can still be resolved; a request whose waiter
	// was cancelled is not returned.
	ActiveRequest() (PermissionRequest, bool)
	AutoApproveSession(sessionID string)
	SetSkipRequests(skip bool)
	SkipRequests() bool
	SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionNotification]
	// SetPrePermission wires the permission hook events: PrePermission
	// runs before the prompt (allow or deny short-circuits it, silence
	// falls through) and PermissionDenied after a denial
	// (fire-and-forget). A nil runner disables them again; the default
	// service has none, so its behavior is unchanged.
	SetPrePermission(runner *hooks.Runner, transcript func(ctx context.Context, sessionID string) (string, error))
}

// PermissionKey is a composite key for session permission lookups.
type PermissionKey struct {
	SessionID string
	ToolName  string
	Action    string
	Path      string
}

type permissionService struct {
	parent              Service
	pendingRequests     *csync.Map[string, chan bool]
	activeRequest       *PermissionRequest
	notificationBroker  *pubsub.Broker[PermissionNotification]
	sessionPermissions  *csync.Map[PermissionKey, bool]
	requestSlot         chan struct{}
	autoApproveSessions map[string]bool
	*pubsub.Broker[PermissionRequest]
	skipFn       func() bool
	workingDir   string
	allowedTools []string
	// prePermissionRunner runs the PrePermission and PermissionDenied
	// hooks. Nil (the default) keeps the service's behavior unchanged:
	// every call costs one nil check.
	prePermissionRunner *hooks.Runner
	// transcript builds the reasoning-blind transcript a permission
	// hook payload carries. Nil together with the runner.
	transcriptFn          func(ctx context.Context, sessionID string) (string, error)
	autoApproveSessionsMu sync.RWMutex
	activeRequestMu       sync.Mutex
	requestMu             sync.Mutex
	skip                  atomic.Bool
}

// resolve atomically removes the pending request entry for the given
// permission and, if it was still pending, publishes exactly one
// PermissionNotification and forwards the outcome to the waiter on
// respCh. It returns true if this call resolved the request, false if
// it had already been resolved (e.g., by another concurrent caller) or
// the request ID is unknown.
//
// If onResolve is non-nil it runs after the pending entry has been
// taken but before the notification is published or the waiter is
// unblocked. This lets GrantPersistent record the session permission
// only when it actually wins the race, so a losing GrantPersistent
// that lost to a Deny does not leak an auto-approve entry.
//
// All three public resolution methods (Grant, GrantPersistent, Deny)
// route through this helper so multi-subscriber UIs can race safely:
// the first caller wins, the rest become no-ops.
func (s *permissionService) resolve(permission PermissionRequest, granted, denied bool, onResolve func()) bool {
	respCh, ok := s.pendingRequests.Take(permission.ID)
	if !ok {
		return false
	}

	if onResolve != nil {
		onResolve()
	}

	// A user denial also fires the PermissionDenied hooks
	// fire-and-forget. The detail mirrors the request the prompt
	// carried.
	if denied && s.prePermissionRunner != nil {
		s.firePermissionDenied(context.Background(), hooks.PermissionDetail{
			ToolName:    permission.ToolName,
			Action:      permission.Action,
			Path:        permission.Path,
			Description: permission.Description,
		}, CreatePermissionRequest{
			SessionID:  permission.SessionID,
			ToolCallID: permission.ToolCallID,
		}, "")
	}

	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		SessionID:  permission.SessionID,
		ToolCallID: permission.ToolCallID,
		Granted:    granted,
		Denied:     denied,
	})

	// respCh is buffered (cap 1) and only ever has at most one sender
	// per request because Take removes the entry under the map lock,
	// so this send never blocks.
	respCh <- granted

	s.activeRequestMu.Lock()
	if s.activeRequest != nil && s.activeRequest.ID == permission.ID {
		s.activeRequest = nil
	}
	s.activeRequestMu.Unlock()
	return true
}

func (s *permissionService) GrantPersistent(permission PermissionRequest) bool {
	// Record the persistent grant only if this call wins the
	// pending-request race. Otherwise a losing GrantPersistent that
	// lost to a Deny would still leave an auto-approve entry behind,
	// silently flipping later denied calls to allowed.
	return s.resolve(permission, true, false, func() {
		s.sessionPermissions.Set(PermissionKey{
			SessionID: permission.SessionID,
			ToolName:  permission.ToolName,
			Action:    permission.Action,
			Path:      permission.Path,
		}, true)
	})
}

func (s *permissionService) Grant(permission PermissionRequest) bool {
	return s.resolve(permission, true, false, nil)
}

func (s *permissionService) Deny(permission PermissionRequest) bool {
	return s.resolve(permission, false, true, nil)
}

func (s *permissionService) Request(ctx context.Context, opts CreatePermissionRequest) (bool, error) {
	if s.skipFn() {
		return true, nil
	}

	// Check if the tool/action combination is in the allowlist
	commandKey := opts.ToolName + ":" + opts.Action
	if slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, opts.ToolName) {
		return true, nil
	}

	// A PreToolUse hook that returned decision=allow stamps the context
	// with the tool call ID. Treat that as a pre-approval and skip the
	// prompt entirely. We still publish a granted notification so the UI
	// and audit subscribers see the outcome.
	if hookApproved(ctx, opts.ToolCallID) {
		s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
			SessionID:  opts.SessionID,
			ToolCallID: opts.ToolCallID,
			Granted:    true,
		})
		return true, nil
	}
	if RequestPolicyFromContext(ctx) == RequestPolicyAutoApprove {
		s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
			SessionID:  opts.SessionID,
			ToolCallID: opts.ToolCallID,
			Granted:    true,
		})
		return true, nil
	}

	// The PrePermission hook runs as the last pre-prompt check: hooks
	// that pre-approve (PreToolUse allow) and allowlisted calls never
	// reach it, so its allow and deny are the only opinions left before
	// a human is asked. Silence and errors fall through to the prompt;
	// the hook layer fails open here because the auto-mode layer
	// (plan-auto.md step 4) makes its own fail-closed call.
	if s.prePermissionRunner != nil && s.transcriptFn != nil {
		detail := hooks.PermissionDetail{
			ToolName:    opts.ToolName,
			Action:      opts.Action,
			Path:        opts.Path,
			Description: opts.Description,
		}
		transcript, terr := s.transcriptFn(ctx, opts.SessionID)
		if terr != nil {
			slog.Warn("Failed to build the PrePermission transcript", "error", terr)
		}
		agg, herr := s.prePermissionRunner.RunPermission(ctx, hooks.EventPrePermission, opts.SessionID, detail, transcript)
		if herr != nil {
			slog.Warn("PrePermission hooks failed", "error", herr)
		} else {
			switch agg.Decision {
			case hooks.DecisionAllow:
				s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
					SessionID:  opts.SessionID,
					ToolCallID: opts.ToolCallID,
					Granted:    true,
				})
				return true, nil
			case hooks.DecisionDeny:
				s.firePermissionDenied(ctx, detail, opts, transcript)
				s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
					SessionID:  opts.SessionID,
					ToolCallID: opts.ToolCallID,
					Denied:     true,
				})
				return false, nil
			}
		}
	}

	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	// tell the UI that a permission was requested
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		SessionID:  opts.SessionID,
		ToolCallID: opts.ToolCallID,
	})

	fileInfo, err := os.Stat(opts.Path)
	dir := opts.Path
	if err == nil {
		if fileInfo.IsDir() {
			dir = opts.Path
		} else {
			dir = filepath.Dir(opts.Path)
		}
	}

	if dir == "." {
		dir = s.workingDir
	}
	permission := PermissionRequest{
		ID:          uuid.New().String(),
		Path:        dir,
		SessionID:   opts.SessionID,
		ToolCallID:  opts.ToolCallID,
		ToolName:    opts.ToolName,
		Description: opts.Description,
		Action:      opts.Action,
		Params:      opts.Params,
	}

	if _, ok := s.sessionPermissions.Get(PermissionKey{
		SessionID: permission.SessionID,
		ToolName:  permission.ToolName,
		Action:    permission.Action,
		Path:      permission.Path,
	}); ok {
		s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
			SessionID:  opts.SessionID,
			ToolCallID: opts.ToolCallID,
			Granted:    true,
		})
		return true, nil
	}

	s.activeRequestMu.Lock()
	s.activeRequest = &permission
	s.activeRequestMu.Unlock()

	respCh := make(chan bool, 1)
	s.pendingRequests.Set(permission.ID, respCh)
	defer s.pendingRequests.Del(permission.ID)

	// Publish the request
	s.Publish(pubsub.CreatedEvent, permission)

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case granted := <-respCh:
		return granted, nil
	}
}

// ActiveRequest returns the request that is currently awaiting a
// decision. The active pointer can outlive the waiter that published
// it (Request leaves it behind when its context is cancelled), so a
// request is only reported while its ID is still registered in
// pendingRequests, i.e. while Grant/Deny can actually resolve it.
func (s *permissionService) ActiveRequest() (PermissionRequest, bool) {
	s.activeRequestMu.Lock()
	active := s.activeRequest
	s.activeRequestMu.Unlock()
	if active == nil {
		return PermissionRequest{}, false
	}
	if _, ok := s.pendingRequests.Get(active.ID); !ok {
		return PermissionRequest{}, false
	}
	return *active, true
}

func (s *permissionService) AutoApproveSession(sessionID string) {
	s.autoApproveSessionsMu.Lock()
	s.autoApproveSessions[sessionID] = true
	s.autoApproveSessionsMu.Unlock()
}

func (s *permissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionNotification] {
	return s.notificationBroker.Subscribe(ctx)
}

func (s *permissionService) SetSkipRequests(skip bool) {
	s.skip.Store(skip)
}

// SetPrePermission wires the permission hook events. The runner
// executes PrePermission before the prompt (an allow or deny
// short-circuits it; silence falls through) and PermissionDenied after
// a denial (fire-and-forget). Either function may be nil together with
// an empty runner; passing nil disables the events again.
// firePermissionDenied runs the PermissionDenied hooks fire-and-forget:
// the denial already happened, so a slow or failing hook never blocks
// or changes it.
func (s *permissionService) firePermissionDenied(ctx context.Context, detail hooks.PermissionDetail, opts CreatePermissionRequest, transcript string) {
	if s.prePermissionRunner == nil {
		return
	}
	go func() {
		fireCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if _, err := s.prePermissionRunner.RunPermission(fireCtx, hooks.EventPermissionDenied, opts.SessionID, detail, transcript); err != nil {
			slog.Warn("PermissionDenied hooks failed", "error", err)
		}
	}()
}

func (s *permissionService) SetPrePermission(runner *hooks.Runner, transcript func(ctx context.Context, sessionID string) (string, error)) {
	s.prePermissionRunner = runner
	s.transcriptFn = transcript
}

func (s *permissionService) SkipRequests() bool {
	return s.skip.Load()
}

func NewPermissionService(workingDir string, skip bool, allowedTools []string) Service {
	svc := &permissionService{
		Broker:             pubsub.NewBroker[PermissionRequest](),
		notificationBroker: pubsub.NewBroker[PermissionNotification](),
		workingDir:         workingDir,
		sessionPermissions: csync.NewMap[PermissionKey, bool](),
		allowedTools:       allowedTools,
		pendingRequests:    csync.NewMap[string, chan bool](),

		autoApproveSessions: make(map[string]bool),
	}
	svc.skip.Store(skip)
	// Consulted on every Request so a runtime toggle reaches the service
	// without re-dialing every tool.
	svc.skipFn = svc.skip.Load
	return svc
}

// scopedPermissionService is a permission service rooted at a dispatch
// workspace that follows another service's live skip state. Everything
// else (grants, session permissions, subscriptions) stays scoped to the
// dispatch; only skip approval is delegated, so the parent's yolo
// toggle applies to dispatched agents at request time.
type scopedPermissionService struct {
	*permissionService
}

func (s *scopedPermissionService) SkipRequests() bool {
	return s.parent.SkipRequests()
}

func (s *scopedPermissionService) SetSkipRequests(skip bool) {
	s.parent.SetSkipRequests(skip)
}

// NewScopedPermissionService returns a service whose Request checks
// parent.SkipRequests at request time instead of a startup snapshot, so
// toggling the parent's yolo at runtime reaches dispatched agents in
// both directions. parent must be non-nil.
func NewScopedPermissionService(parent Service, workingDir string, allowedTools []string) Service {
	svc := &permissionService{
		requestSlot:         make(chan struct{}, 1),
		Broker:              pubsub.NewBroker[PermissionRequest](),
		notificationBroker:  pubsub.NewBroker[PermissionNotification](),
		workingDir:          workingDir,
		sessionPermissions:  csync.NewMap[PermissionKey, bool](),
		autoApproveSessions: make(map[string]bool),
		parent:              parent,
		allowedTools:        allowedTools,
		pendingRequests:     csync.NewMap[string, chan bool](),
	}
	svc.skipFn = parent.SkipRequests
	return &scopedPermissionService{permissionService: svc}
}
