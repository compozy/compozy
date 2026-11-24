#!/usr/bin/env bash
set -euo pipefail

for directory in cmd/compozy-cloud internal/cloud infra/cloud-edge; do
  if [[ ! -s "$directory/LICENSE" ]]; then
    echo "Missing cloud license in $directory" >&2
    exit 1
  fi
done

declare -A historical_migrations=(
  [internal/cloud/store/migrations/00001_cloud.sql]=1d338c3346782a1ea6cabda8962148a4a7640156
  [internal/cloud/store/migrations/00002_security_events.sql]=845d96d9aae146ee71329ecd2b6c0f0e3ccd2964
  [internal/cloud/store/migrations/00003_gate_policy.sql]=682691cf331953b3367d6f1580ef194ecb841228
  [internal/cloud/store/migrations/00004_cloud_auth.sql]=0b2d336847343c077cb05294c8784cfd07d683af
  [internal/cloud/store/migrations/00005_control_link.sql]=e3fd45ea13e1ce993cbcf164c4a959b65e10a641
  [internal/cloud/store/migrations/00006_hosted_tunnels.sql]=d0cd4a5ed6e282a9fa52507c3dbf18e6f965b2db
  [internal/cloud/store/migrations/00007_billing.sql]=e94e78d9be8b0a296418c39405b0f8ce9bd0324b
  [internal/cloud/store/migrations/00008_account_lifecycle.sql]=f749add15c79bd856c28556602151a63003f0ea8
)

failed=0
while IFS= read -r -d '' path; do
  expected_hash=${historical_migrations[$path]:-}
  if [[ -n "$expected_hash" ]]; then
    if [[ "$(git hash-object "$path")" != "$expected_hash" ]]; then
      echo "Published cloud migration changed: $path" >&2
      failed=1
    fi
    continue
  fi

  case "$path" in
    *.go | *.js | *.jsx | *.mjs | *.cjs | *.ts | *.tsx | *.jsonc) header='// SPDX-License-Identifier: FSL-1.1-MIT' ;;
    *.sql) header='-- SPDX-License-Identifier: FSL-1.1-MIT' ;;
    *.css) header='/* SPDX-License-Identifier: FSL-1.1-MIT */' ;;
    *.html) header='<!-- SPDX-License-Identifier: FSL-1.1-MIT -->' ;;
    *.sh | *.py | *.yaml | *.yml | *.toml | */Dockerfile | */sshd_config) header='# SPDX-License-Identifier: FSL-1.1-MIT' ;;
    *) continue ;;
  esac

  {
    IFS= read -r first || true
    if [[ "$first" == '#!'* ]]; then IFS= read -r first || true; fi
  } < "$path"
  if [[ "$first" != "$header" ]]; then
    echo "Missing cloud SPDX header: $path" >&2
    failed=1
  fi
done < <(git ls-files -z -- cmd/compozy-cloud internal/cloud infra/cloud-edge)

exit "$failed"
