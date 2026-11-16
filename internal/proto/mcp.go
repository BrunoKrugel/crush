package proto

import (
	"errors"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
)

// MCPState represents the current state of an MCP client.
type MCPState int

const (
	MCPStateDisabled MCPState = iota
	MCPStateStarting
	MCPStateConnected
	MCPStateError
	MCPStateNeedsAuth
)

// MarshalText implements the [encoding.TextMarshaler] interface.
func (s MCPState) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (s *MCPState) UnmarshalText(data []byte) error {
	switch string(data) {
	case "disabled":
		*s = MCPStateDisabled
	case "starting":
		*s = MCPStateStarting
	case "connected":
		*s = MCPStateConnected
	case "error":
		*s = MCPStateError
	case "needs auth":
		*s = MCPStateNeedsAuth
	default:
		return fmt.Errorf("unknown mcp state: %s", data)
	}
	return nil
}

// String returns the string representation of the MCPState.
func (s MCPState) String() string {
	switch s {
	case MCPStateDisabled:
		return "disabled"
	case MCPStateStarting:
		return "starting"
	case MCPStateConnected:
		return "connected"
	case MCPStateError:
		return "error"
	case MCPStateNeedsAuth:
		return "needs auth"
	default:
		return "unknown"
	}
}

// MCPEventType represents the type of MCP event.
type MCPEventType string

const (
	MCPEventStateChanged         MCPEventType = "state_changed"
	MCPEventToolsListChanged     MCPEventType = "tools_list_changed"
	MCPEventPromptsListChanged   MCPEventType = "prompts_list_changed"
	MCPEventResourcesListChanged MCPEventType = "resources_list_changed"
	MCPEventChannelMessage       MCPEventType = "channel_message"
)

// MarshalText implements the [encoding.TextMarshaler] interface.
func (t MCPEventType) MarshalText() ([]byte, error) {
	return []byte(t), nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (t *MCPEventType) UnmarshalText(data []byte) error {
	*t = MCPEventType(data)
	return nil
}

// MCPEvent represents an event in the MCP system.
type MCPEvent struct {
	Error          error        `json:"error,omitempty"`
	Type           MCPEventType `json:"type"`
	Name           string       `json:"name"`
	ChannelMessage string       `json:"channel_message,omitempty"`
	State          MCPState     `json:"state"`
	ToolCount      int          `json:"tool_count,omitempty"`
	PromptCount    int          `json:"prompt_count,omitempty"`
	ResourceCount  int          `json:"resource_count,omitempty"`
}

// MarshalJSON implements the [json.Marshaler] interface.
func (e MCPEvent) MarshalJSON() ([]byte, error) {
	type Alias MCPEvent
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
func (e *MCPEvent) UnmarshalJSON(data []byte) error {
	type Alias MCPEvent
	aux := &struct {
		Error string `json:"error,omitempty"`
		Alias
	}{
		Alias: Alias(*e),
	}
	if err := sonic.Unmarshal(data, &aux); err != nil {
		return err
	}
	*e = MCPEvent(aux.Alias)
	if aux.Error != "" {
		e.Error = errors.New(aux.Error)
	}
	return nil
}

// MCPClientInfo is the wire-format representation of an MCP client's
// state, suitable for JSON transport between server and client.
type MCPClientInfo struct {
	ConnectedAt   time.Time `json:"connected_at"`
	Error         error     `json:"error,omitempty"`
	Name          string    `json:"name"`
	State         MCPState  `json:"state"`
	ToolCount     int       `json:"tool_count,omitempty"`
	PromptCount   int       `json:"prompt_count,omitempty"`
	ResourceCount int       `json:"resource_count,omitempty"`
	Channel       bool      `json:"channel,omitempty"`
}

type MCPPromptArgument struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type MCPPrompt struct {
	ID          string              `json:"id"`
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	PromptID    string              `json:"prompt_id"`
	ClientID    string              `json:"client_id"`
	Arguments   []MCPPromptArgument `json:"arguments,omitempty"`
}

// MarshalJSON implements the [json.Marshaler] interface.
func (i MCPClientInfo) MarshalJSON() ([]byte, error) {
	type Alias MCPClientInfo
	return sonic.Marshal(&struct {
		Error string `json:"error,omitempty"`
		Alias
	}{
		Error: func() string {
			if i.Error != nil {
				return i.Error.Error()
			}
			return ""
		}(),
		Alias: Alias(i),
	})
}

// UnmarshalJSON implements the [sonic.Unmarshaler] interface.
func (i *MCPClientInfo) UnmarshalJSON(data []byte) error {
	type Alias MCPClientInfo
	aux := &struct {
		Error string `json:"error,omitempty"`
		Alias
	}{
		Alias: Alias(*i),
	}
	if err := sonic.Unmarshal(data, &aux); err != nil {
		return err
	}
	*i = MCPClientInfo(aux.Alias)
	if aux.Error != "" {
		i.Error = errors.New(aux.Error)
	}
	return nil
}
