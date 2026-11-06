package httpapi_test

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/httpapi"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestAquariusAPISeparatesFundingRefundNetInputAndSplitRouteTotals(t *testing.T) {
	s := testutil.Database(t)
	handler := (&httpapi.API{Store: s, MaxLimit: 100}).Handler()
	bundles := []domain.Bundle{
		testutil.AquariusStrictReceiveBundle(t, 10),
		testutil.AquariusSplitBundle(t, 11),
	}
	for index, bundle := range bundles {
		if err := s.Save(context.Background(), bundle); err != nil {
			t.Fatal(err)
		}

		expected := bundle.Transactions[0]
		response := request(handler, "GET", "/api/v1/transactions/"+expected.Hash)
		var transaction domain.Transaction
		if err := json.Unmarshal(response.Body.Bytes(), &transaction); err != nil ||
			response.Code != 200 {
			t.Fatal("Aquarius settlement detail unavailable", response.Body, err)
		}

		if !reflect.DeepEqual(expected.Interpretation, transaction.Interpretation) ||
			len(transaction.Interpretation.Actions) == 0 {
			t.Fatal("API changed Aquarius settlement or route evidence")
		}

		action := transaction.Interpretation.Actions[0]
		swap := action.AquariusSwap
		if action.Kind != "swap" || action.Status != "applied" || swap == nil ||
			swap.AdapterVersion != 2 || swap.Settlement == nil || swap.RouterWasmHash != nil ||
			action.SourceAccount != testutil.Address(1) || action.From != testutil.Contract(2) ||
			swap.Trader != action.From || action.To != swap.Trader || len(swap.Hops) != 3 {
			t.Fatal("API lost adapter identity, trader provenance or settlement evidence", action)
		}

		settlement := swap.Settlement
		if settlement.FundingEventIndex != 0 || settlement.PayoutEventIndex != 13 {
			t.Fatal("API lost the funding and final payout event identities", settlement)
		}

		if index == 0 {
			if swap.Mode != "exact_output" || swap.Routing != "chained" ||
				swap.InputLimit != "21000000" || swap.OutputLimit != "2500000" ||
				swap.Input.Amount != "20000000" || swap.Output.Amount != "2500000" ||
				settlement.FundedAmount != "21000000" || settlement.RefundedAmount != "1000000" ||
				settlement.RefundEventIndex == nil || *settlement.RefundEventIndex != 14 ||
				len(swap.Path) != 4 || len(swap.Routes) != 0 ||
				!slices.Equal(swap.TransferEventIndexes, []int{0, 1, 2, 5, 6, 9, 10, 13, 14}) {
				t.Fatal("API confused gross funding, refunded input or exact received amount", swap)
			}
		} else {
			if swap.Mode != "exact_input" || swap.Routing != "split" ||
				swap.InputLimit != "100000000" || swap.OutputLimit != "39999999" ||
				swap.Input.Amount != "100000000" || swap.Output.Amount != "40000000" ||
				settlement.FundedAmount != "100000000" || settlement.RefundedAmount != "0" ||
				settlement.RefundEventIndex != nil || len(swap.Path) != 0 || len(swap.Routes) != 2 ||
				!slices.Equal(swap.TransferEventIndexes, []int{0, 1, 2, 5, 6, 9, 10, 13}) {
				t.Fatal("API confused split route amounts with the complete exchange", swap)
			}

			first, second := swap.Routes[0], swap.Routes[1]
			if first.WeightBps != 6000 || second.WeightBps != 4000 ||
				first.Input.Amount != "60000000" || first.Output.Amount != "30000000" ||
				second.Input.Amount != "40000000" || second.Output.Amount != "10000000" ||
				!slices.Equal(first.HopIndexes, []int{0}) ||
				!slices.Equal(second.HopIndexes, []int{1, 2}) ||
				len(first.Path) != 2 || len(second.Path) != 3 {
				t.Fatal("API lost split weights, branch amounts or original hop references", swap.Routes)
			}
		}

		var wire struct {
			Interpretation struct {
				Actions []struct {
					AquariusSwap struct {
						Settlement map[string]json.RawMessage `json:"settlement"`
					} `json:"aquarius_swap"`
				} `json:"actions"`
			} `json:"interpretation"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &wire); err != nil {
			t.Fatal(err)
		}

		refund, exists := wire.Interpretation.Actions[0].AquariusSwap.Settlement["refund_event_index"]
		if !exists || (index == 0 && string(refund) != "14") ||
			(index == 1 && string(refund) != "null") {
			t.Fatal("API changed the nullable refund event provenance", string(refund))
		}
	}

	response := request(handler, "GET", "/api/v1/transactions")
	var page domain.Page
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil ||
		response.Code != 200 || len(page.Items) != 2 {
		t.Fatal("Aquarius settlement transaction summaries unavailable", response.Body, err)
	}

	for _, item := range page.Items {
		var summary map[string]json.RawMessage
		if err := json.Unmarshal(item, &summary); err != nil {
			t.Fatal(err)
		}

		if _, exists := summary["interpretation"]; exists {
			t.Fatal("settlement and split routes leaked into a lightweight transaction list")
		}
	}
}
