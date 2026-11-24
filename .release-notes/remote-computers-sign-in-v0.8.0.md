## Migration notes

Official CompozyOS 0.8.0 releases require sign-in on every installation. After updating, open the app or run `compozy account login` and continue with Google or GitHub. Sessions, workspaces, settings, and job definitions stay on the installation.

Sign in on unattended machines before relying on scheduled work. Running turns and claimed leases finish during sign-out. Recurring jobs skip occurrences until sign-in, and each held one-shot job runs once when work resumes. Webhooks ask the sender to retry.

For CI that runs local work, create an account token with `compozy account token create --name ci` and supply it through `COMPOZY_ACCOUNT_TOKEN` at daemon launch. For SSH provisioning, the controller can link the host with a setup token before its first sign-in. [Sign-in guide](https://compozy.com/docs/account/sign-in)
