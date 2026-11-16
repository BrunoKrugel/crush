package chat

import (
	"encoding/json"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// subagentStatusName maps a tool status onto the label the subagent
// header used in the fork UI.
func subagentStatusName(status ToolStatus) string {
	switch status {
	case ToolStatusRunning:
		return "running"
	case ToolStatusSuccess:
		return "done"
	case ToolStatusError:
		return "failed"
	default:
		return ""
	}
}

// subagentHeader renders the fork chat header for dispatched sub-agent
// surfaces: status icon, name, dispatch label, and the running model in
// parentheses, truncating the model rather than dropping it.
func subagentHeader(sty *styles.Styles, status ToolStatus, name, label, model string, width int, opts *ToolRenderOpts) string {
	header := toolHeader(sty, status, name, width, opts, subagentTarget(sty, label))
	const minModelWidth = 12
	room := width - lipgloss.Width(header) - 1
	if model == "" || room < minModelWidth {
		return header
	}
	return header + " " + sty.Tool.ParamMain.Render(ansi.Truncate("(model="+model+")", room, "…)"))
}

// dispatchRunModel reads the model a dispatch recorded once its
// subagent started (empty before the background metadata arrives).
func dispatchRunModel(opts *ToolRenderOpts) string {
	var meta agent.AgentDispatchResponseMetadata
	if opts.HasResult() {
		_ = json.Unmarshal([]byte(opts.Result.Metadata), &meta)
	}
	return meta.Model
}

// SubagentReportTitle renders "Subagent Report → Name label" as a report's
// header does, for places that show a report without its status icon.
func SubagentReportTitle(sty *styles.Styles, label string) string {
	return sty.Tool.NameNormal.Render("Subagent Report") + " " + subagentTarget(sty, label)
}
func subagentTarget(sty *styles.Styles, label string) string {
	return sty.Tool.MCPArrow.String() + " " + RenderSubagentName(sty, label)
}

func RenderSubagentName(sty *styles.Styles, label string) string {
	first, rest := agent.SubagentDisplayName(label)
	if first == "" {
		return sty.Tool.AgentLabel.Render(rest)
	}
	return sty.Tool.AgentName.Render(first) + " " + sty.Tool.AgentLabel.Render(rest)
}
