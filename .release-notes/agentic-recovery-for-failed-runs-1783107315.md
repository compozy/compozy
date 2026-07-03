---
title: Agentic recovery for failed runs
type: feature
---

Run-producing commands can now **automatically remediate and restart a failed run** with a dedicated recovery agent. When enabled, a failed run is handed to the recovery agent, which diagnoses and attempts a fix, then the run is restarted — up to a bounded number of attempts — instead of stopping at the first failure.

### Enabling recovery

Recovery is **off by default**. Turn it on per invocation:

```bash
# Enable agentic recovery for this run
compozy tasks run my-feature --recovery

# Pick the recovery runtime and bound the attempts
compozy tasks run my-feature --recovery \
  --recovery-ide codex --recovery-model gpt-5.5 \
  --recovery-reasoning high --recovery-max-attempts 2
```

Use `--no-recovery` to force it off for a single invocation even when the workspace enables it.

### Flags

| Flag                      | Default   | Meaning                                             |
| ------------------------- | --------- | --------------------------------------------------- |
| `--recovery`              | `false`   | Enable agentic recovery for failed runs             |
| `--no-recovery`           | `false`   | Disable recovery for this invocation                |
| `--recovery-ide`          | `codex`   | ACP runtime used by the recovery agent              |
| `--recovery-model`        | `gpt-5.5` | Model used by the recovery agent                    |
| `--recovery-reasoning`    | `medium`  | Reasoning effort (`low`, `medium`, `high`, `xhigh`) |
| `--recovery-max-attempts` | `1`       | Maximum remediation-and-restart cycles (1–3)        |

### Configuration

Set workspace defaults under `[recovery]`:

```toml
[recovery]
enabled = true
ide = "codex"
model = "gpt-5.5"
reasoning_effort = "medium"
max_attempts = 1
```

Recovery config is resolved fresh for each invocation and is **not** persisted into run or exec metadata; the flags above always override `[recovery]` for a single command.
