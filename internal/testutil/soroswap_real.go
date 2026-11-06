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

type SoroswapFixture struct {
	Ledger      domain.Ledger `json:"ledger"`
	TxIndex     uint32        `json:"tx_index"`
	Hash        string        `json:"hash"`
	EnvelopeXDR string        `json:"envelope_xdr"`
	ResultXDR   string        `json:"result_xdr"`
	MetaXDR     string        `json:"meta_xdr"`
	Expected    struct {
		InputAmount      string   `json:"input_amount"`
		OutputAmount     string   `json:"output_amount"`
		Trader           string   `json:"trader"`
		RouterEventIndex int      `json:"router_event_index"`
		TokenContracts   []string `json:"token_contracts"`
	} `json:"expected"`
}

func SoroswapFixtures(t *testing.T) []SoroswapFixture {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "../../tests/fixtures/soroswap-testnet.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var fixtures struct {
		Network      string            `json:"network"`
		Transactions []SoroswapFixture `json:"transactions"`
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil || fixtures.Network != domain.Network ||
		len(fixtures.Transactions) == 0 {
		t.Fatal("invalid Soroswap fixture", err)
	}

	return fixtures.Transactions
}

func RealSoroswapTransactions(t *testing.T) []domain.Transaction {
	t.Helper()
	transactions := []domain.Transaction{}
	for _, fixture := range SoroswapFixtures(t) {
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
			t.Fatal("Soroswap fixture identity mismatch", fixture.Hash, err)
		}

		transactions = append(transactions, transaction)
	}

	return transactions
}
