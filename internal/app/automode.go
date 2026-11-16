package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/automode"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// PermissionAutoChangedEvent reports a native auto-mode toggle so
// subscribed clients (client/server mode) can mirror the state into
// their workspace snapshot.
type PermissionAutoChangedEvent struct {
	Enabled bool
}

// classifierModelBuilder is implemented by the coordinator; the app
// attaches it to the auto mode so the classifier model is built by the
// same provider machinery the agents use.
type classifierModelBuilder interface {
	AutoModeClassifierModel(ctx context.Context) (fantasy.LanguageModel, error)
}

// newAutoMode builds the native auto mode when the config enables it.
// The toggler it returns is the single runtime on/off switch; nil means
// auto mode is not configured and every toggle is a no-op.
func newAutoMode(cfg *config.Config, transcript automode.TranscriptProvider) *automode.AutoMode {
	resolved, enabled := cfg.ResolvedAutoMode()
	if !enabled {
		return nil
	}
	opts := automode.Options{
		Transcript:            transcript,
		MaxTokens:             int64(resolved.MaxTokens),
		Timeout:               time.Duration(resolved.TimeoutSeconds) * time.Second,
		FailOpen:              resolved.FailOpen,
		Environment:           resolved.Environment,
		PromptStage1File:      resolved.PromptStage1File,
		PromptStage2File:      resolved.PromptStage2File,
		TranscriptMaxChars:    resolved.TranscriptMaxChars,
		MaxConsecutiveDenials: resolved.MaxConsecutiveDenials,
		MaxTotalDenials:       resolved.MaxTotalDenials,
	}
	return automode.New(opts)
}

// autoModeTranscript renders a reasoning-blind excerpt of the session:
// user prompts and tool calls only, no model reasoning and no tool
// output. The classifier sees what was asked and what was run, which is
// the context a permission decision needs, and nothing that could leak
// thinking or large tool results into a side request.
func (app *App) autoModeTranscript(ctx context.Context, sessionID string) string {
	coord := app.AgentCoordinator
	if coord == nil || sessionID == "" {
		return ""
	}
	msgs, err := coord.ListSettledMessages(ctx, sessionID)
	if err != nil {
		slog.Warn("Failed to build auto-mode transcript", "error", err)
		return ""
	}
	var b strings.Builder
	for _, msg := range msgs {
		switch msg.Role {
		case message.User:
			for _, part := range msg.Parts {
				if tc, ok := part.(message.TextContent); ok && strings.TrimSpace(tc.Text) != "" {
					fmt.Fprintf(&b, "User: %s\n", strings.TrimSpace(tc.Text))
				}
			}
		case message.Assistant:
			for _, part := range msg.Parts {
				if call, ok := part.(message.ToolCall); ok {
					fmt.Fprintf(&b, "ToolCall: %s %s\n", call.Name, truncateTranscript(call.Input))
				}
			}
		}
	}
	return b.String()
}

// transcriptMaxToolInput caps each tool-call input in the excerpt so a
// huge write or paste cannot dominate the classifier prompt.
const transcriptMaxToolInput = 400

func truncateTranscript(input string) string {
	input = strings.TrimSpace(input)
	if len(input) <= transcriptMaxToolInput {
		return input
	}
	return input[:transcriptMaxToolInput] + "…"
}

// permissionPreHooks builds the pre-permission hook chain: the native
// auto mode only. Shell hooks keep their existing surfaces (PreToolUse
// before tool execution, the prompt-shown/result events), so pre-prompt
// evaluation stays the classifier's job alone. Nil is fine: the service
// treats absent hooks as "always prompt".
func (app *App) permissionPreHooks() permission.PermissionHooks {
	return app.autoMode
}

// AutoModeToggler returns the runtime auto-mode switch, or nil when auto
// mode is not configured.
func (app *App) AutoModeToggler() permission.AutoModeToggler {
	if app.autoMode == nil {
		return nil
	}
	return app.autoMode
}

// SetAutoMode flips native auto mode and publishes the change so
// subscribed clients (client/server mode) mirror the state.
func (app *App) SetAutoMode(enabled bool) {
	toggler := app.AutoModeToggler()
	if toggler == nil {
		slog.Warn("Auto mode toggle ignored: auto mode is not configured")
		return
	}
	toggler.SetAutoModeEnabled(enabled)
	app.events.Publish(pubsub.UpdatedEvent, pubsub.Event[PermissionAutoChangedEvent]{
		Type:    pubsub.UpdatedEvent,
		Payload: PermissionAutoChangedEvent{Enabled: enabled},
	})
}

// attachAutoModeResolver wires the classifier model builder once the
// coordinator exists; the app constructs auto mode before the
// coordinator, so the resolver is attached late.
func (app *App) attachAutoModeResolver() {
	if app.autoMode == nil || app.AgentCoordinator == nil {
		return
	}
	if builder, ok := app.AgentCoordinator.(classifierModelBuilder); ok {
		app.autoMode.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
			return builder.AutoModeClassifierModel(ctx)
		})
	}
}
