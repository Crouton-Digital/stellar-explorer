package httpapi_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/httpapi"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestInvalidSearchSuggestionsNeverReachStorage(t *testing.T) {
	handler := (&httpapi.API{MaxLimit: 100}).Handler()
	for _, suffix := range []string{
		"", "?q=", "?q=abc", "?q=GAB", "?q=G___", "?q=abcd&q=ffff",
		"?q=abcd&limit=999", "?q=abcd&network=mainnet", "?q=" + strings.Repeat("a", 65),
	} {
		t.Run(suffix, func(t *testing.T) {
			response := request(handler, "GET", "/api/v1/search/suggestions"+suffix)
			if response.Code != 400 {
				t.Fatalf("invalid live search reached storage: %d %s", response.Code, response.Body)
			}
		})
	}
}

func TestSearchSuggestionsReturnIndexedResultsWithoutNavigating(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	bundle := testutil.RealBundle(t)
	if err := s.Save(ctx, bundle); err != nil {
		t.Fatal(err)
	}

	handler := (&httpapi.API{Store: s, MaxLimit: 100}).Handler()
	hash := bundle.Transactions[0].Hash
	response := request(handler, "GET", "/api/v1/search/suggestions?q="+hash[:12])
	var result domain.SearchSuggestions
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 ||
		len(
			result.Items,
		) != 1 || result.Items[0].Key != hash || result.Items[0].Path != "/tx/"+hash {
		t.Fatalf("incorrect live search response: %d %s %v", response.Code, response.Body, err)
	}

	response = request(handler, "GET", "/api/v1/search/suggestions?q="+testutil.Address(99))
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 ||
		len(result.Items) != 0 {
		t.Fatalf(
			"absence should return an empty indexed result set: %d %s %v",
			response.Code,
			response.Body,
			err,
		)
	}
}
