package domain

// StateChange describes one committed metadata write, not the current network state.
type StateChange struct {
	ID           string         `json:"id"`
	Kind         string         `json:"kind"`
	EntryType    string         `json:"entry_type"`
	Scope        string         `json:"scope"`
	OpIndex      *int           `json:"op_index"`
	ChangeIndex  int            `json:"change_index"`
	BeforeIndex  *int           `json:"before_index"`
	Subject      string         `json:"subject"`
	Key          any            `json:"key"`
	KeyXDR       string         `json:"key_xdr"`
	BeforeStatus string         `json:"before_status"`
	AfterStatus  string         `json:"after_status"`
	Before       *StateSnapshot `json:"before"`
	After        *StateSnapshot `json:"after"`
}

type StateSnapshot struct {
	LastModifiedLedger uint32         `json:"last_modified_ledger"`
	Projection         string         `json:"projection"`
	Fields             map[string]any `json:"fields"`
}
