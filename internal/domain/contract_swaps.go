package domain

// ContractSwap is an adapter-specific interpretation of committed source evidence.
type ContractSwap struct {
	Protocol             string            `json:"protocol"`
	AdapterVersion       int               `json:"adapter_version"`
	Verification         string            `json:"verification"`
	RouterContract       string            `json:"router_contract"`
	RouterWasmHash       *string           `json:"router_wasm_hash"`
	Mode                 string            `json:"mode"`
	Trader               string            `json:"trader"`
	InputLimit           string            `json:"input_limit"`
	OutputLimit          string            `json:"output_limit"`
	Deadline             string            `json:"deadline"`
	Path                 []string          `json:"path"`
	Amounts              []string          `json:"amounts"`
	Input                ExchangeLeg       `json:"input"`
	Output               ExchangeLeg       `json:"output"`
	Hops                 []ContractSwapHop `json:"hops"`
	RouterEventIndex     int               `json:"router_event_index"`
	TransferEventIndexes []int             `json:"transfer_event_indexes"`
}

type ContractSwapHop struct {
	Index            int         `json:"index"`
	PoolContract     string      `json:"pool_contract"`
	Input            ExchangeLeg `json:"input"`
	Output           ExchangeLeg `json:"output"`
	InputEventIndex  int         `json:"input_event_index"`
	OutputEventIndex int         `json:"output_event_index"`
}
