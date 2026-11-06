package domain

// Contract describes evidence observed in indexed transactions, not live RPC state.
type Contract struct {
	Network             string  `json:"network"`
	ContractID          string  `json:"contract_id"`
	Kind                string  `json:"kind"`
	WasmHash            *string `json:"wasm_hash"`
	ExecutableSource    *string `json:"executable_source"`
	ExecutableLedger    *uint32 `json:"executable_ledger"`
	ExecutableTxIndex   *uint32 `json:"executable_tx_index"`
	FirstObservedLedger uint32  `json:"first_observed_ledger"`
	LastObservedLedger  uint32  `json:"last_observed_ledger"`
	CreationLedger      *uint32 `json:"creation_ledger"`
	CreationTransaction *string `json:"creation_transaction"`
	Creator             *string `json:"creator"`
}
