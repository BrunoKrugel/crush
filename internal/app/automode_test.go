package app

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/automode"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// transcriptCoordinator satisfies agent.Coordinator for the transcript
// builder; only ListSettledMessages is real.
type transcriptCoordinator struct {
	agent.Coordinator
	msgs []message.Message
}

func (c *transcriptCoordinator) ListSettledMessages(context.Context, string) ([]message.Message, error) {
	return c.msgs, nil
}

func TestNewAutoMode_ConfigGated(t *testing.T) {
	t.Parallel()

	require.Nil(t, newAutoMode(&config.Config{}, nil),
		"no auto_mode block means no classifier to wire")

	enabled := &config.Config{AutoMode: &config.AutoModeConfig{Enabled: true}}
	require.NotNil(t, newAutoMode(enabled, nil))
}

func TestNewAutoMode_TranscriptNilAppSafe(t *testing.T) {
	t.Parallel()

	var app *App
	transcript := func(ctx context.Context, sessionID string) string {
		if app == nil {
			return ""
		}
		return app.autoModeTranscript(ctx, sessionID)
	}
	require.NotNil(t, newAutoMode(&config.Config{AutoMode: &config.AutoModeConfig{Enabled: true}}, transcript))
	require.Equal(t, "", transcript(t.Context(), "s1"))
}

func TestAutoModeToggler_RoundTripAndEvent(t *testing.T) {
	t.Parallel()

	app := NewForTest(t.Context())
	defer app.ShutdownForTest()

	app.autoMode = automode.New(automode.Options{})

	events := app.Events(t.Context())

	app.SetAutoMode(true)
	require.True(t, app.AutoModeToggler().AutoModeEnabled())
	app.SetAutoMode(false)
	require.False(t, app.AutoModeToggler().AutoModeEnabled())

	app.SetAutoMode(true)
	require.Eventually(t, func() bool {
		select {
		case ev := <-events:
			if auto, ok := ev.Payload.(pubsub.Event[PermissionAutoChangedEvent]); ok && auto.Payload.Enabled {
				return true
			}
			return false
		default:
			return false
		}
	}, 2*time.Second, 5*time.Millisecond, "the toggle must publish its state change")
}

func TestAutoModeTranscript_RendersUserAndToolCallsOnly(t *testing.T) {
	t.Parallel()

	app := NewForTest(t.Context())
	defer app.ShutdownForTest()
	app.AgentCoordinator = &transcriptCoordinator{msgs: []message.Message{
		{
			Role: message.User,
			Parts: []message.ContentPart{
				message.TextContent{Text: "please run the tests"},
			},
		},
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ReasoningContent{Thinking: "secret plan"},
				message.ToolCall{ID: "t1", Name: "bash", Input: `{"command":"go test ./..."}`},
			},
		},
	}}

	got := app.autoModeTranscript(t.Context(), "s1")
	require.Contains(t, got, "User: please run the tests")
	require.Contains(t, got, "ToolCall: bash")
	require.Contains(t, got, "go test ./...")
	require.NotContains(t, got, "secret plan",
		"the classifier excerpt must stay reasoning-blind")
}

// compile-time: the stub stays a Coordinator as the app's field expects.
var _ agent.Coordinator = (*transcriptCoordinator)(nil)

// auto mode satisfies the toggler surface the workspace delegates to.
var _ permission.AutoModeToggler = (*automode.AutoMode)(nil)
