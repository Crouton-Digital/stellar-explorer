package httpapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/httpapi"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestStateAndSwapEvidenceInTransactionDetail(t *testing.T) {
	s := testutil.Database(t)
	bundles := []domain.Bundle{
		testutil.StateBundle(t, 10, false),
		testutil.SwapBundle(t, 11),
	}
	handler := (&httpapi.API{Store: s, MaxLimit: 100}).Handler()
	for _, bundle := range bundles {
		if err := s.Save(context.Background(), bundle); err != nil {
			t.Fatal(err)
		}

		response := request(handler, "GET", "/api/v1/transactions/"+bundle.Transactions[0].Hash)
		var transaction domain.Transaction
		if err := json.Unmarshal(response.Body.Bytes(), &transaction); err != nil ||
			response.Code != 200 {
			t.Fatal("transaction detail unavailable", err, response.Body)
		}

		if !transaction.Successful {
			changes := transaction.Interpretation.StateChanges
			if len(changes) != 1 || changes[0].Scope != "transaction_before" ||
				changes[0].OpIndex != nil ||
				changes[0].Before.Fields["balance_stroops"] != "9223372036854775807" {
				t.Fatal("API lost committed state, null provenance or exact values", changes)
			}
		} else {
			action := transaction.Interpretation.Actions[0]
			if action.Kind != "swap" || action.PathPayment.Execution == nil ||
				action.PathPayment.Execution.Input.Amount != "20000000" ||
				action.PathPayment.Execution.Output.Amount != "10000000" ||
				action.PathPayment.SendLimit != "30000000" {
				t.Fatal("API confused swap execution with its limits", action)
			}
		}
	}

	response := request(handler, "GET", "/api/v1/transactions")
	var page domain.Page
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Items) != 2 {
		t.Fatal("missing transaction summaries", err)
	}

	for _, item := range page.Items {
		var summary map[string]json.RawMessage
		if err := json.Unmarshal(item, &summary); err != nil {
			t.Fatal(err)
		}

		if _, exists := summary["interpretation"]; exists {
			t.Fatal("state and swap payload leaked into lightweight transaction lists")
		}
	}
}
