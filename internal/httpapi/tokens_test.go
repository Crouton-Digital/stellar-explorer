package httpapi_test

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/httpapi"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestTokenEndpointsValidationAndPagination(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	asset := xdr.MustNewNativeAsset()
	contract := testutil.SACContract(asset)
	event := testutil.TokenEvent(
		contract,
		"transfer",
		[]string{testutil.Address(3), testutil.Address(4)},
		testutil.I128(0, 10000001),
		testutil.String("native"),
	)
	b := testutil.TokenBundle(t, 10, event, event)
	if e := s.Save(ctx, b); e != nil {
		t.Fatal(e)
	}

	h := (&httpapi.API{Store: s, MaxLimit: 100}).Handler()
	for _, path := range []string{
		"/api/v1/assets",
		"/api/v1/assets/" + contract,
		"/api/v1/assets/" + contract + "/transfers",
		"/api/v1/transfers",
		"/api/v1/accounts/" + testutil.Address(4) + "/transfers",
		"/api/v1/contracts/" + contract + "/transfers",
	} {
		r := request(h, "GET", path)
		if r.Code != 200 {
			t.Fatalf("%s: %d %s", path, r.Code, r.Body)
		}
	}

	for _, path := range []string{
		"/api/v1/assets/" + testutil.Contract(99),
		"/api/v1/assets/" + testutil.Contract(99) + "/transfers",
		"/api/v1/accounts/" + testutil.Address(99) + "/transfers",
	} {
		if r := request(h, "GET", path); r.Code != 404 {
			t.Fatalf("expected 404 for %s: %d", path, r.Code)
		}
	}

	for _, path := range []string{
		"/api/v1/assets/invalid",
		"/api/v1/assets/" + testutil.Address(4) + "/transfers",
		"/api/v1/accounts/invalid/transfers",
		"/api/v1/transfers?cursor=bad",
		"/api/v1/assets?limit=101",
	} {
		if r := request(h, "GET", path); r.Code != 400 {
			t.Fatalf("expected 400 for %s: %d", path, r.Code)
		}
	}

	r := request(h, "GET", "/api/v1/transfers?limit=1")
	var page struct {
		Items  []domain.TokenTransfer `json:"items"`
		Cursor string                 `json:"next_cursor"`
	}
	if e := json.Unmarshal(r.Body.Bytes(), &page); e != nil || len(page.Items) != 1 ||
		page.Cursor == "" {
		t.Fatal("missing transfer pagination", e)
	}

	r = request(
		h,
		"GET",
		"/api/v1/assets/"+contract+"/transfers?cursor="+url.QueryEscape(page.Cursor),
	)
	if r.Code != 400 {
		t.Fatal("global cursor accepted for asset scope")
	}

	r = request(h, "GET", "/api/v1/transfers?limit=1&cursor="+url.QueryEscape(page.Cursor))
	var next struct {
		Items []domain.TokenTransfer `json:"items"`
	}
	if e := json.Unmarshal(r.Body.Bytes(), &next); e != nil || r.Code != 200 ||
		len(next.Items) != 1 ||
		next.Items[0].EventIndex == page.Items[0].EventIndex {
		t.Fatal("token API repeated or lost a movement", e)
	}
}
