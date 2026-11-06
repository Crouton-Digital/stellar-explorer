package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/crouton/stellar-explorer/internal/domain"
)

var ErrInvalidCursor = errors.New("cursor belongs to another dataset")

type Cursor struct {
	Version  int             `json:"v"`
	Dataset  string          `json:"dataset"`
	Kind     string          `json:"kind"`
	Scope    string          `json:"scope"`
	Snapshot uint32          `json:"snapshot"`
	Ledger   uint32          `json:"ledger"`
	Index    int             `json:"index"`
	TxIndex  uint32          `json:"tx_index,omitempty"`
	Hash     string          `json:"hash,omitempty"`
	Filters  *domain.Filters `json:"filters,omitempty"`
}

func ParseCursor(raw, kind, scope string, filters ...domain.Filters) (*Cursor, error) {
	if len(filters) > 1 {
		return nil, fmt.Errorf("expected one filter set")
	}

	if raw == "" {
		return nil, nil
	}

	if len(raw) > 2048 {
		return nil, fmt.Errorf("invalid cursor")
	}

	p, e := base64.RawURLEncoding.DecodeString(raw)
	if e != nil {
		return nil, fmt.Errorf("invalid cursor")
	}

	var c Cursor
	if e = json.Unmarshal(p, &c); e != nil || c.Version != 1 || c.Dataset == "" || c.Kind != kind ||
		c.Scope != scope ||
		c.Snapshot == 0 ||
		c.Ledger == 0 ||
		c.Ledger > c.Snapshot ||
		c.Index < 0 {
		return nil, fmt.Errorf("invalid cursor for this query")
	}

	if cursorFilters(&c) != selectedFilters(filters) {
		return nil, fmt.Errorf("cursor belongs to different filters")
	}

	if (kind == "transfers" || kind == "asset_transfers" || kind == "account_transfers" ||
		kind == "contract_transfers" || nftTransferList(kind)) && c.TxIndex == 0 {
		return nil, fmt.Errorf("token cursor requires a transaction index")
	}

	return &c, nil
}

func (c Cursor) Encode() string {
	p, _ := json.Marshal(c)

	return base64.RawURLEncoding.EncodeToString(p)
}

func (s *Store) List(
	ctx context.Context,
	kind, scope string,
	limit int,
	cursor *Cursor,
	filters ...domain.Filters,
) (domain.Page, error) {
	p := domain.Page{
		Items: []json.RawMessage{},
	}
	if len(filters) > 1 {
		return p, fmt.Errorf("expected one filter set")
	}

	f := selectedFilters(filters)
	var dataset string
	if e := s.Pool.QueryRow(ctx, `
		SELECT
		    dataset_id
		FROM
		    indexer_metadata
		WHERE
		    network = $1
	`, domain.Network).Scan(&dataset); e != nil {
		return p, e
	}

	if cursor != nil && (cursor.Dataset != dataset || cursor.Kind != kind ||
		cursor.Scope != scope || cursorFilters(cursor) != f) {
		return p, ErrInvalidCursor
	}

	checkpoint, err := s.Checkpoint(ctx)
	if err != nil {
		return p, err
	}

	if cursor != nil && cursor.Snapshot > checkpoint {
		return p, ErrInvalidCursor
	}

	c := Cursor{
		Version: 1,
		Dataset: dataset,
		Kind:    kind,
		Scope:   scope,
	}
	if f != (domain.Filters{}) {
		c.Filters = &f
	}
	if cursor != nil {
		c = *cursor
	} else {
		c.Snapshot = checkpoint
	}

	if c.Snapshot == 0 {
		return p, nil
	}

	var args []any
	var q string

	switch kind {
	case "nft_collections", "nft_transfers", "collection_nft_transfers", "nft_token_transfers",
		"account_nft_transfers", "contract_nft_transfers":
		var err error
		q, args, err = nftListQuery(kind, scope, c, limit, f)
		if err != nil {
			return p, err
		}

	case "assets":
		args = []any{domain.Network, c.Snapshot, c.Ledger, c.Hash, limit + 1}
		q = `
			SELECT
			    payload,
			    first_ledger,
			    0,
			    contract_id
			FROM
			    assets
			WHERE
			    network = $1
			    AND first_ledger <= $2
			    AND (
			        $3::bigint = 0
			        OR (first_ledger, contract_id) < ($3, $4)
			    )
			ORDER BY
			    first_ledger DESC,
			    contract_id DESC
			LIMIT
			    $5
		`

	case "transfers", "asset_transfers", "account_transfers", "contract_transfers":
		args = []any{domain.Network, c.Snapshot, c.Ledger, c.TxIndex, c.Index, limit + 1}
		filter := ""
		if kind != "transfers" {
			args = append(args, scope)
		}

		if kind == "asset_transfers" {
			filter = `
				AND m.contract_id = $7
			`
		} else if kind == "account_transfers" {
			filter = `
				AND (m.from_address = $7 OR m.to_address = $7)
			`
		} else if kind == "contract_transfers" {
			filter = `
				AND (m.contract_id = $7 OR m.from_address = $7 OR m.to_address = $7)
			`
		}

		extra, filteredArgs := filterSQL(kind, f, args)
		args = filteredArgs

		q = `
			SELECT
			    jsonb_set(m.payload, '{asset}', a.payload),
			    m.ledger_sequence,
			    m.event_index,
			    m.transaction_hash,
			    m.tx_index
			FROM
			    token_transfers m
			    JOIN assets a USING (network, contract_id)
			WHERE
			    m.network = $1
			    AND m.ledger_sequence <= $2
			    AND (
			        $3::bigint = 0
			        OR (m.ledger_sequence, m.tx_index, m.event_index) < ($3, $4, $5)
			    )
		` + filter + extra + `
			ORDER BY
			    m.ledger_sequence DESC,
			    m.tx_index DESC,
			    m.event_index DESC
			LIMIT
			    $6
		`

	case "ledgers":
		args = []any{
			domain.Network,
			c.Snapshot,
			c.Ledger,
			limit + 1,
		}
		q = `
			SELECT
			    payload,
			    sequence,
			    0,
			    ''
			FROM
			    ledgers
			WHERE
			    network = $1
			    AND sequence <= $2
			    AND (
			        $3::bigint = 0
			        OR sequence < $3
			    )
			ORDER BY
			    sequence DESC
			LIMIT
			    $4
		`

	case "transactions", "account_transactions", "ledger_transactions", "contract_transactions":
		args = []any{
			domain.Network,
			c.Snapshot,
			c.Ledger,
			c.Index,
			limit + 1,
		}
		filter := ""
		if kind != "transactions" {
			args = append(args, scope)
		}

		if kind == "account_transactions" {
			filter = `
				AND EXISTS (
				    SELECT
				        1
				    FROM
				        transaction_accounts a
				    WHERE
				        a.network = t.network
				        AND a.transaction_hash = t.hash
				        AND a.address = $6
				)
			`
		}

		if kind == "ledger_transactions" {
			filter = `
				AND ledger_sequence = $6::text::bigint
			`
		}

		if kind == "contract_transactions" {
			filter = `
				AND EXISTS (
				    SELECT
				        1
				    FROM
				        transaction_contracts a
				    WHERE
				        a.network = t.network
				        AND a.transaction_hash = t.hash
				        AND a.contract_id = $6
				)
			`
		}

		extra, filteredArgs := filterSQL(kind, f, args)
		args = filteredArgs

		q = `
			SELECT
			    payload
			        - 'envelope_xdr'
			        - 'result_xdr'
			        - 'meta_xdr'
			        - 'operations'
			        - 'events'
			        - 'diagnostics'
			        - 'token_transfers'
			        - 'nft_transfers'
			        - 'contracts'
			        - 'interpretation',
			    ledger_sequence,
			    tx_index,
			    hash
			FROM
			    transactions t
			WHERE
			    network = $1
			    AND ledger_sequence <= $2
			    AND (
			        $3::bigint = 0
			        OR (ledger_sequence, tx_index) < ($3, $4)
			    )
		` + filter + extra + `
			ORDER BY
			    ledger_sequence DESC,
			    tx_index DESC
			LIMIT
			    $5
		`

	case "events":
		args = []any{
			domain.Network,
			c.Snapshot,
			c.Ledger,
			c.Hash,
			c.Index,
			scope,
			limit + 1,
		}
		extra, filteredArgs := filterSQL(kind, f, args)
		args = filteredArgs
		q = `
			SELECT
			    payload,
			    ledger_sequence,
			    event_index,
			    transaction_hash
			FROM
			    contract_events e
			WHERE
			    network = $1
			    AND ledger_sequence <= $2
			    AND (
			        $3::bigint = 0
			        OR (ledger_sequence, transaction_hash, event_index) < ($3, $4, $5)
			    )
			    AND ($6::text = '' OR contract_id = $6)
		` + extra + `
			ORDER BY
			    ledger_sequence DESC,
			    transaction_hash DESC,
			    event_index DESC
			LIMIT
			    $7
		`

	default:
		return p, fmt.Errorf("unknown list kind")
	}

	rows, e := s.Pool.Query(ctx, q, args...)
	if e != nil {
		return p, e
	}

	defer rows.Close()

	var last Cursor

	for rows.Next() {
		var raw []byte
		var seq uint32
		var index int
		var hash string
		var txIndex uint32
		targets := []any{&raw, &seq, &index, &hash}
		if kind == "transfers" || kind == "asset_transfers" || kind == "account_transfers" ||
			kind == "contract_transfers" || nftTransferList(kind) {
			targets = append(targets, &txIndex)
		}

		if e = rows.Scan(targets...); e != nil {
			return p, e
		}

		if len(p.Items) == limit {
			next := last.Encode()
			p.NextCursor = &next
			break
		}

		p.Items = append(p.Items, json.RawMessage(raw))
		last = Cursor{
			Version:  1,
			Dataset:  dataset,
			Kind:     kind,
			Scope:    scope,
			Snapshot: c.Snapshot,
			Ledger:   seq,
			Index:    index,
			Hash:     hash,
			TxIndex:  txIndex,
			Filters:  c.Filters,
		}
	}

	return p, rows.Err()
}
