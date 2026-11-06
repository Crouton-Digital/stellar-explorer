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

type NFTFixture struct {
	Ledger      domain.Ledger        `json:"ledger"`
	TxIndex     uint32               `json:"tx_index"`
	Hash        string               `json:"hash"`
	EnvelopeXDR string               `json:"envelope_xdr"`
	ResultXDR   string               `json:"result_xdr"`
	MetaXDR     string               `json:"meta_xdr"`
	Expected    []domain.NFTTransfer `json:"expected"`
}

func NFTFixtures(t *testing.T) []NFTFixture {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "../../tests/fixtures/nfts-testnet.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var fixture struct {
		Network      string       `json:"network"`
		Transactions []NFTFixture `json:"transactions"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil || fixture.Network != domain.Network ||
		len(fixture.Transactions) == 0 {
		t.Fatal("invalid NFT source fixture", err)
	}

	return fixture.Transactions
}

func RealNFTTransactions(t *testing.T) []domain.Transaction {
	t.Helper()
	transactions := []domain.Transaction{}
	for _, fixture := range NFTFixtures(t) {
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
			t.Fatal("NFT fixture identity mismatch", fixture.Hash, err)
		}

		transactions = append(transactions, transaction)
	}

	return transactions
}
