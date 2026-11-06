package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Interpretation upgrades change only the interpretation and its decoder version.
func backfillInterpretation(ctx context.Context, tx pgx.Tx) error {
	var ledger, index uint32
	for {
		rows, err := tx.Query(ctx, `
			SELECT
			    payload,
			    network,
			    hash,
			    ledger_sequence,
			    tx_index,
			    source_account,
			    successful,
			    created_at,
			    envelope_xdr,
			    result_xdr,
			    meta_xdr
			FROM
			    transactions
			WHERE
			    network = $1
			    AND (ledger_sequence, tx_index) > ($2, $3)
			    AND interpretation_version < $4
			ORDER BY
			    ledger_sequence,
			    tx_index
			LIMIT
			    100
		`, domain.Network, ledger, index, decoder.InterpretationVersion)
		if err != nil {
			return err
		}

		batch := []domain.Transaction{}
		for rows.Next() {
			var payload []byte
			var transaction, stored domain.Transaction
			if err = rows.Scan(
				&payload,
				&stored.Network,
				&stored.Hash,
				&stored.LedgerSequence,
				&stored.TxIndex,
				&stored.SourceAccount,
				&stored.Successful,
				&stored.CreatedAt,
				&stored.EnvelopeXDR,
				&stored.ResultXDR,
				&stored.MetaXDR,
			); err == nil {
				err = json.Unmarshal(payload, &transaction)
			}

			if err == nil &&
				(transaction.Network != stored.Network || transaction.Hash != stored.Hash ||
					transaction.LedgerSequence != stored.LedgerSequence || transaction.TxIndex != stored.TxIndex) {
				err = fmt.Errorf(
					"interpretation payload identity conflicts with stored transaction %s",
					stored.Hash,
				)
			}

			if err == nil && (transaction.SourceAccount != stored.SourceAccount ||
				transaction.Successful != stored.Successful || !transaction.CreatedAt.Equal(stored.CreatedAt) ||
				transaction.EnvelopeXDR != stored.EnvelopeXDR || transaction.ResultXDR != stored.ResultXDR ||
				transaction.MetaXDR != stored.MetaXDR) {
				err = fmt.Errorf(
					"interpretation payload source conflicts with stored transaction %s",
					stored.Hash,
				)
			}

			if err != nil {
				rows.Close()

				return err
			}

			batch = append(batch, transaction)
		}

		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}

		if len(batch) == 0 {
			return nil
		}

		updates := &pgx.Batch{}
		for _, transaction := range batch {
			if err = decoder.RebuildInterpretation(&transaction); err != nil {
				return fmt.Errorf("backfill interpretation for %s: %w", transaction.Hash, err)
			}

			payload, err := json.Marshal(transaction.Interpretation)
			if err != nil {
				return err
			}

			updates.Queue(`
				UPDATE transactions
				SET
				    payload = payload || jsonb_build_object('interpretation', $3::jsonb),
				    interpretation_version = $4
				WHERE
				    network = $1
				    AND hash = $2
			`, transaction.Network, transaction.Hash, payload,
				transaction.Interpretation.Version)

			ledger, index = transaction.LedgerSequence, transaction.TxIndex
		}

		results := tx.SendBatch(ctx, updates)
		for _, transaction := range batch {
			updated, writeErr := results.Exec()
			if writeErr == nil && updated.RowsAffected() != 1 {
				writeErr = fmt.Errorf(
					"expected one interpretation update, got %d",
					updated.RowsAffected(),
				)
			}

			if writeErr != nil {
				_ = results.Close()

				return fmt.Errorf("save interpretation for %s: %w", transaction.Hash, writeErr)
			}
		}

		if err = results.Close(); err != nil {
			return fmt.Errorf("finish interpretation batch: %w", err)
		}
	}
}
