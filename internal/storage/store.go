package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found in indexed range")

var ErrContinuity = errors.New("ledger continuity violated; check Testnet reset or RPC history gap")

type Store struct {
	Pool *pgxpool.Pool
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	c, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL configuration")
	}

	c.MaxConns = 10
	c.ConnConfig.ConnectTimeout = 5 * time.Second
	p, e := pgxpool.NewWithConfig(ctx, c)
	if e != nil {
		return nil, e
	}

	s := &Store{p}
	if e = s.Ping(ctx); e != nil {
		p.Close()

		return nil, e
	}

	return s, nil
}

func (s *Store) Close() {
	s.Pool.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.Pool.Ping(ctx)
}

func (s *Store) Migrate(ctx context.Context) error {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return e
	}

	defer tx.Rollback(ctx)

	if _, e = tx.Exec(ctx, `
		SELECT
		    pg_advisory_xact_lock(834501)
	`); e != nil {
		return e
	}

	if _, e = tx.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (version integer PRIMARY KEY)
	`); e != nil {
		return e
	}

	for _, migration := range migrations.All {
		var exists bool
		if e = tx.QueryRow(ctx, `
			SELECT
			    EXISTS (
			        SELECT
			            1
			        FROM
			            schema_migrations
			        WHERE
			            version = $1
			    )
		`, migration.Version).Scan(&exists); e != nil {
			return e
		}

		if exists {
			continue
		}

		if _, e = tx.Exec(ctx, migration.SQL); e != nil {
			return e
		}

		if migration.Version == 3 {
			if e = backfillTokens(ctx, tx); e != nil {
				return e
			}
		}

		if migration.Version == 4 {
			if e = backfillContracts(ctx, tx); e != nil {
				return e
			}
		}

		if migration.Version == 6 {
			if e = backfillTransactionEvents(ctx, tx); e != nil {
				return e
			}
		}

		if migration.Version >= 7 && migration.Version <= 11 {
			if e = backfillInterpretation(ctx, tx); e != nil {
				return e
			}
		}

		if migration.Version == 12 {
			if e = backfillNFTActivity(ctx, tx); e != nil {
				return e
			}
		}

		if _, e = tx.Exec(ctx, `
			INSERT INTO
			    schema_migrations (version)
			VALUES
			    ($1)
			ON CONFLICT DO NOTHING
		`, migration.Version); e != nil {
			return e
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) Checkpoint(ctx context.Context) (uint32, error) {
	var n uint32
	e := s.Pool.QueryRow(ctx, `
		SELECT
		    last_completed_ledger
		FROM
		    indexer_checkpoints
		WHERE
		    network = $1
	`, domain.Network).Scan(&n)
	if errors.Is(e, pgx.ErrNoRows) {
		return 0, nil
	}

	return n, e
}

func (s *Store) Save(ctx context.Context, b domain.Bundle) error {
	l := b.Ledger
	if l.Network != domain.Network || l.Sequence == 0 || len(b.Transactions) != l.TransactionCount {
		return fmt.Errorf("incomplete or invalid ledger bundle")
	}

	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return e
	}

	defer tx.Rollback(ctx)

	if _, e = tx.Exec(ctx, `
		SELECT
		    pg_advisory_xact_lock(834502)
	`); e != nil {
		return e
	}

	var old string
	e = tx.QueryRow(ctx, `
		SELECT
		    hash
		FROM
		    ledgers
		WHERE
		    network = $1
		    AND sequence = $2
	`, l.Network, l.Sequence).Scan(&old)
	if e == nil {
		if old != l.Hash {
			return ErrContinuity
		}

		return nil
	}

	if !errors.Is(e, pgx.ErrNoRows) {
		return e
	}

	var cp uint32
	e = tx.QueryRow(ctx, `
		SELECT
		    last_completed_ledger
		FROM
		    indexer_checkpoints
		WHERE
		    network = $1
		FOR UPDATE
	`, l.Network).Scan(&cp)
	if e == nil {
		if l.Sequence != cp+1 {
			return ErrContinuity
		}

		var prev string
		if e = tx.QueryRow(ctx, `
			SELECT
			    hash
			FROM
			    ledgers
			WHERE
			    network = $1
			    AND sequence = $2
		`, l.Network, cp).Scan(&prev); e != nil {
			return e
		}

		if prev != l.PreviousHash {
			return ErrContinuity
		}
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return e
	}

	raw := l.RawXDR
	l.RawXDR = ""
	payload, e := json.Marshal(l)
	if e != nil {
		return e
	}

	if _, e = tx.Exec(ctx, `
		INSERT INTO
		    ledgers (network, sequence, hash, closed_at, protocol_version, raw_ref, payload)
		VALUES
		    ($1, $2, $3, $4, $5, $6, $7)
	`, l.Network, l.Sequence, l.Hash, l.ClosedAt, l.ProtocolVersion, raw, payload); e != nil {
		return e
	}

	for _, t := range b.Transactions {
		if t.Network != l.Network || t.LedgerSequence != l.Sequence {
			return fmt.Errorf("transaction belongs to another ledger")
		}

		p, e := json.Marshal(t)
		if e != nil {
			return e
		}

		_, e = tx.Exec(ctx, `
			INSERT INTO
			    transactions (
			        network,
			        hash,
			        ledger_sequence,
			        tx_index,
			        source_account,
			        successful,
			        fee_charged,
			        created_at,
			        envelope_xdr,
			        result_xdr,
			        meta_xdr,
			        payload,
			        interpretation_version
			    )
			VALUES
			    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`,
			t.Network,
			t.Hash,
			t.LedgerSequence,
			t.TxIndex,
			t.SourceAccount,
			t.Successful,
			t.FeeCharged,
			t.CreatedAt,
			t.EnvelopeXDR,
			t.ResultXDR,
			t.MetaXDR,
			p,
			t.Interpretation.Version,
		)
		if e != nil {
			return e
		}

		for _, o := range t.Operations {
			p, e := json.Marshal(o)
			if e != nil {
				return e
			}

			if _, e = tx.Exec(ctx, `
				INSERT INTO
				    operations
				VALUES
				    ($1, $2, $3, $4, $5, $6)
			`, t.Network, t.Hash, o.Index, o.Type, o.SourceAccount, p); e != nil {
				return e
			}
		}

		for _, c := range t.Calls {
			p, e := json.Marshal(c.Arguments)
			if e != nil {
				return e
			}

			if _, e = tx.Exec(ctx, `
				INSERT INTO
				    contract_calls
				VALUES
				    ($1, $2, $3, $4, $5, $6, $7)
			`, t.Network, t.Hash, c.Index, c.ContractID, c.FunctionName, p, c.DecodeStatus); e != nil {
				return e
			}
		}

		if e = saveEvents(ctx, tx, t); e != nil {
			return e
		}

		if e = saveFeeEvents(ctx, tx, t); e != nil {
			return e
		}

		if e = saveTokenActivity(ctx, tx, t); e != nil {
			return e
		}

		if e = saveNFTActivity(ctx, tx, t); e != nil {
			return e
		}

		if e = saveContracts(ctx, tx, t); e != nil {
			return e
		}

		for _, address := range t.Participants {
			if _, e = tx.Exec(ctx, `
				INSERT INTO
				    transaction_accounts
				VALUES
				    ($1, $2, $3)
				ON CONFLICT DO NOTHING
			`, t.Network, address, t.Hash); e != nil {
				return e
			}
		}
	}

	_, e = tx.Exec(ctx, `
		INSERT INTO
		    indexer_checkpoints (network, first_ledger, last_completed_ledger)
		VALUES
		    ($1, $2, $2)
		ON CONFLICT (network) DO UPDATE
		SET
		    last_completed_ledger = excluded.last_completed_ledger,
		    updated_at = now()
	`, l.Network, l.Sequence)
	if e != nil {
		return e
	}

	if _, e = tx.Exec(ctx, `
		INSERT INTO
		    indexer_status (network, processed_transactions)
		VALUES
		    ($1, $2)
		ON CONFLICT (network) DO UPDATE
		SET
		    processed_transactions = indexer_status.processed_transactions + excluded.processed_transactions
	`, l.Network, len(b.Transactions)); e != nil {
		return e
	}

	return tx.Commit(ctx)
}

func (s *Store) Observe(ctx context.Context, latest uint32, lastError *string) error {
	_, e := s.Pool.Exec(ctx, `
		INSERT INTO
		    indexer_status (network, latest_network_ledger, observed_at, last_error, error_count)
		VALUES
		    (
		        $1,
		        NULLIF($2::bigint, 0),
		        CASE
		            WHEN $2::bigint = 0 THEN NULL
		            ELSE now()
		        END,
		        $3,
		        CASE
		            WHEN $3::text IS NULL THEN 0
		            ELSE 1
		        END
		    )
		ON CONFLICT (network) DO UPDATE
		SET
		    latest_network_ledger = COALESCE(excluded.latest_network_ledger, indexer_status.latest_network_ledger),
		    observed_at = COALESCE(excluded.observed_at, indexer_status.observed_at),
		    last_error = excluded.last_error,
		    error_count = indexer_status.error_count + excluded.error_count
	`, domain.Network, latest, lastError)

	return e
}

func (s *Store) State(ctx context.Context) (domain.SyncState, error) {
	r := domain.SyncState{
		Network: domain.Network,
	}
	e := s.Pool.QueryRow(ctx, `
		SELECT
		    c.first_ledger,
		    c.last_completed_ledger,
		    s.latest_network_ledger,
		    s.observed_at,
		    s.last_error,
		    COALESCE(s.processed_transactions, 0),
		    COALESCE(s.error_count, 0)
		FROM
		    (
		        SELECT
		            $1::text AS network
		    ) n
		    LEFT JOIN indexer_checkpoints c USING (network)
		    LEFT JOIN indexer_status s USING (network)
	`, domain.Network).Scan(
		&r.FirstIndexedLedger,
		&r.LastIndexedLedger,
		&r.LatestNetworkLedger,
		&r.ObservedAt,
		&r.LastError,
		&r.ProcessedTransactions,
		&r.ErrorCount,
	)
	if r.LatestNetworkLedger != nil && *r.LatestNetworkLedger == 0 {
		r.LatestNetworkLedger = nil
	}

	if r.LastIndexedLedger != nil && r.LatestNetworkLedger != nil &&
		*r.LatestNetworkLedger >= *r.LastIndexedLedger {
		lag := *r.LatestNetworkLedger - *r.LastIndexedLedger
		r.Lag = &lag
	}

	return r, e
}

func (s *Store) Get(ctx context.Context, kind, key string) (json.RawMessage, error) {
	var q string

	switch kind {
	case "ledgers":
		q = `
			SELECT
			    payload
			FROM
			    ledgers
			WHERE
			    network = $1
			    AND sequence = $2::text::bigint
		`

	case "transactions":
		q = `
			SELECT
			    payload
			FROM
			    transactions
			WHERE
			    network = $1
			    AND hash = $2
		`

	case "assets", "contracts":
		q = `
			SELECT
			    payload
			FROM
			    ` + kind + `
			WHERE
			    network = $1
			    AND contract_id = $2
		`

	default:
		return nil, ErrNotFound
	}

	var p []byte
	e := s.Pool.QueryRow(ctx, q, domain.Network, key).Scan(&p)
	if errors.Is(e, pgx.ErrNoRows) {
		e = ErrNotFound
	}

	return p, e
}

func (s *Store) Exists(ctx context.Context, kind, address string) (bool, error) {
	var q string
	if kind == "accounts" {
		q = `
			SELECT
			    EXISTS (
			        SELECT
			            1
			        FROM
			            transaction_accounts
			        WHERE
			            network = $1
			            AND address = $2
			    )
		`
	} else {
		q = `
			SELECT
			    EXISTS (
			        SELECT
			            1
			        FROM
			            contracts
			        WHERE
			            network = $1
			            AND contract_id = $2
			    )
		`
	}

	var yes bool
	e := s.Pool.QueryRow(ctx, q, domain.Network, address).Scan(&yes)

	return yes, e
}
