package ingestion_test

import (
	"context"
	"errors"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/ingestion"
	"github.com/crouton/stellar-explorer/internal/stellar"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

type source struct {
	passphrase string
	oldest     uint32
	latest     uint32
	fail       bool
	requested  []uint32
}

func (s *source) Network(context.Context) (string, error) {
	if s.fail {
		return "", errors.New("offline")
	}

	return s.passphrase, nil
}

func (s *source) Health(context.Context) (stellar.Health, error) {
	return stellar.Health{
		Oldest: s.oldest,
		Latest: s.latest,
		Status: "healthy",
	}, nil
}

func (s *source) Ledgers(_ context.Context, n uint32, limit int) ([]stellar.RemoteLedger, error) {
	s.requested = append(s.requested, n)
	ls := []stellar.RemoteLedger{}

	for j := 0; j < limit; j++ {
		ls = append(ls, testutil.Remote(n+uint32(j), nil))
	}

	return ls, nil
}

type repo struct {
	cp    uint32
	fail  bool
	saved []uint32
}

func (r *repo) Checkpoint(context.Context) (uint32, error) {
	return r.cp, nil
}

func (r *repo) Observe(context.Context, uint32, *string) error {
	return nil
}

func (r *repo) Save(_ context.Context, b domain.Bundle) error {
	if r.fail {
		return errors.New("DB unavailable")
	}

	r.cp = b.Ledger.Sequence
	r.saved = append(r.saved, r.cp)

	return nil
}

func TestResumeAndRetryDoNotSkipLedger(t *testing.T) {
	s := &source{
		passphrase: domain.Passphrase,
		oldest:     1,
		latest:     12,
	}
	r := &repo{
		cp:   10,
		fail: true,
	}
	i := ingestion.Indexer{
		Source: s,
		Store:  r,
		Batch:  2,
	}
	if _, e := i.Step(context.Background()); e == nil || r.cp != 10 {
		t.Fatal("failed ledger advanced checkpoint")
	}

	r.fail = false
	// New indexer instance simulates restart against the same committed repository.
	i = ingestion.Indexer{
		Source: s,
		Store:  r,
		Batch:  2,
	}
	if n, e := i.Step(context.Background()); e != nil || n != 2 || r.cp != 12 {
		t.Fatalf("resume: %d %v cp=%d", n, e, r.cp)
	}

	if len(s.requested) != 2 || s.requested[0] != 11 || s.requested[1] != 11 {
		t.Fatalf("skipped failed ledger: %v", s.requested)
	}
}

func TestUnavailableHistoryAndWrongNetworkAreFatal(t *testing.T) {
	for _, tc := range []struct {
		name   string
		pass   string
		cp     uint32
		start  uint32
		oldest uint32
		latest uint32
	}{
		{
			"gap",
			domain.Passphrase,
			10,
			0,
			12,
			20,
		},
		{
			"future",
			domain.Passphrase,
			30,
			0,
			12,
			20,
		},
		{
			"network",
			"mainnet",
			0,
			12,
			12,
			20,
		},
		{
			"explicit-old",
			domain.Passphrase,
			0,
			1,
			12,
			20,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &source{
				passphrase: tc.pass,
				oldest:     tc.oldest,
				latest:     tc.latest,
			}
			r := &repo{
				cp: tc.cp,
			}
			i := ingestion.Indexer{
				Source: s,
				Store:  r,
				Start:  tc.start,
				Batch:  10,
			}
			_, e := i.Step(context.Background())
			var fatal *ingestion.FatalError
			if !errors.As(e, &fatal) || len(r.saved) > 0 {
				t.Fatalf("expected fatal error, got %v", e)
			}
		})
	}
}

func TestBootstrapAndCaughtUp(t *testing.T) {
	s := &source{
		passphrase: domain.Passphrase,
		oldest:     150,
		latest:     200,
	}
	r := &repo{}
	i := ingestion.Indexer{
		Source: s,
		Store:  r,
		Batch:  10,
	}
	if _, e := i.Step(context.Background()); e != nil {
		t.Fatal(e)
	}

	if s.requested[0] != 150 {
		t.Fatal("bootstrap before available history")
	}

	r.cp = 200
	if n, e := i.Step(context.Background()); e != nil || n != 0 {
		t.Fatal("caught up indexer should wait")
	}
}
