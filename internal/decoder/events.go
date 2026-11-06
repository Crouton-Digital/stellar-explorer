package decoder

import (
	"fmt"
	"math/big"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/amount"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// RebuildTransactionEvents uses retained metadata without altering operation event IDs.
func RebuildTransactionEvents(t *domain.Transaction, protocol uint32) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("invalid event metadata XDR: %v", recovered)
		}
	}()

	var meta xdr.TransactionMeta
	if err = xdr.SafeUnmarshalBase64(t.MetaXDR, &meta); err != nil {
		return fmt.Errorf("event metadata: %w", err)
	}

	return decodeEvents(t, meta, protocol, nil)
}

func decodeEvents(
	t *domain.Transaction,
	meta xdr.TransactionMeta,
	protocol uint32,
	participants map[string]bool,
) error {
	t.Events = []domain.Event{}
	t.Fees = domain.FeeSummary{
		Status: "unavailable",
		Source: "unavailable",
		Events: []domain.FeeEvent{},
		Reason: pointer("metadata_has_no_transaction_events"),
	}

	// Operations are atomic; failed transactions can still have committed fee events.
	if t.Successful {
		perOperation := operationEvents(meta)
		switch meta.V {
		case 3:
			if soroban := meta.MustV3().SorobanMeta; soroban != nil {
				t.ReturnValue = Value(soroban.ReturnValue)
			}
		case 4:
			metadata := meta.MustV4()
			if metadata.SorobanMeta != nil && metadata.SorobanMeta.ReturnValue != nil {
				t.ReturnValue = Value(*metadata.SorobanMeta.ReturnValue)
			}
		}

		for index, events := range perOperation {
			for _, raw := range events {
				event, err := decodeEvent(t, raw, pointer(index), nil)
				if err != nil {
					return err
				}

				t.Events = append(t.Events, event)
				if participants != nil {
					appendTokenActivity(t, event, raw, meta.V, participants)
				}
			}
		}
	}

	if meta.V != 4 {
		return nil
	}

	t.Fees.Reason = pointer("fee_events_not_provided")
	for _, transactionEvent := range meta.MustV4().Events {
		stage := eventStage(transactionEvent.Stage)
		event, err := decodeEvent(t, transactionEvent.Event, nil, &stage)
		if err != nil {
			return err
		}

		// Append transaction events so existing operation/movement cursor keys stay stable.
		t.Events = append(t.Events, event)
		collectFee(t, event, transactionEvent.Event, protocol)
	}

	finalizeFees(t)

	return nil
}

func operationEvents(meta xdr.TransactionMeta) [][]xdr.ContractEvent {
	events := [][]xdr.ContractEvent{}
	switch meta.V {
	case 3:
		if soroban := meta.MustV3().SorobanMeta; soroban != nil {
			events = append(events, soroban.Events)
		}
	case 4:
		for _, operation := range meta.MustV4().Operations {
			events = append(events, operation.Events)
		}
	}

	return events
}

func decodeEvent(
	t *domain.Transaction,
	raw xdr.ContractEvent,
	operation *int,
	stage *string,
) (domain.Event, error) {
	encoded, err := xdr.MarshalBase64(raw)
	if err != nil {
		return domain.Event{}, err
	}

	event := domain.Event{
		TransactionHash: t.Hash,
		Index:           len(t.Events),
		OpIndex:         operation,
		Scope:           "operation",
		Stage:           stage,
		LedgerSequence:  t.LedgerSequence,
		Type:            raw.Type.String(),
		Topics:          []any{},
		RawXDR:          encoded,
	}
	if operation == nil {
		event.Scope = "transaction"
	}

	if raw.ContractId != nil {
		event.ContractID, err = strkey.Encode(strkey.VersionByteContract, raw.ContractId[:])
		if err != nil {
			return event, err
		}
	}

	if raw.Body.V != 0 || raw.Body.V0 == nil {
		event.Value = map[string]any{
			"decode_status": "unknown",
			"raw_xdr":       encoded,
		}

		return event, nil
	}

	for _, topic := range raw.Body.V0.Topics {
		event.Topics = append(event.Topics, Value(topic))
	}

	event.Value = Value(raw.Body.V0.Data)

	return event, nil
}

func eventStage(stage xdr.TransactionEventStage) string {
	switch stage {
	case xdr.TransactionEventStageTransactionEventStageBeforeAllTxs:
		return "before_all_transactions"
	case xdr.TransactionEventStageTransactionEventStageAfterTx:
		return "after_transaction"
	case xdr.TransactionEventStageTransactionEventStageAfterAllTxs:
		return "after_all_transactions"
	default:
		return fmt.Sprintf("unknown:%d", stage)
	}
}

func collectFee(t *domain.Transaction, event domain.Event, raw xdr.ContractEvent, protocol uint32) {
	if raw.Body.V != 0 || raw.Body.V0 == nil || len(raw.Body.V0.Topics) == 0 {
		return
	}

	topics := raw.Body.V0.Topics
	name, ok := topics[0].GetSym()
	if !ok || name != "fee" {
		return
	}

	t.Fees.Source = "transaction_events"
	nativeID, err := xdr.MustNewNativeAsset().ContractID(domain.Passphrase)
	if err != nil || raw.Type != xdr.ContractEventTypeContract || raw.ContractId == nil ||
		[32]byte(*raw.ContractId) != nativeID || len(topics) != 2 {
		invalidFees(t, "invalid_fee_event_shape")

		return
	}

	account, valid := tokenAddress(topics[1])
	parts, isAmount := raw.Body.V0.Data.GetI128()
	if !valid || account != t.FeeAccount || !isAmount {
		invalidFees(t, "invalid_fee_account_or_amount")

		return
	}

	signed := amount.String128Raw(parts)
	quantity, _ := new(big.Int).SetString(signed, 10)
	stage := *event.Stage
	if quantity.Sign() >= 0 {
		if stage != "before_all_transactions" {
			invalidFees(t, "invalid_fee_stage")

			return
		}
	} else {
		expected := "after_all_transactions"
		if protocol < 23 {
			expected = "after_transaction"
		}

		if stage != expected {
			invalidFees(t, "invalid_refund_stage")

			return
		}
	}

	t.Fees.Events = append(t.Fees.Events, domain.FeeEvent{
		EventIndex: event.Index,
		Account:    account,
		Amount:     signed,
		Stage:      stage,
	})
}

func invalidFees(t *domain.Transaction, reason string) {
	t.Fees.Status = "invalid"
	t.Fees.Reason = &reason
}

func finalizeFees(t *domain.Transaction) {
	if t.Fees.Status == "invalid" || len(t.Fees.Events) == 0 {
		return
	}

	charged, refunded := new(big.Int), new(big.Int)
	charges, refunds := 0, 0
	for _, event := range t.Fees.Events {
		quantity, _ := new(big.Int).SetString(event.Amount, 10)
		if quantity.Sign() < 0 {
			refunded.Sub(refunded, quantity)
			refunds++
		} else {
			charged.Add(charged, quantity)
			charges++
		}
	}

	if charges != 1 || refunds > 1 || refunded.Cmp(charged) > 0 {
		invalidFees(t, "incomplete_or_duplicate_fee_events")

		return
	}

	net := new(big.Int).Sub(charged, refunded)
	if net.String() != t.FeeCharged {
		invalidFees(t, "fee_events_do_not_match_result")

		return
	}

	t.Fees.Status = "complete"
	t.Fees.Reason = nil
	t.Fees.Charged = pointer(charged.String())
	t.Fees.Refunded = pointer(refunded.String())
	t.Fees.Net = pointer(net.String())
}
