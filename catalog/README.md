# Curated Marketplace Catalog

The three JSON documents in this directory are AGH's default discovery feeds. Each feed must remain
non-empty and pass the same schema parser used by the daemon.

Extension entries point to one exact HTTPS `.tar.gz` artifact and pin its SHA-256 digest. Keep the
reviewable source under `packages/<name>/` and the deterministic archive under `artifacts/`.

Package and validate a first-party extension from the repository root:

```bash
go run ./cmd/agh-catalog package \
  ./catalog/packages/repository-orientation \
  ./catalog/artifacts/repository-orientation-v1.0.0.tar.gz

./scripts/catalog-digest.sh \
  ./catalog/artifacts/repository-orientation-v1.0.0.tar.gz

go run ./cmd/agh-catalog validate ./catalog
```

Update `extensions.json` with the generated digest and a versioned `artifact_url`. Validation runs
the local artifact through the production installer, including archive limits, manifest discovery,
content scanning, digest verification, and version matching.
