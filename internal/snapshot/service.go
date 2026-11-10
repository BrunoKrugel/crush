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
	// DeleteSnapshot removes one snapshot's metadata row.
	DeleteSnapshot(sessionID, messageID string) error
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

// Prune keeps only the newest keep snapshots of a session: the git refs
// and the metadata rows of older snapshots go, while content-addressed
// objects stay until the store reclaims them.
func (s *Service) Prune(sessionID string, keep int) error {
	if keep <= 0 {
		return nil
	}
	if err := s.Repo.PruneKeep(sessionID, keep); err != nil {
		return err
	}
	infos, err := s.Store.ListSnapshotsBySession(sessionID)
	if err != nil {
		return err
	}
	for i := keep; i < len(infos); i++ {
		if err := s.Store.DeleteSnapshot(sessionID, infos[i].MessageID); err != nil {
			return fmt.Errorf("failed to delete snapshot metadata: %w", err)
		}
	}
	return nil
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

// DeleteSnapshot removes one snapshot's metadata record.
func (m *MemoryStore) DeleteSnapshot(sessionID, messageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := m.snapshots[:0]
	for _, s := range m.snapshots {
		if s.SessionID == sessionID && s.MessageID == messageID {
			continue
		}
		kept = append(kept, s)
	}
	m.snapshots = kept
	return nil
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
