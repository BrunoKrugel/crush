package chat

import (
	"time"
)

// DispatchStatus is the subagents sidebar row's snapshot of one dispatched
// agent, kept field-compatible with the fork UI's usage.
type DispatchStatus struct {
	Label    string
	Running  bool
	Started  time.Time
	Finished time.Time
}

// DispatchStatus reports what a dispatched agent-tool block is up to. The
// second return is false for plain agent-tool calls without a dispatch.
func (a *AgentToolMessageItem) DispatchStatus() (DispatchStatus, bool) {
	return DispatchStatus{}, false
}

// SubagentDisplayName splits an agent label into the leading agent id and
// the rest, mirroring the fork helper name.
func SubagentDisplayName(label string) (string, string) {
	fields := []rune(label)
	for i, r := range fields {
		if r == ' ' {
			return label[:i], label[i+1:]
		}
	}
	return label, ""
}
