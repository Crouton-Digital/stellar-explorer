package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func saveNFTActivity(ctx context.Context, tx pgx.Tx, transaction domain.Transaction) error {
	if len(transaction.NFTTransfers) == 0 {
		return nil
	}

	evidence := transaction
	if err := decoder.RebuildNFTActivity(&evidence); err != nil {
		return err
	}

	if !reflect.DeepEqual(evidence.NFTTransfers, transaction.NFTTransfers) {
		return fmt.Errorf("NFT activity conflicts with committed source evidence")
	}

	sources, err := nftSourceEvents(transaction.MetaXDR)
	if err != nil {
		return err
	}

	for _, movement := range transaction.NFTTransfers {
		var parentMatches bool
		if err = tx.QueryRow(ctx, `
			SELECT
			    EXISTS (
			        SELECT
			            1
			        FROM
			            contract_events
			        WHERE
			            network = $1
			            AND transaction_hash = $2
			            AND event_index = $3
			            AND raw_xdr = $4
			            AND event_scope = 'operation'
			            AND contract_id = $5
			            AND ledger_sequence = $6
			    )
		`, transaction.Network, transaction.Hash, movement.EventIndex,
			sources[movement.EventIndex], movement.ContractID, transaction.LedgerSequence,
		).Scan(&parentMatches); err != nil {
			return err
		}

		if !parentMatches {
			return fmt.Errorf("NFT parent event %d conflicts with source", movement.EventIndex)
		}

		collection := domain.NFTCollection{
			Network:             transaction.Network,
			ContractID:          movement.ContractID,
			Verification:        "event_shape",
			FirstObservedLedger: transaction.LedgerSequence,
		}
		payload, err := json.Marshal(collection)
		if err != nil {
			return err
		}

		if _, err = tx.Exec(ctx, `
			INSERT INTO
			    nft_collections (network, contract_id, first_ledger, payload)
			VALUES
			    ($1, $2, $3, $4)
			ON CONFLICT (network, contract_id) DO UPDATE
			SET
			    first_ledger = LEAST(nft_collections.first_ledger, excluded.first_ledger),
			    payload = nft_collections.payload || jsonb_build_object(
			        'first_observed_ledger', LEAST(nft_collections.first_ledger, excluded.first_ledger)
			    )
		`, collection.Network, collection.ContractID, collection.FirstObservedLedger, payload); err != nil {
			return err
		}

		payload, err = json.Marshal(movement)
		if err != nil {
			return err
		}

		if _, err = tx.Exec(ctx, `
			INSERT INTO
			    nft_transfers (
			        network, transaction_hash, event_index, ledger_sequence,
			        tx_index, op_index, contract_id, token_id, token_id_type,
			        kind, from_address, to_address, payload
			    )
			VALUES
			    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, transaction.Network, movement.TransactionHash, movement.EventIndex,
			movement.LedgerSequence, movement.TxIndex, movement.OpIndex, movement.ContractID,
			movement.TokenID, movement.TokenIDType, movement.Kind, movement.From, movement.To,
			payload); err != nil {
			return err
		}

		for _, address := range []string{movement.From, movement.To} {
			if !strkey.IsValidEd25519PublicKey(address) {
				continue
			}

			if _, err = tx.Exec(ctx, `
				INSERT INTO
				    transaction_accounts (network, address, transaction_hash)
				VALUES
				    ($1, $2, $3)
				ON CONFLICT DO NOTHING
			`, transaction.Network, address, transaction.Hash); err != nil {
				return err
			}
		}
	}

	return nil
}

func nftSourceEvents(encoded string) (map[int]string, error) {
	var meta xdr.TransactionMeta
	if err := xdr.SafeUnmarshalBase64(encoded, &meta); err != nil {
		return nil, err
	}

	groups := [][]xdr.ContractEvent{}
	switch meta.V {
	case 3:
		if soroban := meta.MustV3().SorobanMeta; soroban != nil {
			groups = append(groups, soroban.Events)
		}
	case 4:
		for _, operation := range meta.MustV4().Operations {
			groups = append(groups, operation.Events)
		}
	}

	result := map[int]string{}
	index := 0
	for _, group := range groups {
		for _, event := range group {
			encoded, err := xdr.MarshalBase64(event)
			if err != nil {
				return nil, err
			}

			result[index] = encoded
			index++
		}
	}

	return result, nil
}

func (s *Store) NFTCollection(
	ctx context.Context,
	contract string,
	snapshot uint32,
) (domain.NFTCollectionView, error) {
	result := domain.NFTCollectionView{
		NFTCollection: domain.NFTCollection{
			Network:      domain.Network,
			ContractID:   contract,
			Verification: "event_shape",
		},
	}
	if snapshot == 0 {
		var err error
		snapshot, err = s.Checkpoint(ctx)
		if err != nil {
			return result, err
		}
	}

	err := s.Pool.QueryRow(ctx, `
		SELECT
		    min(ledger_sequence),
		    count(DISTINCT token_id),
		    count(*)
		FROM
		    nft_transfers
		WHERE
		    network = $1
		    AND contract_id = $2
		    AND ledger_sequence <= $3
		HAVING
		    count(*) > 0
	`, domain.Network, contract, snapshot).Scan(&result.FirstObservedLedger,
		&result.ObservedTokenCount, &result.TransferCount)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}

	return result, err
}

func (s *Store) NFTSnapshot(ctx context.Context) (Cursor, error) {
	result := Cursor{Version: 1}
	err := s.Pool.QueryRow(ctx, `
		SELECT
		    m.dataset_id,
		    COALESCE(c.last_completed_ledger, 0)
		FROM
		    indexer_metadata m
		    LEFT JOIN indexer_checkpoints c USING (network)
		WHERE
		    m.network = $1
	`, domain.Network).Scan(&result.Dataset, &result.Snapshot)

	return result, err
}

func (s *Store) NFTItem(
	ctx context.Context,
	contract, token string,
	snapshot uint32,
) (domain.NFTItem, error) {
	result := domain.NFTItem{
		Network:      domain.Network,
		ContractID:   contract,
		TokenID:      token,
		Verification: "event_shape",
	}
	if snapshot == 0 {
		var err error
		snapshot, err = s.Checkpoint(ctx)
		if err != nil {
			return result, err
		}
	}

	err := s.Pool.QueryRow(ctx, `
		SELECT
		    min(ledger_sequence),
		    count(*)
		FROM
		    nft_transfers
		WHERE
		    network = $1
		    AND contract_id = $2
		    AND token_id = $3::numeric
		    AND ledger_sequence <= $4
		HAVING
		    count(*) > 0
	`, domain.Network, contract, token, snapshot).Scan(&result.FirstObservedLedger,
		&result.TransferCount)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}

	return result, err
}
