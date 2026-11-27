# Respond to an OAuth incident

Use the sign-in error rate by provider and environment. Try a synthetic sign-in to the affected provider in staging. Compare the deployed client ID, the allowed OAuth callback URL, the client secret version, and the provider application's publication or suspension state. Inspect provider incident notices and the sanitized Sentry events.

For a bad secret or callback, correct it in the provider and the matching deploy environment, redeploy and verify both a new device sign-in and an already linked computer. If the outage blocks installation sign-in, enable temporary grace and publish the affected provider on the status page. Restore enforcement when activation and device enrollment work again. Keep ownership of both OAuth apps with the operator.

[Gate grace](gate-grace.md)
