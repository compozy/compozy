#!/usr/bin/env bash
set -euo pipefail

: "${COMPOZY_CLOUD_NEON_DIRECT_URL:?missing Neon backup connection}"
: "${COMPOZY_CLOUD_R2_ENDPOINT:?missing R2 endpoint}"
: "${COMPOZY_CLOUD_R2_BACKUP_BUCKET:?missing R2 bucket}"
export AWS_ACCESS_KEY_ID="${COMPOZY_CLOUD_R2_ACCESS_KEY_ID:?missing R2 access key}"
export AWS_SECRET_ACCESS_KEY="${COMPOZY_CLOUD_R2_SECRET_ACCESS_KEY:?missing R2 secret key}"
export AWS_DEFAULT_REGION=auto

umask 077
dump=$(mktemp "$RUNNER_TEMP/compozy-cloud.XXXXXX")
trap 'rm -f "$dump"' EXIT
snapshot=$(date -u +%Y%m%dT%H%M%SZ)
docker run --rm --env COMPOZY_CLOUD_NEON_DIRECT_URL --env PGSSLROOTCERT=system \
  --entrypoint sh postgres:17-alpine -c \
  'exec pg_dump --format=custom --no-owner --no-acl --dbname="$COMPOZY_CLOUD_NEON_DIRECT_URL"' > "$dump"

aws --endpoint-url "$COMPOZY_CLOUD_R2_ENDPOINT" s3 cp --only-show-errors "$dump" \
  "s3://$COMPOZY_CLOUD_R2_BACKUP_BUCKET/postgres/$snapshot-$GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT.dump"
