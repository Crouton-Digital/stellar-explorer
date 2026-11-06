package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/crouton/stellar-explorer/internal/domain"
)

type searchRepository interface {
	Suggest(context.Context, string) (domain.SearchSuggestions, error)
}

func (a *API) suggestions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	for key, values := range query {
		if (key != "q" && key != "network") || len(values) != 1 {
			fail(w, 400, "invalid_query", "Expected one search prefix and an optional network")

			return
		}
	}

	prefix := strings.TrimSpace(query.Get("q"))
	if len(domain.SearchKinds(prefix)) == 0 {
		fail(
			w,
			400,
			"invalid_query",
			"Enter a ledger number or at least four hash/address characters",
		)

		return
	}

	repository, ok := a.Store.(searchRepository)
	if !ok {
		fail(w, 503, "storage_unavailable", "Indexed search is temporarily unavailable")

		return
	}

	result, err := repository.Suggest(r.Context(), prefix)
	if err != nil {
		a.problem(w, err)

		return
	}

	write(w, 200, result)
}
