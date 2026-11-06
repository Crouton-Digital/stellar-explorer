package domain

type NFTCollectionView struct {
	NFTCollection
	ObservedTokenCount uint64 `json:"observed_token_count"`
	TransferCount      uint64 `json:"transfer_count"`
}

type NFTItem struct {
	Network             string `json:"network"`
	ContractID          string `json:"contract_id"`
	TokenID             string `json:"token_id"`
	FirstObservedLedger uint32 `json:"first_observed_ledger"`
	TransferCount       uint64 `json:"transfer_count"`
	Verification        string `json:"verification"`
}
