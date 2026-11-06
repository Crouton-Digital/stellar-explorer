package ingestion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/stellar"
	"github.com/crouton/stellar-explorer/internal/storage"
)

type Source interface {
	Network(context.Context) (string, error)
	Health(context.Context) (stellar.Health, error)
	Ledgers(context.Context, uint32, int) ([]stellar.RemoteLedger, error)
}

type Repository interface {
	Checkpoint(context.Context) (uint32, error)
	Save(context.Context, domain.Bundle) error
	Observe(context.Context, uint32, *string) error
}

type Indexer struct {
	Source   Source
	Store    Repository
	Start    uint32
	Batch    int
	Poll     time.Duration
	RetryMax time.Duration
}

type FatalError struct {
	Message string
}

func (e *FatalError) Error() string {
	return e.Message
}

func (i *Indexer) Step(ctx context.Context) (int, error) {
	network, e := i.Source.Network(ctx)
	if e != nil {
		return 0, e
	}

	if network != domain.Passphrase {
		return 0, &FatalError{"RPC is not Stellar Testnet"}
	}

	h, e := i.Source.Health(ctx)
	if e != nil {
		return 0, e
	}

	if e = i.Store.Observe(ctx, h.Latest, nil); e != nil {
		return 0, e
	}

	cp, e := i.Store.Checkpoint(ctx)
	if e != nil {
		return 0, e
	}

	next := cp + 1
	if cp == 0 {
		next = i.Start
		if next == 0 {
			next = h.Oldest
			if h.Latest > 100 && h.Latest-100 > next {
				next = h.Latest - 100
			}
		}
	}

	if next < h.Oldest {
		return 0, &FatalError{
			fmt.Sprintf(
				"ledger %d is outside RPC history [%d,%d]; choose an archive or a new database explicitly",
				next,
				h.Oldest,
				h.Latest,
			),
		}
	}

	if next > h.Latest+1 || (cp == 0 && next > h.Latest) {
		return 0, &FatalError{
			"start/checkpoint is ahead of RPC; check provider synchronization or Testnet reset",
		}
	}

	if next == h.Latest+1 {
		return 0, nil
	}

	count := i.Batch
	if remaining := int(h.Latest - next + 1); remaining < count {
		count = remaining
	}

	ledgers, e := i.Source.Ledgers(ctx, next, count)
	if e != nil {
		return 0, e
	}

	if len(ledgers) == 0 {
		return 0, fmt.Errorf("RPC returned no ledgers before observed tip")
	}

	n := 0

	for _, l := range ledgers {
		if l.Sequence != next {
			return n, fmt.Errorf(
				"RPC returned non-contiguous ledgers: want %d, got %d",
				next,
				l.Sequence,
			)
		}

		b, e := decoder.Ledger(l)
		if e != nil {
			return n, fmt.Errorf("decode ledger %d: %w", l.Sequence, e)
		}

		if e = i.Store.Save(ctx, b); e != nil {
			if errors.Is(e, storage.ErrContinuity) {
				return n, &FatalError{e.Error()}
			}

			return n, e
		}

		slog.Info("ledger committed", "ledger", l.Sequence, "transactions", len(b.Transactions))
		n++
		next++
	}

	return n, nil
}

func (i *Indexer) Run(ctx context.Context) error {
	delay := min(time.Second, i.RetryMax)

	for {
		if e := ctx.Err(); e != nil {
			return nil
		}

		n, e := i.Step(ctx)
		wait := i.Poll
		if e != nil {
			message := e.Error()
			_ = i.Store.Observe(ctx, 0, &message)
			var fatal *FatalError
			if errors.As(e, &fatal) {
				return e
			}

			slog.Error("indexing failed; retrying", "error", e, "retry_in", delay)
			wait = delay
			delay = min(delay*2, i.RetryMax)
		} else {
			delay = min(time.Second, i.RetryMax)
			if n >= i.Batch {
				wait = 0
			}
		}

		timer := time.NewTimer(wait)

		select {
		case <-ctx.Done():
			timer.Stop()

			return nil

		case <-timer.C:
		}
	}
}
