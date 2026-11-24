package main

import (
	"errors"

	"github.com/compozy/compozy/internal/cloud/billing"
)

func loadStripe(getenv func(string) string) (*billing.Stripe, error) {
	config := billing.StripeConfig{
		SecretKey:     getenv("COMPOZY_CLOUD_STRIPE_SECRET_KEY"),
		WebhookSecret: getenv("COMPOZY_CLOUD_STRIPE_WEBHOOK_SECRET"),
	}
	if config.SecretKey == "" && config.WebhookSecret == "" {
		return nil, nil
	}
	if getenv("STRIPE_PRICE_PRO_ANNUAL") == "" || getenv("STRIPE_PRICE_PRO_MONTHLY") == "" {
		return nil, errors.New("Stripe requires STRIPE_PRICE_PRO_ANNUAL and STRIPE_PRICE_PRO_MONTHLY")
	}
	return billing.NewStripe(config)
}
