package agent

import (
	"context"
	"fmt"
	"testing"

	"charm.land/fantasy"
)

func TestDbgSummaryChain(t *testing.T) {
	_, sess, model, sa := newLifecycleTestAgent(t, "session")
	calls := 0
	model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
		calls++
		fmt.Println("STREAM", calls, "prompt:", call.Prompt[:min(20, len(call.Prompt))])
		if calls == 1 {
			result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "B", Prompt: "B"})
			fmt.Println("  B:", result != nil, err)
			return compactionStream, nil
		}
		if calls == 2 {
			return (&finishStreamModel{text: "summary"}).Stream(ctx, call)
		}
		if calls == 3 {
			return (&finishStreamModel{text: "B answer"}).Stream(ctx, call)
		}
		return (&finishStreamModel{text: "A resumed"}).Stream(ctx, call)
	}
	result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "A", Prompt: "A"})
	fmt.Println("FINAL err:", err, "text:", resultText2(result), "queued:", sa.QueuedPrompts(sess.ID), "busy:", sa.IsSessionBusy(sess.ID))
}

func resultText2(r *fantasy.AgentResult) string {
	if r == nil {
		return "<nil>"
	}
	for _, part := range r.Response.Content {
		if tc, ok := part.(fantasy.TextContent); ok {
			return tc.Text
		}
	}
	return "<?>"
}
