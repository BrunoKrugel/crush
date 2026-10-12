package herdr

import (
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTranslateProtoSummaryMessage covers client/server mode, where the
// summary flag marks the message that replaced the transcript with a
// compaction checkpoint: herdr must report the run as still working
// rather than finished.
func TestTranslateProtoSummaryMessage(t *testing.T) {
	t.Parallel()

	summary := pubsub.Event[proto.Message]{
		Payload: proto.Message{Role: proto.Assistant, SessionID: "s-1", IsSummaryMessage: true},
	}
	assert.Equal(t, SummarizeStarted{SessionID: "s-1"}, Translate(summary))

	normal := pubsub.Event[proto.Message]{
		Payload: proto.Message{Role: proto.Assistant, SessionID: "s-1"},
	}
	event := Translate(normal)
	require.IsType(t, AssistantMessage{}, event)
	assert.Equal(t, "s-1", event.(AssistantMessage).SessionID)
}
