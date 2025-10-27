# Security Policy

## Accepted Risks

### AWS SDK for Go (github.com/aws/aws-sdk-go v1.55.6)

- Advisories: GO-2022-0635, GO-2022-0646 (AWS S3 Crypto client-side encryption flaws)
- Impact Scope: affects packages `s3crypto`/`s3/encryption`; Compozy does not invoke these modules.
- Mitigation: storage interactions rely on server-side encryption; no client-side encryption helpers are linked into binaries.
- Verification: `rg "s3crypto" -n` and `rg "s3/encryption" -n` return no matches in the repository (checked 2025-10-27).
- Action: continue monitoring upstream advisories; upgrade or disable the dependency if future releases require the S3 Crypto helpers.
