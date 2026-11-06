// replay imports captured ledgers into an empty or matching Testnet index without RPC access.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/crouton/stellar-explorer/internal/config"
	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/stellar"
	"github.com/crouton/stellar-explorer/internal/storage"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: replay <captured-fixtures.json>")
	}

	raw, e := os.ReadFile(os.Args[1])
	if e != nil {
		return e
	}

	var f struct {
		Network string                 `json:"network"`
		Ledgers []stellar.RemoteLedger `json:"ledgers"`
	}
	if e = json.Unmarshal(raw, &f); e != nil {
		return e
	}

	if f.Network != domain.Network || len(f.Ledgers) == 0 {
		return fmt.Errorf("expected non-empty Testnet fixture")
	}

	c, e := config.Load()
	if e != nil {
		return e
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s, e := storage.Open(ctx, c.DSN)
	if e != nil {
		return e
	}

	defer s.Close()

	if e = s.Migrate(ctx); e != nil {
		return e
	}

	for _, remote := range f.Ledgers {
		b, e := decoder.Ledger(remote)
		if e != nil {
			return e
		}

		if e = s.Save(ctx, b); e != nil {
			return e
		}

		fmt.Printf("replayed ledger %d: %d transactions\n", b.Ledger.Sequence, len(b.Transactions))
	}

	return nil
}
