package permission

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// recordingToggler is a PermissionHooks that also implements
// AutoModeToggler, so a test can prove SetAutoModeEnabled reaches the
// installed hook rather than a local flag.
type recordingToggler struct {
	enabled []bool
	on      bool
}

func (recordingToggler) PrePermission(context.Context, PermissionRequest) PreHookResult {
	return PreHookResult{}
}

func (recordingToggler) PermissionDenied(context.Context, PermissionRequest) {}

func (r *recordingToggler) AutoModeEnabled() bool {
	return r.on
}

func (r *recordingToggler) SetAutoModeEnabled(enabled bool) {
	r.on = enabled
	r.enabled = append(r.enabled, enabled)
}

// silentHooks implements PermissionHooks but not AutoModeToggler. The
// selection still has to stick: ctrl+y must not depend on a classifier
// hook being installed.
type silentHooks struct{}

func (silentHooks) PrePermission(context.Context, PermissionRequest) PreHookResult {
	return PreHookResult{}
}

func (silentHooks) PermissionDenied(context.Context, PermissionRequest) {}

func TestSetAutoModeEnabledReachesInstalledToggler(t *testing.T) {
	t.Parallel()

	svc := NewPermissionService(t.TempDir(), false, nil)
	hook := &recordingToggler{}
	svc.SetPermissionHooks(hook)

	require.False(t, svc.AutoModeEnabled())

	svc.SetAutoModeEnabled(true)
	require.Equal(t, []bool{true}, hook.enabled)
	require.True(t, svc.AutoModeEnabled())

	svc.SetAutoModeEnabled(false)
	require.Equal(t, []bool{true, false}, hook.enabled)
	require.False(t, svc.AutoModeEnabled())
}

func TestSetAutoModeEnabledNoTogglerIsNoop(t *testing.T) {
	t.Parallel()

	svc := NewPermissionService(t.TempDir(), false, nil)
	svc.SetPermissionHooks(silentHooks{})

	require.NotPanics(t, func() {
		svc.SetAutoModeEnabled(true)
	})
	require.True(t, svc.AutoModeEnabled(), "selecting auto mode must stick without a classifier hook")

	svc.SetAutoModeEnabled(false)
	require.False(t, svc.AutoModeEnabled())
}

func TestSetAutoModeEnabledScopedFollowsParent(t *testing.T) {
	t.Parallel()

	parent := NewPermissionService(t.TempDir(), false, nil)
	hook := &recordingToggler{}
	parent.SetPermissionHooks(hook)

	scoped := NewScopedPermissionService(parent, t.TempDir(), nil)
	scoped.SetAutoModeEnabled(true)

	require.Equal(t, []bool{true}, hook.enabled)
	require.True(t, scoped.AutoModeEnabled())
	require.True(t, parent.AutoModeEnabled())
}
