# Respond to a Stripe incident

Check Stripe status, the delivery log for `/v1/billing/stripe-webhook`, and the backend webhook error alert. Compare environment, API key mode, endpoint signing secret and price IDs. Confirm Stripe is sending subscription create, update and delete events, invoice payment success and failure, and checkout completion and expiry.

If the cloud failed to process a delivery, fix the provider or deploy issue and redeliver the events from Stripe. The webhook deduplicates Stripe event IDs and refreshes the current subscription before applying entitlements. Verify a previously affected account's Pro state, checkout recovery and computer lease. Confirm email dunning and annual reminders in Stripe settings.
