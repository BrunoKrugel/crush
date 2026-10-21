package snapshot

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const storeDir = ".crush/git"

// Options configures a snapshot repository.
type Options struct {
	// ProjectRoot is the working directory the snapshots capture.
	ProjectRoot string
	// Exclude holds glob or prefix patterns of paths to skip.
	Exclude []string
}

// SnapshotInfo describes one stored snapshot.
type SnapshotInfo struct {
	SessionID   string    `json:"session_id"`
	MessageID   string    `json:"message_id"`
	CommitHash  string    `json:"commit_hash"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Repo manages the private git object store under <projectRoot>/.crush/git.
type Repo struct {
	projectRoot string
	storer      storage.Storer
	exclude     []string
	lastTree    map[string]plumbing.Hash
	lastWhen    map[string]time.Time
}

func refName(sessionID, messageID string) plumbing.ReferenceName {
	return plumbing.ReferenceName("refs/snapshots/" + sessionID + "/" + messageID)
}

// IsSupported reports whether snapshots make sense for the project root:
// it must be inside a git repository and must not be the home directory.
func IsSupported(projectRoot string) bool {
	root := filepath.Clean(projectRoot)
	if root == filepath.Clean(os.Getenv("HOME")) {
		return false
	}
	dir := root
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// Open opens or initializes the private snapshot store for the project.
func Open(opts Options) (*Repo, error) {
	if !IsSupported(opts.ProjectRoot) {
		return nil, fmt.Errorf("snapshots are not supported for %s", opts.ProjectRoot)
	}
	storePath := filepath.Join(opts.ProjectRoot, storeDir)
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create snapshot store dir: %w", err)
	}
	headPath := filepath.Join(storePath, "HEAD")
	if _, err := os.Stat(headPath); os.IsNotExist(err) {
		if err := os.WriteFile(headPath, []byte("ref: refs/heads/snapshots\n"), 0o644); err != nil {
			return nil, fmt.Errorf("failed to initialize snapshot store: %w", err)
		}
	}
	fs := osfs.New(storePath)
	st := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
	return &Repo{
		projectRoot: opts.ProjectRoot,
		storer:      st,
		exclude:     append([]string{".crush"}, opts.Exclude...),
		lastTree:    map[string]plumbing.Hash{},
		lastWhen:    map[string]time.Time{},
	}, nil
}

// Snapshot captures the current working tree and pins it to the session
// and message. When nothing changed since the previous snapshot of the
// session, the previous commit is reused and no new objects are written.
func (r *Repo) Snapshot(sessionID, messageID, description string) (string, error) {
	ignore, err := loadGitIgnore(r.projectRoot)
	if err != nil {
		return "", err
	}
	blobHashes, err := r.walkTree(ignore)
	if err != nil {
		return "", err
	}
	treeHash, err := r.writeTrees(blobHashes)
	if err != nil {
		return "", err
	}
	if prev, ok := r.lastTree[sessionID]; ok && prev == treeHash {
		return r.lastCommit(sessionID)
	}
	commitHash, err := r.writeCommit(sessionID, messageID, description, treeHash)
	if err != nil {
		return "", err
	}
	if err := r.storer.SetReference(plumbing.NewHashReference(refName(sessionID, messageID), commitHash)); err != nil {
		return "", fmt.Errorf("failed to set snapshot ref: %w", err)
	}
	branch := plumbing.ReferenceName("refs/heads/snapshots")
	if _, err := r.storer.Reference(branch); err != nil {
		if err := r.storer.SetReference(plumbing.NewHashReference(branch, commitHash)); err != nil {
			return "", fmt.Errorf("failed to set snapshot branch: %w", err)
		}
	}
	r.lastTree[sessionID] = treeHash
	return commitHash.String(), nil
}

// List returns the session's snapshots newest first.
func (r *Repo) List(sessionID string) ([]SnapshotInfo, error) {
	refs, err := r.storer.IterReferences()
	if err != nil {
		return nil, fmt.Errorf("failed to iterate snapshot refs: %w", err)
	}
	defer refs.Close()

	prefix := "refs/snapshots/" + sessionID + "/"
	var out []SnapshotInfo
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().String()
		if !strings.HasPrefix(name, prefix) {
			return nil
		}
		commit, err := object.GetCommit(r.storer, ref.Hash())
		if err != nil {
			return fmt.Errorf("failed to read snapshot commit %s: %w", ref.Hash(), err)
		}
		parts := strings.SplitN(strings.TrimPrefix(name, prefix), "/", 2)
		out = append(out, SnapshotInfo{
			SessionID:   sessionID,
			MessageID:   parts[0],
			CommitHash:  ref.Hash().String(),
			Description: commit.Message,
			CreatedAt:   commit.Committer.When,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// Restore writes the snapshot's tracked files back into the worktree.
// Files that were never part of the snapshot are left untouched.
func (r *Repo) Restore(commitHash string) error {
	commit, err := object.GetCommit(r.storer, plumbing.NewHash(commitHash))
	if err != nil {
		return fmt.Errorf("failed to read snapshot commit: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("failed to read snapshot tree: %w", err)
	}
	return tree.Files().ForEach(func(f *object.File) error {
		target := filepath.Join(r.projectRoot, f.Name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		reader, err := f.Reader()
		if err != nil {
			return err
		}
		defer reader.Close()
		writer, err := os.Create(target)
		if err != nil {
			return err
		}
		defer writer.Close()
		_, err = io.Copy(writer, reader)
		return err
	})
}

// Diff returns the textual patch between two snapshot commits.
func (r *Repo) Diff(from, to string) (string, error) {
	fromCommit, err := object.GetCommit(r.storer, plumbing.NewHash(from))
	if err != nil {
		return "", fmt.Errorf("failed to read from commit: %w", err)
	}
	toCommit, err := object.GetCommit(r.storer, plumbing.NewHash(to))
	if err != nil {
		return "", fmt.Errorf("failed to read to commit: %w", err)
	}
	fromTree, err := fromCommit.Tree()
	if err != nil {
		return "", err
	}
	toTree, err := toCommit.Tree()
	if err != nil {
		return "", err
	}
	patch, err := fromTree.Patch(toTree)
	if err != nil {
		return "", err
	}
	return patch.String(), nil
}

// PruneKeep deletes the session's oldest snapshot refs, keeping the newest
// keep entries.
func (r *Repo) PruneKeep(sessionID string, keep int) error {
	infos, err := r.List(sessionID)
	if err != nil {
		return err
	}
	for i := keep; i < len(infos); i++ {
		name := refName(sessionID, infos[i].MessageID)
		if err := r.storer.RemoveReference(name); err != nil {
			return fmt.Errorf("failed to remove snapshot ref %s: %w", name, err)
		}
	}
	return nil
}

func (r *Repo) lastCommit(sessionID string) (string, error) {
	infos, err := r.List(sessionID)
	if err != nil {
		return "", err
	}
	if len(infos) == 0 {
		return "", fmt.Errorf("no previous snapshot for session %s", sessionID)
	}
	return infos[0].CommitHash, nil
}

func (r *Repo) writeCommit(sessionID, messageID, description string, treeHash plumbing.Hash) (plumbing.Hash, error) {
	when := time.Now().Truncate(time.Second)
	if prev, ok := r.lastWhen[sessionID]; ok && !when.After(prev) {
		when = prev.Add(time.Second)
	}
	r.lastWhen[sessionID] = when
	sig := object.Signature{Name: "Crush", Email: "crush@localhost", When: when}
	commit := &object.Commit{
		Author:    sig,
		Committer: sig,
		Message:   description,
		TreeHash:  treeHash,
	}
	obj := r.storer.NewEncodedObject()
	obj.SetType(plumbing.CommitObject)
	if err := commit.Encode(obj); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to encode snapshot commit: %w", err)
	}
	hash, err := r.storer.SetEncodedObject(obj)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to store snapshot commit: %w", err)
	}
	return hash, nil
}

// writeTrees builds the tree hierarchy from a flat relpath-to-blob map and
// stores it, returning the root tree hash. It writes nothing for entries
// whose objects already exist in the content-addressed store.
func (r *Repo) writeTrees(blobs map[string]plumbing.Hash) (plumbing.Hash, error) {
	type entry struct {
		name string
		hash plumbing.Hash
		mode fileMode
	}
	dirName := func(rel string) string {
		if i := strings.LastIndex(rel, "/"); i >= 0 {
			return rel[:i]
		}
		return ""
	}
	baseName := func(rel string) string {
		if i := strings.LastIndex(rel, "/"); i >= 0 {
			return rel[i+1:]
		}
		return rel
	}

	byDir := map[string][]entry{}
	for rel, hash := range blobs {
		byDir[dirName(rel)] = append(byDir[dirName(rel)], entry{baseName(rel), hash, blobMode})
	}

	var build func(dir string) (plumbing.Hash, error)
	build = func(dir string) (plumbing.Hash, error) {
		for childDir := range byDir {
			if dirName(childDir) == dir && childDir != "" {
				childHash, err := build(childDir)
				if err != nil {
					return plumbing.ZeroHash, err
				}
				byDir[dir] = append(byDir[dir], entry{baseName(childDir), childHash, treeMode})
			}
		}
		entries := byDir[dir]
		sort.Slice(entries, func(i, j int) bool {
			// Git sorts tree entries as if directory names had a trailing slash.
			ni, nj := entries[i].name, entries[j].name
			if entries[i].mode == treeMode {
				ni += "/"
			}
			if entries[j].mode == treeMode {
				nj += "/"
			}
			return ni < nj
		})
		tree := &object.Tree{}
		for _, e := range entries {
			tree.Entries = append(tree.Entries, object.TreeEntry{
				Name: e.name,
				Mode: e.mode.plumbing(),
				Hash: e.hash,
			})
		}
		obj := r.storer.NewEncodedObject()
		obj.SetType(plumbing.TreeObject)
		if err := tree.Encode(obj); err != nil {
			return plumbing.ZeroHash, fmt.Errorf("failed to encode snapshot tree: %w", err)
		}
		hash, err := r.storer.SetEncodedObject(obj)
		if err != nil {
			return plumbing.ZeroHash, fmt.Errorf("failed to store snapshot tree: %w", err)
		}
		return hash, nil
	}
	return build("")
}

func (r *Repo) walkTree(ignore *ignoreList) (map[string]plumbing.Hash, error) {
	blobs := map[string]plumbing.Hash{}
	err := filepath.WalkDir(r.projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(r.projectRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if rel == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if d.IsDir() {
			if r.excluded(relSlash) || ignore.matchesDir(relSlash) {
				return filepath.SkipDir
			}
			return nil
		}
		if r.excluded(relSlash) || ignore.matchesFile(relSlash) {
			return nil
		}
		hash, err := r.storeBlob(path)
		if err != nil {
			return err
		}
		blobs[relSlash] = hash
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk project for snapshot: %w", err)
	}
	return blobs, nil
}

func (r *Repo) excluded(rel string) bool {
	for _, pattern := range r.exclude {
		pattern = strings.TrimSuffix(pattern, "/")
		if rel == pattern || strings.HasPrefix(rel, pattern+"/") {
			return true
		}
		if ok, err := filepath.Match(pattern, filepath.Base(rel)); err == nil && ok {
			return true
		}
		if ok, err := filepath.Match(pattern, rel); err == nil && ok {
			return true
		}
	}
	return false
}

func (r *Repo) storeBlob(path string) (plumbing.Hash, error) {
	f, err := os.Open(path)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	defer f.Close()
	obj := r.storer.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	writer, err := obj.Writer()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if _, err := io.Copy(writer, f); err != nil {
		writer.Close()
		return plumbing.ZeroHash, err
	}
	if err := writer.Close(); err != nil {
		return plumbing.ZeroHash, err
	}
	hash, err := r.storer.SetEncodedObject(obj)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return hash, nil
}
