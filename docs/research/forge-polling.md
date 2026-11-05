# Forge polling demand, batching, and attribution

The session catalog uses cached PR facts to filter, count, and sort. Once it has
cut an authorized page, its response projection schedules stale display reads for
the branches contributing to that page, including linked and rolled-up work. Facet reads and hidden pages do not discover every historical
branch. A cold PR-specific column can therefore stay empty until the branch is
shown in an unfiltered page, explicitly refreshed, or observed by a monitor.
Explicit refresh and delivery evidence keep their existing paths and the shared
provider quota hold. Removed worktrees retain their cached observations.

The GitHub provider coalesces concurrent branch selections for the same repository
and credential over five milliseconds, up to twenty branches per query. Branch
values travel as GraphQL variables; aliases are generated locally. Each branch
has separate OPEN and MERGED/CLOSED connections in the same query, ordered by
update time. Selection
prefers a same-repository open PR. It pages each connection at most twice and reports incomplete
instead of selecting a terminal PR from an unfinished population. Checks remain
bound to the selected immutable head; review activity retains its current bounds.
The provider owns, cancels and joins the short-lived batching workers. Each
caller retains independent cancellation, while a shared lookup has an eight
second deadline. No extension protocol, config key or persisted shape changes.

Every outgoing GitHub HTTP request emits a structured `forge.call` record on the
extension's stderr. Records identify the provider operation (`source`), HTTP
method, REST/GraphQL API, HTTP status, remaining quota (`-1` when unknown),
available reset/retry timestamps, numeric Retry-After, and secondary/GraphQL
rate-limit signals. Request headers, tokens, URLs, variables and response bodies
are excluded. HTTP 200 GraphQL rate-limit failures retain their actual HTTP
status and an explicit GraphQL limit flag. These records attribute traffic to
provider operations; they do not identify an individual session or operator.

## Conditional REST assessment

GitHub documents authenticated conditional GETs with `If-None-Match`/ETag or
`If-Modified-Since`. A correctly authorized request returning **304** does not
consume the **primary** REST rate limit. This is not a guarantee that all
conditional calls are free: changed responses still consume quota, and the
secondary concurrency/abuse restrictions remain applicable.

Sources: [GitHub REST best practices](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api#use-conditional-requests),
[REST rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api),
[GraphQL repository connections](https://docs.github.com/en/graphql/reference/objects#repository).

ETags are deferred. The current display path uses GraphQL POSTs, so adding an ETag
header there cannot provide REST's documented 304 benefit. A future conditional
REST implementation must retain the response body alongside the validator,
partition both by credential, repository, endpoint, query and page, invalidate on
credential/repository changes, and reconstruct a complete observation after 304.
Checks and reviews require independent validation; an unchanged PR payload alone
does not prove they are unchanged. Capability caching already suppresses repeated
repository reads. Batching and limiting demand reduce requests without adding a
second durable cache or changing freshness guarantees.

## Verification

Owning suites:

- `TestProviderGraphQL`: open/terminal aliases, open preference, incomplete
  pagination, immutable checks, optional reviews and sanitized quota attribution.
- `TestManagerBoardCatalog`: real SQLite catalog paging and passive facets;
  refresh requests are observed at the provider-reader boundary.
- `TestPullRequestService`: preserved absence TTL, explicit freshness, admission,
  failure retention and Retry-After behavior.
- `TestSessionPullRequestAdapters`: repository resolution and projection parity.

Local checks are focused. Repository-wide, race and E2E gates remain CI-owned.
Live-provider and final-head receipts are recorded in the PR's verification section.

### Live GitHub receipt (2026-11-04)

An isolated, temporary Go overlay executed the production GitHub provider against
`compozy/compozy`, using the existing credential resolver. Three concurrent branch
observations completed with one aliased GraphQL request, plus one REST capability
request. Attribution included API and remaining-quota fields without authorization
headers. This proves the initial shared lookup; it does not claim that sequential
checks/reviews or a large backlog fit the batching window. The display service's
four concurrent refresh slots bound usual batches to four branches.
