// Package transcript renders a session's messages as the reasoning-blind
// JSON a permission judge reads: what was asked and what was called, with
// the model's own prose and the tools' outputs stripped.
//
// The stripping is structural, not cosmetic: a judge that reads tool
// outputs reads every payload the model fetched, so a hostile web page or
// repository file could address the judge directly. Outputs are the one
// channel an attacker can write into at scale, so they never reach a
// permission decision.
package transcript

import (
	"encoding/json"

	"github.com/bytedance/sonic"
	"github.com/charmbracelet/crush/internal/message"
)

// blind is the JSON shape of the stripped transcript.
type blind struct {
	UserMessages []string    `json:"user_messages,omitempty"`
	ToolCalls    []blindTool `json:"tool_calls,omitempty"`
}

type blindTool struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input,omitempty"`
}

// Blind renders the messages into the reasoning-blind transcript. Hidden
// and steer messages are harness scaffolding and are left out.
func Blind(msgs []message.Message) string {
	out := blind{}
	for _, msg := range msgs {
		switch msg.Role {
		case message.User:
			text := msg.Content()
			if text.Hidden || text.Steer {
				continue
			}
			if text.Text != "" {
				out.UserMessages = append(out.UserMessages, text.Text)
			}
		case message.Assistant:
			for _, part := range msg.Parts {
				call, ok := part.(message.ToolCall)
				if !ok || call.Name == "" {
					continue
				}
				tool := blindTool{Name: call.Name}
				if call.Input != "" {
					tool.Input = json.RawMessage(call.Input)
				}
				out.ToolCalls = append(out.ToolCalls, tool)
			}
		}
	}
	data, err := sonic.Marshal(out)
	if err != nil {
		return "{}"
	}
	return string(data)
}
