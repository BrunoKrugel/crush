package lsp

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/stretchr/testify/require"
)

func publish(t *testing.T, c *Client, uri string, diags []protocol.Diagnostic) {
	t.Helper()
	params, err := json.Marshal(protocol.PublishDiagnosticsParams{
		URI:         protocol.DocumentURI(uri),
		Diagnostics: diags,
	})
	require.NoError(t, err)
	HandleDiagnostics(c, params)
}

func TestDiagnosticsClearReducesCounts(t *testing.T) {
	c := newTestClient()
	uri := "file:///test.go"
	errSev := protocol.SeverityError

	publish(t, c, uri, []protocol.Diagnostic{
		{Message: "a", Severity: errSev},
		{Message: "b", Severity: errSev},
	})
	counts := c.GetDiagnosticCounts()
	require.Equal(t, 2, counts.Error, "two errors published")

	publish(t, c, uri, nil)
	counts = c.GetDiagnosticCounts()
	require.Equal(t, 0, counts.Error, "cleared publish must drop the count to zero")
}

func TestResetDiagnosticsStateDropsStaleEntries(t *testing.T) {
	c := newTestClient()
	errSev := protocol.SeverityError
	publish(t, c, "file:///test.go", []protocol.Diagnostic{
		{Message: "a", Severity: errSev},
		{Message: "b", Severity: errSev},
	})
	require.Equal(t, 2, c.GetDiagnosticCounts().Error)

	c.resetDiagnosticsState()

	require.Empty(t, c.GetDiagnostics(), "a restarted server's diagnostics must be dropped")
	require.Equal(t, DiagnosticCounts{}, c.GetDiagnosticCounts())
}
