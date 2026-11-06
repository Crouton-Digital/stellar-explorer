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

func TestAquariusAPIKeepsExactHopEvidenceAndTraderProvenance(t *testing.T) {
	s := testutil.Database(t)
	bundles := []domain.Bundle{
		testutil.AquariusBundle(t, 10),
		testutil.AquariusChainedBundle(t, 11),
	}
	handler := (&httpapi.API{Store: s, MaxLimit: 100}).Handler()
	for index, bundle := range bundles {
		if err := s.Save(context.Background(), bundle); err != nil {
			t.Fatal(err)
		}

		expected := bundle.Transactions[0]
		response := request(handler, "GET", "/api/v1/transactions/"+expected.Hash)
		var transaction domain.Transaction
		if err := json.Unmarshal(response.Body.Bytes(), &transaction); err != nil ||
			response.Code != 200 {
			t.Fatal("Aquarius transaction detail unavailable", response.Body, err)
		}

		if !reflect.DeepEqual(expected.Interpretation, transaction.Interpretation) ||
			len(transaction.Interpretation.Actions) == 0 {
			t.Fatal("API changed Aquarius interpretation or its evidence")
		}

		action := transaction.Interpretation.Actions[0]
		swap := action.AquariusSwap
		if action.Kind != "swap" || action.Status != "applied" || swap == nil ||
			swap.Protocol != "aquarius" ||
			swap.Verification != "registered_router_events_and_transfers" ||
			swap.Mode != "exact_input" || swap.RouterWasmHash != nil ||
			action.SourceAccount != testutil.Address(1) || action.From != testutil.Contract(2) ||
			swap.Trader != action.From || action.To != swap.Trader ||
			swap.Input.Amount != "20000000" || len(swap.Hops) != index+1 ||
			len(swap.Path) != len(swap.Hops)+1 || action.ContractSwap != nil {
			t.Fatal("API lost Aquarius provenance, exact amounts or hop attribution", action)
		}

		expectedRouting, expectedOutput := "direct", "10000000"
		if index == 1 {
			expectedRouting, expectedOutput = "chained", "5000000"
			if swap.Output.Asset.Decimals != nil ||
				swap.Output.Asset.ContractID != testutil.Contract(10) {
				t.Fatal("API invented precision for an unidentified output token", swap.Output)
			}
		}

		if swap.Routing != expectedRouting || swap.Output.Amount != expectedOutput {
			t.Fatal("API confused exact hop output with request limits", swap)
		}

		if index == 1 {
			settlement := swap.Settlement
			if settlement == nil || settlement.FundedAmount != swap.Input.Amount ||
				settlement.RefundedAmount != "0" || settlement.RefundEventIndex != nil ||
				settlement.FundingEventIndex != 0 || settlement.PayoutEventIndex != 9 {
				t.Fatal(
					"exact-input custody was confused with a refund or net swap amount",
					settlement,
				)
			}
		} else if swap.Settlement != nil {
			t.Fatal("direct pool settlement unexpectedly included router custody")
		}

		for _, hop := range swap.Hops {
			if hop.InputEventIndex < 0 || hop.OutputEventIndex <= hop.InputEventIndex ||
				hop.TradeEventIndex <= hop.OutputEventIndex ||
				hop.RouterEventIndex <= hop.TradeEventIndex || hop.PoolIndex == "" ||
				hop.Input.Amount == "" || hop.Output.Amount == "" || len(hop.PoolTokens) != 2 {
				t.Fatal("API omitted coherent pool and router event provenance", hop)
			}
		}

		var wire struct {
			Interpretation struct {
				Actions []struct {
					AquariusSwap map[string]json.RawMessage `json:"aquarius_swap"`
				} `json:"actions"`
			} `json:"interpretation"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &wire); err != nil {
			t.Fatal(err)
		}

		hash, exists := wire.Interpretation.Actions[0].AquariusSwap["router_wasm_hash"]
		if !exists || string(hash) != "null" {
			t.Fatal("unknown Aquarius router code identity must be an explicit null", string(hash))
		}
	}

	response := request(handler, "GET", "/api/v1/transactions")
	var page domain.Page
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil ||
		response.Code != 200 || len(page.Items) != 2 {
		t.Fatal("Aquarius transaction summaries unavailable", response.Body, err)
	}

	for _, item := range page.Items {
		var summary map[string]json.RawMessage
		if err := json.Unmarshal(item, &summary); err != nil {
			t.Fatal(err)
		}

		if _, exists := summary["interpretation"]; exists {
			t.Fatal("Aquarius evidence leaked into a lightweight transaction list")
		}
	}
}
