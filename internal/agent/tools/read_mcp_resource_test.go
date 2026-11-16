package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetContentWidthFromContext(t *testing.T) {
	t.Parallel()

	require.Zero(t, GetContentWidthFromContext(context.Background()))
	ctx := context.WithValue(context.Background(), ContentWidthContextKey, 114)
	require.Equal(t, 114, GetContentWidthFromContext(ctx))
}
