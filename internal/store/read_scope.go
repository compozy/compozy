package store

import (
	"errors"
	"fmt"
	"strings"
)

var ErrReadScopeInvalid = errors.New("store: read scope is invalid")

// ReadScope selects exactly one profile or the explicit labeled aggregate.
type ReadScope struct {
	ProfileID   string
	AllProfiles bool
}

// Validate rejects every implicit or contradictory read boundary.
func (s ReadScope) Validate() error {
	profileID := strings.TrimSpace(s.ProfileID)
	if s.AllProfiles && profileID != "" {
		return fmt.Errorf("%w: aggregate read forbids profile id", ErrReadScopeInvalid)
	}
	if !s.AllProfiles && profileID == "" {
		return fmt.Errorf("%w: scoped read requires profile id", ErrReadScopeInvalid)
	}
	return nil
}

// Matches reports whether an owner belongs to this validated read boundary.
func (s ReadScope) Matches(profileID string) bool {
	return s.AllProfiles || strings.TrimSpace(profileID) == strings.TrimSpace(s.ProfileID)
}

// ReadScopeClause builds the SQL predicate for a validated scope. Invalid
// input produces an always-false predicate as a second fail-closed guard.
func ReadScopeClause(column string, scope ReadScope) Clause {
	if err := scope.Validate(); err != nil {
		return alwaysFalseClause()
	}
	if scope.AllProfiles {
		return Clause{}
	}
	return StringClause(column, scope.ProfileID)
}

// ReadScopeCoalescedClause builds a scoped owner predicate across nullable,
// ordered owner columns.
func ReadScopeCoalescedClause(columns []string, scope ReadScope) Clause {
	if err := scope.Validate(); err != nil {
		return alwaysFalseClause()
	}
	if scope.AllProfiles {
		return Clause{}
	}
	if len(columns) == 0 {
		return alwaysFalseClause()
	}
	for _, column := range columns {
		if err := validateSQLiteColumnReference(column); err != nil {
			return alwaysFalseClause()
		}
	}
	return Clause{
		sql:    "COALESCE(" + strings.Join(columns, ", ") + ") = ?",
		arg:    strings.TrimSpace(scope.ProfileID),
		ok:     true,
		hasArg: true,
	}
}
