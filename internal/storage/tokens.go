package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/jackc/pgx/v5"
)

func saveTokenActivity(ctx context.Context, tx pgx.Tx, transaction domain.Transaction) error {
	if !transaction.Successful && len(transaction.TokenTransfers) > 0 {
		return fmt.Errorf("failed transaction cannot contain token movements")
	}

	for _, movement := range transaction.TokenTransfers {
		if movement.TransactionHash != transaction.Hash ||
			movement.LedgerSequence != transaction.LedgerSequence ||
			movement.TxIndex != transaction.TxIndex ||
			movement.Asset.Network != transaction.Network {
			return fmt.Errorf("token movement belongs to another transaction")
		}

		asset, e := json.Marshal(movement.Asset)
		if e != nil {
			return e
		}

		if _, e = tx.Exec(ctx, `
			INSERT INTO
			    assets (network, contract_id, kind, first_ledger, payload)
			VALUES
			    ($1, $2, $3, $4, $5)
			ON CONFLICT (network, contract_id) DO UPDATE
			SET
			    first_ledger = LEAST(assets.first_ledger, excluded.first_ledger),
			    kind = CASE
			        WHEN assets.kind = 'sac' THEN assets.kind
			        ELSE excluded.kind
			    END,
			    payload = (
			        CASE
			            WHEN assets.kind = 'sac' THEN assets.payload
			            ELSE excluded.payload
			        END
			    ) || jsonb_build_object(
			        'first_observed_ledger', LEAST(assets.first_ledger, excluded.first_ledger)
			    )
		`, transaction.Network, movement.Asset.ContractID, movement.Asset.Kind,
			movement.Asset.FirstObservedLedger, asset); e != nil {
			return e
		}

		payload, e := json.Marshal(movement)
		if e != nil {
			return e
		}

		if _, e = tx.Exec(ctx, `
			INSERT INTO
			    token_transfers (
			        network, transaction_hash, event_index, ledger_sequence,
			        tx_index, op_index, contract_id, kind, from_address, to_address, amount, payload
			    )
			VALUES
			    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, transaction.Network, transaction.Hash, movement.EventIndex, movement.LedgerSequence,
			movement.TxIndex, movement.OpIndex, movement.Asset.ContractID, movement.Kind,
			movement.From, movement.To, movement.Amount, payload); e != nil {
			return e
		}
	}

	return nil
}

// Migration 3 rebuilds derived token data under the ingestion lock in the same transaction.
// Raw source data, checkpoint, dataset ID, and processing counters remain unchanged.
func backfillTokens(ctx context.Context, tx pgx.Tx) error {
	lastHash := ""
	for {
		rows, e := tx.Query(ctx, `
			SELECT
			    payload
			FROM
			    transactions
			WHERE
			    network = $1
			    AND hash > $2
			ORDER BY
			    hash
			LIMIT
			    100
		`, domain.Network, lastHash)
		if e != nil {
			return e
		}

		batch := []domain.Transaction{}
		for rows.Next() {
			var payload []byte
			if e = rows.Scan(&payload); e != nil {
				rows.Close()

				return e
			}

			var transaction domain.Transaction
			if e = json.Unmarshal(payload, &transaction); e != nil {
				rows.Close()

				return e
			}

			batch = append(batch, transaction)
		}

		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}

		if len(batch) == 0 {
			return nil
		}

		for _, transaction := range batch {
			if e = decoder.RebuildTokenActivity(&transaction); e != nil {
				return fmt.Errorf("backfill transaction %s: %w", transaction.Hash, e)
			}

			if e = saveTokenActivity(ctx, tx, transaction); e != nil {
				return e
			}

			movements, e := json.Marshal(transaction.TokenTransfers)
			if e != nil {
				return e
			}

			participants, e := json.Marshal(transaction.Participants)
			if e != nil {
				return e
			}

			if _, e = tx.Exec(ctx, `
				UPDATE transactions
				SET
				    payload = payload || jsonb_build_object(
				        'token_transfers', $3::jsonb,
				        'participants', $4::jsonb
				    )
				WHERE
				    network = $1
				    AND hash = $2
			`, transaction.Network, transaction.Hash, movements, participants); e != nil {
				return e
			}

			for _, address := range transaction.Participants {
				if _, e = tx.Exec(ctx, `
					INSERT INTO
					    transaction_accounts (network, address, transaction_hash)
					VALUES
					    ($1, $2, $3)
					ON CONFLICT DO NOTHING
				`, transaction.Network, address, transaction.Hash); e != nil {
					return e
				}
			}

			lastHash = transaction.Hash
		}
	}
}
