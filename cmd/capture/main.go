// capture downloads public Testnet fixtures; it never signs or submits transactions.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	urlpkg "net/url"
	"os"
	"time"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/stellar"
)

func main() {
	output := flag.String("output", "tests/fixtures/testnet.json", "fixture output")
	span := flag.Uint("span", 200, "look back at most this many ledgers")
	flag.Parse()
	if e := capture(*output, uint32(*span)); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}

func capture(output string, span uint32) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	url := os.Getenv("STELLAR_RPC_URL")
	if url == "" {
		url = "https://soroban-testnet.stellar.org"
	}

	c := stellar.New(url)
	p, e := c.Network(ctx)
	if e != nil {
		return e
	}

	if p != domain.Passphrase {
		return fmt.Errorf("expected Testnet")
	}

	h, e := c.Health(ctx)
	if e != nil {
		return e
	}

	start := h.Oldest
	if h.Latest > span && h.Latest-span > start {
		start = h.Latest - span
	}

	selected := []stellar.RemoteLedger{}
	seen := map[string]bool{}

	for next := start; next <= h.Latest; {
		ls, e := c.Ledgers(ctx, next, 10)
		if e != nil {
			return e
		}

		if len(ls) == 0 {
			break
		}

		for _, l := range ls {
			b, e := decoder.Ledger(l)
			if e != nil {
				return e
			}

			tags := map[string]bool{}

			for _, t := range b.Transactions {
				if !t.Successful {
					tags["failed"] = true
				}

				for _, o := range t.Operations {
					if o.Type == "OperationTypePayment" {
						tags["payment"] = true
					}
				}

				if len(t.Calls) > 0 && t.Successful {
					tags["soroban"] = true
				}

				if len(t.Events) > 0 {
					tags["events"] = true
				}
			}

			use := false

			for tag := range tags {
				if !seen[tag] {
					use = true
				}
			}

			if use {
				selected = append(selected, l)

				for tag := range tags {
					seen[tag] = true
				}

				fmt.Printf(
					"captured ledger %d (%d transactions)\n",
					l.Sequence,
					len(b.Transactions),
				)
			}

			next = l.Sequence + 1
		}

		if seen["payment"] && seen["soroban"] && seen["events"] && seen["failed"] {
			break
		}
	}

	if len(selected) == 0 {
		return fmt.Errorf("no useful fixtures in observed range")
	}

	origin, _ := urlpkg.Parse(url)
	fixture := map[string]any{
		"network":     domain.Network,
		"source":      origin.Scheme + "://" + origin.Host,
		"captured_at": time.Now().UTC(),
		"coverage":    seen,
		"ledgers":     selected,
	}
	raw, e := json.MarshalIndent(fixture, "", "  ")
	if e != nil {
		return e
	}

	return os.WriteFile(output, append(raw, '\n'), 0644)
}
