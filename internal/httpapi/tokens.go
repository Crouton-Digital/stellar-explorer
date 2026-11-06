package httpapi

import (
	"errors"
	"net/http"
)

func (a *API) tokenRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/assets", func(w http.ResponseWriter, r *http.Request) {
		a.list(w, r, "assets", "")
	})

	m.HandleFunc("GET /api/v1/transfers", func(w http.ResponseWriter, r *http.Request) {
		a.list(w, r, "transfers", "")
	})

	m.HandleFunc("GET /api/v1/assets/{address}", a.asset)

	for _, kind := range []string{"assets", "accounts", "contracts"} {
		m.HandleFunc(
			"GET /api/v1/"+kind+"/{address}/transfers",
			func(w http.ResponseWriter, r *http.Request) {
				a.tokenHistory(w, r, kind)
			},
		)
	}
}

func (a *API) asset(w http.ResponseWriter, r *http.Request) {
	address := r.PathValue("address")
	if !validAddress(address, "contracts") {
		fail(w, 400, "invalid_address", "Expected a token contract C-address")

		return
	}

	asset, e := a.Store.Get(r.Context(), "assets", address)
	if e != nil {
		a.problem(w, e)

		return
	}

	transfers, e := a.page(r, "asset_transfers", address)
	if e != nil {
		if errors.Is(e, errQuery) {
			fail(w, 400, "invalid_pagination", "Invalid limit or cursor")
		} else {
			a.problem(w, e)
		}

		return
	}

	state, e := a.Store.State(r.Context())
	if e != nil {
		a.problem(w, e)

		return
	}

	write(w, 200, map[string]any{
		"asset":     asset,
		"transfers": transfers,
		"range":     state,
	})
}

func (a *API) tokenHistory(w http.ResponseWriter, r *http.Request, kind string) {
	address := r.PathValue("address")
	addressKind := kind
	if kind == "assets" {
		addressKind = "contracts"
	}

	if !validAddress(address, addressKind) {
		fail(w, 400, "invalid_address", "Invalid address type or StrKey checksum")

		return
	}

	if kind == "assets" {
		if _, e := a.Store.Get(r.Context(), "assets", address); e != nil {
			a.problem(w, e)

			return
		}
	} else {
		exists, e := a.Store.Exists(r.Context(), kind, address)
		if e != nil {
			a.problem(w, e)

			return
		}

		if !exists {
			fail(w, 404, "not_found", "Entity not found in indexed data")

			return
		}
	}

	queryKind := "asset_transfers"
	if kind == "accounts" {
		queryKind = "account_transfers"
	} else if kind == "contracts" {
		queryKind = "contract_transfers"
	}

	a.list(w, r, queryKind, address)
}
