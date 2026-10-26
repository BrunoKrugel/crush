package dialog

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/snapshot"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

const (
	// SnapshotsID is the identifier for the snapshots dialog.
	SnapshotsID             = "snapshots"
	snapshotsDialogMaxWidth = 70
)

// ActionRestoreSnapshot asks the UI to restore a snapshot, with the
// current tree captured first so the restore can be undone.
type ActionRestoreSnapshot struct {
	SessionID  string
	CommitHash string
}

// ActionDiffSnapshot asks the UI to show a snapshot's diff.
type ActionDiffSnapshot struct {
	SessionID  string
	CommitHash string
}

// Snapshots is a dialog listing a session's filesystem snapshots.
type Snapshots struct {
	com       *common.Common
	sessionID string
	list      *list.FilterableList
	help      help.Model
	input     textinput.Model
	keyMap    struct {
		Select   key.Binding
		Diff     key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

// SnapshotItem is one snapshot row.
type SnapshotItem struct {
	*list.Versioned
	t       *styles.Styles
	cache   map[int]string
	info    snapshot.SnapshotInfo
	m       fuzzy.Match
	focused bool
}

var (
	_ Dialog   = (*Snapshots)(nil)
	_ ListItem = (*SnapshotItem)(nil)
)

// NewSnapshots builds the snapshots dialog for a session.
func NewSnapshots(com *common.Common, sessionID string, snapshots []snapshot.SnapshotInfo) (*Snapshots, error) {
	s := &Snapshots{com: com, sessionID: sessionID}

	s.help = help.New()
	s.help.Styles = com.Styles.DialogHelpStyles()

	s.list = list.NewFilterableList()
	s.list.Focus()

	s.input = textinput.New()
	s.input.SetVirtualCursor(false)
	s.input.Placeholder = "Type to filter"
	s.input.SetStyles(com.Styles.TextInput)
	s.input.Focus()

	s.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "restore"),
	)
	s.keyMap.Diff = key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "copy diff"),
	)
	s.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	s.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	s.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	s.keyMap.Close = CloseKey

	items := make([]list.FilterableItem, 0, len(snapshots))
	for _, info := range snapshots {
		items = append(items, &SnapshotItem{
			Versioned: list.NewVersioned(),
			t:         com.Styles,
			info:      info,
		})
	}
	s.list.SetItems(items...)

	return s, nil
}

// ID implements Dialog.
func (s *Snapshots) ID() string {
	return SnapshotsID
}

// HandleMsg implements [Dialog].
func (s *Snapshots) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, s.keyMap.Previous):
			s.list.Focus()
			if s.list.IsSelectedFirst() {
				s.list.SelectLast()
				s.list.ScrollToBottom()
				break
			}
			s.list.SelectPrev()
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Next):
			s.list.Focus()
			if s.list.IsSelectedLast() {
				s.list.SelectFirst()
				s.list.ScrollToTop()
				break
			}
			s.list.SelectNext()
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Select):
			if item, ok := s.selected(); ok {
				return ActionRestoreSnapshot{SessionID: s.sessionID, CommitHash: item.info.CommitHash}
			}
		case key.Matches(msg, s.keyMap.Diff):
			if item, ok := s.selected(); ok {
				return ActionDiffSnapshot{SessionID: s.sessionID, CommitHash: item.info.CommitHash}
			}
		default:
			prevValue := s.input.Value()
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			value := s.input.Value()
			if value != prevValue {
				s.list.SetFilter(value)
				s.list.ScrollToTop()
				s.list.SetSelected(0)
			}
			return ActionCmd{cmd}
		}
	}
	return nil
}

func (s *Snapshots) selected() (*SnapshotItem, bool) {
	item, ok := s.list.SelectedItem().(*SnapshotItem)
	return item, ok && item != nil
}

// Cursor returns the cursor position relative to the dialog.
func (s *Snapshots) Cursor() *tea.Cursor {
	return InputCursor(s.com.Styles, s.input.Cursor())
}

// Draw implements [Dialog].
func (s *Snapshots) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	width := max(0, min(snapshotsDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	s.input.SetWidth(dialogInputTextWidth(t, s.input, innerWidth))

	listTotalHeight := s.list.TotalHeight()
	desiredHeight := dialogChromeHeight(t) + listTotalHeight
	maxAvailable := area.Dy() - t.Dialog.View.GetVerticalBorderSize()
	height := max(defaultDialogHeight/2, min(defaultDialogHeight, desiredHeight, maxAvailable))

	listHeight, listTotalHeight, _ := sizeDialogList(t, s.list, innerWidth, height)

	if s.list.Height() >= len(s.list.FilteredItems()) {
		s.list.ScrollToTop()
	} else {
		s.list.ScrollToSelected()
	}

	rc := NewRenderContext(t, width)
	rc.Title = "Snapshots"
	inputView := t.Dialog.InputPrompt.Render(s.input.View())
	rc.AddPart(inputView)
	listView := t.Dialog.List.Height(s.list.Height()).Render(s.list.Render())
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, s.list.Offset())
	rc.AddPart(listView)
	rc.Help = renderDialogHelp(t, &s.help, s, innerWidth)

	view := rc.Render()

	cur := s.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (s *Snapshots) ShortHelp() []key.Binding {
	return []key.Binding{
		s.keyMap.UpDown,
		s.keyMap.Select,
		s.keyMap.Diff,
		s.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (s *Snapshots) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{s.keyMap.Select, s.keyMap.Diff, s.keyMap.Next, s.keyMap.Previous},
		{s.keyMap.Close},
	}
}

// Finished implements list.Item. Snapshot items are render-stable
// outside of explicit SetFocused / SetMatch.
func (i *SnapshotItem) Finished() bool {
	return true
}

// Filter returns the filter value for the snapshot item.
func (i *SnapshotItem) Filter() string {
	return i.info.Description + " " + i.info.CommitHash
}

// ID returns the snapshot's commit hash.
func (i *SnapshotItem) ID() string {
	return i.info.CommitHash
}

// SetFocused sets the focus state of the snapshot item.
func (i *SnapshotItem) SetFocused(focused bool) {
	if i.focused == focused {
		return
	}
	i.cache = nil
	i.focused = focused
	if i.Versioned != nil {
		i.Bump()
	}
}

// SetMatch sets the fuzzy match for the snapshot item.
func (i *SnapshotItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(i.m, m) {
		return
	}
	i.cache = nil
	i.m = m
	if i.Versioned != nil {
		i.Bump()
	}
}

// Render returns the string representation of the snapshot item.
func (i *SnapshotItem) Render(width int) string {
	title := fmt.Sprintf("%s  %s",
		i.info.CreatedAt.Format(time.DateTime),
		i.info.Description,
	)
	title = ansi.Truncate(title, max(0, width), "…")
	info := shortSnapshotHash(i.info.CommitHash)
	itemStyles := ListItemStyles{
		ItemBlurred:     i.t.Dialog.NormalItem,
		ItemFocused:     i.t.Dialog.SelectedItem,
		InfoTextBlurred: i.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: i.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(itemStyles, title, info, i.focused, width, i.cache, &i.m)
}

// shortSnapshotHash shortens a commit hash for display.
func shortSnapshotHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}
