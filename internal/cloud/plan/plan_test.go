package plan

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

func TestPlan(t *testing.T) {
	t.Parallel()

	t.Run("Should combine enabled computers and active reservations on Free (UT-004)", func(t *testing.T) {
		t.Parallel()
		for _, tt := range []struct {
			name  string
			tier  Plan
			usage Usage
			want  error
		}{
			{name: "Should refuse an enabled computer", tier: Free, usage: Usage{EnabledComputers: 1}, want: ErrLimitReached},
			{name: "Should refuse an active reservation", tier: Free, usage: Usage{ActiveReservations: 1}, want: ErrLimitReached},
			{name: "Should admit Free with no slot in use", tier: Free},
			{name: "Should admit Pro over the Free limit", tier: Pro, usage: Usage{EnabledComputers: 1, ActiveReservations: 1}},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				if err := CanReserve(tt.tier, tt.usage); !errors.Is(err, tt.want) {
					t.Fatalf("CanReserve = %v, want %v", err, tt.want)
				}
			})
		}
	})

	t.Run("Should expose only the granted route kinds for each plan (UT-010)", func(t *testing.T) {
		t.Parallel()
		direct := []remoteproto.RouteKind{remoteproto.RouteDirect, remoteproto.RouteSSH}
		if got := GrantRoutes(Free); !slices.Equal(got, direct) {
			t.Fatalf("Free routes = %v", got)
		}
		if got := GrantRoutes(Pro); !slices.Equal(got, append(direct, remoteproto.RouteCloud)) {
			t.Fatalf("Pro routes = %v", got)
		}
	})

	t.Run("Should derive Pro from any currently entitled subscription", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
		rows := []Subscription{
			{Source: "stripe", PriceID: "yearly", Status: "canceled"},
			{Source: "stripe", PriceID: "other-price", Status: "active"},
			{Source: "complimentary", EndsAt: now},
		}
		if got := Derive(rows, "monthly", "yearly", now); got != Free {
			t.Fatalf("plan = %v, want free", got)
		}
		rows = append(rows, Subscription{Source: "stripe", PriceID: "monthly", Status: "past_due"})
		if got := Derive(rows, "monthly", "yearly", now); got != Pro {
			t.Fatalf("plan = %v, want pro", got)
		}
	})
}
