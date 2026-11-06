package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/jackc/pgx/v5"
)

func saveContracts(ctx context.Context, tx pgx.Tx, transaction domain.Transaction) error {
	for _, observed := range transaction.Contracts {
		if observed.Network != transaction.Network ||
			observed.FirstObservedLedger != transaction.LedgerSequence ||
			observed.LastObservedLedger != transaction.LedgerSequence ||
			(!transaction.Successful && observed.ExecutableSource != nil) {
			return fmt.Errorf("invalid contract observation")
		}

		var previous []byte
		err := tx.QueryRow(ctx, `
			SELECT
			    payload
			FROM
			    contracts
			WHERE
			    network = $1
			    AND contract_id = $2
		`, observed.Network, observed.ContractID).Scan(&previous)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		var merged domain.Contract
		if err == nil {
			if err = json.Unmarshal(previous, &merged); err != nil {
				return err
			}

			merged.FirstObservedLedger = min(
				merged.FirstObservedLedger,
				observed.FirstObservedLedger,
			)
			merged.LastObservedLedger = max(merged.LastObservedLedger, observed.LastObservedLedger)
			if newerExecutable(observed, merged) {
				merged.Kind = observed.Kind
				merged.WasmHash = observed.WasmHash
				merged.ExecutableSource = observed.ExecutableSource
				merged.ExecutableLedger = observed.ExecutableLedger
				merged.ExecutableTxIndex = observed.ExecutableTxIndex
			}

			if observed.CreationLedger != nil && (merged.CreationLedger == nil ||
				*observed.CreationLedger < *merged.CreationLedger) {
				merged.CreationLedger = observed.CreationLedger
				merged.CreationTransaction = observed.CreationTransaction
				merged.Creator = observed.Creator
			}
		} else {
			merged = observed
		}

		payload, err := json.Marshal(merged)
		if err != nil {
			return err
		}

		if _, err = tx.Exec(ctx, `
			INSERT INTO
			    contracts (network, contract_id, payload)
			VALUES
			    ($1, $2, $3)
			ON CONFLICT (network, contract_id) DO UPDATE
			SET
			    payload = excluded.payload
		`, merged.Network, merged.ContractID, payload); err != nil {
			return err
		}

		if _, err = tx.Exec(ctx, `
			INSERT INTO
			    transaction_contracts (network, contract_id, transaction_hash)
			VALUES
			    ($1, $2, $3)
			ON CONFLICT DO NOTHING
		`, transaction.Network, observed.ContractID, transaction.Hash); err != nil {
			return err
		}
	}

	return nil
}

func newerExecutable(observed, previous domain.Contract) bool {
	if observed.ExecutableLedger == nil || observed.ExecutableTxIndex == nil {
		return false
	}

	// An instance entry is stronger evidence than a derived asset identity.
	if previous.ExecutableSource != nil && *previous.ExecutableSource == "instance_entry" &&
		observed.ExecutableSource != nil && *observed.ExecutableSource == "asset_contract_id" {
		return false
	}

	if previous.ExecutableLedger == nil || previous.ExecutableTxIndex == nil {
		return true
	}

	return *observed.ExecutableLedger > *previous.ExecutableLedger ||
		(*observed.ExecutableLedger == *previous.ExecutableLedger &&
			*observed.ExecutableTxIndex > *previous.ExecutableTxIndex)
}

// Migration 4 derives metadata without modifying saved envelope, result, or metadata XDR.
func backfillContracts(ctx context.Context, tx pgx.Tx) error {
	var ledger, index uint32
	for {
		rows, err := tx.Query(ctx, `
			SELECT
			    payload
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
			var payload []byte
			if err = rows.Scan(&payload); err != nil {
				rows.Close()

				return err
			}

			var transaction domain.Transaction
			if err = json.Unmarshal(payload, &transaction); err != nil {
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

		for _, transaction := range batch {
			if err = decoder.RebuildContractMetadata(&transaction); err != nil {
				return fmt.Errorf("backfill contract transaction %s: %w", transaction.Hash, err)
			}

			if err = saveContracts(ctx, tx, transaction); err != nil {
				return err
			}

			contracts, err := json.Marshal(transaction.Contracts)
			if err != nil {
				return err
			}

			if _, err = tx.Exec(ctx, `
				UPDATE transactions
				SET
				    payload = payload || jsonb_build_object('contracts', $3::jsonb)
				WHERE
				    network = $1
				    AND hash = $2
			`, transaction.Network, transaction.Hash, contracts); err != nil {
				return err
			}

			ledger, index = transaction.LedgerSequence, transaction.TxIndex
		}
	}
}
