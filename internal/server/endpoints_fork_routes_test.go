package server

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// forkOnlyRoutes are the v1 routes this fork carries that upstream does
// not. They exist because the fork adds behavior upstream lacks: model
// discovery reload, skills reload, MCP tool calls, MCP
// reconnect, and the child-sessions listings (#314, #408).
//
// The sync from upstream replaced the hand-written mux.HandleFunc list
// with the declarative registry in endpoints.go, which meant porting
// these six by hand. A route silently dropped in that port is invisible
// at compile time and only shows up as a 404 from a client that used to
// work, so pin them by method+path.
var forkOnlyRoutes = []struct {
	method string
	path   string
}{
	{"POST", "/v1/workspaces/{id}/config/reload-discovery"},
	{"POST", "/v1/workspaces/{id}/skills/reload"},
	{"POST", "/v1/workspaces/{id}/mcp/call-tool"},
	{"POST", "/v1/workspaces/{id}/mcp/reconnect"},
	{"GET", "/v1/workspaces/{id}/child-sessions"},
	{"GET", "/v1/workspaces/{id}/sessions/{sid}/children"},
	{"GET", "/v1/workspaces/{id}/permissions/auto"},
	{"POST", "/v1/workspaces/{id}/permissions/auto"},
}

// TestEndpoints_ForkRoutesRegistered asserts every fork-only route is
// still in the registry. TestDocsEndpoints then proves each one reaches
// the served OpenAPI spec, and installHandler registers them on the mux.
func TestEndpoints_ForkRoutesRegistered(t *testing.T) {
	t.Parallel()

	c := &controllerV1{}
	registered := map[string]bool{}
	for _, e := range c.endpoints() {
		registered[e.Method()+" "+e.Path()] = true
	}

	for _, want := range forkOnlyRoutes {
		key := want.method + " " + want.path
		require.True(t, registered[key],
			"fork route %s is missing from the endpoints registry; it was dropped in an upstream sync port", key)
	}
}
