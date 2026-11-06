package httpapi_test

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/httpapi"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestInvalidFiltersNeverReachStorage(t *testing.T) {
	h := (&httpapi.API{MaxLimit: 100}).Handler()
	for _, query := range []string{
		"address=invalid",
		"contract=" + testutil.Address(1),
		"status=unknown",
		"status=success&status=failed",
		"status=",
		"function=a%27%20OR%201%3D1",
		"function=" + strings.Repeat("x", 33),
		"min_ledger=0",
		"min_ledger=-1",
		"max_ledger=4294967296",
		"min_ledger=11&max_ledger=10",
		"min_ledger=01",
		"min_ledger=1&min_ledger=2",
		"asset=" + testutil.Contract(1),
		"unknown=value",
	} {
		t.Run(query, func(t *testing.T) {
			response := request(h, "GET", "/api/v1/transactions?"+query)
			if response.Code != 400 {
				t.Fatalf("invalid filter reached storage: %d %s", response.Code, response.Body)
			}
		})
	}

	for _, path := range []string{
		"/api/v1/events?event_type=unknown",
		"/api/v1/events?event_name=invalid%20name",
		"/api/v1/events?event_scope=diagnostic",
		"/api/v1/events?event_scope=",
		"/api/v1/events?event_scope=operation&event_scope=transaction",
		"/api/v1/transactions?event_scope=transaction",
		"/api/v1/transfers?from=invalid",
		"/api/v1/transfers?to=invalid",
		"/api/v1/transfers?asset=" + testutil.Address(1),
		"/api/v1/ledgers?status=failed",
	} {
		if response := request(h, "GET", path); response.Code != 400 {
			t.Fatal("invalid endpoint filters accepted", path, response.Code)
		}
	}
}

func TestFilteredAPIPagesAndCursorBinding(t *testing.T) {
	s := testutil.Database(t)
	bundle := testutil.RealBundle(t)
	if err := s.Save(context.Background(), bundle); err != nil {
		t.Fatal(err)
	}

	h := (&httpapi.API{Store: s, MaxLimit: 100}).Handler()
	response := request(h, "GET", "/api/v1/transactions?status=success&limit=1")
	var page domain.Page
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != 200 ||
		len(page.Items) != 1 || page.NextCursor == nil {
		t.Fatal("missing filtered transaction page", response.Body, err)
	}

	cursor := url.QueryEscape(*page.NextCursor)
	for _, query := range []string{"status=failed", "min_ledger=1", "status=success&min_ledger=1"} {
		if response = request(h, "GET", "/api/v1/transactions?"+query+"&cursor="+cursor); response.Code != 400 {
			t.Fatal("cursor accepted with different filters", response.Code)
		}
	}

	response = request(h, "GET", "/api/v1/transactions?limit=2&cursor="+cursor+"&status=success")
	var next domain.Page
	if err := json.Unmarshal(response.Body.Bytes(), &next); err != nil || response.Code != 200 ||
		len(next.Items) != 2 || string(next.Items[0]) == string(page.Items[0]) {
		t.Fatal("equivalent query with changed limit lost pagination", response.Body, err)
	}

	for _, path := range []string{
		"/api/v1/transactions?address=" + testutil.Address(99),
		"/api/v1/events?event_name=not_present",
		"/api/v1/transfers?from=" + testutil.Address(99),
	} {
		response = request(h, "GET", path)
		var empty domain.Page
		if err := json.Unmarshal(response.Body.Bytes(), &empty); err != nil ||
			response.Code != 200 ||
			empty.Items == nil ||
			len(empty.Items) != 0 ||
			empty.NextCursor != nil {
			t.Fatal("unmatched valid filters must return an empty page", path, response.Body, err)
		}
	}
}
