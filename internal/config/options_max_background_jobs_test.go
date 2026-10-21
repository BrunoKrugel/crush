package config

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/require"
)

func TestOptionsGetMaxBackgroundJobs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		options  *Options
		name     string
		expected int
	}{
		{name: "nil options", options: nil, expected: DefaultMaxBackgroundJobs},
		{name: "unset", options: &Options{}, expected: DefaultMaxBackgroundJobs},
		{name: "zero means default", options: &Options{MaxBackgroundJobs: new(0)}, expected: DefaultMaxBackgroundJobs},
		{name: "negative means default", options: &Options{MaxBackgroundJobs: new(-5)}, expected: DefaultMaxBackgroundJobs},
		{name: "configured", options: &Options{MaxBackgroundJobs: new(200)}, expected: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, tt.options.GetMaxBackgroundJobs())
		})
	}
}

func TestOptionsMaxBackgroundJobsFromJSON(t *testing.T) {
	t.Parallel()

	var cfg Config
	require.NoError(t, sonic.Unmarshal([]byte(`{"options":{"max_background_jobs":10}}`), &cfg))
	require.Equal(t, 10, cfg.Options.GetMaxBackgroundJobs())
}
