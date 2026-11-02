---
title: Recover superseded bundled extensions and interrupted installs
type: fix
---

Bundled extension upgrades now retire superseded registrations without discarding user state. The `dev-cycle` to `spec-cycle` boundary migration transfers scoped installations and profile enablement atomically, preserves existing successor installations, and retains the old package files. If inputs, credentials, development links, profile markers, or conflicting enablement prevent safe translation, the old registration remains and `compozy doctor` names its successor and the exact removal command. Review and migrate that retained state before removing it. This N-1 translation is scheduled for removal in v0.6.0 after the v0.5 migration window.

An unrelated extension startup failure no longer aborts another extension's install, update, enablement, removal, or rollback. Its failure stays visible through status and doctor. Operators can recover a package left without any scoped installation using `compozy extension remove <name> --global`; removal clears the package registration and managed files through the normal lifecycle.

Update authored references from `ext__dev_cycle__*` to the corresponding `ext__spec_cycle__*` tools. The former `cy-create-prd` and `cy-create-techspec` workflows were consolidated into `cy-create-spec`; review the unified workflow inputs when updating scripts and Loop definitions. These retired extension resources do not receive aliases; this change adds no new native `compozy__*` rename.
