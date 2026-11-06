package storage

import (
	"fmt"
	"strings"

	"github.com/crouton/stellar-explorer/internal/domain"
)

func selectedFilters(filters []domain.Filters) domain.Filters {
	if len(filters) == 0 {
		return domain.Filters{}
	}

	return filters[0]
}

func cursorFilters(cursor *Cursor) domain.Filters {
	if cursor == nil || cursor.Filters == nil {
		return domain.Filters{}
	}

	return *cursor.Filters
}

// All user values are parameters; only fixed SQL fragments are assembled here.
func filterSQL(kind string, filters domain.Filters, args []any) (string, []any) {
	var sql strings.Builder
	add := func(condition string, value any) {
		args = append(args, value)
		sql.WriteString(fmt.Sprintf(condition, len(args)))
	}
	column := "t.ledger_sequence"

	switch kind {
	case "transactions", "account_transactions", "contract_transactions", "ledger_transactions":
		if filters.Address != "" {
			if strings.HasPrefix(filters.Address, "G") {
				add(`
					AND EXISTS (
					    SELECT
					        1
					    FROM
					        transaction_accounts participant
					    WHERE
					        participant.network = t.network
					        AND participant.transaction_hash = t.hash
					        AND participant.address = $%d
					)
				`, filters.Address)
			} else {
				add(`
					AND EXISTS (
					    SELECT
					        1
					    FROM
					        transaction_contracts participant
					    WHERE
					        participant.network = t.network
					        AND participant.transaction_hash = t.hash
					        AND participant.contract_id = $%d
					)
				`, filters.Address)
			}
		}

		if filters.Function != "" {
			args = append(args, filters.Function)
			functionParameter := len(args)
			contractCondition := ""
			if filters.Contract != "" {
				args = append(args, filters.Contract)
				contractCondition = fmt.Sprintf("AND call.contract_id = $%d", len(args))
			}

			sql.WriteString(fmt.Sprintf(`
				AND EXISTS (
				    SELECT
				        1
				    FROM
				        contract_calls call
				    WHERE
				        call.network = t.network
				        AND call.transaction_hash = t.hash
				        AND call.function_name = $%d
				        %s
				)
			`, functionParameter, contractCondition))
		} else if filters.Contract != "" {
			add(`
				AND EXISTS (
				    SELECT
				        1
				    FROM
				        transaction_contracts participant
				    WHERE
				        participant.network = t.network
				        AND participant.transaction_hash = t.hash
				        AND participant.contract_id = $%d
				)
			`, filters.Contract)
		}

		if filters.Status != "" {
			add(" AND t.successful = $%d\n", filters.Status == "success")
		}

	case "events":
		column = "e.ledger_sequence"
		if filters.EventScope != "" {
			add(" AND e.event_scope = $%d\n", filters.EventScope)
		}
		if filters.Contract != "" {
			add(" AND e.contract_id = $%d\n", filters.Contract)
		}

		if filters.EventType != "" {
			name := map[string]string{
				"contract":   "ContractEventTypeContract",
				"system":     "ContractEventTypeSystem",
				"diagnostic": "ContractEventTypeDiagnostic",
			}[filters.EventType]
			add(" AND e.event_type = $%d\n", name)
		}

		if filters.EventName != "" {
			add(`
				AND e.topics_json->0->>'type' = 'ScValTypeScvSymbol'
				AND e.topics_json->0->>'value' = $%d
			`, filters.EventName)
		}

	case "transfers", "asset_transfers", "account_transfers", "contract_transfers":
		column = "m.ledger_sequence"
		if filters.Asset != "" {
			add(" AND m.contract_id = $%d\n", filters.Asset)
		}

		if filters.From != "" {
			add(" AND m.from_address = $%d\n", filters.From)
		}

		if filters.To != "" {
			add(" AND m.to_address = $%d\n", filters.To)
		}
	}

	if filters.MinLedger != 0 {
		add(" AND "+column+" >= $%d\n", filters.MinLedger)
	}

	if filters.MaxLedger != 0 {
		add(" AND "+column+" <= $%d\n", filters.MaxLedger)
	}

	return sql.String(), args
}
