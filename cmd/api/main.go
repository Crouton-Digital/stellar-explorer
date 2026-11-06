package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/crouton/stellar-explorer/internal/app"
	"github.com/crouton/stellar-explorer/internal/httpapi"
)

func main() {
	ctx, cancel, c, s, e := app.Start()
	defer cancel()

	if e != nil {
		slog.Error("API startup failed", "error", e)
		os.Exit(1)
	}

	defer s.Close()

	api := httpapi.API{
		Store:    s,
		MaxLimit: c.MaxLimit,
	}
	server := &http.Server{
		Addr:              c.Listen,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()

		_ = server.Shutdown(shutdown)
	}()
	slog.Info("API listening", "address", c.Listen)
	if e = server.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		slog.Error("API failed", "error", e)
		os.Exit(1)
	}
}
