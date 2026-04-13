# Trust and Enablement

Compozy does not use marketplace-style trust tiers in v1. Instead it uses explicit local enablement plus declarative capabilities.

## Discovery levels

Extensions can exist at three levels:

| Level     | Location                                  | Default enablement |
| --------- | ----------------------------------------- | ------------------ |
| bundled   | built into the Compozy binary             | enabled            |
| user      | `~/.compozy/extensions/<name>/`           | disabled           |
| workspace | `<workspace>/.compozy/extensions/<name>/` | disabled           |

## Precedence

When names collide, the effective declaration is:

```text
workspace > user > bundled
```

This applies to executable extensions, provider overlays, and skill packs.

## Why workspace and user extensions start disabled

Cloning a repository should not automatically activate code from `.compozy/extensions/` on another developer's machine. Local enablement keeps discovery reproducible while preserving operator consent.

## Install and enable flow

For user-scoped installs:

```bash
compozy ext install --yes /path/to/extension
compozy ext enable my-ext
compozy ext inspect my-ext
compozy ext doctor
```

What happens:

- install copies the extension into the user extension directory
- install records the new extension as disabled
- enable flips local operator state on that machine only
- inspect shows the winning manifest and any overridden declarations
- doctor reports capability, provider, and skill-pack issues

## Capability review

Compozy prints the requested capabilities during install. That is the review point for author intent.

Pay special attention to:

- `artifacts.write`
- `tasks.create`
- `runs.start`
- `subprocess.spawn`
- `network.egress`

## Audit trail

Every run records extension activity in:

```text
.compozy/runs/<run-id>/extensions.jsonl
```

That file is the first place to inspect when you need to answer:

- which hooks ran
- which Host API methods were called
- which capabilities were exercised

## Recommended operator practices

- keep capabilities minimal
- review workspace extension manifests in code review
- use `compozy ext doctor` after introducing new provider or skill-pack declarations
- disable extensions that are no longer needed instead of leaving them active indefinitely
