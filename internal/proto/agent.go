package proto

import (
	"errors"

	"github.com/bytedance/sonic"
	"github.com/charmbracelet/crush/internal/agent/notify"
)

// SummaryResponse distinguishes cancellation from other HTTP conflicts.
type SummaryResponse struct {
	Cancelled bool `json:"cancelled"`
}

// AgentEventType represents the type of agent event.
type AgentEventType string

const (
	AgentEventTypeError     AgentEventType = "error"
	AgentEventTypeResponse  AgentEventType = "response"
	AgentEventTypeSummarize AgentEventType = "summarize"
)

// MarshalText implements the [encoding.TextMarshaler] interface.
func (t AgentEventType) MarshalText() ([]byte, error) {
	return []byte(t), nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (t *AgentEventType) UnmarshalText(text []byte) error {
	*t = AgentEventType(text)
	return nil
}

// AgentEvent represents an event emitted by the agent.
type AgentEvent struct {
	Error        error              `json:"error,omitempty"`
	Type         AgentEventType     `json:"type"`
	RunID        string             `json:"run_id,omitempty"`
	FinishState  notify.FinishState `json:"finish_state,omitempty"`
	SessionID    string             `json:"session_id,omitempty"`
	SessionTitle string             `json:"session_title,omitempty"`
	Progress     string             `json:"progress,omitempty"`
	AWSSOCommand string             `json:"aws_sso_command,omitempty"`
	AWSSOURL     string             `json:"aws_sso_url,omitempty"`
	Message      Message            `json:"message"`
	Done         bool               `json:"done,omitempty"`
}

// MarshalJSON implements the [json.Marshaler] interface.
func (e AgentEvent) MarshalJSON() ([]byte, error) {
	type Alias AgentEvent
	return sonic.Marshal(&struct {
		Error string `json:"error,omitempty"`
		Alias
	}{
		Error: func() string {
			if e.Error != nil {
				return e.Error.Error()
			}
			return ""
		}(),
		Alias: Alias(e),
	})
}

// UnmarshalJSON implements the [sonic.Unmarshaler] interface.
func (e *AgentEvent) UnmarshalJSON(data []byte) error {
	type Alias AgentEvent
	aux := &struct {
		Error string `json:"error,omitempty"`
		Alias
	}{
		Alias: Alias(*e),
	}
	if err := sonic.Unmarshal(data, &aux); err != nil {
		return err
	}
	*e = AgentEvent(aux.Alias)
	if aux.Error != "" {
		e.Error = errors.New(aux.Error)
	}
	return nil
}
