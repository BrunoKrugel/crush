package snapshot

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type fileMode struct {
	dir bool
}

var (
	blobMode = fileMode{dir: false}
	treeMode = fileMode{dir: true}
)

func (m fileMode) plumbing() filemode.FileMode {
	if m.dir {
		return filemode.Dir
	}
	return filemode.Regular
}

// ignoreList holds the parsed patterns of the project's root .gitignore.
type ignoreList struct {
	patterns []ignorePattern
}

type ignorePattern struct {
	glob     string
	isDir    bool
	isPrefix bool
	negate   bool
}

func loadGitIgnore(projectRoot string) (*ignoreList, error) {
	f, err := os.Open(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		if os.IsNotExist(err) {
			return &ignoreList{}, nil
		}
		return nil, err
	}
	defer f.Close()

	list := &ignoreList{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pattern := ignorePattern{}
		if strings.HasPrefix(line, "!") {
			pattern.negate = true
			line = strings.TrimPrefix(line, "!")
		}
		if strings.HasSuffix(line, "/") {
			pattern.isDir = true
			line = strings.TrimSuffix(line, "/")
		}
		if strings.HasPrefix(line, "/") {
			pattern.isPrefix = true
			line = strings.TrimPrefix(line, "/")
		}
		if line == "" {
			continue
		}
		pattern.glob = line
		list.patterns = append(list.patterns, pattern)
	}
	return list, scanner.Err()
}

// matchesDir reports whether a slash-separated relative directory is ignored.
func (l *ignoreList) matchesDir(rel string) bool {
	return l.match(rel, true)
}

// matchesFile reports whether a slash-separated relative file is ignored.
func (l *ignoreList) matchesFile(rel string) bool {
	return l.match(rel, false)
}

func (l *ignoreList) match(rel string, isDir bool) bool {
	ignored := false
	for _, p := range l.patterns {
		if matched := p.matches(rel, isDir); matched {
			ignored = !p.negate
		}
	}
	return ignored
}

func (p ignorePattern) matches(rel string, isDir bool) bool {
	if p.isDir && !isDir {
		if !hasDirPrefix(rel, p.glob) {
			return false
		}
		return true
	}
	if hasDirPrefix(rel, p.glob) {
		return true
	}
	if ok, err := filepath.Match(p.glob, rel); err == nil && ok {
		return true
	}
	if ok, err := filepath.Match(p.glob, filepath.Base(rel)); err == nil && ok {
		return true
	}
	if strings.Contains(p.glob, "/") && !p.isPrefix {
		if ok, err := filepath.Match("*/"+p.glob, rel); err == nil && ok {
			return true
		}
	}
	return false
}

func hasDirPrefix(rel, dir string) bool {
	return rel == dir || strings.HasPrefix(rel, dir+"/")
}

// unused guards against go-git import pruning if helpers change.
var (
	_ = object.File{}
	_ = plumbing.ZeroHash
)
