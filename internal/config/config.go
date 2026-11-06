package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DSN      string
	RPCURL   string
	Listen   string
	Start    uint32
	Batch    int
	MaxLimit int
	Poll     time.Duration
	RetryMax time.Duration
}

func Load() (Config, error) {
	c := Config{
		DSN:    os.Getenv("DATABASE_URL"),
		RPCURL: env("STELLAR_RPC_URL", "https://soroban-testnet.stellar.org"),
		Listen: env("API_LISTEN", ":8080"),
	}
	if env("STELLAR_NETWORK", "testnet") != "testnet" {
		return c, fmt.Errorf("only STELLAR_NETWORK=testnet is supported")
	}

	if c.DSN == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}

	u, e := url.Parse(c.RPCURL)
	if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return c, fmt.Errorf("invalid STELLAR_RPC_URL")
	}

	n, e := strconv.ParseUint(env("START_LEDGER", "0"), 10, 32)
	if e != nil {
		return c, fmt.Errorf("invalid START_LEDGER")
	}

	c.Start = uint32(n)

	for _, p := range []struct {
		name   string
		def    string
		target *int
		max    int
	}{
		{
			"BATCH_SIZE",
			"10",
			&c.Batch,
			200,
		},
		{
			"API_MAX_LIMIT",
			"100",
			&c.MaxLimit,
			200,
		},
	} {
		v, e := strconv.Atoi(env(p.name, p.def))
		if e != nil || v < 1 || v > p.max {
			return c, fmt.Errorf("%s must be 1..%d", p.name, p.max)
		}

		*p.target = v
	}

	for _, p := range []struct {
		name   string
		def    string
		target *time.Duration
	}{
		{"POLL_INTERVAL", "5s", &c.Poll},
		{"RETRY_MAX", "30s", &c.RetryMax},
	} {
		v, e := time.ParseDuration(env(p.name, p.def))
		if e != nil || v <= 0 {
			return c, fmt.Errorf("invalid %s", p.name)
		}

		*p.target = v
	}

	return c, nil
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}

	return d
}
