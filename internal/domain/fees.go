package domain

type FeeSummary struct {
	Status   string     `json:"status"`
	Source   string     `json:"source"`
	Charged  *string    `json:"charged"`
	Refunded *string    `json:"refunded"`
	Net      *string    `json:"net"`
	Events   []FeeEvent `json:"events"`
	Reason   *string    `json:"reason"`
}

type FeeEvent struct {
	EventIndex int    `json:"event_index"`
	Account    string `json:"account"`
	Amount     string `json:"amount"`
	Stage      string `json:"stage"`
}
