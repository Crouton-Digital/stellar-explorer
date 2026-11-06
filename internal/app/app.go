package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/crouton/stellar-explorer/internal/config"
	"github.com/crouton/stellar-explorer/internal/storage"
)

func Start() (context.Context, context.CancelFunc, config.Config, *storage.Store, error) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	c, e := config.Load()
	if e != nil {
		return ctx, cancel, c, nil, e
	}

	s, e := storage.Open(ctx, c.DSN)
	if e != nil {
		return ctx, cancel, c, nil, e
	}

	if e = s.Migrate(ctx); e != nil {
		s.Close()

		return ctx, cancel, c, nil, e
	}

	return ctx, cancel, c, s, nil
}
