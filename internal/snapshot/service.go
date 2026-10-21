package snapshot

import (
	"fmt"
	"sort"
	"sync"
)

// Store persists snapshot metadata. The production implementation lives in
// the db package; tests use MemoryStore.
type Store interface {
	CreateSnapshot(info SnapshotInfo) error
	GetSnapshotByMessageID(sessionID, messageID string) (SnapshotInfo, error)
	GetLatestSnapshotBySession(sessionID string) (SnapshotInfo, error)
	ListSnapshotsBySession(sessionID string) ([]SnapshotInfo, error)
}

// Service ties the git object store to snapshot metadata storage.
type Service struct {
	Repo  *Repo
	Store Store
}

// New opens the repository and returns a service without a store; callers
// must set Store before calling its methods.
func New(opts Options) (*Service, error) {
	repo, err := Open(opts)
	if err != nil {
		return nil, err
	}
	return &Service{Repo: repo}, nil
}

// SnapshotForMessage captures the working tree and records its metadata.
func (s *Service) SnapshotForMessage(sessionID, messageID, description string) (SnapshotInfo, error) {
	hash, err := s.Repo.Snapshot(sessionID, messageID, description)
	if err != nil {
		return SnapshotInfo{}, fmt.Errorf("failed to snapshot working tree: %w", err)
	}
	info := SnapshotInfo{
		SessionID:   sessionID,
		MessageID:   messageID,
		CommitHash:  hash,
		Description: description,
	}
	if err := s.Store.CreateSnapshot(info); err != nil {
		return SnapshotInfo{}, fmt.Errorf("failed to record snapshot: %w", err)
	}
	return info, nil
}

// ListForSession returns the session's snapshots newest first.
func (s *Service) ListForSession(sessionID string) ([]SnapshotInfo, error) {
	return s.Store.ListSnapshotsBySession(sessionID)
}

// Restore writes a snapshot's files back into the working tree.
func (s *Service) Restore(commitHash string) error {
	return s.Repo.Restore(commitHash)
}

// DiffForMessage returns the patch of a snapshot's message compared to the
// most recent earlier snapshot with a different commit.
func (s *Service) DiffForMessage(info SnapshotInfo) (string, error) {
	list, err := s.Store.ListSnapshotsBySession(info.SessionID)
	if err != nil {
		return "", err
	}
	for _, prev := range list {
		if prev.CommitHash != info.CommitHash {
			return s.Repo.Diff(prev.CommitHash, info.CommitHash)
		}
	}
	return "", nil
}

// MemoryStore is an in-memory Store for tests.
type MemoryStore struct {
	mu        sync.Mutex
	snapshots []SnapshotInfo
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// CreateSnapshot appends a snapshot record.
func (m *MemoryStore) CreateSnapshot(info SnapshotInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots = append(m.snapshots, info)
	return nil
}

// GetSnapshotByMessageID finds a snapshot by session and message.
func (m *MemoryStore) GetSnapshotByMessageID(sessionID, messageID string) (SnapshotInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.snapshots {
		if s.SessionID == sessionID && s.MessageID == messageID {
			return s, nil
		}
	}
	return SnapshotInfo{}, fmt.Errorf("snapshot not found for message %s", messageID)
}

// GetLatestSnapshotBySession returns the newest snapshot of a session.
func (m *MemoryStore) GetLatestSnapshotBySession(sessionID string) (SnapshotInfo, error) {
	list, err := m.ListSnapshotsBySession(sessionID)
	if err != nil {
		return SnapshotInfo{}, err
	}
	if len(list) == 0 {
		return SnapshotInfo{}, fmt.Errorf("no snapshots for session %s", sessionID)
	}
	return list[0], nil
}

// ListSnapshotsBySession returns the snapshots newest first.
func (m *MemoryStore) ListSnapshotsBySession(sessionID string) ([]SnapshotInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []SnapshotInfo{}
	for _, s := range m.snapshots {
		if s.SessionID == sessionID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MessageID > out[j].MessageID })
	return out, nil
}
