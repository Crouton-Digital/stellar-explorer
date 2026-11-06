package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/stellar"
)

func RealBundle(t *testing.T) domain.Bundle {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	raw, e := os.ReadFile(filepath.Join(filepath.Dir(file), "../../tests/fixtures/testnet.json"))
	if e != nil {
		t.Fatal(e)
	}

	var f struct {
		Ledgers []stellar.RemoteLedger `json:"ledgers"`
	}
	if e = json.Unmarshal(raw, &f); e != nil {
		t.Fatal(e)
	}

	if len(f.Ledgers) == 0 {
		t.Fatal("empty fixture")
	}

	b, e := decoder.Ledger(f.Ledgers[0])
	if e != nil {
		t.Fatal(e)
	}

	return b
}
