package storage

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/crouton/stellar-explorer/internal/domain"
)

func ValidNFTTokenID(value string) bool {
	if len(value) == 0 || len(value) > 78 || (len(value) > 1 && value[0] == '0') {
		return false
	}

	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}

	number, valid := new(big.Int).SetString(value, 10)
	return valid && number.BitLen() <= 256
}

func nftTransferList(kind string) bool {
	switch kind {
	case "nft_transfers", "collection_nft_transfers", "nft_token_transfers",
		"account_nft_transfers", "contract_nft_transfers":
		return true
	}

	return false
}

func nftListQuery(
	kind, scope string,
	cursor Cursor,
	limit int,
	filters domain.Filters,
) (string, []any, error) {
	if kind == "nft_collections" {
		return `
			SELECT
			    jsonb_build_object(
			        'network', $1::text,
			        'contract_id', contract_id,
			        'verification', 'event_shape',
			        'first_observed_ledger', min(ledger_sequence),
			        'observed_token_count', count(DISTINCT token_id),
			        'transfer_count', count(*)
			    ),
			    min(ledger_sequence),
			    0,
			    contract_id
			FROM
			    nft_transfers
			WHERE
			    network = $1
			    AND ledger_sequence <= $2
			GROUP BY
			    contract_id
			HAVING
			    $3::bigint = 0
			    OR (min(ledger_sequence), contract_id) < ($3, $4)
			ORDER BY
			    min(ledger_sequence) DESC,
			    contract_id DESC
			LIMIT
			    $5
		`, []any{domain.Network, cursor.Snapshot, cursor.Ledger, cursor.Hash, limit + 1}, nil
	}

	args := []any{domain.Network, cursor.Snapshot, cursor.Ledger,
		cursor.TxIndex, cursor.Index, limit + 1}
	var conditions strings.Builder
	add := func(condition string, value any) {
		args = append(args, value)
		conditions.WriteString(fmt.Sprintf(condition, len(args)))
	}
	switch kind {
	case "collection_nft_transfers":
		add(" AND m.contract_id = $%d\n", scope)
	case "nft_token_transfers":
		contract, token, valid := strings.Cut(scope, ":")
		if !valid || !ValidNFTTokenID(token) {
			return "", nil, fmt.Errorf("invalid NFT token scope")
		}

		add(" AND m.contract_id = $%d\n", contract)
		add(" AND m.token_id = $%d::numeric\n", token)
	case "account_nft_transfers":
		args = append(args, scope)
		conditions.WriteString(fmt.Sprintf(
			" AND (m.from_address = $%d OR m.to_address = $%d)\n", len(args), len(args)))
	case "contract_nft_transfers":
		args = append(args, scope)
		conditions.WriteString(fmt.Sprintf(
			" AND (m.contract_id = $%d OR m.from_address = $%d OR m.to_address = $%d)\n",
			len(args), len(args), len(args)))
	}

	for _, filter := range []struct {
		condition string
		value     string
	}{
		{" AND m.contract_id = $%d\n", filters.Contract},
		{" AND m.token_id = $%d::numeric\n", filters.NFTTokenID},
		{" AND m.kind = $%d\n", filters.NFTKind},
		{" AND m.from_address = $%d\n", filters.From},
		{" AND m.to_address = $%d\n", filters.To},
	} {
		if filter.value != "" {
			add(filter.condition, filter.value)
		}
	}

	if filters.MinLedger != 0 {
		add(" AND m.ledger_sequence >= $%d\n", filters.MinLedger)
	}

	if filters.MaxLedger != 0 {
		add(" AND m.ledger_sequence <= $%d\n", filters.MaxLedger)
	}

	return `
		SELECT
		    payload,
		    ledger_sequence,
		    event_index,
		    transaction_hash,
		    tx_index
		FROM
		    nft_transfers m
		WHERE
		    network = $1
		    AND ledger_sequence <= $2
		    AND (
		        $3::bigint = 0
		        OR (ledger_sequence, tx_index, event_index) < ($3, $4, $5)
		    )
	` + conditions.String() + `
		ORDER BY
		    ledger_sequence DESC,
		    tx_index DESC,
		    event_index DESC
		LIMIT
		    $6
	`, args, nil
}
