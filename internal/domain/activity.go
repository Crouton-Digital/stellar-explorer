package domain

import "time"

// Activity describes matching records in a bounded window of indexed ledgers.
type Activity struct {
	Network     string             `json:"network"`
	Kind        string             `json:"kind"`
	WindowLimit int                `json:"window_limit"`
	FirstLedger *uint32            `json:"first_ledger"`
	LastLedger  *uint32            `json:"last_ledger"`
	LedgerCount int                `json:"ledger_count"`
	From        *time.Time         `json:"from"`
	To          *time.Time         `json:"to"`
	Total       uint64             `json:"total"`
	Buckets     []ActivityBucket   `json:"buckets"`
	Breakdown   []ActivityCategory `json:"breakdown"`
}

type ActivityBucket struct {
	FirstLedger uint32    `json:"first_ledger"`
	LastLedger  uint32    `json:"last_ledger"`
	ClosedAt    time.Time `json:"closed_at"`
	Count       uint64    `json:"count"`
}

type ActivityCategory struct {
	Key   string `json:"key"`
	Count uint64 `json:"count"`
}
