package workspace

import (
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

// TestAppWorkspacePermissionSkipRequests pins the skip-request surface the
// Shift+Tab cycle and the YOLO toggle sit on: skipping mirrors the enforced
// permission mode, a sysadmin rung survives a skip write that keeps
// skipping on, and turning skip off drops to normal.
func TestAppWorkspacePermissionSkipRequests(t *testing.T) {
	t.Parallel()

	w := &AppWorkspace{app: &app.App{Permissions: permission.NewPermissionService("", nil, nil)}}

	require.False(t, w.PermissionSkipRequests())

	w.PermissionSetSkipRequests(true)
	require.Equal(t, permission.PermissionModeYolo, w.PermissionMode())
	require.True(t, w.PermissionSkipRequests())

	w.PermissionSetMode(permission.PermissionModeSysadmin)
	w.PermissionSetSkipRequests(true)
	require.Equal(t, permission.PermissionModeSysadmin, w.PermissionMode())
	require.True(t, w.PermissionSkipRequests())

	w.PermissionSetSkipRequests(false)
	require.Equal(t, permission.PermissionModeNormal, w.PermissionMode())
	require.False(t, w.PermissionSkipRequests())
}
