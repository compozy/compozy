# Temporarily enable sign-in grace

When a verified sign-in outage blocks new installations or updates, open the Access-protected admin console and set the gate policy to `grace`. Set an expiry and a short user-facing incident message. Confirm the gate policy JWT verifies through the current JWKS, with the intended deadline and message, and confirm the change is logged.

Open the app signed out and verify the grace banner and ordinary local work. Remote and Pro continue to require sign-in. Existing installations that cannot reach the cloud at all can use their separate seven-day offline grace.

After sign-in recovers, set the policy to `enforce` and verify the next policy fetch and enrollment. Keep 0.8.0 in beta with the sign-in gate enforced for at least seven days before stable.
