package httpapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/httpapi"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestContractMetadataOnlyCreationIsSearchable(t *testing.T) {
	s := testutil.Database(t)
	address := testutil.Contract(8)
	change := testutil.InstanceChange(address, 10,
		xdr.LedgerEntryChangeTypeLedgerEntryCreated, testutil.WasmExecutable(1))
	bundle := testutil.ContractBundle(t, 10, change)
	if err := s.Save(context.Background(), bundle); err != nil {
		t.Fatal(err)
	}

	h := (&httpapi.API{Store: s, MaxLimit: 100}).Handler()
	response := request(h, "GET", "/api/v1/contracts/"+address)
	var result struct {
		Contract     domain.Contract
		Transactions domain.Page
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 ||
		result.Contract.Kind != "wasm" || result.Contract.CreationLedger == nil ||
		*result.Contract.CreationLedger != 10 || result.Contract.Creator != nil ||
		len(result.Transactions.Items) != 1 {
		t.Fatalf("missing metadata-only creation: %d %s %v", response.Code, response.Body, err)
	}

	for _, path := range []string{
		"/api/v1/search?q=" + address,
		"/api/v1/contracts/" + address + "/events",
		"/api/v1/contracts/" + address + "/transfers",
	} {
		if response = request(h, "GET", path); response.Code != 200 {
			t.Fatalf("metadata-only contract unavailable at %s: %s", path, response.Body)
		}
	}

	response = request(h, "GET", "/api/v1/contracts/"+testutil.Contract(9))
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &result) != nil ||
		result.Contract.Kind != "unknown" || result.Contract.CreationLedger != nil ||
		result.Contract.WasmHash != nil {
		t.Fatal("call-only contract metadata should remain unknown")
	}
}
