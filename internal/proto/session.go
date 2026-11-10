package proto

// Session represents a session in the proto layer.
//
// IsBusy is computed on read (it is not persisted with the session) and
// reflects whether an agent run is currently in flight for this session.
// It is populated by REST handlers in internal/server/proto.go from the
// workspace's AgentCoordinator. The Session SSE event path does not set
// it, since SSE consumers can compute presence from other agent signals.
//
// AttachedClients counts the number of clients currently viewing this
// session — i.e. entries in the workspace's clients map whose
// currentSessionID equals this session's ID and which have at least one
// live SSE stream. Hold-only clients (streams == 0) do not contribute.
// Like IsBusy, it is computed on read by REST handlers.
// ForkRequest is the body of the fork-session endpoint: the message to
// branch from, the new session's title, and whether to materialize the
// fork point in a fresh worktree.
type ForkRequest struct {
	MessageID string `json:"message_id"`
	Title     string `json:"title,omitempty"`
	Worktree  bool   `json:"worktree,omitempty"`
}

type Session struct {
	Channel          string  `json:"channel,omitempty"`
	Title            string  `json:"title"`
	SummaryMessageID string  `json:"summary_message_id"`
	ID               string  `json:"id"`
	ParentSessionID  string  `json:"parent_session_id"`
	Todos            []Todo  `json:"todos,omitempty"`
	CreatedAt        int64   `json:"created_at"`
	CompletionTokens int64   `json:"completion_tokens"`
	PromptTokens     int64   `json:"prompt_tokens"`
	MessageCount     int64   `json:"message_count"`
	UpdatedAt        int64   `json:"updated_at"`
	Cost             float64 `json:"cost"`
	AttachedClients  int     `json:"attached_clients"`
	IsBusy           bool    `json:"is_busy"`
}

// Todo represents a single todo entry on a session in the proto layer.
type Todo struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"active_form"`
}
