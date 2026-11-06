package httpapi_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/httpapi"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestTransactionInterpretationAPIAndLightweightLists(t *testing.T) {
	s := testutil.Database(t)
	bundle := testutil.InterpretationBundle(t, 10, false)
	if err := s.Save(context.Background(), bundle); err != nil {
		t.Fatal(err)
	}

	handler := (&httpapi.API{Store: s, MaxLimit: 100}).Handler()
	response := request(handler, "GET", "/api/v1/transactions/"+bundle.Transactions[0].Hash)
	var transaction domain.Transaction
	if err := json.Unmarshal(response.Body.Bytes(), &transaction); err != nil ||
		response.Code != 200 ||
		transaction.Interpretation.Version != decoder.InterpretationVersion ||
		transaction.Interpretation.Failure == nil ||
		!strings.Contains(transaction.Interpretation.Failure.Message, "trap") ||
		len(transaction.Interpretation.Actions) != 1 ||
		transaction.Interpretation.Actions[0].Status != "failed" ||
		transaction.Interpretation.DiagnosticErrors[0].Code != "7" ||
		transaction.Fees.Status != "complete" {
		t.Fatal("API lost interpretation, diagnostic evidence or committed fee", response.Body, err)
	}

	response = request(handler, "GET", "/api/v1/transactions?status=failed")
	var page domain.Page
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Items) != 1 {
		t.Fatal("transaction summary missing", response.Body, err)
	}

	var summary map[string]json.RawMessage
	if err := json.Unmarshal(page.Items[0], &summary); err != nil {
		t.Fatal(err)
	}

	if _, exists := summary["interpretation"]; exists {
		t.Fatal("full interpretation was included in a lightweight transaction list")
	}
}
