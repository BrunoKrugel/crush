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

func TestGatherSkillUsageReadsTaggedAttachments(t *testing.T) {
	t.Parallel()

	dataDir := filepath.Join(t.TempDir(), "data")
	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	defer conn.Close()

	// One message carrying two skill attachments and one plain markdown
	// file, plus a second message in another session reusing the same
	// skill. The plain file must not be counted.
	for _, session := range []string{"s1", "s2"} {
		_, err := conn.Exec(`INSERT INTO sessions (id, title, updated_at, created_at) VALUES (?, 't', 1, 1)`, session)
		require.NoError(t, err)
	}
	insert := func(session, parts string) {
		_, err := conn.Exec(`INSERT INTO messages (id, session_id, role, parts, created_at, updated_at) VALUES (?, ?, 'user', ?, 1, 1)`, session+"-msg", session, parts)
		require.NoError(t, err)
	}
	insert("s1", `[
		{"type":"text","data":{"text":"hi"}},
		{"type":"binary","data":{"Path":"eli5.md","FileName":"eli5","MIMEType":"text/markdown","Kind":"skill","Data":"x"}},
		{"type":"binary","data":{"Path":"pair.md","FileName":"pair-with-bk","MIMEType":"text/markdown","Kind":"skill","Data":"x"}},
		{"type":"binary","data":{"Path":"notes.md","FileName":"notes","MIMEType":"text/markdown","Data":"x"}}
	]`)
	insert("s2", `[
		{"type":"binary","data":{"Path":"eli5.md","FileName":"eli5","MIMEType":"text/markdown","Kind":"skill","Data":"x"}}
	]`)

	fork := gatherForkStats(t.Context(), conn)
	require.Len(t, fork.SkillUses, 2)
	require.Equal(t, 2, int(fork.SkillUses[0].Uses))
	require.Equal(t, "eli5", fork.SkillUses[0].Skill)
	require.Equal(t, 1, int(fork.SkillUses[1].Uses))
	require.Equal(t, 2, int(fork.SkillSessions))
}

func TestMergeStatsAggregatesSkillUses(t *testing.T) {
	t.Parallel()

	projectStats := []ProjectStats{
		{Stats: &Stats{Fork: &ForkStats{
			SkillSessions: 3,
			SkillUses:     []SkillUsage{{Skill: "eli5", Uses: 4}},
		}}},
		{Stats: &Stats{Fork: &ForkStats{
			SkillSessions: 1,
			SkillUses:     []SkillUsage{{Skill: "eli5", Uses: 2}, {Skill: "pair-with-bk", Uses: 1}},
		}}},
	}

	merged := mergeStats(projectStats)
	require.Len(t, merged.Fork.SkillUses, 2)
	require.Equal(t, int64(6), merged.Fork.SkillUses[0].Uses)
	require.Equal(t, "eli5", merged.Fork.SkillUses[0].Skill)
	// Session counts across projects do not add; the largest is kept.
	require.Equal(t, int64(3), merged.Fork.SkillSessions)
}
