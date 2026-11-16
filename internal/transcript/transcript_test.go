package transcript

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestBlindKeepsUserPromptsAndToolCalls(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{ID: "m1", Role: message.User, Parts: []message.ContentPart{
			message.TextContent{Text: "fix the flaky test"},
		}},
		{ID: "m2", Role: message.Assistant, Parts: []message.ContentPart{
			message.TextContent{Text: "I will now edit the file, here is my reasoning about the race"},
			message.ToolCall{ID: "tc1", Name: "edit", Input: `{"path":"a_test.go"}`},
		}},
		{ID: "m3", Role: message.Assistant, Parts: []message.ContentPart{
			message.TextContent{Text: "done, the test passes now"},
		}},
	}

	got := Blind(msgs)
	require.Contains(t, got, "fix the flaky test")
	require.Contains(t, got, `"name":"edit"`)
	require.Contains(t, got, `"path":"a_test.go"`)

	// Assistant prose and tool outputs are stripped: the judge sees intent,
	// not results.
	require.NotContains(t, got, "here is my reasoning")
	require.NotContains(t, got, "done, the test passes now")
}

func TestBlindEmptyHistory(t *testing.T) {
	t.Parallel()
	require.Equal(t, "{}", Blind(nil))
}

func TestBlindSkipsHiddenAndSteer(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{ID: "m1", Role: message.User, Parts: []message.ContentPart{
			message.TextContent{Text: "visible prompt"},
		}},
		{ID: "m2", Role: message.User, Parts: []message.ContentPart{
			message.TextContent{Text: "hidden scaffolding", Hidden: true},
		}},
		{ID: "m3", Role: message.User, Parts: []message.ContentPart{
			message.TextContent{Text: "steer injection", Steer: true},
		}},
	}

	got := Blind(msgs)
	require.Contains(t, got, "visible prompt")
	require.NotContains(t, got, "hidden scaffolding")
	require.NotContains(t, got, "steer injection")
}
