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
	Source  string
	PriceID string
	Status  string
	EndsAt  time.Time
}

func Derive(subscriptions []Subscription, monthlyPrice, annualPrice string, now time.Time) Plan {
	for _, subscription := range subscriptions {
		switch subscription.Source {
		case "complimentary":
			if now.Before(subscription.EndsAt) {
				return Pro
			}
		case "stripe":
			if subscription.PriceID == "" ||
				(subscription.PriceID != monthlyPrice && subscription.PriceID != annualPrice) {
				continue
			}
			switch subscription.Status {
			case "active", "trialing", "past_due":
				return Pro
			}
		}
	}
	return Free
}
