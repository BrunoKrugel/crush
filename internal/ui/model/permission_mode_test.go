package model

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestCtrlYCyclesPermissionMode pins the permission-mode cycle. One
// ctrl+y enables auto and leaves skip off, a second enables skip and
// disables auto, and a third clears both. Auto mode must not turn yolo
// on, and yolo must not turn auto mode on.
func TestCtrlYCyclesPermissionMode(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	m.keyMap = DefaultKeyMap()

	press := func() {
		t.Helper()
		m.handleKeyPressMsg(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
	}

	press()
	require.True(t, ws.auto, "first ctrl+y enables auto mode")
	require.False(t, ws.yolo, "auto mode must not turn yolo on")
	require.True(t, m.autoModeCached())
	require.False(t, m.yoloModeCached())

	press()
	require.False(t, ws.auto, "second ctrl+y disables auto mode")
	require.True(t, ws.yolo, "second ctrl+y enables yolo")
	require.False(t, m.autoModeCached())
	require.True(t, m.yoloModeCached())

	press()
	require.False(t, ws.auto, "third ctrl+y clears auto mode")
	require.False(t, ws.yolo, "third ctrl+y clears yolo")
	require.False(t, m.autoModeCached())
	require.False(t, m.yoloModeCached())
}

// TestAutoPromptBadgeStaysVisible pins the empty-editor case. Selecting
// auto mode must keep the A badge visible beside the placeholder, not
// paint it and then let the placeholder style erase its background.
func TestAutoPromptBadgeStaysVisible(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	m.textarea.SetStyles(m.com.Styles.Editor.Textarea)
	m.textarea.SetWidth(40)
	m.textarea.Focus()
	m.textarea.Placeholder = "Ready!"

	m.handleKeyPressMsg(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})

	view := m.textarea.View()
	require.Contains(t, ansi.Strip(view), "A", "the auto badge must stay in the prompt")
	require.NotNil(t, drawnBadgeBackground(view, "A"),
		"the screen must keep the auto badge background after the outer reset")

	// A probe that started before the key, or a workspace that cannot
	// persist auto mode, must not clear the badge the key just painted.
	m.applyBusyState(busyStateMsg{gen: m.busyFetchGen, auto: false})

	view = m.textarea.View()
	require.True(t, m.autoModeCached(), "a failed persist must not clear the local auto selection")
	require.Contains(t, ansi.Strip(view), "A", "the auto badge must survive the refresh that follows ctrl+y")
}

// drawnBadgeBackground is the background the screen parser keeps for the
// badge. An outer reset that lands before the glyph clears it, which is
// what makes the indicator vanish after it is painted.
func drawnBadgeBackground(view, badge string) any {
	scr := uv.NewScreenBuffer(80, 8)
	uv.NewStyledString(view).Draw(scr, uv.Rect(0, 0, 80, 8))
	for y := range 8 {
		var plain strings.Builder
		for x := range 80 {
			if cell := scr.CellAt(x, y); cell != nil {
				plain.WriteString(cell.Content)
			}
		}
		idx := strings.Index(plain.String(), badge)
		if idx < 0 {
			continue
		}
		cell := scr.CellAt(idx, y)
		if cell == nil {
			return nil
		}
		return cell.Style.Bg
	}
	return nil
}
