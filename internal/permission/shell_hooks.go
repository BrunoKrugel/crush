package permission

import (
	"context"
	"log/slog"
	"time"

	"github.com/charmbracelet/crush/internal/hooks"
)

// shellPermissionHooks adapts a shell-hook runner to the PermissionHooks
// interface. The PrePermission event runs before the prompt; an allow or
// deny short-circuits it. PermissionDenied fires after a denial,
// fire-and-forget.
type shellPermissionHooks struct {
	runner     *hooks.Runner
	transcript func(ctx context.Context, sessionID string) (string, error)
}

// NewShellPermissionHooks wraps a runner as a permission hook. A nil
// runner yields a no-op hook, so wiring it is always safe.
func NewShellPermissionHooks(runner *hooks.Runner, transcript func(ctx context.Context, sessionID string) (string, error)) PermissionHooks {
	return &shellPermissionHooks{runner: runner, transcript: transcript}
}

func (h *shellPermissionHooks) PrePermission(ctx context.Context, req PermissionRequest) PreHookResult {
	if h.runner == nil {
		return PreHookResult{Decision: HookDecisionNone}
	}
	detail := hooks.PermissionDetail{
		ToolName:    req.ToolName,
		Action:      req.Action,
		Path:        req.Path,
		Description: req.Description,
	}
	transcriptJSON := "{}"
	if h.transcript != nil {
		if t, err := h.transcript(ctx, req.SessionID); err == nil && t != "" {
			transcriptJSON = t
		}
	}
	agg, err := h.runner.RunPermission(ctx, hooks.EventPermissionRequest, req.SessionID, detail, transcriptJSON)
	if err != nil {
		slog.Warn("PrePermission hooks failed", "error", err)
		return PreHookResult{Decision: HookDecisionNone}
	}
	switch agg.Decision {
	case hooks.DecisionAllow:
		return PreHookResult{Decision: HookDecisionAllow, Reason: agg.Context}
	case hooks.DecisionDeny:
		return PreHookResult{Decision: HookDecisionDeny, Reason: agg.Reason}
	default:
		return PreHookResult{Decision: HookDecisionNone, Reason: agg.Reason}
	}
}

// PermissionDenied fires the PermissionDenied hooks fire-and-forget.
func (h *shellPermissionHooks) PermissionDenied(ctx context.Context, req PermissionRequest) {
	if h.runner == nil {
		return
	}
	detail := hooks.PermissionDetail{
		ToolName:    req.ToolName,
		Action:      req.Action,
		Path:        req.Path,
		Description: req.Description,
	}
	transcriptJSON := "{}"
	if h.transcript != nil {
		if t, err := h.transcript(ctx, req.SessionID); err == nil && t != "" {
			transcriptJSON = t
		}
	}
	go func() {
		fireCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if _, err := h.runner.RunPermission(fireCtx, hooks.EventPermissionResult, req.SessionID, detail, transcriptJSON); err != nil {
			slog.Warn("PermissionDenied hooks failed", "error", err)
		}
	}()
}
