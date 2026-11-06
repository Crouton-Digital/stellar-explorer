package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/jackc/pgx/v5"
)

func saveEvents(ctx context.Context, tx pgx.Tx, transaction domain.Transaction) error {
	for _, event := range transaction.Events {
		if event.TransactionHash != transaction.Hash ||
			event.LedgerSequence != transaction.LedgerSequence {
			return fmt.Errorf("event belongs to another transaction")
		}

		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}

		topics, err := json.Marshal(event.Topics)
		if err != nil {
			return err
		}

		value, err := json.Marshal(event.Value)
		if err != nil {
			return err
		}

		result, err := tx.Exec(ctx, `
			INSERT INTO
			    contract_events (
			        network, transaction_hash, event_index, ledger_sequence,
			        contract_id, event_type, topics_json, value_json, raw_xdr, payload, event_scope
			    )
			VALUES
			    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (network, transaction_hash, event_index) DO UPDATE
			SET
			    payload = excluded.payload,
			    event_scope = excluded.event_scope
			WHERE
			    contract_events.raw_xdr = excluded.raw_xdr
			    AND contract_events.contract_id = excluded.contract_id
			    AND contract_events.event_type = excluded.event_type
			    AND contract_events.ledger_sequence = excluded.ledger_sequence
		`, transaction.Network, transaction.Hash, event.Index, event.LedgerSequence,
			event.ContractID, event.Type, topics, value, event.RawXDR, payload, event.Scope)
		if err != nil {
			return err
		}

		if result.RowsAffected() != 1 {
			return fmt.Errorf("retained event %d disagrees with source metadata", event.Index)
		}
	}

	return nil
}

func saveFeeEvents(ctx context.Context, tx pgx.Tx, transaction domain.Transaction) error {
	for _, event := range transaction.Fees.Events {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}

		if _, err = tx.Exec(ctx, `
			INSERT INTO
			    fee_events (network, transaction_hash, event_index, account, amount, stage, payload)
			VALUES
			    ($1, $2, $3, $4, $5, $6, $7)
		`, transaction.Network, transaction.Hash, event.EventIndex, event.Account,
			event.Amount, event.Stage, payload); err != nil {
			return err
		}
	}

	return nil
}

// Migration 6 adds transaction events and fees without changing existing event/cursor keys.
func backfillTransactionEvents(ctx context.Context, tx pgx.Tx) error {
	var ledger, index uint32
	for {
		rows, err := tx.Query(ctx, `
			SELECT
			    t.payload,
			    l.protocol_version
			FROM
			    transactions t
			    JOIN ledgers l ON l.network = t.network AND l.sequence = t.ledger_sequence
			WHERE
			    t.network = $1
			    AND (t.ledger_sequence, t.tx_index) > ($2, $3)
			ORDER BY
			    t.ledger_sequence,
			    t.tx_index
			LIMIT
			    100
		`, domain.Network, ledger, index)
		if err != nil {
			return err
		}

		type source struct {
			transaction domain.Transaction
			protocol    uint32
		}

		batch := []source{}
		for rows.Next() {
			var payload []byte
			var item source
			if err = rows.Scan(&payload, &item.protocol); err == nil {
				err = json.Unmarshal(payload, &item.transaction)
			}

			if err != nil {
				rows.Close()

				return err
			}

			batch = append(batch, item)
		}

		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}

		if len(batch) == 0 {
			return nil
		}

		for _, item := range batch {
			transaction := item.transaction
			if err = decoder.RebuildTransactionEvents(&transaction, item.protocol); err != nil {
				return fmt.Errorf("backfill events for transaction %s: %w", transaction.Hash, err)
			}

			if err = saveEvents(ctx, tx, transaction); err != nil {
				return err
			}

			if err = saveFeeEvents(ctx, tx, transaction); err != nil {
				return err
			}

			events, err := json.Marshal(transaction.Events)
			if err != nil {
				return err
			}

			fees, err := json.Marshal(transaction.Fees)
			if err != nil {
				return err
			}

			if _, err = tx.Exec(ctx, `
				UPDATE transactions
				SET
				    payload = payload || jsonb_build_object('events', $3::jsonb, 'fees', $4::jsonb)
				WHERE
				    network = $1
				    AND hash = $2
			`, transaction.Network, transaction.Hash, events, fees); err != nil {
				return err
			}

			ledger, index = transaction.LedgerSequence, transaction.TxIndex
		}
	}
}
