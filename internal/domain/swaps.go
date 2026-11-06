package domain

type PathPayment struct {
	Mode             string    `json:"mode"`
	SendAsset        Asset     `json:"send_asset"`
	DestinationAsset Asset     `json:"destination_asset"`
	SendLimit        string    `json:"send_limit"`
	DestinationLimit string    `json:"destination_limit"`
	Path             []Asset   `json:"path"`
	Execution        *Exchange `json:"execution"`
}

type Exchange struct {
	Protocol string      `json:"protocol"`
	Input    ExchangeLeg `json:"input"`
	Output   ExchangeLeg `json:"output"`
	Trades   []Trade     `json:"trades"`
}

type ExchangeLeg struct {
	Asset  Asset  `json:"asset"`
	Amount string `json:"amount"`
}

type Trade struct {
	ClaimIndex int         `json:"claim_index"`
	Venue      string      `json:"venue"`
	Input      ExchangeLeg `json:"input"`
	Output     ExchangeLeg `json:"output"`
}
