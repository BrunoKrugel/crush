package session

import (
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/goal"
	"github.com/stretchr/testify/require"
)

// newGoalTestService builds a session service over a temporary database.
func newGoalTestService(t *testing.T) Service {
	t.Helper()
	dataDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, db.Release(dataDir))
		db.ResetPool()
	})
	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	return NewService(db.New(conn), conn)
}

// TestSetGoalRoundTrip pins the goal's persistence: a goal written on a
// session comes back with it, field by field.
func TestSetGoalRoundTrip(t *testing.T) {
	sessions := newGoalTestService(t)
	created, err := sessions.Create(t.Context(), "goal session")
	require.NoError(t, err)

	written := &goal.Goal{
		Text:       "make the tests green",
		State:      goal.StateActive,
		TurnBudget: 7,
		TurnsUsed:  2,
		Verdict:    "the suite still fails",
		Pending:    true,
	}
	saved, err := sessions.SetGoal(t.Context(), created.ID, written)
	require.NoError(t, err)
	require.NotNil(t, saved.Goal)
	require.Equal(t, written.Text, saved.Goal.Text)
	require.Equal(t, goal.StateActive, saved.Goal.State)
	require.Equal(t, 7, saved.Goal.TurnBudget)
	require.Equal(t, 2, saved.Goal.TurnsUsed)
	require.Equal(t, "the suite still fails", saved.Goal.Verdict)
	require.True(t, saved.Goal.Pending)

	fetched, err := sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.Goal)
	require.Equal(t, written.Text, fetched.Goal.Text)
	require.Equal(t, goal.StateActive, fetched.Goal.State)
	require.Equal(t, 7, fetched.Goal.TurnBudget)
	require.Equal(t, 2, fetched.Goal.TurnsUsed)
	require.True(t, fetched.Goal.Pending)
}

// TestClearGoalRemovesTheGoal pins that cancelling a goal leaves the
// session without one.
func TestClearGoalRemovesTheGoal(t *testing.T) {
	sessions := newGoalTestService(t)
	created, err := sessions.Create(t.Context(), "goal session")
	require.NoError(t, err)

	_, err = sessions.SetGoal(t.Context(), created.ID, &goal.Goal{
		Text:       "make the tests green",
		State:      goal.StateActive,
		TurnBudget: 3,
	})
	require.NoError(t, err)

	cleared, err := sessions.ClearGoal(t.Context(), created.ID)
	require.NoError(t, err)
	require.Nil(t, cleared.Goal)

	fetched, err := sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.Nil(t, fetched.Goal)
}

// TestSaveKeepsTheGoal pins that the session's full-row save does not
// drop the goal: the goal has its own writer, and a concurrent save of
// the rest of the row must not clear it.
func TestSaveKeepsTheGoal(t *testing.T) {
	sessions := newGoalTestService(t)
	created, err := sessions.Create(t.Context(), "goal session")
	require.NoError(t, err)

	_, err = sessions.SetGoal(t.Context(), created.ID, &goal.Goal{
		Text:       "make the tests green",
		State:      goal.StateActive,
		TurnBudget: 3,
	})
	require.NoError(t, err)

	created.Title = "renamed"
	_, err = sessions.Save(t.Context(), created)
	require.NoError(t, err)

	fetched, err := sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.Goal, "a full-row save must not clear the goal")
	require.Equal(t, "make the tests green", fetched.Goal.Text)
}
