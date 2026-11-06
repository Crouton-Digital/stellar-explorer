package domain

import (
	"regexp"
	"strings"
)

var searchHashPrefix = regexp.MustCompile(`^[0-9a-fA-F]{4,64}$`)

var searchAddressPrefix = regexp.MustCompile(`^[GC][A-Z2-7]{3,55}$`)

var searchLedgerPrefix = regexp.MustCompile(`^[1-9][0-9]{0,9}$`)

type SearchMatch struct {
	Type string `json:"type"`
	Key  string `json:"key"`
	Path string `json:"path"`
}

type SearchSuggestions struct {
	Network string        `json:"network"`
	Query   string        `json:"query"`
	Limit   int           `json:"limit"`
	Items   []SearchMatch `json:"items"`
}

// SearchKinds permits bounded prefixes without treating arbitrary text as a database pattern.
func SearchKinds(query string) []string {
	kinds := []string{}
	if searchLedgerPrefix.MatchString(query) {
		kinds = append(kinds, "ledger")
	}

	if searchHashPrefix.MatchString(query) {
		kinds = append(kinds, "transaction")
	}

	if searchAddressPrefix.MatchString(strings.ToUpper(query)) {
		if strings.HasPrefix(strings.ToUpper(query), "G") {
			kinds = append(kinds, "account")
		} else {
			kinds = append(kinds, "contract")
		}
	}

	return kinds
}
