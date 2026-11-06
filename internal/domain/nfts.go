package domain

import "time"

// NFTCollection records an observed NFT event emitter without asserting metadata or ownership.
type NFTCollection struct {
	Network             string `json:"network"`
	ContractID          string `json:"contract_id"`
	Verification        string `json:"verification"`
	FirstObservedLedger uint32 `json:"first_observed_ledger"`
}

// NFTTransfer keeps unsigned token identifiers separate from fungible amounts.
type NFTTransfer struct {
	TransactionHash string    `json:"transaction_hash"`
	LedgerSequence  uint32    `json:"ledger_sequence"`
	TxIndex         uint32    `json:"tx_index"`
	EventIndex      int       `json:"event_index"`
	OpIndex         int       `json:"op_index"`
	Kind            string    `json:"kind"`
	ContractID      string    `json:"contract_id"`
	TokenID         string    `json:"token_id"`
	TokenIDType     string    `json:"token_id_type"`
	From            string    `json:"from,omitempty"`
	To              string    `json:"to,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	Verification    string    `json:"verification"`
	EventStandard   string    `json:"event_standard"`
	Format          string    `json:"format"`
}
