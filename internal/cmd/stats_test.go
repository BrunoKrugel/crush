package cmd

import (
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/stretchr/testify/require"
)

func TestMergeStatsAggregatesForkCounters(t *testing.T) {
	t.Parallel()

	projectStats := []ProjectStats{
		{Stats: &Stats{Total: TotalStats{TotalSessions: 2}, Fork: &ForkStats{
			ForkedSessions:    1,
			Snapshots:         10,
			DispatchedAgents:  3,
			CompletedDispatch: 2,
			CompactedSessions: 1,
			CompactionEntries: 4,
			ToolCalls:         []ToolUsage{{ToolName: "Dispatch_Agent", CallCount: 5}},
		}}},
		{Stats: &Stats{Total: TotalStats{TotalSessions: 3}, Fork: &ForkStats{
			GoalSessions:      2,
			DispatchedAgents:  4,
			CompletedDispatch: 1,
			ToolCalls:         []ToolUsage{{ToolName: "dispatch_agent", CallCount: 7}, {ToolName: "semantic_search", CallCount: 1}},
		}}},
		// A database from before the fork's migrations: no fork block at all.
		{Stats: &Stats{Total: TotalStats{TotalSessions: 1}}},
	}

	merged := mergeStats(projectStats)

	require.NotNil(t, merged.Fork)
	require.Equal(t, int64(1), merged.Fork.ForkedSessions)
	require.Equal(t, int64(2), merged.Fork.GoalSessions)
	require.Equal(t, int64(10), merged.Fork.Snapshots)
	require.Equal(t, int64(7), merged.Fork.DispatchedAgents)
	require.Equal(t, int64(3), merged.Fork.CompletedDispatch)
	require.Equal(t, int64(1), merged.Fork.CompactedSessions)
	require.Equal(t, int64(4), merged.Fork.CompactionEntries)
	require.Equal(t, int64(6), merged.Total.TotalSessions)

	// Same tool, differently cased across projects, merges into one entry.
	require.Len(t, merged.Fork.ToolCalls, 2)
	require.Equal(t, int64(12), merged.Fork.ToolCalls[0].CallCount)
	// The name keeps its first-seen casing; matching is case-insensitive.
	require.Equal(t, "Dispatch_Agent", merged.Fork.ToolCalls[0].ToolName)
}

func TestGatherForkStatsToleratesLegacyDatabase(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), filepath.Join(t.TempDir(), "data"))
	require.NoError(t, err)
	defer conn.Close()

	fork := gatherForkStats(t.Context(), conn)
	require.NotNil(t, fork)
	require.Zero(t, fork.ForkedSessions)
	require.Zero(t, fork.DispatchedAgents)
}
