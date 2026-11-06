package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type AquariusFixture struct {
	Ledger      domain.Ledger `json:"ledger"`
	TxIndex     uint32        `json:"tx_index"`
	Hash        string        `json:"hash"`
	EnvelopeXDR string        `json:"envelope_xdr"`
	ResultXDR   string        `json:"result_xdr"`
	MetaXDR     string        `json:"meta_xdr"`
	Expected    struct {
		Supported            bool     `json:"supported"`
		Function             string   `json:"function"`
		Routing              string   `json:"routing"`
		Mode                 string   `json:"mode"`
		InputAmount          string   `json:"input_amount"`
		OutputAmount         string   `json:"output_amount"`
		InputLimit           string   `json:"input_limit"`
		OutputLimit          string   `json:"output_limit"`
		FundedAmount         string   `json:"funded_amount"`
		RefundedAmount       string   `json:"refunded_amount"`
		FundingEventIndex    *int     `json:"funding_event_index"`
		RefundEventIndex     *int     `json:"refund_event_index"`
		PayoutEventIndex     *int     `json:"payout_event_index"`
		Trader               string   `json:"trader"`
		TokenContracts       []string `json:"token_contracts"`
		PoolContracts        []string `json:"pool_contracts"`
		RouterEventIndexes   []int    `json:"router_event_indexes"`
		TransferEventIndexes []int    `json:"transfer_event_indexes"`
		Routes               []struct {
			Index        int      `json:"index"`
			WeightBps    uint32   `json:"weight_bps"`
			Path         []string `json:"path"`
			InputAmount  string   `json:"input_amount"`
			OutputAmount string   `json:"output_amount"`
			HopIndexes   []int    `json:"hop_indexes"`
		} `json:"routes"`
	} `json:"expected"`
}

func AquariusFixtures(t *testing.T) []AquariusFixture {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "../../tests/fixtures/aquarius-testnet.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var fixture struct {
		Network      string            `json:"network"`
		Transactions []AquariusFixture `json:"transactions"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil || fixture.Network != domain.Network ||
		len(fixture.Transactions) == 0 {
		t.Fatal("invalid Aquarius fixture", err)
	}

	return fixture.Transactions
}

func RealAquariusTransactions(t *testing.T) []domain.Transaction {
	t.Helper()
	transactions := []domain.Transaction{}
	for _, fixture := range AquariusFixtures(t) {
		var envelope xdr.TransactionEnvelope
		var result xdr.TransactionResult
		var meta xdr.TransactionMeta
		if err := xdr.SafeUnmarshalBase64(fixture.EnvelopeXDR, &envelope); err != nil {
			t.Fatal(err)
		}

		if err := xdr.SafeUnmarshalBase64(fixture.ResultXDR, &result); err != nil {
			t.Fatal(err)
		}

		if err := xdr.SafeUnmarshalBase64(fixture.MetaXDR, &meta); err != nil {
			t.Fatal(err)
		}

		identity := Result(envelope, result.Successful())
		identity.Result = result
		transaction, err := decoder.Transaction(envelope, identity, meta,
			fixture.Ledger, fixture.TxIndex)
		if err != nil || transaction.Hash != fixture.Hash {
			t.Fatal("Aquarius fixture identity mismatch", fixture.Hash, err)
		}

		transactions = append(transactions, transaction)
	}

	return transactions
}
