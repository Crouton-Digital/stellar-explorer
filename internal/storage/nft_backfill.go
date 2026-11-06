package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/jackc/pgx/v5"
)

// NFT upgrades add source-derived activity without rebuilding existing interpretations.
func backfillNFTActivity(ctx context.Context, tx pgx.Tx) error {
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
			ORDER BY
			    ledger_sequence,
			    tx_index
			LIMIT
			    100
		`, domain.Network, ledger, index)
		if err != nil {
			return err
		}

		batch := []domain.Transaction{}
		for rows.Next() {
			var raw []byte
			var transaction, source domain.Transaction
			if err = rows.Scan(
				&raw,
				&source.Network,
				&source.Hash,
				&source.LedgerSequence,
				&source.TxIndex,
				&source.SourceAccount,
				&source.Successful,
				&source.CreatedAt,
				&source.EnvelopeXDR,
				&source.ResultXDR,
				&source.MetaXDR,
			); err == nil {
				err = json.Unmarshal(raw, &transaction)
			}

			if err == nil && !nftStoredSourceMatches(transaction, source) {
				err = fmt.Errorf(
					"NFT payload source conflicts with stored transaction %s",
					source.Hash,
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
			if err = decoder.RebuildNFTActivity(&transaction); err != nil {
				return fmt.Errorf("backfill NFT activity for %s: %w", transaction.Hash, err)
			}

			if err = saveNFTActivity(ctx, tx, transaction); err != nil {
				return fmt.Errorf("save NFT activity for %s: %w", transaction.Hash, err)
			}

			payload, err := json.Marshal(transaction.NFTTransfers)
			if err != nil {
				return err
			}

			updates.Queue(`
				UPDATE transactions
				SET
				    payload = payload || jsonb_build_object('nft_transfers', $3::jsonb)
				WHERE
				    network = $1
				    AND hash = $2
			`, transaction.Network, transaction.Hash, payload)
			ledger, index = transaction.LedgerSequence, transaction.TxIndex
		}

		results := tx.SendBatch(ctx, updates)
		for _, transaction := range batch {
			updated, err := results.Exec()
			if err == nil && updated.RowsAffected() != 1 {
				err = fmt.Errorf("expected one NFT payload update, got %d", updated.RowsAffected())
			}

			if err != nil {
				_ = results.Close()

				return fmt.Errorf("update NFT payload for %s: %w", transaction.Hash, err)
			}
		}

		if err = results.Close(); err != nil {
			return fmt.Errorf("finish NFT payload batch: %w", err)
		}
	}
}

func nftStoredSourceMatches(transaction, source domain.Transaction) bool {
	return transaction.Network == source.Network &&
		transaction.Hash == source.Hash &&
		transaction.LedgerSequence == source.LedgerSequence &&
		transaction.TxIndex == source.TxIndex &&
		transaction.SourceAccount == source.SourceAccount &&
		transaction.Successful == source.Successful &&
		transaction.CreatedAt.Equal(source.CreatedAt) &&
		transaction.EnvelopeXDR == source.EnvelopeXDR &&
		transaction.ResultXDR == source.ResultXDR &&
		transaction.MetaXDR == source.MetaXDR
}
