package agent

const (
	CancelDispatchToolName   = "cancel_dispatch"
	MessageAgentToolName     = "message_agent"
	ApplyDispatchToolName    = "apply_dispatch"
	DismissDispatchToolName  = "dismiss_dispatch"
)

// DispatchAgentToolName names the sub-agent dispatch tool the fork UI
// routes to the dispatch block renderer.
const DispatchAgentToolName = "dispatch_agent"

// DispatchAgentParams is the fork UI's parse target for the dispatch
// tool call input. Only the fields the renderer displays are kept.
type DispatchAgentParams struct {
	Prompt string
}
