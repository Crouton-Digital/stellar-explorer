package migrations

import _ "embed"

//go:embed 001_init.sql
var Schema string

//go:embed 002_observability.sql
var Observability string

//go:embed 003_tokens.sql
var Tokens string

//go:embed 004_contracts.sql
var Contracts string

//go:embed 005_filters.sql
var Filters string

//go:embed 006_transaction_events.sql
var TransactionEvents string

//go:embed 007_interpretation.sql
var Interpretation string

//go:embed 008_state_and_swaps.sql
var StateAndSwaps string

//go:embed 009_contract_swaps.sql
var ContractSwaps string

//go:embed 010_aquarius_swaps.sql
var AquariusSwaps string

//go:embed 011_aquarius_settlement.sql
var AquariusSettlement string

//go:embed 012_nfts.sql
var NFTs string

//go:embed 013_search.sql
var Search string

type Migration struct {
	Version int
	SQL     string
}

var All = []Migration{
	{1, Schema},
	{2, Observability},
	{3, Tokens},
	{4, Contracts},
	{5, Filters},
	{6, TransactionEvents},
	{7, Interpretation},
	{8, StateAndSwaps},
	{9, ContractSwaps},
	{10, AquariusSwaps},
	{11, AquariusSettlement},
	{12, NFTs},
	{13, Search},
}
