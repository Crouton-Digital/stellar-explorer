package domain

import (
	"encoding/json"
	"time"
)

const Network = "testnet"

const Passphrase = "Test SDF Network ; September 2015"

type Ledger struct {
	Network          string    `json:"network"`
	Sequence         uint32    `json:"sequence"`
	Hash             string    `json:"hash"`
	PreviousHash     string    `json:"previous_hash"`
	ClosedAt         time.Time `json:"closed_at"`
	ProtocolVersion  uint32    `json:"protocol_version"`
	TransactionCount int       `json:"transaction_count"`
	RawXDR           string    `json:"raw_xdr,omitempty"`
}

type Transaction struct {
	Network        string          `json:"network"`
	Hash           string          `json:"hash"`
	LedgerSequence uint32          `json:"ledger_sequence"`
	TxIndex        uint32          `json:"tx_index"`
	SourceAccount  string          `json:"source_account"`
	FeeAccount     string          `json:"fee_account"`
	Successful     bool            `json:"successful"`
	FeeCharged     string          `json:"fee_charged"`
	Fees           FeeSummary      `json:"fees"`
	ResultCode     string          `json:"result_code"`
	Interpretation Interpretation  `json:"interpretation"`
	CreatedAt      time.Time       `json:"created_at"`
	EnvelopeXDR    string          `json:"envelope_xdr"`
	ResultXDR      string          `json:"result_xdr"`
	MetaXDR        string          `json:"meta_xdr"`
	Operations     []Operation     `json:"operations"`
	Calls          []Call          `json:"calls"`
	Events         []Event         `json:"events"`
	TokenTransfers []TokenTransfer `json:"token_transfers"`
	NFTTransfers   []NFTTransfer   `json:"nft_transfers"`
	Contracts      []Contract      `json:"contracts"`
	ReturnValue    any             `json:"return_value"`
	Diagnostics    any             `json:"diagnostics"`
	Participants   []string        `json:"participants"`
}

type Operation struct {
	Index         int    `json:"op_index"`
	Type          string `json:"type"`
	SourceAccount string `json:"source_account"`
	Details       any    `json:"details"`
	DecodeStatus  string `json:"decode_status"`
	RawXDR        string `json:"raw_xdr"`
}

type Call struct {
	Index        int    `json:"op_index"`
	ContractID   string `json:"contract_id"`
	FunctionName string `json:"function_name"`
	Arguments    []any  `json:"arguments"`
	DecodeStatus string `json:"decode_status"`
}

type Event struct {
	TransactionHash string  `json:"transaction_hash"`
	Index           int     `json:"event_index"`
	OpIndex         *int    `json:"op_index"`
	Scope           string  `json:"event_scope"`
	Stage           *string `json:"stage"`
	LedgerSequence  uint32  `json:"ledger_sequence"`
	ContractID      string  `json:"contract_id"`
	Type            string  `json:"event_type"`
	Topics          []any   `json:"topics"`
	Value           any     `json:"value"`
	RawXDR          string  `json:"raw_xdr"`
}

type Bundle struct {
	Ledger       Ledger
	Transactions []Transaction
}

type Page struct {
	Items      []json.RawMessage `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}

type SyncState struct {
	Network               string     `json:"network"`
	FirstIndexedLedger    *uint32    `json:"first_indexed_ledger"`
	LastIndexedLedger     *uint32    `json:"last_indexed_ledger"`
	LatestNetworkLedger   *uint32    `json:"latest_network_ledger"`
	Lag                   *uint32    `json:"lag"`
	ObservedAt            *time.Time `json:"observed_at"`
	LastError             *string    `json:"last_error"`
	HistoryComplete       bool       `json:"history_complete"`
	ProcessedTransactions uint64     `json:"processed_transactions"`
	ErrorCount            uint64     `json:"error_count"`
}
