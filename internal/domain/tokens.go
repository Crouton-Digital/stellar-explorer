package domain

import "time"

type Asset struct {
	Network             string  `json:"network"`
	ContractID          string  `json:"contract_id"`
	Kind                string  `json:"kind"`
	Verification        string  `json:"verification"`
	ClassicAsset        *string `json:"classic_asset"`
	Code                *string `json:"code"`
	Issuer              *string `json:"issuer"`
	Decimals            *uint32 `json:"decimals"`
	FirstObservedLedger uint32  `json:"first_observed_ledger"`
}

type TokenTransfer struct {
	TransactionHash string    `json:"transaction_hash"`
	LedgerSequence  uint32    `json:"ledger_sequence"`
	TxIndex         uint32    `json:"tx_index"`
	EventIndex      int       `json:"event_index"`
	OpIndex         int       `json:"op_index"`
	Kind            string    `json:"kind"`
	Asset           Asset     `json:"asset"`
	From            string    `json:"from,omitempty"`
	To              string    `json:"to,omitempty"`
	Amount          string    `json:"amount"`
	ToMuxed         any       `json:"to_muxed,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}
