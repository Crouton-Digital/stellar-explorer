package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
)

type nftRepository interface {
	NFTSnapshot(context.Context) (storage.Cursor, error)
	NFTCollection(context.Context, string, uint32) (domain.NFTCollectionView, error)
	NFTItem(context.Context, string, string, uint32) (domain.NFTItem, error)
}

func (a *API) nftRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/nfts/collections", func(w http.ResponseWriter, r *http.Request) {
		a.list(w, r, "nft_collections", "")
	})
	m.HandleFunc("GET /api/v1/nfts/transfers", func(w http.ResponseWriter, r *http.Request) {
		a.list(w, r, "nft_transfers", "")
	})
	m.HandleFunc("GET /api/v1/nfts/collections/{address}", a.nftDetail)
	m.HandleFunc("GET /api/v1/nfts/collections/{address}/tokens/{id}", a.nftDetail)
	m.HandleFunc("GET /api/v1/nfts/collections/{address}/transfers", a.nftCollectionHistory)
	for _, kind := range []string{"accounts", "contracts"} {
		m.HandleFunc("GET /api/v1/"+kind+"/{address}/nft-transfers",
			func(w http.ResponseWriter, r *http.Request) {
				address := r.PathValue("address")
				if !validAddress(address, kind) {
					fail(w, 400, "invalid_address", "Invalid address type or StrKey checksum")

					return
				}

				queryKind := "account_nft_transfers"
				if kind == "contracts" {
					queryKind = "contract_nft_transfers"
				}

				a.list(w, r, queryKind, address)
			})
	}
}

func (a *API) nftCollectionHistory(w http.ResponseWriter, r *http.Request) {
	address := r.PathValue("address")
	if !validAddress(address, "contracts") {
		fail(w, 400, "invalid_address", "Expected an NFT collection contract C-address")

		return
	}

	a.list(w, r, "collection_nft_transfers", address)
}

func (a *API) nftDetail(w http.ResponseWriter, r *http.Request) {
	address, token := r.PathValue("address"), r.PathValue("id")
	if !validAddress(address, "contracts") {
		fail(w, 400, "invalid_address", "Expected an NFT collection contract C-address")

		return
	}

	if token != "" && !storage.ValidNFTTokenID(token) {
		fail(
			w,
			400,
			"invalid_token_id",
			"Expected a canonical unsigned decimal token ID up to u256",
		)

		return
	}

	kind, scope := "collection_nft_transfers", address
	if token != "" {
		kind, scope = "nft_token_transfers", address+":"+token
	}

	if _, err := pageLimit(r, a.MaxLimit); err != nil {
		fail(w, 400, "invalid_pagination", "Invalid NFT history limit or cursor")

		return
	}

	filters, err := parseFilters(r.URL.Query(), kind)
	if err != nil {
		a.problem(w, err)

		return
	}

	cursor, err := storage.ParseCursor(r.URL.Query().Get("cursor"), kind, scope, filters)
	if err != nil {
		fail(w, 400, "invalid_pagination", "Invalid NFT history cursor")

		return
	}

	repository, ok := a.Store.(nftRepository)
	if !ok {
		fail(w, 503, "storage_unavailable", "Indexed NFT data is temporarily unavailable")

		return
	}

	if cursor == nil {
		initial, err := repository.NFTSnapshot(r.Context())
		if err != nil {
			a.problem(w, err)

			return
		}

		initial.Kind, initial.Scope = kind, scope
		if filters != (domain.Filters{}) {
			initial.Filters = &filters
		}

		cursor = &initial
	}

	if cursor.Snapshot == 0 {
		a.problem(w, storage.ErrNotFound)

		return
	}

	transfers, err := a.pageWithCursor(r, kind, scope, cursor)
	if err != nil {
		if errors.Is(err, errQuery) {
			fail(w, 400, "invalid_pagination", "Invalid NFT history limit or cursor")
		} else {
			a.problem(w, err)
		}

		return
	}

	result := map[string]any{
		"transfers":       transfers,
		"snapshot_ledger": cursor.Snapshot,
	}
	if token == "" {
		collection, err := repository.NFTCollection(r.Context(), address, cursor.Snapshot)
		if err != nil {
			a.problem(w, err)

			return
		}

		result["collection"] = collection
	} else {
		item, err := repository.NFTItem(r.Context(), address, token, cursor.Snapshot)
		if err != nil {
			a.problem(w, err)

			return
		}

		result["item"] = item
	}

	state, err := a.Store.State(r.Context())
	if err != nil {
		a.problem(w, err)

		return
	}

	result["range"] = state
	write(w, 200, result)
}
