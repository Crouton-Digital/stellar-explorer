package stellar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	URL  string
	HTTP *http.Client
}

type Health struct {
	Latest uint32 `json:"latestLedger"`
	Oldest uint32 `json:"oldestLedger"`
	Status string `json:"status"`
}

type RemoteLedger struct {
	Sequence    uint32 `json:"sequence"`
	Hash        string `json:"hash"`
	HeaderXDR   string `json:"headerXdr"`
	MetadataXDR string `json:"metadataXdr"`
}

func New(url string) *Client {
	return &Client{
		URL: url,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) call(ctx context.Context, method string, params any, out any) error {
	body, e := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if e != nil {
		return e
	}

	req, e := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if e != nil {
		return e
	}

	req.Header.Set("Content-Type", "application/json")
	res, e := c.HTTP.Do(req)
	if e != nil {
		var requestError *url.Error
		if errors.As(e, &requestError) {
			e = requestError.Err
		}

		return fmt.Errorf("RPC %s: %w", method, e)
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		return fmt.Errorf("RPC %s: HTTP %d", method, res.StatusCode)
	}

	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	const max = 64 << 20
	raw, e := io.ReadAll(io.LimitReader(res.Body, max+1))
	if e != nil {
		return e
	}

	if len(raw) > max {
		return fmt.Errorf("RPC response exceeds 64 MiB")
	}

	if e = json.Unmarshal(raw, &envelope); e != nil {
		return fmt.Errorf("RPC response: %w", e)
	}

	if envelope.Error != nil {
		return fmt.Errorf("RPC %s (%d): %s", method, envelope.Error.Code, envelope.Error.Message)
	}

	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return fmt.Errorf("RPC %s: missing result", method)
	}

	return json.Unmarshal(envelope.Result, out)
}

func (c *Client) Network(ctx context.Context) (string, error) {
	var r struct {
		Passphrase string `json:"passphrase"`
	}
	e := c.call(ctx, "getNetwork", map[string]any{}, &r)

	return r.Passphrase, e
}

func (c *Client) Health(ctx context.Context) (Health, error) {
	var r Health
	e := c.call(ctx, "getHealth", map[string]any{}, &r)
	if e == nil && (r.Oldest == 0 || r.Latest < r.Oldest || r.Status != "healthy") {
		e = fmt.Errorf("RPC unhealthy or invalid ledger range")
	}

	return r, e
}

func (c *Client) Ledgers(ctx context.Context, start uint32, limit int) ([]RemoteLedger, error) {
	var r struct {
		Ledgers []RemoteLedger `json:"ledgers"`
	}
	e := c.call(ctx, "getLedgers", map[string]any{
		"startLedger": start,
		"pagination": map[string]int{
			"limit": limit,
		},
	}, &r)

	return r.Ledgers, e
}
