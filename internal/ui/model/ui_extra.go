package model

import (
	"context"
	"time"
)

// skillCompletionValues and promptCompletionValues name the fork-only
// slash-completion sources this build keeps unpopulated.

// Snapshot surfaces rely on the workspace interface; fork UI helpers keep
// them inert in this build.

func (m *UI) fakeListSnapshots(ctx context.Context, sessionID string) ([]string, error) { return nil, nil }

type fakeSnapshot struct{ hash string }

func (f fakeSnapshot) CommitParent() string { return f.hash }

var _ = func(ctx context.Context, t time.Duration) error { return nil }
