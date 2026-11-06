package domain

// Filters contains exact-match conditions. Ledger bounds are inclusive.
type Filters struct {
	Address    string `json:"address,omitempty"`
	Contract   string `json:"contract,omitempty"`
	Function   string `json:"function,omitempty"`
	Status     string `json:"status,omitempty"`
	Asset      string `json:"asset,omitempty"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	NFTTokenID string `json:"token_id,omitempty"`
	NFTKind    string `json:"kind,omitempty"`
	EventType  string `json:"event_type,omitempty"`
	EventName  string `json:"event_name,omitempty"`
	EventScope string `json:"event_scope,omitempty"`
	MinLedger  uint32 `json:"min_ledger,omitempty"`
	MaxLedger  uint32 `json:"max_ledger,omitempty"`
}
