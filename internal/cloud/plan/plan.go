package plan

import (
	"errors"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

type Plan string

const (
	Free Plan = "free"
	Pro  Plan = "pro"
)

var ErrLimitReached = errors.New("plan_limit_reached")

type Usage struct {
	EnabledComputers   int64
	ActiveReservations int64
}

// CanReserve is evaluated with usage read under the account row lock.
func CanReserve(tier Plan, usage Usage) error {
	if tier != Pro && (usage.EnabledComputers > 0 || usage.ActiveReservations > 0) {
		return ErrLimitReached
	}
	return nil
}

func GrantRoutes(tier Plan) []remoteproto.RouteKind {
	routes := []remoteproto.RouteKind{remoteproto.RouteDirect, remoteproto.RouteSSH}
	if tier == Pro {
		routes = append(routes, remoteproto.RouteCloud)
	}
	return routes
}

type Subscription struct {
	ID, AccountID, Source, PriceID, Status, Interval, CustomerID string
	LastInvoiceID                                                string
	CurrentPeriodEnd, LastPaymentAt, EndsAt                      time.Time
	CancelAtPeriodEnd, RefundablePayment                         bool
}

func Derive(subscriptions []Subscription, monthlyPrice, annualPrice string, now time.Time) Plan {
	for _, subscription := range subscriptions {
		if subscription.Entitled(monthlyPrice, annualPrice, now) {
			return Pro
		}
	}
	return Free
}

func (s Subscription) Entitled(monthlyPrice, annualPrice string, now time.Time) bool {
	switch s.Source {
	case "complimentary":
		return now.Before(s.EndsAt)
	case "stripe":
		return s.PriceID != "" && (s.PriceID == monthlyPrice || s.PriceID == annualPrice) &&
			(s.Status == "active" || s.Status == "trialing" || s.Status == "past_due")
	default:
		return false
	}
}
