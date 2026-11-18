package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateOpenCodeID_LengthAndPrefix(t *testing.T) {
	t.Parallel()

	msg := GenerateOpenCodeID("msg_", false)
	require.Len(t, msg, 30, "msg_ ID must be exactly 30 characters")
	require.True(t, strings.HasPrefix(msg, "msg_"), "msg_ ID must carry the msg_ prefix")

	ses := GenerateOpenCodeID("ses_", true)
	require.Len(t, ses, 30, "ses_ ID must be exactly 30 characters")
	require.True(t, strings.HasPrefix(ses, "ses_"), "ses_ ID must carry the ses_ prefix")
}

func TestGenerateOpenCodeID_Unique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, 128)
	for range 128 {
		id := GenerateOpenCodeID("msg_", false)
		_, dup := seen[id]
		require.False(t, dup, "consecutive GenerateOpenCodeID calls must be unique, got duplicate %q", id)
		seen[id] = struct{}{}
	}
}
