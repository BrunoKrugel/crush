package tools

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/stretchr/testify/require"
)

func TestScoreMeter(t *testing.T) {
	t.Parallel()
	require.Equal(t, strings.Repeat("░", scoreMeterCells), scoreMeter(0))
	require.Equal(t, strings.Repeat("█", scoreMeterCells), scoreMeter(1))
	require.Equal(t, strings.Repeat("░", scoreMeterCells), scoreMeter(-2))
	require.Equal(t, strings.Repeat("█", scoreMeterCells), scoreMeter(9))
	require.Equal(t, strings.Repeat("█", 5)+strings.Repeat("░", 5), scoreMeter(0.5))
}

// The store holds absolute paths. Showing them verbatim spends most of a
// card's width on the part every hit shares.
func TestNumberedSnippetTruncatesToWidth(t *testing.T) {
	t.Parallel()
	out := numberedSnippet(strings.Repeat("x", 200), 1, 6, 20)
	require.Equal(t, "1  "+strings.Repeat("x", 16)+"…", out)
	for line := range strings.SplitSeq(out, "\n") {
		require.LessOrEqual(t, len([]rune(line)), 20)
	}
}

// Chunks arrive indented at their nesting depth; spending a card's width on
// leading space is the one thing a preview cannot afford.
func TestNumberedSnippetDedentsAndElides(t *testing.T) {
	t.Parallel()
	snippet := "\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tmore()"
	require.Equal(t,
		"1  if err != nil {\n2      return err\n3  }\n   …",
		numberedSnippet(snippet, 1, 3, 60))
}

// An empty or whitespace-only chunk gets no code block at all — an empty
// fence renders as a stray box.
func TestNumberedSnippetSkipsEmpty(t *testing.T) {
	t.Parallel()
	require.Empty(t, numberedSnippet("", 1, 6, 80))
	require.Empty(t, numberedSnippet("  \n\t\n", 1, 6, 80))
}

// The fence language is what buys the surface real syntax highlighting from
// the host's Markdown renderer, so every extension the indexer accepts needs
// one — and it has to name a lexer that actually exists.
func TestFenceLanguageCoversIndexableExtensions(t *testing.T) {
	t.Parallel()
	for ext := range indexableExtensions {
		lang := fenceLanguage("file" + ext)
		require.NotEmpty(t, lang, "no fence language for %s", ext)
		lexer := lexers.Get(lang)
		require.NotNil(t, lexer, "fence language %q for %s is not a chroma lexer", lang, ext)
		require.NotEqual(t, lexers.Fallback.Config().Name, lexer.Config().Name,
			"fence language %q for %s resolves to the fallback lexer", lang, ext)
	}
	// An extension the indexer never produces still fences, just untagged.
	require.Empty(t, fenceLanguage("a.bin"))
	require.True(t, strings.HasPrefix(fenceCode("x", fenceLanguage("a.bin")), "```\n"))
}

// Markdown is indexable, so a chunk can carry fences of its own. A
// three-backtick wrapper would end at the chunk's first fence and dump the
// rest of the snippet into the card as prose.
func TestFenceCodeEscapesNestedFences(t *testing.T) {
	t.Parallel()
	code := "Example:\n```go\nx := 1\n```"
	out := fenceCode(code, "markdown")
	require.True(t, strings.HasPrefix(out, "````markdown\n"), out)
	require.True(t, strings.HasSuffix(out, "\n````"), out)
	require.Contains(t, out, code)
}

func TestSemanticSearchDigestHeadlessFormat(t *testing.T) {
	t.Parallel()
	hits := []semanticSearchHit{
		{Path: "a.go", Symbol: "Foo", StartLine: 0, EndLine: 1, Score: 0.5, Snippet: "one\ntwo"},
		{Path: "b.go", StartLine: 4, EndLine: 4, Score: 0.25, Snippet: "solo"},
	}
	require.Equal(t, "Found 2 results:\n\n"+
		"1. a.go :: Foo (lines 1-2, score 0.500)\n   one\n   two\n\n"+
		"2. b.go (lines 5-5, score 0.250)\n   solo\n\n",
		semanticSearchDigest(hits, true))

	require.Equal(t, "Found 2 results:\n\n"+
		"1. a.go :: Foo (lines 1-2, score 0.500)\n"+
		"2. b.go (lines 5-5, score 0.250)\n",
		semanticSearchDigest(hits, false))
}

// The surface is user-facing copy, so a single hit reads "1 result".
