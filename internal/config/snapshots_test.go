package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshotsConfig(t *testing.T) {
	t.Parallel()

	var cfg Config
	data := `{"snapshots": {"enabled": false, "exclude": ["dist", "target"]}}`
	require.NoError(t, json.Unmarshal([]byte(data), &cfg))
	require.NotNil(t, cfg.Snapshots)
	require.NotNil(t, cfg.Snapshots.Enabled)
	require.False(t, *cfg.Snapshots.Enabled)
	require.Equal(t, []string{"dist", "target"}, cfg.Snapshots.Exclude)

	// Without the block, snapshots default to enabled with the standard
	// exclusions.
	var empty Config
	require.NoError(t, json.Unmarshal([]byte(`{}`), &empty))
	require.True(t, empty.SnapshotsEnabled())
	require.Equal(t, DefaultSnapshotExclusions, empty.SnapshotExclusions())
	require.Equal(t, []string{"node_modules", "vendor", "dist", "build"}, DefaultSnapshotExclusions)
}

func TestSnapshotsDefaults(t *testing.T) {
	t.Parallel()

	var cfg Config
	require.NoError(t, json.Unmarshal([]byte(`{"snapshots": {"exclude": ["secret"]}}`), &cfg))
	require.True(t, cfg.SnapshotsEnabled())
	require.Equal(t, []string{"secret"}, cfg.SnapshotExclusions())
}
