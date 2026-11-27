# Roll back a Cloud release

Find the last compatible production `cloud-v<semver>` tag that passed staging. Dispatch **Compozy Cloud deploy** from `main` with that tag as `rollback_tag`. The production environment reviewer approves the rollback. The workflow redeploys the tagged binary with current environment bindings and repeats the public smoke.

Keep current signing secrets and any emergency retirements. Keep the current Goose migration history. A version that cannot use the expanded schema requires a forward compatibility fix or a different prior tag. Record the failed and restored revisions in the incident, then check activation and existing computer reconnects.
