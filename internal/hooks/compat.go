package hooks

import (
	"context"
	"encoding/json"

	"github.com/bytedance/sonic"
)

// CompactionDetail is the `tool_input` handed to a PreCompact or
// PostCompact hook. Compaction has no tool call, so it describes what is
// about to be rewritten instead: which turn asked for it, any emphasis the
// user supplied, and how much text the checkpoint stands in for. Fork
// extension: upstream's compaction events carry only the trigger, which
// rides on Event.Trigger.
type CompactionDetail struct {
	Trigger      string `json:"trigger"`
	Instructions string `json:"instructions,omitempty"`
	Messages     int    `json:"messages"`
	Tokens       int64  `json:"tokens"`
}

// PermissionDetail is the tool_input handed to a permission hook: the
// action the permission service was about to ask about.
type PermissionDetail struct {
	ToolName    string `json:"tool_name"`
	Action      string `json:"action"`
	Path        string `json:"path,omitempty"`
	Description string `json:"description,omitempty"`
}

// jsonField renders the detail as the JSON object a hook reads as
// tool_input.
func jsonField(detail any) string {
	data, err := sonic.Marshal(detail)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// CompactionEvent builds the Event for a compaction hook, carrying the
// fork's detail as tool_input and the trigger both ways (Event.Trigger for
// upstream readers, tool_input.trigger for fork-era scripts).
func CompactionEvent(name, sessionID string, detail CompactionDetail) Event {
	return Event{
		Name:      name,
		SessionID: sessionID,
		Trigger:   detail.Trigger,
		Tool:      &ToolCall{Name: name, Input: jsonField(detail)},
	}
}

// PermissionEvent builds the Event for a permission hook, carrying the
// detail as tool_input and the reasoning-blind transcript when the hook
// asked for one.
func PermissionEvent(name, sessionID string, detail PermissionDetail, transcriptJSON string) Event {
	ev := Event{
		Name:      name,
		SessionID: sessionID,
		Tool:      &ToolCall{Name: detail.ToolName, Input: permissionInputJSON(detail)},
	}
	if transcriptJSON != "" && json.Valid([]byte(transcriptJSON)) {
		t := transcriptJSON
		ev.Transcript = &t
	}
	return ev
}

// permissionInputJSON renders the detail as tool_input, keeping the
// fork-era field spellings (action, path, description).
func permissionInputJSON(detail PermissionDetail) string {
	input := map[string]any{
		"tool_name": detail.ToolName,
		"action":    detail.Action,
	}
	if detail.Path != "" {
		input["path"] = detail.Path
	}
	if detail.Description != "" {
		input["description"] = detail.Description
	}
	return jsonField(input)
}

// RunCompaction runs the compaction hooks for the fork's detail shape.
// Deprecated: build an Event with [CompactionEvent] and call Run.
func (r *Runner) RunCompaction(ctx context.Context, eventName, sessionID string, detail CompactionDetail) (AggregateResult, error) {
	return r.Run(ctx, CompactionEvent(eventName, sessionID, detail)), nil
}

// RunPermission runs the permission hooks for the fork's detail shape,
// attaching the reasoning-blind transcript when one is supplied.
// Deprecated: build an Event with [PermissionEvent] and call Run.
func (r *Runner) RunPermission(ctx context.Context, eventName, sessionID string, detail PermissionDetail, transcriptJSON string) (AggregateResult, error) {
	return r.Run(ctx, PermissionEvent(eventName, sessionID, detail, transcriptJSON)), nil
}
