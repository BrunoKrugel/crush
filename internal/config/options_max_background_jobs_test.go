package config

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/require"
)

func TestOptionsGetMaxBackgroundJobs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		options  *Options
		expected int
	}{
		{"nil options", nil, DefaultMaxBackgroundJobs},
		{"unset", &Options{}, DefaultMaxBackgroundJobs},
		{"zero means default", &Options{MaxBackgroundJobs: new(0)}, DefaultMaxBackgroundJobs},
		{"negative means default", &Options{MaxBackgroundJobs: new(-5)}, DefaultMaxBackgroundJobs},
		{"configured", &Options{MaxBackgroundJobs: new(200)}, 200},
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
