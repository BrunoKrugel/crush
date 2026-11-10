package chat

import (
	"strings"

	"github.com/bytedance/sonic"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// GenericToolMessageItem is a message item that represents an unknown tool call.
type GenericToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*GenericToolMessageItem)(nil)

// NewGenericToolMessageItem creates a new [GenericToolMessageItem].
func NewGenericToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &GenericToolRenderContext{}, canceled)
}

// GenericToolRenderContext renders unknown/generic tool messages.
type GenericToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (g *GenericToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	name := humanizedToolName(opts.ToolCall.Name)

	if opts.IsPending() {
		return pendingTool(sty, name, opts.Anim, opts.Compact)
	}

	var params map[string]any
	if err := sonic.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	var toolParams []string
	if len(params) > 0 {
		parsed, _ := sonic.Marshal(params)
		toolParams = append(toolParams, string(parsed))
	}

	header := toolHeader(sty, opts.Status, name, cappedWidth, opts, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() || opts.Result.Content == "" {
		return header
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal

	if opts.Result.Data != "" && strings.HasPrefix(opts.Result.MIMEType, "image/") {
		// toolOutputImageContent applies sty.Tool.Body itself — wrapping it
		// again here double-indented image results.
		return joinToolParts(header, toolOutputImageContent(sty, opts.Result.Data, opts.Result.MIMEType))
	}

	widths := toolResultContentWidths{Body: bodyWidth, Diff: cappedWidth}

	body := renderToolResultTextContent(sty, opts.Result.Content, widths, opts.ExpandedContent)
	return joinToolParts(header, body)
}
