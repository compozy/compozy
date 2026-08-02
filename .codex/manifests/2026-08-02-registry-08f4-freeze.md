# Registry 08-F4 Source Freeze

Generated on 2026-08-02 from `/home/pedronauck/Projects/compozy` after the P2 remediation.

## Scope

- `core`: every non-recursive Go source or test file currently present directly under `internal/fileutil/` and `internal/registry/`.
- `surfaces`: only callers, contracts, and Web tests that actually carry the 08-F4 `CleanupDiagnostics` propagation. Unrelated concurrent bridge, provider, task, and extension-manager cleanup edits are excluded.
- `artifacts`: only generated contracts, public documentation, the official Compozy skill reference, and the QA scenario co-shipped for the cleanup-diagnostic contract.

The ordered path and content hash for every member is stored in the corresponding `.sha256` file.

## Manifest Digests

```text
dfdbf18fe41f177b51973a0d1218a9dfe4f8cafc81b37f0e456bacee38e68d06  .codex/manifests/2026-08-02-registry-08f4-core.sha256
8a03bae02b2636e2d91ea59d9670b65e33e8bf07cf6b1e402d44eb4d85a3101a  .codex/manifests/2026-08-02-registry-08f4-surfaces.sha256
3515fed2853bd6691aeee09a10d117287b8302f77c1f0daf113532b980591e20  .codex/manifests/2026-08-02-registry-08f4-artifacts.sha256
```

## Reproduction

Regenerate the ordered core manifest:

```bash
rtk awk '{print $2}' .codex/manifests/2026-08-02-registry-08f4-core.sha256 \
  | rtk xargs sha256sum
```

Regenerate either explicit surface or artifact manifest without discovering additional concurrent files:

```bash
rtk awk '{print $2}' .codex/manifests/2026-08-02-registry-08f4-surfaces.sha256 \
  | rtk xargs sha256sum
rtk awk '{print $2}' .codex/manifests/2026-08-02-registry-08f4-artifacts.sha256 \
  | rtk xargs sha256sum
```

Validate the frozen content and then recompute the three manifest digests:

```bash
rtk sha256sum --check .codex/manifests/2026-08-02-registry-08f4-core.sha256
rtk sha256sum --check .codex/manifests/2026-08-02-registry-08f4-surfaces.sha256
rtk sha256sum --check .codex/manifests/2026-08-02-registry-08f4-artifacts.sha256
rtk sha256sum \
  .codex/manifests/2026-08-02-registry-08f4-core.sha256 \
  .codex/manifests/2026-08-02-registry-08f4-surfaces.sha256 \
  .codex/manifests/2026-08-02-registry-08f4-artifacts.sha256
```

These manifests freeze source content only. Native Windows runtime behavior and the ET-009 real-user QA walk remain separate workstream-close evidence.
