package tools

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const (
	// snippetLines is how many lines of a hit's chunk the card shows before
	// eliding. The card is a scanning aid, not a viewer — the model is told
	// to open the file for authoritative content.
	snippetLines = 6
	// scoreMeterCells is the width of the relevance meter drawn beside each
	// score, so neighboring hits can be compared at a glance.
	scoreMeterCells = 10
	// gutterTabWidth expands tabs in snippets. Terminal tab stops interact
	// badly with the code block's own indentation, and a fixed expansion
	// keeps the line-number gutter aligned.
	gutterTabWidth = 4
	// errorMessageLimit caps a per-file index error. Provider errors nest
	// escaped JSON several layers deep; the leading clause is the part that
	// says what actually went wrong.
	errorMessageLimit = 160
	// defaultSnippetWidth sizes snippet lines when the turn carries no width
	// hint, and minSnippetWidth floors the computed one so a very narrow pane
	// cannot truncate every line down to its gutter.
	defaultSnippetWidth = 100
	minSnippetWidth     = 40
)

type semanticSearchHit struct {
	Path      string
	Symbol    string
	Snippet   string
	StartLine int
	EndLine   int
	Score     float64
}

// semanticSearchDigest renders the compact model-facing result list. It
// deliberately omits snippets when a surface shows them to the user — the
// tool description already steers the model to view for authoritative
// content, and dropping the snippets saves the corresponding tokens.
func semanticSearchDigest(results []semanticSearchHit, withSnippets bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d results:\n\n", len(results))
	for i, r := range results {
		fmt.Fprintf(&b, "%d. %s", i+1, r.Path)
		if r.Symbol != "" {
			fmt.Fprintf(&b, " :: %s", r.Symbol)
		}
		fmt.Fprintf(&b, " (lines %d-%d, score %.3f)\n", r.StartLine+1, r.EndLine+1, r.Score)
		if withSnippets {
			snippet := firstLines(r.Snippet, 5)
			// The trailing blank line is part of the headless format this
			// path must reproduce byte for byte on headless runs.
			fmt.Fprintf(&b, "   %s\n\n", strings.ReplaceAll(snippet, "\n", "\n   "))
		}
	}
	return b.String()
}

// relativizePath renders an indexed path relative to the working directory
// so the card shows repository-shaped paths instead of the absolute ones the
// store holds. Paths outside the working directory (and any path when the
// directory is unknown) are left alone rather than rendered as a pile of
// "../".
func relativizePath(workingDir, path string) string {
	if workingDir == "" || path == "" || !filepath.IsAbs(path) {
		return path
	}
	rel, err := filepath.Rel(workingDir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

// scoreMeter draws a fixed-width bar for a similarity score. Scores are
// 1 - cosine distance, so they nominally land in [0,1]; the bar clamps
// rather than overflowing on an out-of-range value.
func scoreMeter(score float64) string {
	filled := int(math.Round(math.Max(0, math.Min(1, score)) * scoreMeterCells))
	return strings.Repeat("█", filled) + strings.Repeat("░", scoreMeterCells-filled)
}

// numberedSnippet renders at most maxLines lines of a chunk with a right-aligned
// line-number gutter starting at startLine (1-based), marking an elision with
// a gutter-aligned ellipsis. Tabs are expanded and the block's common
// indentation is stripped so a deeply nested chunk spends its width on code
// rather than leading space, and each line is truncated to width so the
// gutter survives (see snippetWidth).
func numberedSnippet(snippet string, startLine, maxLines, width int) string {
	snippet = strings.TrimRight(snippet, "\n")
	if strings.TrimSpace(snippet) == "" {
		return ""
	}
	lines := strings.Split(snippet, "\n")
	elided := false
	if len(lines) > maxLines {
		lines, elided = lines[:maxLines], true
	}
	for i, ln := range lines {
		lines[i] = strings.TrimRight(strings.ReplaceAll(ln, "\t", strings.Repeat(" ", gutterTabWidth)), " ")
	}
	lines = dedent(lines)

	gutter := len(strconv.Itoa(startLine + len(lines) - 1))
	var b strings.Builder
	for i, ln := range lines {
		fmt.Fprintf(&b, "%*d  %s\n", gutter, startLine+i, ansi.Truncate(ln, max(width-gutter-2, 1), "…"))
	}
	if elided {
		fmt.Fprintf(&b, "%*s  …", gutter, "")
	}
	return strings.TrimRight(b.String(), "\n")
}

// dedent strips the indentation common to every non-blank line.
func dedent(lines []string) []string {
	common := -1
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		n := len(ln) - len(strings.TrimLeft(ln, " "))
		if common < 0 || n < common {
			common = n
		}
	}
	if common <= 0 {
		return lines
	}
	for i, ln := range lines {
		if len(ln) >= common {
			lines[i] = ln[common:]
		} else {
			lines[i] = strings.TrimLeft(ln, " ")
		}
	}
	return lines
}

// fenceCode wraps code in a Markdown fence tagged with lang. The fence is
// grown past the longest backtick run in the code so an indexed Markdown file
// — which can hold fences of its own — cannot break out of the block.
func fenceCode(code, lang string) string {
	longest, run := 0, 0
	for _, r := range code {
		if r == '`' {
			run++
			longest = max(longest, run)
			continue
		}
		run = 0
	}
	fence := strings.Repeat("`", max(3, longest+1))
	return fence + lang + "\n" + code + "\n" + fence
}

// fenceLanguage maps a path to the Markdown fence tag the host's Markdown
// renderer highlights it with. Every extension the indexer accepts (see
// indexableExtensions) has an entry, so a hit can always be tagged; unknown
// extensions fall back to an untagged fence, which still renders as code.
var fenceLanguage = func() func(path string) string {
	byExt := map[string]string{
		".bash": "bash", ".c": "c", ".cc": "cpp", ".cljc": "clojure",
		".clj": "clojure", ".cpp": "cpp", ".cs": "csharp", ".css": "css",
		".cxx": "cpp", ".dart": "dart", ".elixir": "elixir", ".elm": "elm",
		".ex": "elixir", ".exs": "elixir", ".go": "go", ".groovy": "groovy",
		".h": "c", ".hpp": "cpp", ".hs": "haskell", ".htm": "html",
		".html": "html", ".java": "java", ".js": "javascript", ".json": "json",
		".jsx": "jsx", ".kt": "kotlin", ".kts": "kotlin", ".lua": "lua",
		".md": "markdown", ".mjs": "javascript", ".ml": "ocaml",
		".mts": "typescript", ".php": "php", ".py": "python", ".rb": "ruby",
		".rs": "rust", ".scala": "scala", ".scm": "scheme", ".sh": "bash",
		".sql": "sql", ".svelte": "svelte", ".swift": "swift", ".toml": "toml",
		".ts": "typescript", ".tsx": "tsx", ".vue": "vue", ".xml": "xml",
		".yaml": "yaml", ".yml": "yaml", ".zig": "zig",
	}
	return func(path string) string {
		return byExt[strings.ToLower(filepath.Ext(path))]
	}
}()

// splitIndexError pulls the path off a per-file index error, which the index
// tool formats as "<path>: <err>", and condenses the message. The prefix only
// counts as a path when it ends in an extension the indexer accepts — a
// message like "indexing cancelled: context deadline exceeded" would
// otherwise lose its first clause to the path column, and a heuristic on
// spaces or separators would drop a real path under a directory with a space
// in its name. An error with no path prefix returns an empty path and the
// whole condensed string.
func splitIndexError(workingDir, e string) (path, msg string) {
	if i := strings.Index(e, ": "); i > 0 && indexableExtensions[strings.ToLower(filepath.Ext(e[:i]))] {
		return relativizePath(workingDir, e[:i]), condenseError(e[i+2:])
	}
	return "", condenseError(e)
}

// condenseError flattens an error onto one line and truncates it. Provider
// errors arrive as several layers of escaped JSON, which wraps into an
// unreadable wall inside a card; the leading clause carries the cause.
func condenseError(s string) string {
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	if r := []rune(s); len(r) > errorMessageLimit {
		return strings.TrimRight(string(r[:errorMessageLimit]), " ") + "…"
	}
	return s
}

// pluralize appends an s to word unless n is exactly 1. The surface is the
// user-facing copy, so "1 results" would read as a bug there; the
// model-facing digest keeps its original wording.
func pluralize(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// firstLines truncates s to at most n lines, marking the cut.
func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:n], "\n") + "\n..."
}
