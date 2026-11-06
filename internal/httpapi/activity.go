package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
)

type activityRepository interface {
	Activity(context.Context, string, *storage.Cursor, domain.Filters) (domain.Activity, error)
}

func (a *API) activityRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/activity/{kind}", a.activity)
}

func (a *API) activity(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if kind != "transactions" && kind != "transfers" && kind != "events" {
		fail(w, 400, "invalid_activity", "Expected transactions, transfers or events")

		return
	}

	query := r.URL.Query()
	if _, exists := query["limit"]; exists || len(query["cursor"]) > 1 {
		fail(
			w,
			400,
			"invalid_activity",
			"Activity uses a fixed ledger window and one optional list cursor",
		)

		return
	}

	filters, err := parseFilters(query, kind)
	if err != nil {
		a.problem(w, err)

		return
	}

	cursor, err := storage.ParseCursor(query.Get("cursor"), kind, "", filters)
	if err != nil {
		fail(w, 400, "invalid_pagination", "Invalid list cursor for this activity query")

		return
	}

	repository, ok := a.Store.(activityRepository)
	if !ok {
		fail(w, 503, "storage_unavailable", "Indexed activity is temporarily unavailable")

		return
	}

	activity, err := repository.Activity(r.Context(), kind, cursor, filters)
	if errors.Is(err, storage.ErrInvalidCursor) {
		fail(w, 400, "invalid_pagination", "Invalid list cursor for this activity query")

		return
	}

	if err != nil {
		a.problem(w, err)

		return
	}

	write(w, 200, activity)
}
