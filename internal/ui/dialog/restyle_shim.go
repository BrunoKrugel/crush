package dialog

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/textarea"
)

// questionTextareaStyles returns the styles of a question textarea,
// adapted to the vendored textarea package used by the fork UI.
func questionTextareaStyles(sty *styles.Styles) textarea.Styles {
	taStyles := sty.Editor.Textarea
	return taStyles
}

var _ = lipgloss.NewStyle
