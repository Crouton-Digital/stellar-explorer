package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/httpapi"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func request(h http.Handler, method, path string) *httptest.ResponseRecorder {
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(method, path, nil))

	return r
}

func TestInvalidInputsNeverReachStorage(t *testing.T) {
	h := (&httpapi.API{
		MaxLimit: 100,
	}).Handler()

	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/api/v1/ledgers/0", 400},
		{"/api/v1/ledgers/-1", 400},
		{"/api/v1/ledgers/4294967296", 400},
		{"/api/v1/transactions/nope", 400},
		{"/api/v1/accounts/GINVALID", 400},
		{"/api/v1/contracts/" + testutil.Address(1), 400},
		{"/api/v1/assets/invalid", 400},
		{"/api/v1/assets/" + testutil.Address(1) + "/transfers", 400},
		{"/api/v1/accounts/invalid/transfers", 400},
		{"/api/v1/transfers?cursor=bad", 400},
		{"/api/v1/search?q=invalid", 400},
		{"/api/v1/search", 400},
		{"/api/v1/ledgers?limit=-1", 400},
		{"/api/v1/ledgers?limit=101", 400},
		{"/api/v1/ledgers?limit=1&limit=2", 400},
		{"/api/v1/ledgers?cursor=bad", 400},
		{"/api/v1/overview?network=mainnet", 400},
		{"/missing", 404},
	} {
		t.Run(tc.path, func(t *testing.T) {
			r := request(h, "GET", tc.path)
			if r.Code != tc.status {
				t.Fatalf("status %d: %s", r.Code, r.Body)
			}

			var err struct {
				Error struct {
					Code    string
					Message string
				}
			}
			if e := json.Unmarshal(r.Body.Bytes(), &err); e != nil || err.Error.Code == "" ||
				err.Error.Message == "" {
				t.Fatalf("invalid error envelope: %s", r.Body)
			}

			if r.Header().Get("X-Request-ID") == "" {
				t.Fatal("missing correlation ID")
			}
		})
	}

	if r := request(h, "POST", "/api/v1/transactions"); r.Code != 405 {
		t.Fatal("read-only API should return 405")
	}

	if r := request(h, "GET", "/health/live"); r.Code != 200 {
		t.Fatal("liveness should not need storage")
	}
}

func TestEndpointsWithPostgresAndRealLedger(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	b := testutil.RealBundle(t)
	if e := s.Save(ctx, b); e != nil {
		t.Fatal(e)
	}

	if e := s.Observe(ctx, b.Ledger.Sequence, nil); e != nil {
		t.Fatal(e)
	}

	h := (&httpapi.API{
		Store:    s,
		MaxLimit: 100,
	}).Handler()
	tx := b.Transactions[0]
	contract := ""

	for _, t := range b.Transactions {
		if len(t.Calls) > 0 {
			contract = t.Calls[0].ContractID
			break
		}
	}

	if contract == "" {
		t.Fatal("fixture missing contract")
	}

	for _, path := range []string{
		"/health/ready",
		"/api/v1/overview",
		"/api/v1/ledgers?limit=1",
		fmt.Sprintf("/api/v1/ledgers/%d", b.Ledger.Sequence),
		"/api/v1/transactions?limit=2",
		"/api/v1/transactions/" + tx.Hash,
		"/api/v1/accounts/" + tx.SourceAccount,
		"/api/v1/accounts/" + tx.SourceAccount + "/transactions",
		"/api/v1/contracts/" + contract,
		"/api/v1/contracts/" + contract + "/events",
	} {
		t.Run(path, func(t *testing.T) {
			r := request(h, "GET", path)
			if r.Code != 200 || !json.Valid(r.Body.Bytes()) {
				t.Fatalf("%d %s", r.Code, r.Body)
			}
		})
	}

	for _, tc := range []struct {
		q    string
		kind string
		path string
	}{
		{tx.Hash, "transaction", "/tx/" + tx.Hash},
		{strings.ToUpper(tx.Hash), "transaction", "/tx/" + tx.Hash},
		{fmt.Sprint(b.Ledger.Sequence), "ledger", fmt.Sprintf("/ledgers/%d", b.Ledger.Sequence)},
		{tx.SourceAccount, "account", "/accounts/" + tx.SourceAccount},
		{contract, "contract", "/contracts/" + contract},
	} {
		r := request(h, "GET", "/api/v1/search?q="+tc.q)
		var result struct {
			Type string
			Path string
		}
		_ = json.Unmarshal(r.Body.Bytes(), &result)
		if r.Code != 200 || result.Type != tc.kind || result.Path != tc.path {
			t.Fatalf("search %s: %d %s", tc.q, r.Code, r.Body)
		}
	}

	for _, path := range []string{
		"/api/v1/transactions/" + strings.Repeat("0", 64),
		"/api/v1/ledgers/1",
		"/api/v1/accounts/" + testutil.Address(99),
		"/api/v1/search?q=" + testutil.Contract(99),
	} {
		if r := request(h, "GET", path); r.Code != 404 {
			t.Fatalf("not-found must be 404: %s => %d", path, r.Code)
		}
	}

	r := request(h, "GET", "/api/v1/transactions/"+tx.Hash)
	var detail domain.Transaction
	if e := json.Unmarshal(r.Body.Bytes(), &detail); e != nil {
		t.Fatal(e)
	}

	if detail.FeeCharged != tx.FeeCharged || detail.MetaXDR == "" ||
		len(detail.Operations) != len(tx.Operations) {
		t.Fatal("transaction detail lost fields")
	}

	r = request(h, "GET", "/api/v1/overview")
	var overview struct {
		Sync domain.SyncState
	}
	_ = json.Unmarshal(r.Body.Bytes(), &overview)
	if overview.Sync.Lag == nil || *overview.Sync.Lag != 0 || overview.Sync.HistoryComplete {
		t.Fatal("sync completeness misleading")
	}

	// Stale upstream data must not prevent readiness or reads of committed data.
	message := "RPC unavailable"
	if e := s.Observe(ctx, 0, &message); e != nil {
		t.Fatal(e)
	}

	if r = request(h, "GET", "/health/ready"); r.Code != 200 {
		t.Fatal("RPC outage must not disable DB reads")
	}

	if state, e := s.State(ctx); e != nil || state.ErrorCount != 1 ||
		state.ProcessedTransactions != uint64(len(b.Transactions)) {
		t.Fatalf("incorrect observability counters: %+v %v", state, e)
	}
}
