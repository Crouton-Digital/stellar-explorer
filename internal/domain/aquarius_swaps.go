package domain

// AquariusSwap describes a supported router exchange using committed evidence.
type AquariusSwap struct {
	Protocol             string              `json:"protocol"`
	AdapterVersion       int                 `json:"adapter_version"`
	Verification         string              `json:"verification"`
	RouterContract       string              `json:"router_contract"`
	RouterWasmHash       *string             `json:"router_wasm_hash"`
	Mode                 string              `json:"mode"`
	Routing              string              `json:"routing"`
	Trader               string              `json:"trader"`
	InputLimit           string              `json:"input_limit"`
	OutputLimit          string              `json:"output_limit"`
	Path                 []string            `json:"path"`
	Input                ExchangeLeg         `json:"input"`
	Output               ExchangeLeg         `json:"output"`
	Hops                 []AquariusSwapHop   `json:"hops"`
	TransferEventIndexes []int               `json:"transfer_event_indexes"`
	Settlement           *AquariusSettlement `json:"settlement,omitempty"`
	Routes               []AquariusSwapRoute `json:"routes,omitempty"`
}

type AquariusSwapRoute struct {
	Index      int         `json:"index"`
	WeightBps  uint32      `json:"weight_bps"`
	Path       []string    `json:"path"`
	Input      ExchangeLeg `json:"input"`
	Output     ExchangeLeg `json:"output"`
	HopIndexes []int       `json:"hop_indexes"`
}

type AquariusSettlement struct {
	FundedAmount      string `json:"funded_amount"`
	RefundedAmount    string `json:"refunded_amount"`
	FundingEventIndex int    `json:"funding_event_index"`
	RefundEventIndex  *int   `json:"refund_event_index"`
	PayoutEventIndex  int    `json:"payout_event_index"`
}

type AquariusSwapHop struct {
	Index            int         `json:"index"`
	PoolTokens       []string    `json:"pool_tokens"`
	PoolIndex        string      `json:"pool_index"`
	PoolContract     string      `json:"pool_contract"`
	Input            ExchangeLeg `json:"input"`
	Output           ExchangeLeg `json:"output"`
	InputEventIndex  int         `json:"input_event_index"`
	OutputEventIndex int         `json:"output_event_index"`
	TradeEventIndex  int         `json:"trade_event_index"`
	RouterEventIndex int         `json:"router_event_index"`
}
