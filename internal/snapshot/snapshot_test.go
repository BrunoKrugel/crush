package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/stretchr/testify/require"
)

func newProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "internal", "a.txt"), []byte("alpha\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0o644))
	_, err = wt.Add(".")
	require.NoError(t, err)
	_, err = wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)
	return dir
}

func TestIsSupported(t *testing.T) {
	require.False(t, IsSupported(os.Getenv("HOME")))
	dir := t.TempDir()
	require.False(t, IsSupported(dir))
}

func TestSnapshotAndList(t *testing.T) {
	dir := newProject(t)
	r, err := Open(Options{ProjectRoot: dir})
	require.NoError(t, err)

	h1, err := r.Snapshot("s1", "m1", "first")
	require.NoError(t, err)
	require.NotEmpty(t, h1)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n// changed\n"), 0o644))
	h2, err := r.Snapshot("s1", "m2", "second")
	require.NoError(t, err)
	require.NotEqual(t, h1, h2)

	list, err := r.List("s1")
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "m2", list[0].MessageID)
	require.Equal(t, "m1", list[1].MessageID)
	require.Equal(t, "second", list[0].Description)
	require.Equal(t, h2, list[0].CommitHash)
	require.False(t, list[0].CreatedAt.IsZero())

	store := openStoreRepo(t, dir)
	_, err = store.Reference(plumbing.ReferenceName("refs/snapshots/s1/m1"), true)
	require.NoError(t, err)
}

func TestRestoreOverwritesTrackedFile(t *testing.T) {
	dir := newProject(t)
	r, err := Open(Options{ProjectRoot: dir})
	require.NoError(t, err)
	h1, err := r.Snapshot("s1", "m1", "first")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("dirty\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("keep me\n"), 0o644))

	require.NoError(t, r.Restore(h1))

	got, err := os.ReadFile(filepath.Join(dir, "main.go"))
	require.NoError(t, err)
	require.Equal(t, "package main\n", string(got))
	got, err = os.ReadFile(filepath.Join(dir, "untracked.txt"))
	require.NoError(t, err)
	require.Equal(t, "keep me\n", string(got))
}

func TestExclusion(t *testing.T) {
	dir := newProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("nope\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secret.key"), []byte("nope\n"), 0o644))
	r, err := Open(Options{ProjectRoot: dir, Exclude: []string{"*.key"}})
	require.NoError(t, err)
	_, err = r.Snapshot("s1", "m1", "first")
	require.NoError(t, err)

	files := snapshotFiles(t, r, "s1", "m1")
	require.NotContains(t, files, "ignored.txt")
	require.NotContains(t, files, "secret.key")
	require.Contains(t, files, "main.go")
	for f := range files {
		require.False(t, strings.HasPrefix(f, ".crush"+string(filepath.Separator)), f)
	}
}

func TestUnchangedFileNotRewritten(t *testing.T) {
	dir := newProject(t)
	r, err := Open(Options{ProjectRoot: dir})
	require.NoError(t, err)
	_, err = r.Snapshot("s1", "m1", "first")
	require.NoError(t, err)

	storePath := filepath.Join(dir, ".crush", "git", "objects")
	countBefore := countFiles(t, storePath)
	_, err = r.Snapshot("s1", "m2", "second")
	require.NoError(t, err)
	countAfter := countFiles(t, storePath)
	require.Equal(t, countBefore, countAfter, "an unchanged tree must reuse the previous commit object")
}

func TestDiff(t *testing.T) {
	dir := newProject(t)
	r, err := Open(Options{ProjectRoot: dir})
	require.NoError(t, err)
	h1, err := r.Snapshot("s1", "m1", "first")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n// v2\n"), 0o644))
	h2, err := r.Snapshot("s1", "m2", "second")
	require.NoError(t, err)

	diff, err := r.Diff(h1, h2)
	require.NoError(t, err)
	require.Contains(t, diff, "main.go")
	require.Contains(t, diff, "+// v2")
}

func TestPruneKeep(t *testing.T) {
	dir := newProject(t)
	r, err := Open(Options{ProjectRoot: dir})
	require.NoError(t, err)
	for i, id := range []string{"m1", "m2", "m3"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("v"+strings.Repeat("x", i+1)+"\n"), 0o644))
		_, err := r.Snapshot("s1", id, "desc "+id)
		require.NoError(t, err)
	}

	require.NoError(t, r.PruneKeep("s1", 1))
	list, err := r.List("s1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "m3", list[0].MessageID)
}

func TestService(t *testing.T) {
	dir := newProject(t)
	svc, err := New(Options{ProjectRoot: dir})
	require.NoError(t, err)
	store := NewMemoryStore()
	svc.Store = store

	_, err = svc.SnapshotForMessage("s1", "m1", "hello")
	require.NoError(t, err)
	_, err = svc.SnapshotForMessage("s1", "m2", "world")
	require.NoError(t, err)

	got, err := store.GetSnapshotByMessageID("s1", "m1")
	require.NoError(t, err)
	require.Equal(t, "hello", got.Description)

	list, err := store.ListSnapshotsBySession("s1")
	require.NoError(t, err)
	require.Len(t, list, 2)

	latest, err := store.GetLatestSnapshotBySession("s1")
	require.NoError(t, err)
	require.Equal(t, "m2", latest.MessageID)

	require.NoError(t, svc.Restore(latest.CommitHash))
}

func openStoreRepo(t *testing.T, projectRoot string) *git.Repository {
	t.Helper()
	fs := osfs.New(filepath.Join(projectRoot, ".crush", "git"))
	st := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
	repo, err := git.Open(st, osfs.New(projectRoot))
	require.NoError(t, err)
	return repo
}

func snapshotFiles(t *testing.T, r *Repo, sessionID, messageID string) map[string]bool {
	t.Helper()
	repo := openStoreRepo(t, r.projectRoot)
	ref, err := repo.Reference(plumbing.ReferenceName("refs/snapshots/"+sessionID+"/"+messageID), true)
	require.NoError(t, err)
	commit, err := repo.CommitObject(ref.Hash())
	require.NoError(t, err)
	tree, err := commit.Tree()
	require.NoError(t, err)
	files := map[string]bool{}
	require.NoError(t, tree.Files().ForEach(func(f *object.File) error {
		files[f.Name] = true
		return nil
	}))
	return files
}

func countFiles(t *testing.T, root string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	require.NoError(t, err)
	return n
}
