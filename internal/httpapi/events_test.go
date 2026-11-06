package httpapi_test

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/httpapi"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestTransactionFeeDetailsAndEventSourceAPI(t *testing.T) {
	s := testutil.Database(t)
	bundle := testutil.FeeBundle(t, 10, false)
	if err := s.Save(context.Background(), bundle); err != nil {
		t.Fatal(err)
	}

	handler := (&httpapi.API{Store: s, MaxLimit: 100}).Handler()
	response := request(handler, "GET", "/api/v1/transactions/"+bundle.Transactions[0].Hash)
	var transaction domain.Transaction
	if err := json.Unmarshal(response.Body.Bytes(), &transaction); err != nil ||
		response.Code != 200 ||
		transaction.Fees.Status != "complete" ||
		transaction.Successful ||
		*transaction.Fees.Charged != "1000" ||
		*transaction.Fees.Refunded != "900" ||
		*transaction.Fees.Net != "100" ||
		len(transaction.TokenTransfers) != 0 {
		t.Fatal("failed transaction fee breakdown missing from API", response.Body, err)
	}

	response = request(
		handler,
		"GET",
		"/api/v1/events?event_scope=transaction&event_name=fee&limit=1",
	)
	var page domain.Page
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != 200 ||
		len(page.Items) != 1 || page.NextCursor == nil {
		t.Fatal("transaction fee filter returned no page", response.Body, err)
	}

	var event domain.Event
	if err := json.Unmarshal(page.Items[0], &event); err != nil || event.Scope != "transaction" ||
		event.OpIndex != nil || event.Stage == nil {
		t.Fatal("transaction event source or nullable operation index lost", err)
	}

	cursor := url.QueryEscape(*page.NextCursor)
	response = request(
		handler,
		"GET",
		"/api/v1/events?event_scope=operation&event_name=fee&cursor="+cursor,
	)
	if response.Code != 400 {
		t.Fatal("fee cursor reused for a different source", response.Code)
	}

	response = request(handler, "GET", "/api/v1/events?event_scope=operation")
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != 200 ||
		len(page.Items) != 0 {
		t.Fatal("failed operation events leaked into API", response.Body, err)
	}
}
