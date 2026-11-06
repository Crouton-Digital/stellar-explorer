package httpapi

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
)

var errFilters = errors.New("invalid filters")

var symbolPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,32}$`)

func parseFilters(query url.Values, kind string) (domain.Filters, error) {
	filters := domain.Filters{}
	allowed := map[string]bool{
		"network": true,
		"limit":   true,
		"cursor":  true,
	}
	var fields []string
	switch kind {
	case "transactions", "account_transactions", "contract_transactions", "ledger_transactions":
		fields = []string{"address", "contract", "function", "status", "min_ledger", "max_ledger"}
	case "events":
		fields = []string{
			"contract",
			"event_type",
			"event_name",
			"event_scope",
			"min_ledger",
			"max_ledger",
		}
	case "transfers", "asset_transfers", "account_transfers", "contract_transfers":
		fields = []string{"asset", "from", "to", "min_ledger", "max_ledger"}
	case "nft_transfers", "collection_nft_transfers", "nft_token_transfers",
		"account_nft_transfers", "contract_nft_transfers":
		fields = []string{
			"collection",
			"token_id",
			"kind",
			"from",
			"to",
			"min_ledger",
			"max_ledger",
		}
	}

	for _, field := range fields {
		allowed[field] = true
	}

	for key, values := range query {
		if key == "limit" || key == "cursor" {
			continue
		}

		if !allowed[key] || len(values) != 1 || (key != "cursor" && values[0] == "") {
			return filters, errFilters
		}
	}

	filters.Address = query.Get("address")
	filters.Contract = query.Get("contract")
	filters.Function = query.Get("function")
	filters.Status = query.Get("status")
	filters.Asset = query.Get("asset")
	filters.From = query.Get("from")
	filters.To = query.Get("to")
	filters.EventType = query.Get("event_type")
	filters.EventName = query.Get("event_name")
	filters.EventScope = query.Get("event_scope")
	if collection := query.Get("collection"); collection != "" {
		filters.Contract = collection
	}

	filters.NFTTokenID = query.Get("token_id")
	filters.NFTKind = query.Get("kind")
	if filters.NFTTokenID != "" && !storage.ValidNFTTokenID(filters.NFTTokenID) {
		return filters, errFilters
	}

	if filters.NFTKind != "" && filters.NFTKind != "transfer" &&
		filters.NFTKind != "mint" && filters.NFTKind != "burn" {
		return filters, errFilters
	}

	for _, address := range []string{filters.Address, filters.From, filters.To} {
		if address != "" && !validAddress(address, "accounts") &&
			!validAddress(address, "contracts") {
			return filters, errFilters
		}
	}

	for _, address := range []string{filters.Contract, filters.Asset} {
		if address != "" && !validAddress(address, "contracts") {
			return filters, errFilters
		}
	}

	for _, symbol := range []string{filters.Function, filters.EventName} {
		if symbol != "" && !symbolPattern.MatchString(symbol) {
			return filters, errFilters
		}
	}

	if filters.Status != "" && filters.Status != "success" && filters.Status != "failed" {
		return filters, errFilters
	}

	if filters.EventType != "" && filters.EventType != "contract" &&
		filters.EventType != "system" && filters.EventType != "diagnostic" {
		return filters, errFilters
	}

	if filters.EventScope != "" && filters.EventScope != "operation" &&
		filters.EventScope != "transaction" {
		return filters, errFilters
	}

	for key, target := range map[string]*uint32{
		"min_ledger": &filters.MinLedger,
		"max_ledger": &filters.MaxLedger,
	} {
		if raw := query.Get(key); raw != "" {
			if !sequence(raw) {
				return filters, errFilters
			}

			value, _ := strconv.ParseUint(raw, 10, 32)
			*target = uint32(value)
		}
	}

	if filters.MaxLedger != 0 && filters.MinLedger > filters.MaxLedger {
		return filters, errFilters
	}

	return filters, nil
}
