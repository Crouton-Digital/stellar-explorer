package stellar_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/crouton/stellar-explorer/internal/stellar"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func client(body string, status int, check func(*http.Request)) *stellar.Client {
	c := stellar.New("https://example.invalid")
	c.HTTP = &http.Client{
		Transport: transport(func(r *http.Request) (*http.Response, error) {
			if check != nil {
				check(r)
			}

			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	return c
}

func TestRPCHealthAndNamedLedgerParameters(t *testing.T) {
	c := client(`{"result":{"status":"healthy","oldestLedger":10,"latestLedger":20}}`, 200, nil)
	if h, e := c.Health(context.Background()); e != nil || h.Oldest != 10 || h.Latest != 20 {
		t.Fatalf("health: %+v %v", h, e)
	}

	c = client(`{"result":{"ledgers":[]}}`, 200, func(r *http.Request) {
		var body struct {
			Method string
			Params struct {
				Start      uint32 `json:"startLedger"`
				Pagination struct {
					Limit int
				}
			}
		}
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Fatal(e)
		}

		if r.Method != "POST" || body.Method != "getLedgers" || body.Params.Start != 12 ||
			body.Params.Pagination.Limit != 5 {
			t.Fatal("incorrect RPC request")
		}
	})
	if _, e := c.Ledgers(context.Background(), 12, 5); e != nil {
		t.Fatal(e)
	}
}

func TestRPCFailuresAndInvalidRanges(t *testing.T) {
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"error":{"code":-32601,"message":"unsupported"}}`, 200},
		{`{`, 200},
		{`{"result":null}`, 200},
		{`{"result":{"status":"healthy","oldestLedger":20,"latestLedger":10}}`, 200},
		{`{"result":{"status":"unhealthy","oldestLedger":10,"latestLedger":20}}`, 200},
		{`{}`, 429},
	} {
		t.Run(fmt.Sprint(tc.status, tc.body), func(t *testing.T) {
			if _, e := client(tc.body, tc.status, nil).Health(context.Background()); e == nil {
				t.Fatal("RPC failure silently accepted")
			}
		})
	}
}
