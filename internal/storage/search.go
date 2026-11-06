package storage

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/jackc/pgx/v5"
)

const searchMatchesPerKind = 5

func (s *Store) Suggest(ctx context.Context, query string) (domain.SearchSuggestions, error) {
	query = strings.TrimSpace(query)
	result := domain.SearchSuggestions{
		Network: domain.Network,
		Query:   query,
		Limit:   searchMatchesPerKind * 2,
		Items:   []domain.SearchMatch{},
	}
	kinds := domain.SearchKinds(query)
	if len(kinds) == 0 {
		return result, fmt.Errorf("invalid search prefix")
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return result, err
	}

	defer tx.Rollback(ctx)

	for _, kind := range kinds {
		var matches []domain.SearchMatch
		if kind == "ledger" {
			matches, err = suggestLedgers(ctx, tx, query)
		} else {
			matches, err = suggestKeys(ctx, tx, kind, query)
		}

		if err != nil {
			return result, err
		}

		result.Items = append(result.Items, matches...)
	}

	// A complete identifier stays ahead of its partial matches.
	slices.SortStableFunc(result.Items, func(a, b domain.SearchMatch) int {
		aExact := strings.EqualFold(a.Key, query)
		bExact := strings.EqualFold(b.Key, query)
		if aExact == bExact {
			return 0
		}

		if aExact {
			return -1
		}

		return 1
	})

	return result, tx.Commit(ctx)
}

func suggestLedgers(ctx context.Context, tx pgx.Tx, prefix string) ([]domain.SearchMatch, error) {
	value, err := strconv.ParseUint(prefix, 10, 64)
	if err != nil {
		return nil, err
	}

	queries := []string{}
	args := []any{domain.Network}
	lower, upper := value, value
	for digits := len(prefix); digits <= 10 && lower <= math.MaxUint32; digits++ {
		args = append(args, lower, min(upper, uint64(math.MaxUint32)))
		queries = append(queries, fmt.Sprintf(`
			(
			    SELECT
			        sequence
			    FROM
			        ledgers
			    WHERE
			        network = $1
			        AND sequence BETWEEN $%d AND $%d
			    ORDER BY
			        sequence DESC
			    LIMIT
			        5
			)
		`, len(args)-1, len(args)))
		lower *= 10
		upper = upper*10 + 9
	}

	if len(queries) == 0 {
		return []domain.SearchMatch{}, nil
	}

	args = append(args, value)
	query := strings.Join(queries, "\nUNION ALL\n")
	query = fmt.Sprintf(`
		SELECT
		    sequence
		FROM (
		    %s
		) matches
		ORDER BY
		    (sequence = $%d) DESC,
		    sequence DESC
		LIMIT
		    5
	`, query, len(args))
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	matches := []domain.SearchMatch{}
	for rows.Next() {
		var sequence uint32
		if err = rows.Scan(&sequence); err != nil {
			return nil, err
		}

		key := strconv.FormatUint(uint64(sequence), 10)
		matches = append(matches, domain.SearchMatch{
			Type: "ledger",
			Key:  key,
			Path: "/ledgers/" + key,
		})
	}

	return matches, rows.Err()
}

func suggestKeys(
	ctx context.Context,
	tx pgx.Tx,
	kind, prefix string,
) ([]domain.SearchMatch, error) {
	table, column, path := "", "", ""
	if kind == "transaction" {
		table, column, path = "transactions", "hash", "/tx/"
		prefix = strings.ToLower(prefix)
	} else if kind == "account" {
		table, column, path = "transaction_accounts", "address", "/accounts/"
		prefix = strings.ToUpper(prefix)
	} else {
		table, column, path = "contracts", "contract_id", "/contracts/"
		prefix = strings.ToUpper(prefix)
	}

	matches := []domain.SearchMatch{}
	previous := ""
	for len(matches) < searchMatchesPerKind {
		// Byte-wise bounds use the matching C-collation index. Skipping the previous
		// key avoids scanning every transaction for a frequently observed account.
		query := fmt.Sprintf(`
			SELECT
			    %s
			FROM
			    %s
			WHERE
			    network = $1
			    AND %s COLLATE "C" >= $2
			    AND %s COLLATE "C" < $3
			    AND %s COLLATE "C" > $4
			ORDER BY
			    %s COLLATE "C"
			LIMIT
			    1
		`, column, table, column, column, column, column)
		var key string
		err := tx.QueryRow(ctx, query, domain.Network, prefix, prefix+"~", previous).Scan(&key)
		if errors.Is(err, pgx.ErrNoRows) {
			break
		}

		if err != nil {
			return nil, err
		}

		matches = append(matches, domain.SearchMatch{Type: kind, Key: key, Path: path + key})
		previous = key
	}

	return matches, nil
}
