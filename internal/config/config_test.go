package config_test

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/config"
)

func TestConfigRejectsWrongNetworkAndUnsafeLimits(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test@localhost/test")

	for _, tc := range []struct {
		key   string
		value string
	}{
		{"STELLAR_NETWORK", "mainnet"},
		{"BATCH_SIZE", "201"},
		{"BATCH_SIZE", "0"},
		{"POLL_INTERVAL", "0s"},
		{"RETRY_MAX", "-1s"},
		{"API_MAX_LIMIT", "-1"},
		{"START_LEDGER", "-1"},
		{"STELLAR_RPC_URL", "file:///tmp/rpc"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, e := config.Load(); e == nil {
				t.Fatal("accepted invalid config")
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test@localhost/test")
	c, e := config.Load()
	if e != nil || c.Batch != 10 || c.Start != 0 || c.MaxLimit != 100 {
		t.Fatalf("defaults: %+v %v", c, e)
	}
}
