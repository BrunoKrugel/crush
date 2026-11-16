// Package fork branches a conversation from any message: a new session
// is created that shares the source's history up to the fork point, with
// the filesystem state that the snapshot engine captured at (or captured
// on demand for) that message. The original session is untouched.
package fork

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/snapshot"
)

// ForkOptions tunes one fork.
type ForkOptions struct {
	// Title names the forked session; empty derives one from the source.
	Title string
	// RestoreFiles writes the fork point's snapshot into the current
	// working directory.
	RestoreFiles bool
	// Worktree creates a fresh git worktree under .crush/forks and
	// materializes the fork point's files there instead.
	Worktree bool
}

// Result carries the forked session plus where its files live.
type Result struct {
	Session session.Session
	// Worktree is the absolute path of the fresh worktree when
	// ForkOptions.Worktree was requested; empty otherwise.
	Worktree string
}

// Service forks conversations.
type Service struct {
	sessions    session.Service
	messages    message.Service
	queries     db.Querier
	snapshots   *snapshot.Service
	projectRoot string
}

// New builds a fork service. A nil snapshots service is tolerated:
// forking still works, the fork point is just not captured and no
// filesystem state is restored.
func New(
	sessions session.Service,
	messages message.Service,
	queries db.Querier,
	snapshots *snapshot.Service,
	projectRoot string,
) *Service {
	return &Service{
		sessions:    sessions,
		messages:    messages,
		queries:     queries,
		snapshots:   snapshots,
		projectRoot: projectRoot,
	}
}

// Fork creates a new session sharing the source's history up to and
// including messageID. Hidden and steer messages never travel to a fork:
// they are harness scaffolding, not conversation.
func (s *Service) Fork(ctx context.Context, sourceSessionID, messageID string, opts ForkOptions) (Result, error) {
	src, err := s.sessions.Get(ctx, sourceSessionID)
	if err != nil {
		return Result{}, fmt.Errorf("failed to read source session: %w", err)
	}

	history, err := s.messages.List(ctx, sourceSessionID)
	if err != nil {
		return Result{}, fmt.Errorf("failed to read source history: %w", err)
	}

	cut := -1
	for i, msg := range history {
		if msg.ID == messageID {
			cut = i
			break
		}
	}
	if cut == -1 {
		return Result{}, fmt.Errorf("message %s does not belong to session %s", messageID, sourceSessionID)
	}

	title := opts.Title
	if title == "" {
		title = src.Title + " (fork)"
	}

	forked, err := s.sessions.Create(ctx, title)
	if err != nil {
		return Result{}, fmt.Errorf("failed to create forked session: %w", err)
	}

	var snapshotHash string
	if s.snapshots != nil {
		snapshotHash, err = s.ensureSnapshot(ctx, sourceSessionID, messageID)
		if err != nil {
			return Result{}, fmt.Errorf("failed to capture the fork point: %w", err)
		}
	}

	var worktreePath string
	if opts.Worktree {
		if s.snapshots == nil {
			return Result{}, fmt.Errorf("a worktree fork needs the snapshot engine, which is not available")
		}
		worktreePath, err = s.createWorktree(forked.ID)
		if err != nil {
			return Result{}, fmt.Errorf("failed to create the fork worktree: %w", err)
		}
		if err := s.snapshots.Repo.RestoreTo(snapshotHash, worktreePath); err != nil {
			return Result{}, fmt.Errorf("failed to materialize the fork point in the worktree: %w", err)
		}
	} else if opts.RestoreFiles {
		if s.snapshots == nil {
			return Result{}, fmt.Errorf("restoring files needs the snapshot engine, which is not available")
		}
		if err := s.snapshots.Restore(snapshotHash); err != nil {
			return Result{}, fmt.Errorf("failed to restore the fork point: %w", err)
		}
	}

	for _, msg := range history[:cut+1] {
		if isScaffolding(msg) {
			continue
		}
		// The copy gets its own id: messages.id is the table's primary
		// key, and the source session keeps its own copy.
		if err := s.queries.ForkMessage(ctx, db.ForkMessageParams{
			NewID:     uuid.NewString(),
			SessionID: forked.ID,
			SourceID:  msg.ID,
		}); err != nil {
			return Result{}, fmt.Errorf("failed to copy message %s into the fork: %w", msg.ID, err)
		}
	}

	if err := s.queries.SetSessionForkOrigin(ctx, db.SetSessionForkOriginParams{
		ForkedFromSessionID:  sql.NullString{String: sourceSessionID, Valid: sourceSessionID != ""},
		ForkedFromSnapshotID: sql.NullString{String: snapshotHash, Valid: snapshotHash != ""},
		ForkedFromMessageID:  sql.NullString{String: messageID, Valid: messageID != ""},
		ID:                   forked.ID,
	}); err != nil {
		return Result{}, fmt.Errorf("failed to record the fork origin: %w", err)
	}

	final, err := s.sessions.Get(ctx, forked.ID)
	if err != nil {
		return Result{}, fmt.Errorf("failed to reload the forked session: %w", err)
	}
	return Result{Session: final, Worktree: worktreePath}, nil
}

// ensureSnapshot returns the snapshot commit for the fork point,
// capturing one on demand when the message does not have one yet.
func (s *Service) ensureSnapshot(ctx context.Context, sourceSessionID, messageID string) (string, error) {
	info, err := s.snapshots.Store.GetSnapshotByMessageID(sourceSessionID, messageID)
	if err == nil {
		return info.CommitHash, nil
	}
	info, err = s.snapshots.SnapshotForMessage(sourceSessionID, messageID, "fork point")
	if err != nil {
		return "", err
	}
	return info.CommitHash, nil
}

// createWorktree registers a fresh detached git worktree for the forked
// session under .crush/forks, checked out at the project's HEAD.
func (s *Service) createWorktree(sessionID string) (string, error) {
	name := "fork-" + shortID(sessionID)
	forksDir := filepath.Join(s.projectRoot, ".crush", "forks")
	path := filepath.Join(forksDir, name)
	if err := os.MkdirAll(forksDir, 0o755); err != nil {
		return "", fmt.Errorf("create forks directory: %w", err)
	}
	// The worktrees live inside the project, so they must not show up as
	// untracked files in the user's repository.
	if err := ignoreDir(forksDir); err != nil {
		return "", fmt.Errorf("keep the forks directory out of git: %w", err)
	}
	cmd := exec.Command("git", "-C", s.projectRoot, "worktree", "add", "--detach", path, "HEAD")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return path, nil
}

// ignoreDir writes a "*" .gitignore beside dir — one level up, so the
// directory itself stays visible to git while its contents do not.
func ignoreDir(dir string) error {
	ignorePath := filepath.Join(filepath.Dir(dir), ".gitignore")
	if _, err := os.Stat(ignorePath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(ignorePath, []byte("*\n"), 0o644)
}

// shortID trims a session id to a filesystem-friendly prefix.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// isScaffolding reports whether a message exists for the harness rather
// than the conversation: hidden prompts and steering injections.
func isScaffolding(msg message.Message) bool {
	c := msg.Content()
	return c.Hidden || c.Steer
}
