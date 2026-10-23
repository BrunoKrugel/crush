package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
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
