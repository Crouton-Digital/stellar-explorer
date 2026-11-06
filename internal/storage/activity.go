package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/jackc/pgx/v5"
)

const activityWindowLimit = 120

// Activity uses a repeatable snapshot, with the same filters and cursor as its list.
func (s *Store) Activity(
	ctx context.Context,
	kind string,
	cursor *Cursor,
	filters domain.Filters,
) (domain.Activity, error) {
	result := domain.Activity{
		Network:     domain.Network,
		Kind:        kind,
		WindowLimit: activityWindowLimit,
		Buckets:     []domain.ActivityBucket{},
		Breakdown:   []domain.ActivityCategory{},
	}

	table, column, category := "", "", ""
	var categories []string
	switch kind {
	case "transactions":
		table, column = "transactions t", "t.ledger_sequence"
		category = "CASE WHEN t.successful THEN 'success' ELSE 'failed' END"
		categories = []string{"success", "failed"}
	case "transfers":
		table, column, category = "token_transfers m", "m.ledger_sequence", "m.kind"
		categories = []string{"transfer", "mint", "burn", "clawback"}
	case "events":
		table, column = "contract_events e", "e.ledger_sequence"
		category = `CASE e.event_type
			WHEN 'ContractEventTypeContract' THEN 'contract'
			WHEN 'ContractEventTypeSystem' THEN 'system'
			WHEN 'ContractEventTypeDiagnostic' THEN 'diagnostic'
			ELSE 'other'
		END`
		categories = []string{"contract", "system", "diagnostic", "other"}
	default:
		return result, fmt.Errorf("unknown activity kind")
	}

	for _, key := range categories {
		result.Breakdown = append(result.Breakdown, domain.ActivityCategory{Key: key})
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return result, err
	}

	defer tx.Rollback(ctx)

	var dataset string
	var checkpoint uint32
	err = tx.QueryRow(ctx, `
		SELECT
		    metadata.dataset_id,
		    COALESCE(checkpoint.last_completed_ledger, 0)
		FROM
		    indexer_metadata metadata
		    LEFT JOIN indexer_checkpoints checkpoint USING (network)
		WHERE
		    metadata.network = $1
	`, domain.Network).Scan(&dataset, &checkpoint)
	if err != nil {
		return result, err
	}

	if cursor != nil {
		if cursor.Dataset != dataset || cursor.Kind != kind || cursor.Scope != "" ||
			cursorFilters(cursor) != filters || cursor.Snapshot > checkpoint {
			return result, ErrInvalidCursor
		}

		checkpoint = cursor.Snapshot
	}

	rows, err := tx.Query(ctx, `
		SELECT
		    sequence,
		    closed_at
		FROM (
		    SELECT
		        sequence,
		        closed_at
		    FROM
		        ledgers
		    WHERE
		        network = $1
		        AND sequence <= $2
		        AND sequence >= $3
		        AND ($4::bigint = 0 OR sequence <= $4)
		    ORDER BY
		        sequence DESC
		    LIMIT
		        $5
		) recent
		ORDER BY
		    sequence
	`, domain.Network, checkpoint, filters.MinLedger, filters.MaxLedger, activityWindowLimit)
	if err != nil {
		return result, err
	}

	bucketByLedger := map[uint32]int{}
	for rows.Next() {
		var sequence uint32
		var closedAt time.Time
		if err = rows.Scan(&sequence, &closedAt); err != nil {
			rows.Close()

			return result, err
		}

		if result.LedgerCount%10 == 0 {
			result.Buckets = append(result.Buckets, domain.ActivityBucket{
				FirstLedger: sequence,
			})
		}

		bucket := len(result.Buckets) - 1
		result.Buckets[bucket].LastLedger = sequence
		result.Buckets[bucket].ClosedAt = closedAt
		bucketByLedger[sequence] = bucket
		if result.FirstLedger == nil {
			result.FirstLedger = &sequence
			result.From = &closedAt
		}

		result.LastLedger = &sequence
		result.To = &closedAt
		result.LedgerCount++
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}

	if result.LedgerCount == 0 {
		return result, tx.Commit(ctx)
	}

	extra, args := filterSQL(kind, filters, []any{
		domain.Network,
		*result.FirstLedger,
		*result.LastLedger,
	})
	query := fmt.Sprintf(`
		SELECT
		    %s,
		    %s AS category,
		    count(*)
		FROM
		    %s
		WHERE
		    network = $1
		    AND %s BETWEEN $2 AND $3
		%s
		GROUP BY
		    1, 2
	`, column, category, table, column, extra)
	rows, err = tx.Query(ctx, query, args...)
	if err != nil {
		return result, err
	}

	for rows.Next() {
		var sequence uint32
		var key string
		var count uint64
		if err = rows.Scan(&sequence, &key, &count); err != nil {
			rows.Close()

			return result, err
		}

		bucket, exists := bucketByLedger[sequence]
		if !exists {
			rows.Close()

			return result, fmt.Errorf("activity record is outside indexed ledgers")
		}

		result.Total += count
		result.Buckets[bucket].Count += count
		for index := range result.Breakdown {
			if result.Breakdown[index].Key == key {
				result.Breakdown[index].Count += count
				break
			}
		}
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}

	return result, tx.Commit(ctx)
}
