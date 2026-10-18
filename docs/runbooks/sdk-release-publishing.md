# SDK release publishing

The release plan owns one version for the daemon, `@compozy/cli`,
`@compozy/extension-sdk`, and `@compozy/client`. The Go SDK uses the existing
`sdk/go/v<version>` module tag. The `npm-publish` job in
`.github/workflows/release.yml` publishes packages only after the GitHub release
is public and the CLI package installs successfully against its public archives.

## Registry setup

An npm package maintainer must configure trusted publishing for `@compozy/client`
before its first automated release. This is an external setup action; a repository
change or successful package dry-run does not prove that npm has granted access.

The package must already exist on npm. If it does not, an organization maintainer
must complete the approved initial package publication before adding its trusted
publisher. Trust management requires package write access and account 2FA.
See [npm trust prerequisites](https://docs.npmjs.com/cli/v11/commands/npm-trust/).

In the package's npm settings, add a GitHub Actions trusted publisher with:

| Field | Value |
| --- | --- |
| Organization or user | `compozy` |
| Repository | `compozy` |
| Workflow filename | `release.yml` |
| Environment | Unset; the publishing job does not select an environment |
| Allowed action | Direct `npm publish` |

New configurations can default to staged publishing, so explicitly allow direct
publication. Complete the first successful publish within two days of creating
the trust configuration. npm requires a hosted runner, Node >=22.14.0, npm
>=11.5.1, and `id-token: write`. Public OIDC publications produce provenance.
The package's `repository.url` must match the publishing repository. See
[npm trusted publishing](https://docs.npmjs.com/trusted-publishers/).

The workflow already supplies the runner, pinned npm, OIDC permission, and
provenance option. It does not provision npm account permissions. Keep the
existing CLI and extension SDK trusted publishers configured for the same
workflow. Do not add an npm write token to bypass a missing trust configuration.

## Package preparation

Both TypeScript SDKs use `scripts/publish-sdk-package.sh`:

```bash
scripts/publish-sdk-package.sh --dry-run sdk/typescript <version> <npm-tag>
scripts/publish-sdk-package.sh --dry-run sdk/client <version> <npm-tag>
```

The release dry-run runs these commands in CI. Each invocation builds its package
through Turborepo and stages `package.json`, `LICENSE`, `README.md`, and `dist` in
a temporary directory. Only the staged manifest receives the planned version.
For SDKs with `src/version.ts`, the staged ESM, CommonJS, and declaration version
artifacts receive the same version. Missing artifacts or a missing development
version marker fail before publication. Repository sources remain unchanged.

Production uses the same script without `--dry-run`, with public access, the
planned dist-tag, and provenance. A registry lookup skips an exact version that
is already published. An explicit 404 permits publication; an uncertain registry
response fails without rebuilding or publishing.

## Recovery and completion evidence

`release-publication-state.sh` reports GitHub, CLI, extension SDK, and app SDK
publication states separately. A retry publishes only missing packages. An SDK
published without its matching CLI version is an inconsistent state and stops
the release. Automatic beta allocation scans immutable versions from all three
npm packages; an unpublished package does not prevent allocating its first beta.

After publication, `release-npm-readiness.sh` waits for all three package dist-tags
to name the exact release version. A beta must not move `latest`. Delayed
propagation is retried within the existing deadline; query failures and policy
violations fail immediately. Recovery never overwrites an immutable version.

The next hosted release must retain the publication job, exact version, package
provenance, and final dist-tag observations as evidence. The shell fixture tests
exercise preparation and recovery behavior; they do not certify registry trust
or substitute for that hosted publication.
