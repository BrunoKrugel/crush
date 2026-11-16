package model

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// openSnapshotsDialog lists the session's filesystem snapshots: restore
// one with enter, copy its diff with "d".
func (m *UI) openSnapshotsDialog() tea.Cmd {
	sessionID := m.currentSessionID()
	if sessionID == "" {
		return util.ReportWarn("There is no session to list snapshots for yet.")
	}
	if m.dialog.ContainsDialog(dialog.SnapshotsID) {
		m.dialog.BringToFront(dialog.SnapshotsID)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	snapshots, err := m.com.Workspace.ListSnapshots(ctx, sessionID)
	if err != nil {
		return util.ReportError(err)
	}
	if len(snapshots) == 0 {
		return util.ReportWarn("No snapshots yet: they are captured as you send prompts.")
	}
	dlg, err := dialog.NewSnapshots(m.com, sessionID, snapshots)
	if err != nil {
		return util.ReportError(err)
	}
	m.dialog.OpenDialogWithGrace(dlg)
	return nil
}

// restoreSnapshot restores a snapshot off the UI loop, capturing the
// current tree first so the restore can be undone.
func (m *UI) restoreSnapshot(sessionID, commitHash string) tea.Cmd {
	ws := m.com.Workspace
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := ws.RestoreSnapshot(ctx, sessionID, commitHash); err != nil {
			return snapshotRestoredMsg{err: err}
		}
		return snapshotRestoredMsg{hash: commitHash}
	}
}

// snapshotRestoredMsg carries the outcome of a snapshot restore.
type snapshotRestoredMsg struct {
	hash string
	err  error
}

// showSnapshotDiff copies a snapshot's diff to the clipboard off the UI
// loop: diffs are long, and the transcript is a poor place to paste one.
func (m *UI) showSnapshotDiff(sessionID, commitHash string) tea.Cmd {
	ws := m.com.Workspace
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		diff, err := ws.DiffSnapshot(ctx, sessionID, commitHash)
		if err != nil {
			return snapshotDiffMsg{err: err}
		}
		if diff == "" {
			return snapshotDiffMsg{hash: commitHash}
		}
		return snapshotDiffMsg{hash: commitHash, diff: diff}
	}
}

// snapshotDiffMsg carries the outcome of a snapshot diff.
type snapshotDiffMsg struct {
	hash string
	diff string
	err  error
}

// applySnapshotRestored reports the restore outcome.
func (m *UI) applySnapshotRestored(msg snapshotRestoredMsg) tea.Cmd {
	if msg.err != nil {
		return util.ReportError(fmt.Errorf("could not restore the snapshot: %w", msg.err))
	}
	return util.CmdHandler(util.NewInfoMsg(
		fmt.Sprintf("Snapshot %s restored. The state before the restore was captured, so restoring it undoes this.", shortHash(msg.hash)),
	))
}

// applySnapshotDiff reports the diff outcome, copying the patch to the
// clipboard.
func (m *UI) applySnapshotDiff(msg snapshotDiffMsg) tea.Cmd {
	if msg.err != nil {
		return util.ReportError(fmt.Errorf("could not diff the snapshot: %w", msg.err))
	}
	if msg.diff == "" {
		return util.CmdHandler(util.NewInfoMsg("Snapshot has no changes against the previous one."))
	}
	return common.CopyToClipboard(msg.diff, fmt.Sprintf("Diff of snapshot %s copied to clipboard", shortHash(msg.hash)))
}

// shortHash shortens a commit hash for display.
func shortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}
