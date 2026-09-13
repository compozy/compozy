# Catalog brand icons

These files identify the services provided by the packaged MCP extensions. Source URLs and SHA-256 digests are recorded in `sources.json`. Downloads retain the original pixels and geometry. Where an official favicon is an ICO container, the original embedded PNG bytes are extracted without resizing or recoloring.

GitHub and Linear reuse the existing `@compozy/ui` components, rendered as standalone SVGs for the feed image field. No app-local brand component is introduced. Other assets come from each vendor website or official repository.

| Entry        | Source page                                                                   | Asset            |
| ------------ | ----------------------------------------------------------------------------- | ---------------- |
| airtable     | https://www.airtable.com                                                      | airtable.png     |
| atlassian    | https://www.atlassian.com/legal/trademark                                     | atlassian.svg    |
| brave-search | https://brave.com                                                             | brave-search.png |
| calcom       | https://cal.com                                                               | calcom.png       |
| cloudflare   | https://www.cloudflare.com                                                    | cloudflare.png   |
| context7     | https://context7.com                                                          | context7.png     |
| github       | packages/ui/src/logos/github.tsx                                              | github.svg       |
| gitlab       | https://about.gitlab.com                                                      | gitlab.png       |
| grafana      | https://github.com/grafana/grafana/tree/main/public/img                       | grafana.svg      |
| linear       | packages/ui/src/logos/linear.tsx                                              | linear.svg       |
| notion       | https://www.notion.com                                                        | notion.png       |
| playwright   | https://playwright.dev                                                        | playwright.png   |
| postgres     | https://www.postgresql.org                                                    | postgres.png     |
| posthog      | https://posthog.com                                                           | posthog.svg      |
| sentry       | https://github.com/getsentry/sentry/blob/master/static/images/logo-sentry.svg | sentry.svg       |
| stripe       | https://stripe.com                                                            | stripe.svg       |
| supabase     | https://supabase.com                                                          | supabase.png     |

`catalog/sources.json` owns the icon URLs. Republish through `go run ./cmd/compozy-catalog publish ./catalog ./catalog`; the v3 family carries icons and the retained v2 family omits them.
