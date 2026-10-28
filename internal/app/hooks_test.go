package app

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// newHookTestDB opens a fresh session database in a temp dir.
func newHookTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Connect(t.Context(), filepath.Join(t.TempDir(), "data"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// recordingHookRunner builds a hook runner whose hooks append the fired
// event name (plus the source/reason field when present) to a log file.
func recordingHookRunner(t *testing.T, events ...string) (*hooks.Runner, string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "hooks.log")
	record := config.HookConfig{
		Command: `echo fired >> ` + logPath,
	}
	cfg := make(map[string][]config.HookConfig, len(events))
	for _, ev := range events {
		cfg[ev] = []config.HookConfig{record}
	}
	return hooks.NewRunner(hooks.Static(cfg), dir, dir), logPath
}

func readLog(t *testing.T, path string) []string {
	t.Helper()
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func TestSessionCreatedFiresSessionStart(t *testing.T) {
	t.Parallel()

	runner, logPath := recordingHookRunner(t, hooks.EventSessionStart)
	app := NewForTest(t.Context())
	app.hookRunner = runner
	t.Cleanup(app.ShutdownForTest)
	conn := newHookTestDB(t)
	app.Sessions = session.NewService(db.New(conn), conn)

	sess, err := app.Sessions.Create(t.Context(), "hook me")
	require.NoError(t, err)
	app.SessionCreated(t.Context(), sess)

	require.Eventually(t, func() bool {
		return len(readLog(t, logPath)) == 1
	}, 5*time.Second, 10*time.Millisecond)
}

func TestSessionResumedFiresSessionStartWithResumeSource(t *testing.T) {
	t.Parallel()

	runner, logPath := recordingHookRunner(t, hooks.EventSessionStart)
	app := NewForTest(t.Context())
	app.hookRunner = runner
	t.Cleanup(app.ShutdownForTest)
	conn := newHookTestDB(t)
	app.Sessions = session.NewService(db.New(conn), conn)

	sess, err := app.Sessions.Create(t.Context(), "resume me")
	require.NoError(t, err)

	app.SessionResumed(t.Context(), sess)

	require.Eventually(t, func() bool {
		return len(readLog(t, logPath)) == 1
	}, 5*time.Second, 10*time.Millisecond)
}
