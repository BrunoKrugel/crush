package app

import (
	"database/sql"
	"log/slog"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/snapshot"
)

// newSnapshotService builds the filesystem snapshot service, or nil when
// snapshots are disabled or unsupported for the project.
//
// The service is shared by the agent (which captures the working tree at
// every user message) and by conversation forking (which restores the
// files captured at a fork point), so both see the same store.
func newSnapshotService(conn *sql.DB, store *config.ConfigStore) *snapshot.Service {
	if conn == nil {
		return nil
	}
	cfg := store.Config()
	if !cfg.SnapshotsEnabled() || !snapshot.IsSupported(store.WorkingDir()) {
		return nil
	}
	svc, err := snapshot.New(snapshot.Options{
		ProjectRoot: store.WorkingDir(),
		Exclude:     cfg.SnapshotExclusions(),
	})
	if err != nil {
		slog.Warn("Filesystem snapshots disabled", "error", err)
		return nil
	}
	svc.Store = snapshot.NewDBStore(db.New(conn))
	return svc
}
