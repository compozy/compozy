package store

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
)

const maxSecurityDetails = 16 << 10

var ErrSecurityEventDetails = errors.New("invalid security event details")

type SecurityEvent struct {
	AccountID string
	Type      string
	Actor     string
	At        time.Time
	Details   map[string]any
}

// AppendSecurityEvent can participate in the same transaction as its domain change.
func (q *Queries) AppendSecurityEvent(ctx context.Context, event SecurityEvent) error {
	if event.Details == nil {
		event.Details = map[string]any{}
	}
	details, err := json.Marshal(event.Details)
	if err != nil || len(details) > maxSecurityDetails {
		return ErrSecurityEventDetails
	}
	var object map[string]any
	if err := json.Unmarshal(details, &object); err != nil || !safeSecurityDetails(object) {
		return ErrSecurityEventDetails
	}
	params := sqlcgen.AppendSecurityEventParams{
		AccountID:   pgtype.Text{String: event.AccountID, Valid: event.AccountID != ""},
		EventType:   event.Type,
		Actor:       event.Actor,
		At:          timestamp(event.At),
		DetailsJson: string(details),
	}
	if err := q.generated.AppendSecurityEvent(ctx, params); err != nil {
		return errors.New("append cloud security event failed")
	}
	return nil
}

func safeSecurityDetails(value any) bool {
	switch detail := value.(type) {
	case map[string]any:
		for key, child := range detail {
			if sensitiveSecurityKey(key) || !safeSecurityDetails(child) {
				return false
			}
		}
	case []any:
		for _, child := range detail {
			if !safeSecurityDetails(child) {
				return false
			}
		}
	case string:
		_, err := netip.ParseAddr(strings.TrimSpace(detail))
		return err != nil
	}
	return true
}

func sensitiveSecurityKey(key string) bool {
	key = strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(key))
	switch key {
	case "token", "secret", "refreshtoken", "accesstoken", "enrollmenttoken", "accounttoken",
		"password", "ip", "ipaddress", "clientip", "remoteip", "authorization", "cookie", "credential",
		"privatekey", "codeverifier", "devicecode", "signingkey", "clientsecret":
		return true
	default:
		return false
	}
}

func (s *Store) CompactSecurityEvents(ctx context.Context, now time.Time) error {
	if err := s.generated.CompactSecurityEvents(ctx, timestamp(now.AddDate(-1, 0, 0))); err != nil {
		return fmt.Errorf("compact cloud security events: %w", err)
	}
	return nil
}

func timestamp(at time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: at, Valid: !at.IsZero()}
}
