package main

import (
	"log/slog"
	"os"

	"github.com/crouton/stellar-explorer/internal/app"
	"github.com/crouton/stellar-explorer/internal/ingestion"
	"github.com/crouton/stellar-explorer/internal/stellar"
)

func main() {
	ctx, cancel, c, s, e := app.Start()
	defer cancel()

	if e != nil {
		slog.Error("indexer startup failed", "error", e)
		os.Exit(1)
	}

	defer s.Close()

	i := ingestion.Indexer{
		Source:   stellar.New(c.RPCURL),
		Store:    s,
		Start:    c.Start,
		Batch:    c.Batch,
		Poll:     c.Poll,
		RetryMax: c.RetryMax,
	}
	if e = i.Run(ctx); e != nil {
		slog.Error("indexer stopped", "error", e)
		os.Exit(1)
	}
}
