package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/crouton/stellar-explorer/docs"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/stellar/go-stellar-sdk/strkey"
)

type Repository interface {
	Ping(context.Context) error
	State(context.Context) (domain.SyncState, error)
	Get(context.Context, string, string) (json.RawMessage, error)
	Exists(context.Context, string, string) (bool, error)
	List(
		context.Context,
		string,
		string,
		int,
		*storage.Cursor,
		...domain.Filters,
	) (domain.Page, error)
}

type API struct {
	Store    Repository
	MaxLimit int
}

func (a *API) Handler() http.Handler {
	m := http.NewServeMux()

	m.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write(docs.OpenAPI)
	})

	m.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]string{
			"status": "alive",
		})
	})

	m.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		if e := a.Store.Ping(r.Context()); e != nil {
			fail(w, 503, "not_ready", "Database unavailable")

			return
		}

		write(w, 200, map[string]string{
			"status": "ready",
		})
	})

	m.HandleFunc("GET /api/v1/overview", a.overview)

	m.HandleFunc("GET /api/v1/ledgers", func(w http.ResponseWriter, r *http.Request) {
		a.list(w, r, "ledgers", "")
	})

	m.HandleFunc("GET /api/v1/transactions", func(w http.ResponseWriter, r *http.Request) {
		a.list(w, r, "transactions", "")
	})

	m.HandleFunc("GET /api/v1/events", func(w http.ResponseWriter, r *http.Request) {
		a.list(w, r, "events", "")
	})

	m.HandleFunc("GET /api/v1/ledgers/{sequence}", a.ledger)

	m.HandleFunc("GET /api/v1/transactions/{hash}", a.transaction)

	m.HandleFunc("GET /api/v1/accounts/{address}", func(w http.ResponseWriter, r *http.Request) {
		a.address(w, r, "accounts", false)
	})

	m.HandleFunc(
		"GET /api/v1/accounts/{address}/transactions",
		func(w http.ResponseWriter, r *http.Request) {
			a.address(w, r, "accounts", true)
		},
	)

	m.HandleFunc("GET /api/v1/contracts/{address}", func(w http.ResponseWriter, r *http.Request) {
		a.address(w, r, "contracts", false)
	})

	m.HandleFunc(
		"GET /api/v1/contracts/{address}/events",
		func(w http.ResponseWriter, r *http.Request) {
			a.address(w, r, "contracts", true)
		},
	)

	m.HandleFunc("GET /api/v1/search", a.search)
	m.HandleFunc("GET /api/v1/search/suggestions", a.suggestions)

	a.tokenRoutes(m)
	a.nftRoutes(m)
	a.activityRoutes(m)

	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fail(w, 404, "not_found", "Endpoint not found")
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := make([]byte, 12)
		_, _ = rand.Read(id)
		w.Header().Set("X-Request-ID", hex.EncodeToString(id))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		defer func() {
			if e := recover(); e != nil {
				slog.Error("HTTP panic", "request_id", w.Header().Get("X-Request-ID"))
				fail(w, 500, "internal_error", "Internal server error")
			}
		}()

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			fail(w, 405, "method_not_allowed", "Only GET is supported")

			return
		}

		if n := r.URL.Query().Get("network"); n != "" && n != domain.Network {
			fail(w, 400, "invalid_network", "Only testnet is supported")

			return
		}

		m.ServeHTTP(w, r.WithContext(ctx))
	})
}

func write(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, code, message string) {
	write(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func (a *API) problem(w http.ResponseWriter, e error) {
	if errors.Is(e, errFilters) {
		fail(
			w,
			400,
			"invalid_filters",
			"Invalid filters: check parameter names, addresses, symbols, status and ledger bounds",
		)

		return
	}

	if errors.Is(e, storage.ErrNotFound) {
		fail(w, 404, "not_found", "Entity not found in indexed data")

		return
	}

	slog.Error("API storage error", "request_id", w.Header().Get("X-Request-ID"), "error", e)
	fail(w, 503, "storage_unavailable", "Indexed data temporarily unavailable")
}

func (a *API) page(r *http.Request, kind, scope string) (domain.Page, error) {
	return a.pageWithCursor(r, kind, scope, nil)
}

func (a *API) pageWithCursor(
	r *http.Request,
	kind, scope string,
	first *storage.Cursor,
) (domain.Page, error) {
	filters, e := parseFilters(r.URL.Query(), kind)
	if e != nil {
		return domain.Page{}, e
	}

	limit, e := pageLimit(r, a.MaxLimit)
	if e != nil {
		return domain.Page{}, e
	}

	c, e := storage.ParseCursor(r.URL.Query().Get("cursor"), kind, scope, filters)
	if e != nil {
		return domain.Page{}, errQuery
	}

	if c == nil {
		c = first
	}

	p, e := a.Store.List(r.Context(), kind, scope, limit, c, filters)
	if errors.Is(e, storage.ErrInvalidCursor) {
		e = errQuery
	}

	return p, e
}

func pageLimit(r *http.Request, maxLimit int) (int, error) {
	limit := min(20, maxLimit)
	if raw, ok := r.URL.Query()["limit"]; ok {
		if len(raw) != 1 {
			return 0, errQuery
		}

		v, e := strconv.Atoi(raw[0])
		if e != nil || v < 1 || v > maxLimit {
			return 0, errQuery
		}

		limit = v
	}

	if len(r.URL.Query()["cursor"]) > 1 {
		return 0, errQuery
	}

	return limit, nil
}

var errQuery = errors.New("invalid pagination")

func (a *API) list(w http.ResponseWriter, r *http.Request, kind, scope string) {
	p, e := a.page(r, kind, scope)
	if e != nil {
		if errors.Is(e, errQuery) {
			fail(w, 400, "invalid_pagination", "Invalid limit or cursor")
		} else {
			a.problem(w, e)
		}

		return
	}

	write(w, 200, p)
}

func (a *API) overview(w http.ResponseWriter, r *http.Request) {
	s, e := a.Store.State(r.Context())
	if e != nil {
		a.problem(w, e)

		return
	}

	l, e := a.Store.List(r.Context(), "ledgers", "", 8, nil)
	if e != nil {
		a.problem(w, e)

		return
	}

	t, e := a.Store.List(r.Context(), "transactions", "", 10, nil)
	if e != nil {
		a.problem(w, e)

		return
	}

	write(w, 200, map[string]any{
		"sync":         s,
		"ledgers":      l,
		"transactions": t,
	})
}

func sequence(v string) bool {
	n, e := strconv.ParseUint(v, 10, 32)

	return e == nil && n > 0 && strconv.FormatUint(n, 10) == v
}

func hash(v string) bool {
	if len(v) != 64 {
		return false
	}

	_, e := hex.DecodeString(v)

	return e == nil
}

func validAddress(v, kind string) bool {
	version := strkey.VersionByteAccountID
	if kind == "contracts" {
		version = strkey.VersionByteContract
	}

	p, e := strkey.Decode(version, v)

	return e == nil && len(p) == 32
}

func (a *API) ledger(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("sequence")
	if !sequence(key) {
		fail(w, 400, "invalid_sequence", "Expected a positive uint32 ledger sequence")

		return
	}

	l, e := a.Store.Get(r.Context(), "ledgers", key)
	if e != nil {
		a.problem(w, e)

		return
	}

	p, e := a.page(r, "ledger_transactions", key)
	if e != nil {
		if errors.Is(e, errQuery) {
			fail(w, 400, "invalid_pagination", "Invalid limit or cursor")
		} else {
			a.problem(w, e)
		}

		return
	}

	write(w, 200, map[string]any{
		"ledger":       l,
		"transactions": p,
	})
}

func (a *API) transaction(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("hash")
	if !hash(key) {
		fail(w, 400, "invalid_hash", "Expected a 64-character hexadecimal transaction hash")

		return
	}

	p, e := a.Store.Get(r.Context(), "transactions", strings.ToLower(key))
	if e != nil {
		a.problem(w, e)

		return
	}

	write(w, 200, p)
}

func (a *API) address(w http.ResponseWriter, r *http.Request, kind string, list bool) {
	key := r.PathValue("address")
	if !validAddress(key, kind) {
		fail(w, 400, "invalid_address", "Invalid address type or StrKey checksum")

		return
	}

	exists, e := a.Store.Exists(r.Context(), kind, key)
	if e != nil {
		a.problem(w, e)

		return
	}

	if !exists {
		a.problem(w, storage.ErrNotFound)

		return
	}

	queryKind := "account_transactions"
	if kind == "contracts" {
		queryKind = "events"
	}

	if list {
		a.list(w, r, queryKind, key)

		return
	}

	s, e := a.Store.State(r.Context())
	if e != nil {
		a.problem(w, e)

		return
	}

	txKind := "account_transactions"
	if kind == "contracts" {
		txKind = "contract_transactions"
	}

	p, e := a.page(r, txKind, key)
	if e != nil {
		if errors.Is(e, errQuery) {
			fail(w, 400, "invalid_pagination", "Invalid limit or cursor")
		} else {
			a.problem(w, e)
		}

		return
	}

	result := map[string]any{
		"address":           key,
		"source":            "indexed",
		"range":             s,
		"transactions":      p,
		"history_semantics": "transaction source, fee source, operation source; payment/create-account/account-merge destinations; decoded token senders and recipients",
	}
	if kind == "contracts" {
		contract, e := a.Store.Get(r.Context(), "contracts", key)
		if e != nil {
			a.problem(w, e)

			return
		}

		result["contract"] = contract
		events, e := a.Store.List(r.Context(), "events", key, 10, nil)
		if e != nil {
			a.problem(w, e)

			return
		}

		result["events"] = events
		result["history_semantics"] = "observed instance entries, top-level invocations, committed events, and decoded token movements; no complete internal call tree"
	}

	write(w, 200, result)
}

func (a *API) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	kind, path, key := "", "", q

	switch {
	case sequence(q):
		kind, path = "ledger", "/ledgers/"

	case hash(q):
		kind, path, key = "transaction", "/tx/", strings.ToLower(q)

	case validAddress(q, "accounts"):
		kind, path = "account", "/accounts/"

	case validAddress(q, "contracts"):
		kind, path = "contract", "/contracts/"

	default:
		fail(
			w,
			400,
			"invalid_query",
			"Enter a transaction hash, G-address, C-address or ledger sequence",
		)

		return
	}

	if kind == "ledger" || kind == "transaction" {
		table := "ledgers"
		if kind == "transaction" {
			table = "transactions"
		}

		if _, e := a.Store.Get(r.Context(), table, key); e != nil {
			a.problem(w, e)

			return
		}
	} else {
		table := "accounts"
		if kind == "contract" {
			table = "contracts"
		}

		ok, e := a.Store.Exists(r.Context(), table, key)
		if e != nil {
			a.problem(w, e)

			return
		}

		if !ok {
			a.problem(w, storage.ErrNotFound)

			return
		}
	}

	write(w, 200, map[string]string{
		"type":    kind,
		"key":     key,
		"path":    path + key,
		"network": domain.Network,
	})
}
