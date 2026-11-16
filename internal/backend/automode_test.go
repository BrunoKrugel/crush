package backend

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestSetPermissionAutoMode_NotConfigured pins the surface for workspaces
// without an auto_mode config block: toggling is rejected and the
// workspace snapshot never claims a running classifier.
func TestSetPermissionAutoMode_NotConfigured(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	args := protoWS(t.TempDir(), t.TempDir(), uuid.New().String())
	ws, created, err := b.CreateWorkspace(args)
	require.NoError(t, err)
	require.False(t, created.PermissionAuto,
		"a workspace without auto mode must report the classifier off")

	err = b.SetPermissionAutoMode(ws.ID, true)
	require.ErrorIs(t, err, ErrAutoModeNotConfigured)

	got, err := b.GetWorkspaceProto(ws.ID)
	require.NoError(t, err)
	require.False(t, got.PermissionAuto,
		"a rejected toggle must not leak into the workspace snapshot")
}
