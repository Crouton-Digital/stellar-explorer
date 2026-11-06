package httpapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/httpapi"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestInvalidActivityQueriesNeverReachStorage(t *testing.T) {
	handler := (&httpapi.API{MaxLimit: 100}).Handler()
	for _, path := range []string{
		"/api/v1/activity/assets",
		"/api/v1/activity/transactions?limit=1",
		"/api/v1/activity/transactions?cursor=bad",
		"/api/v1/activity/transactions?cursor=a&cursor=b",
		"/api/v1/activity/transactions?status=unknown",
		"/api/v1/activity/transfers?status=success",
		"/api/v1/activity/events?contract=invalid",
		"/api/v1/activity/transactions?max_ledger=9&min_ledger=10",
		"/api/v1/activity/transactions?network=mainnet",
		"/api/v1/activity/transactions?unknown=1",
	} {
		t.Run(path, func(t *testing.T) {
			response := request(handler, "GET", path)
			if response.Code != 400 {
				t.Fatalf("invalid query accepted: %d %s", response.Code, response.Body)
			}
		})
	}
}

func TestActivityEndpointReturnsBoundedAggregatesAndRejectsForeignCursor(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	bundle := testutil.RealBundle(t)
	if err := s.Save(ctx, bundle); err != nil {
		t.Fatal(err)
	}

	handler := (&httpapi.API{Store: s, MaxLimit: 100}).Handler()
	for _, kind := range []string{"transactions", "events", "transfers"} {
		response := request(handler, "GET", "/api/v1/activity/"+kind)
		var actual domain.Activity
		if err := json.Unmarshal(response.Body.Bytes(), &actual); err != nil ||
			response.Code != 200 {
			t.Fatalf("activity unavailable: %d %s %v", response.Code, response.Body, err)
		}

		expected, err := s.List(ctx, kind, "", 100, nil)
		if err != nil || actual.Total != uint64(len(expected.Items)) ||
			actual.Kind != kind || actual.Network != domain.Network || actual.WindowLimit != 120 ||
			actual.FirstLedger == nil || *actual.FirstLedger != bundle.Ledger.Sequence ||
			actual.LedgerCount != 1 {
			t.Fatalf("incorrect activity contract: %+v %v", actual, err)
		}
	}

	foreign := storage.Cursor{
		Version:  1,
		Dataset:  "another-dataset",
		Kind:     "transactions",
		Snapshot: bundle.Ledger.Sequence,
		Ledger:   bundle.Ledger.Sequence,
		Index:    1,
	}
	response := request(handler, "GET", "/api/v1/activity/transactions?cursor="+foreign.Encode())
	if response.Code != 400 {
		t.Fatalf("foreign dataset cursor accepted: %d %s", response.Code, response.Body)
	}
}
