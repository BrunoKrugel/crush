package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// ForkID is the identifier for the conversation-forking dialog.
const ForkID = "fork"

// Fork is the dialog that names a fork and asks whether it should get
// its own worktree.
type Fork struct {
	com       *common.Common
	help      help.Model
	keyMap    struct{ Submit, Close, ToggleWorktree key.Binding }
	input     textinput.Model
	width     int
	worktree  bool
	sourceID  string
	messageID string
}

var _ Dialog = (*Fork)(nil)

// NewFork creates the fork dialog for a source session and message.
func NewFork(com *common.Common, sourceSessionID, messageID string) *Fork {
	f := &Fork{
		com:       com,
		sourceID:  sourceSessionID,
		messageID: messageID,
	}
	f.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter", "fork"),
	)
	f.keyMap.Close = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	)
	f.keyMap.ToggleWorktree = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "toggle worktree"),
	)
	f.input = textinput.New()
	f.input.SetVirtualCursor(false)
	f.input.Placeholder = "Name the fork..."
	f.input.SetStyles(com.Styles.TextInput)
	f.input.Focus()
	return f
}

// ID returns the dialog's identifier.
func (f *Fork) ID() string { return ForkID }

// Title returns the dialog's title.
func (f *Fork) Title() string { return "Fork Conversation" }

// HandleMsg processes a message and returns the action to take.
func (f *Fork) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, f.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, f.keyMap.ToggleWorktree):
			f.worktree = !f.worktree
			return ActionCmd{func() tea.Msg { return nil }}
		case key.Matches(msg, f.keyMap.Submit):
			return ActionFork{
				SessionID: f.sourceID,
				MessageID: f.messageID,
				Title:     strings.TrimSpace(f.input.Value()),
				Worktree:  f.worktree,
			}
		}
	}
	return ActionCmd{func() tea.Msg { return nil }}
}

// Draw renders the dialog.
func (f *Fork) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := f.com.Styles
	f.width = max(0, min(60, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := f.width - t.Dialog.View.GetHorizontalFrameSize() - 2
	f.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1))

	textStyle := t.Dialog.SecondaryText
	dialogStyle := t.Dialog.View.Width(f.width)
	helpView := renderDialogHelp(t, &f.help, f, f.width-dialogStyle.GetHorizontalFrameSize())

	worktree := "No"
	if f.worktree {
		worktree = "Yes"
	}
	content := strings.Join([]string{
		t.Dialog.Title.Render("Fork Conversation"),
		textStyle.Render("The fork continues this conversation from the selected message."),
		t.Dialog.InputPrompt.Render(f.input.View()),
		textStyle.Render(fmt.Sprintf("Fresh worktree: %s (tab to toggle)", worktree)),
		"",
		helpView,
	}, "\n")

	cur := f.Cursor()
	view := dialogStyle.Render(content)
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// Cursor returns the cursor position for the dialog.
func (f *Fork) Cursor() *tea.Cursor {
	return f.input.Cursor()
}

// ShortHelp implements help.KeyMap.
func (f *Fork) ShortHelp() []key.Binding {
	return []key.Binding{f.keyMap.Submit, f.keyMap.ToggleWorktree, f.keyMap.Close}
}

// FullHelp implements help.KeyMap.
func (f *Fork) FullHelp() [][]key.Binding {
	return [][]key.Binding{{f.keyMap.Submit, f.keyMap.ToggleWorktree, f.keyMap.Close}}
}
