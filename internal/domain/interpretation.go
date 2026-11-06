package domain

type Interpretation struct {
	Version              int               `json:"version"`
	Summary              string            `json:"summary"`
	Actions              []Action          `json:"actions"`
	Failure              *Failure          `json:"failure"`
	DiagnosticErrors     []DiagnosticError `json:"diagnostic_errors"`
	DiagnosticsTruncated bool              `json:"diagnostics_truncated"`
	StateChanges         []StateChange     `json:"state_changes"`
}

type Action struct {
	ID            string        `json:"id"`
	Kind          string        `json:"kind"`
	Title         string        `json:"title"`
	Description   string        `json:"description"`
	Status        string        `json:"status"`
	Source        string        `json:"source"`
	OpIndex       int           `json:"op_index"`
	EventIndexes  []int         `json:"event_indexes"`
	From          string        `json:"from,omitempty"`
	SourceAccount string        `json:"source_account,omitempty"`
	To            string        `json:"to,omitempty"`
	ContractID    string        `json:"contract_id,omitempty"`
	Function      string        `json:"function,omitempty"`
	Amount        *string       `json:"amount"`
	Asset         *Asset        `json:"asset"`
	ToMuxed       any           `json:"to_muxed,omitempty"`
	ResultCode    *string       `json:"result_code"`
	PathPayment   *PathPayment  `json:"path_payment,omitempty"`
	ContractSwap  *ContractSwap `json:"contract_swap,omitempty"`
	AquariusSwap  *AquariusSwap `json:"aquarius_swap,omitempty"`
}

type Failure struct {
	Code       string             `json:"code"`
	InnerCode  *string            `json:"inner_code"`
	Message    string             `json:"message"`
	Operations []OperationFailure `json:"operations"`
}

type OperationFailure struct {
	OpIndex int    `json:"op_index"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type DiagnosticError struct {
	EventIndex int    `json:"event_index"`
	ContractID string `json:"contract_id,omitempty"`
	Type       string `json:"type"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}
