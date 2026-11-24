## Migration notes

Official CompozyOS 0.8.0 releases require sign-in on every installation. After updating, open the app or run `compozy account login` and continue with Google or GitHub. Sessions, workspaces, settings, and job definitions stay on the installation.

Sign in on unattended machines before relying on scheduled work. Running turns and claimed leases finish during sign-out. Recurring jobs skip occurrences until sign-in, and each held one-shot job runs once when work resumes. Webhooks ask the sender to retry.

For CI that runs local work, create an account token with `compozy account token create --name ci` and supply it through `COMPOZY_ACCOUNT_TOKEN` at daemon launch. For SSH provisioning, the controller can link the host with a setup token before its first sign-in. [Sign-in guide](https://compozy.com/docs/account/sign-in)

Remote Computers replaces Remote Gateway in 0.8.0. The upgrade removes paired devices, CLI connection profiles and their credentials, the bundled Tailscale extension, connectivity providers and their scaffolds, and the `[gateway]` configuration. Add your computers again from **Settings → Computers** or with `compozy computer`. The removed `gateway`, `pair`, `device`, and `connect` commands and `--confirm-gateway-requirement` print a migration pointer and exit 2 in 0.8.0. The pointers expire in 0.9.0.

App tokens, their allowed origins, and webhook bindings migrate automatically. Update scripts to use `/api/ingress/bindings` and remove references to other Gateway routes and `compozy__gateway`. For public webhook delivery, use a hosted route or set `remote.ingress.public_url` for your HTTPS tunnel. Confirm any trigger marked **Needs confirmation** and update the sender's URL. [Remote Computers setup](https://compozy.com/docs/computers)
