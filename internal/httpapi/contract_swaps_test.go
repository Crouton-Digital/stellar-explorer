package httpapi_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/httpapi"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestContractSwapAPIKeepsTraderEvidenceAndNullableCodeIdentity(t *testing.T) {
	s := testutil.Database(t)
	bundle := testutil.SoroswapBundle(t, 10)
	if err := s.Save(context.Background(), bundle); err != nil {
		t.Fatal(err)
	}

	handler := (&httpapi.API{Store: s, MaxLimit: 100}).Handler()
	expected := bundle.Transactions[0]
	response := request(handler, "GET", "/api/v1/transactions/"+expected.Hash)
	var transaction domain.Transaction
	if err := json.Unmarshal(response.Body.Bytes(), &transaction); err != nil ||
		response.Code != 200 {
		t.Fatal("contract swap detail unavailable", response.Body, err)
	}

	if !reflect.DeepEqual(expected.Interpretation, transaction.Interpretation) {
		t.Fatal("API changed decoded contract swap evidence")
	}

	if len(transaction.Interpretation.Actions) == 0 {
		t.Fatal("contract swap interpretation has no actions")
	}

	action := transaction.Interpretation.Actions[0]
	swap := action.ContractSwap
	if action.Kind != "swap" || action.Status != "applied" || swap == nil ||
		swap.Protocol != "soroswap" || swap.Verification != "registered_router_and_transfers" ||
		action.SourceAccount != testutil.Address(1) || action.From != testutil.Address(2) ||
		swap.Trader != testutil.Address(2) ||
		action.To != swap.Trader || swap.RouterWasmHash != nil ||
		swap.Input.Amount != "20000000" || swap.Output.Amount != "10000000" ||
		swap.RouterEventIndex < 0 || len(swap.TransferEventIndexes) != 2 ||
		len(swap.Hops) != 1 {
		t.Fatal("API lost exact swap amounts, trader provenance or identity limitations", action)
	}

	var wire struct {
		Interpretation struct {
			Actions []struct {
				ContractSwap map[string]json.RawMessage `json:"contract_swap"`
			} `json:"actions"`
		} `json:"interpretation"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}

	hash, exists := wire.Interpretation.Actions[0].ContractSwap["router_wasm_hash"]
	if !exists || string(hash) != "null" {
		t.Fatal("unknown router code identity must be an explicit null", string(hash))
	}

	response = request(handler, "GET", "/api/v1/transactions")
	var page domain.Page
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil ||
		response.Code != 200 || len(page.Items) != 1 {
		t.Fatal("contract swap transaction summary unavailable", response.Body, err)
	}

	var summary map[string]json.RawMessage
	if err := json.Unmarshal(page.Items[0], &summary); err != nil {
		t.Fatal(err)
	}

	if _, exists := summary["interpretation"]; exists {
		t.Fatal("contract swap evidence leaked into a lightweight transaction list")
	}
}
