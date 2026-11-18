package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/fork"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/skills"
)

// The routes in this file are the fork's session-fork, child-session,
// discovery-reload, skills-reload, MCP call-tool/reconnect, and native
// auto-mode status endpoints (#314, #408). Upstream has none of them, and
// the sync that replaced the hand-written mux with the declarative
// registry in endpoints.go dropped them silently, so they are registered
// and implemented together here.

// handlePostWorkspaceSessionFork branches a conversation: the new session
// continues the source session up to the given message, with the filesystem
// state restored from the snapshot at that point.
func (c *controllerV1) handlePostWorkspaceSessionFork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")

	var args struct {
		MessageID string `json:"message_id"`
		Title     string `json:"title"`
		Worktree  bool   `json:"worktree"`
	}
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	ws, err := c.backend.GetWorkspace(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	if ws.Forks == nil {
		jsonError(w, http.StatusInternalServerError, "forking is unavailable without a session database")
		return
	}
	result, err := ws.Forks.Fork(r.Context(), sid, args.MessageID, fork.ForkOptions{
		Title:    args.Title,
		Worktree: args.Worktree,
	})
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	sess := result.Session
	out := sessionToProto(sess)
	out.IsBusy = isSessionBusy(ws, sess.ID)
	out.AttachedClients = attachedClients(ws, sess.ID)
	jsonEncode(w, out)
}

// handleGetWorkspaceSessionChildren lists the child sessions of one session.
func (c *controllerV1) handleGetWorkspaceSessionChildren(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")

	ws, err := c.backend.GetWorkspace(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	sessions, err := ws.Sessions.ListChildren(r.Context(), sid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	result := make([]proto.Session, len(sessions))
	for i, s := range sessions {
		result[i] = sessionToProto(s)
		result[i].IsBusy = isSessionBusy(ws, s.ID)
		result[i].AttachedClients = attachedClients(ws, s.ID)
	}
	jsonEncode(w, result)
}

// handleGetWorkspaceChildSessions lists every child session of every
// session in the workspace in one query, grouped by parent.
func (c *controllerV1) handleGetWorkspaceChildSessions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	ws, err := c.backend.GetWorkspace(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	sessions, err := ws.Sessions.ListAllChildren(r.Context())
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	result := make([]proto.Session, len(sessions))
	for i, s := range sessions {
		result[i] = sessionToProto(s)
		result[i].IsBusy = isSessionBusy(ws, s.ID)
		result[i].AttachedClients = attachedClients(ws, s.ID)
	}
	jsonEncode(w, result)
}

// handlePostWorkspaceConfigReloadDiscovery re-runs model discovery.
func (c *controllerV1) handlePostWorkspaceConfigReloadDiscovery(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	ws, err := c.backend.GetWorkspace(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	added, err := ws.Cfg.ReloadModelDiscovery(r.Context())
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.ReloadModelDiscoveryResponse{Added: added})
}

// handlePostWorkspaceSkillsReload triggers a skill re-discovery.
func (c *controllerV1) handlePostWorkspaceSkillsReload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	ws, err := c.backend.GetWorkspace(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	mgr := ws.Skills
	if mgr == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	discoveryCfg := skills.DiscoveryConfig{
		WorkingDir: ws.Cfg.WorkingDir(),
		Resolver:   ws.Cfg.Resolver().ResolveValue,
	}
	opts := ws.Cfg.Config().Options
	if opts != nil {
		discoveryCfg.SkillsPaths = opts.SkillsPaths
		discoveryCfg.DisabledSkills = opts.DisabledSkills
	}
	mgr.Reload(discoveryCfg)
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceMCPCallTool calls a tool on an MCP server.
func (c *controllerV1) handlePostWorkspaceMCPCallTool(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.MCPCallToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	ws, err := c.backend.GetWorkspace(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	input, err := json.Marshal(req.Args)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "failed to encode tool arguments")
		return
	}
	res, err := mcp.RunTool(r.Context(), ws.Cfg, req.Name, req.ToolName, string(input))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, map[string]any{"content": res.Content})
}

// handlePostWorkspaceMCPReconnect restarts a single MCP server by name.
func (c *controllerV1) handlePostWorkspaceMCPReconnect(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.MCPNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	ws, err := c.backend.GetWorkspace(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	if err := mcp.DisableSingle(ws.Cfg, req.Name); err != nil {
		c.handleError(w, r, err)
		return
	}
	if err := ws.Cfg.ReloadFromDisk(r.Context()); err != nil {
		slog.Warn("Failed to reload config from disk before reconnecting MCP; using existing config", "name", req.Name, "error", err)
	} else {
		// Remote clients render from a cached config snapshot that only
		// refreshes on ConfigChanged, so a reload that picked up on-disk
		// edits must be published like every other config mutation here.
		ws.SendEvent(pubsub.Event[proto.ConfigChanged]{
			Type:    pubsub.UpdatedEvent,
			Payload: proto.ConfigChanged{WorkspaceID: ws.ID},
		})
	}
	if err := mcp.InitializeSingle(r.Context(), req.Name, ws.Cfg); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleGetWorkspacePermissionsAuto returns whether native auto mode is
// enabled for the workspace.
func (c *controllerV1) handleGetWorkspacePermissionsAuto(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	ws, err := c.backend.GetWorkspace(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	toggler := ws.AutoModeToggler()
	var enabled bool
	if toggler != nil {
		enabled = toggler.AutoModeEnabled()
	}
	jsonEncode(w, proto.PermissionAutoRequest{Enabled: enabled})
}
