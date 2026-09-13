# marketplace-catalog — design notes

Five boards for the Marketplace rework requested 2026-09-10 (Pedro, with Codex
and Grok Bot marketplace screenshots as references). The set is the reference for
`cy-create-spec`; every decision (Q1–Q3) is closed as of 2026-09-10. (Historical:
when this set was delivered there was no spec yet; the spec now lives at
`.compozy/tasks/marketplace-catalog/` and is the technical authority.) Boards:
`marketplace-catalog-browse.html` (S1) · `-card.html` (S2) · `-installed.html` (S3)
· `-cut.html` (S4) · `-sources.html` (S5). Hub: `index.html`. Lane:
`marketplace-catalog.css`.

## What was asked, and where it lands

| Ask (2026-09-10) | Decision | Board |
| --- | --- | --- |
| One marketplace screen; stop splitting Installed vs Marketplace | Scope toggle deleted. The whole catalog is listed; an installed entry reads **Installed** on its card. | 01 |
| No Extensions · MCPs · Skills switch — everything is an extension (rev 2) | Kinds deleted. The 17 catalog MCP servers are re-published as extensions that carry a server; Documentation Writer and the skills catalog are retired. | 01 · 04 |
| An Installed button | The **installed shelf** (logo stack + count + updates) at the top of the body drills into `/marketplace/installed`, one flat list with the management controls on each row and what each extension contains. | 01 · 03 |
| Descriptions on every card; icons and logos | Row-card anatomy (logo 40 · name · one-line description · trail). Logo ladder: feed `icon` → brand registry → Boring Avatars `marble` tile → monogram. Installed rows always carry the description. | 02 |
| Use the agent-plugins compatibility to have all the plugins | Plugin marketplaces (`marketplace.json`) become catalog sources with a Compozy preset list; each plugin is an entry under a section named after its marketplace. | 05 |
| Remove “Claude Hub” from skills search | It is **ClawHub** (`clawhub.ai`). With no skills kind the whole skills marketplace goes, ClawHub included; delete targets listed with their compatibility regime. | 04 §04 |

## Revision log

- **2026-09-10 — Q1 and Q2 decided (yes · yes).** Client plugin layouts are
  accepted as loader aliases; the layout blocker state is retired from board
  02 §03 and board 05 §04. `inputs[]` lands on extension entries and manifests.
  No open decisions remain; next step is `cy-create-spec`.
- **2026-09-12 — Marketplace hard cut approved by Pedro.** ADR-005/007 supersede the compatibility-only decisions below. Remove old Marketplace endpoints, acquisition verbs, redirects, dual feeds and compatibility-only installer behavior in this release. Preserve existing extension packages, lifecycle and installed state. Historical boards remain visual references; the revised spec owns executable contracts.
- **2026-09-10 — spec peer review round 2 incorporated (historical).** Compatibility table
  refined: the retained MCP install keeps `name`, scope selectors, vault refs,
  and HTTP 200; runtime names are allocated once and persisted; ref-based
  origin identity; package cache for marketplace plugins; approved digest on
  install. Boards unchanged.
- **2026-09-10 — spec peer review round 1 incorporated (historical).** Compatibility notes
  rewritten to SD-013 letter: retained routes, verbs, keys, and tool arguments
  keep performing their operations one release through boundary translations
  (ClawHub fenced behind the retained skills routes, deleted v0.6.0); the feed
  ships as `catalog/v3/` with the root family retained one release (ADR-007);
  client plugin layouts load through grammar adapters, not path aliases
  (ADR-002); extension-provided MCP servers carry an owner (ADR-008); the
  sources surface is labeled experimental. Boards unchanged.
- **2026-09-10 — Q3 decided.** Pedro picked Boring Avatars `marble` (default
  palette) as rung 3 of the logo ladder. Every monogram well on boards 01–05
  became a marble tile seeded by the entry id; the monogram is rung 4 (render
  failure only). Lane chapter 07 gained `.mkt-logo--full`.
- **2026-09-10 — proposal P1.** `marketplace-catalog-avatars.html`: hash-based
  avatar candidates for rung 3 of the logo ladder, generated from real entry
  ids (see Q3).
- **2026-09-10 — rev 2 (same day).** Pedro: no Skills and no MCPs in the
  marketplace; everything is an extension. Kind views deleted from every
  board; board 04 rewritten from “MCPs & Skills” to **The cut**; Installed
  view is flat with a contents summary per row and an updates line; the
  “Official” tag on cards replaced by a faint “community · author” word for
  third-party entries; Add ▾ loses “Add MCP server…” (manual servers stay in
  Settings). Q2 re-stated (inputs grammar) since curated skills no longer exist.
- **2026-09-10 — rev 1.** Five boards, lane, hub, this contract.

## Locked decisions

### One surface, one kind (S1)

- Routes: `/marketplace` (browse) · `/marketplace/installed` ·
  `/marketplace/$entryId` (detail). Search state: `?q=` only. The kind
  segment and old redirects are gone; retired kind paths use normal not-found behavior.
- `?tab=market` has no mode or redirect adapter.
  `manage_path` on the listing points to `/marketplace/installed`.
- Head: Store glyph · “Marketplace” · mono count of listed entries · Refresh
  (ghost sm) · **Add ▾** (outline sm: install from GitHub · from a local build
  · add plugin marketplace). Head budget ≤ 2 actions; accent in chrome = 0.
- Strip: search only (production `ListingToolbar.Search`, 28px, `/`). No
  views, no filters, no display mode (cards-only family, standing decision
  from `_done/marketplace/LISTING-STANDARD.md`).
- Body: installed shelf (only when ≥ 1 installed) → sections by source →
  grid. With a single source there is no section header.

### Installed shelf (new `MarketplaceInstalledShelf`)

40px line; up to six 24px logos stacked (−6px, 2px canvas ring) + “+n”; mono
count + “installed” + chevron; warning text “n updates available” only when
n > 0. The whole line is one link. Reference: Grok Bot “7 installed ›”.

### Sections by source (new `MarketplaceCatalogSection`)

`details/summary`, open by default, 36px summary: chevron · title (source name)
· count-chip · gist. Order: Compozy catalog first (name order), then plugin
marketplaces in registration order. Search narrows every section; empty
sections do not render; the gist reads “n of m matches ‘q’”.

### Entry card (new `MarketplaceEntryCard`, replaces two cards)

- Grid `40px · 1fr · auto`, 12px gaps, padding 10/12, min-height 60,
  radius-lg, canvas-soft, hover elevated (CatalogCard tokens). Two columns at
  ≥ 960px window width (container query), one below; in one column the
  description may wrap to two lines.
- Name 13/510 `--fg-strong` (links to detail) · optional faint mono
  “community · author” after the name when `tier ≠ official` (official
  renders nothing) · optional hollow warning xs pill `unverified`.
- Description 12px `--muted`, one line, full text in `title`. Missing →
  italic faint “No description yet”, never a gap.
- The card never says what an entry contains and never shows author,
  version, downloads, transport, format, or source: the Installed row shows
  contents, the detail shows the rest.
- Catalog trail decision table (top wins): pending → “Installing…/Updating…”
  · `trust.decision = blocked` → hollow danger pill `Blocked`
  · `update_available` → faint mono target version + **Update** ·
  `installed` → `.mkt-installed` (success text + check) · else **Install**
  (neutral filled `.btn--sm`). Install is direct when the entry is verified
  and declares no inputs; otherwise the extension install dialog opens
  (inputs step, unverified confirmation). Just-installed flashes success-tint
  once (1.4s; reduced-motion → static elevated).
- Installed trail: update (version + Update) first; then, when the extension
  provides an MCP server, the server’s status word + dot (running success ·
  needs authorization warning · stopped hollow) and **Authorize** when
  pending; then Switch (enable) and the overflow. After the name: contents
  summary in 12px subtle (“1 MCP server · 2 skills”, from the inventory) and,
  for plugins from a marketplace, the faint mono marketplace name. Scope word
  (`workspace · acme-api`) only when not global. Overflow: View details ·
  Edit server configuration (only with a server) · Remove… (type-to-confirm
  ConfirmDialog, unchanged).

### Logo ladder (new `MarketplaceEntryLogo`)

1. `icon` on the listing (https or data URL; ≤ 64 KiB; png/svg/webp) → 28px
   contain in the 40px well, `referrerpolicy="no-referrer"`, error → 3.
2. Brand registry keyed by `entry_id`, then the tail of `install_slug`
   (`KindIcon` registry pattern; reuses `@compozy/ui/logos`, adds no logo).
   currentColor marks read `--fg-strong`; multicolor marks keep their fills.
3. **Generated tile — Boring Avatars `marble`, default palette** (decided by
   Pedro 2026-09-10 on `marketplace-catalog-avatars.html`). `name = entry_id`,
   `square`, rendered as a memoized SVG string that fills the well at every
   size (radius inherited); no well shows behind it. Dependency:
   `bun add boring-avatars` (MIT), used only inside `MarketplaceEntryLogo`.
   Authorized delta to “color = state, never taxonomy”: in this one well,
   color is the identity of a thing; the rest of the row stays neutral.
4. Monogram: first grapheme of `name`, uppercase, Geist 600, neutral well —
   only if rung 3 throws (pure function; in practice never).

No generic glyph is ever a card fallback; the window glyph is Store, the
empty-state well uses Puzzle. Sizes: sm 24 (shelf) · md 40 (row) · lg 56
(detail lede).

### MCP servers as extensions (S4)

- Each of the 17 `catalog/mcp.json` entries is re-published in
  `catalog/v3/extensions.json` as a packaged extension (`format: compozy`,
  `tier: official`) whose manifest declares the server in
  `resources.mcp_servers` (launch npm/uvx/docker/remote → command/args/url)
  and its inputs. The feed `inputs[]` grammar (secret · string · identifier ·
  boolean; binding env / url_query) moves onto the extension entry.
- Install: the existing extension install dialog gains an inputs step
  rendered from the entry (secret → vault). OAuth servers install first and
  read “Needs authorization” on the Installed row; Authorize uses the existing
  MCP auth routes keyed by the server the extension provides.
- Detail: `MarketplaceDetailExtension` gains a **Server** section (Status ·
  Launch · Auth · Inputs · Scope · actions) built from the retired MCP detail
  rail pieces, only when the manifest provides a server.
- Manual MCP servers (Settings › MCP servers) are untouched; they never were
  marketplace objects.

### Skills (S4 §03)

Skills are not marketplace objects. They arrive inside extensions (contents
summary, detail list) and from skill folders (Settings › Skills › Sources,
skill-sources set unchanged). Documentation Writer leaves the catalog.

### The cut — delete targets (S4 §04)

The revised ADR-005/007 contract supersedes the historical cut board's compatibility copy. Ship only catalog/v3; remove root v2 publication/fallback, old Marketplace kind/MCP-install/remote-skills routes, flags, verbs, tool arguments and obsolete config consumers. No compatibility package or later removal release. Preserve existing extension packages, refs, lifecycle and installed state; manual MCP and installed skills remain usable. The spec's hardcut-removal-plan.md owns observed branch debt and task assignments.

### Plugin marketplaces as sources (S5)

- Backend: `internal/marketplace/pluginsource` reads a `marketplace.json`
  (root or `.claude-plugin/`) from GitHub, git, or a folder; `plugins[]` →
  extension listings with `format: agent-plugin`, `source: <marketplace
  name>`, `layout`, and `icon` when present. Config
  `[[marketplace.plugin_sources]] name · source · enabled`; presets from a new
  feed `catalog/v3/marketplaces.json` (manifest v3, `default: on|off`). Routes
  `GET/POST/DELETE /api/marketplace/sources`,
  `POST /api/marketplace/sources/:name/refresh`, `?dry_run=true` for the
  dialog check. CLI `compozy marketplace sources list|add|remove|refresh`.
- Web: Settings › Marketplace page (new; also hosts the existing
  `marketplace.catalog.*` keys) with `SettingsMarketplaceSourcesSection` on
  the skill-sources collapsible-row grammar: Compozy catalog always on · presets
  with a switch · custom rows with a `custom` pill and Remove in the fold.
  `AddMarketplaceDialog`: one field, dry-run check, success notice, Add.
- Degraded: `couldn’t refresh` warning xs pill + sentence + last-read age +
  micro-mono reason; the last-read plugins keep serving. Not a marketplace →
  danger notice in the dialog naming the two paths checked.
- Turning a marketplace off hides its section; installed plugins stay
  installed and keep their origin in the Installed view.

## Open decisions (for Pedro)

- ~~**Q1 — client layouts.**~~ **Decided 2026-09-10: yes.** The manifest loader
  accepts `.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`, and
  `.cursor-plugin/plugin.json` as read-only aliases of the standard layout and
  records `layout` in provenance (`internal/extension/manifest_load_agent_plugin.go`
  — the detection that raises `AgentPluginClientLayoutError` becomes the
  locate-then-adapt path: the client manifest is decoded by a grammar adapter
  (no `$schema` required, `mcpServers` and `skills` honored, commands/agents/
  hooks reported as ignored), never sent through the strict decoder unchanged
  (ADR-002); the error stays only for a package with no manifest anywhere). The
  “Claude Code layout” card state and the “k need the standard layout” source
  sentence are retired; `layout` on the listing stays for provenance.
- ~~**Q2 — inputs on extension entries.**~~ **Decided 2026-09-10: yes.** The
  `inputs[]` grammar (id · prompt · type secret/string/identifier/boolean ·
  required · binding env/url_query) moves onto extension feed entries and the
  extension manifest; the extension install dialog renders it (board 04 §01)
  and stores secrets in the vault; `requires_env` stays for names-only
  packages. The 17 re-published servers carry their inputs unchanged.

- ~~**Q3 — generated mark instead of the letter monogram.**~~ **Decided
  2026-09-10: Boring Avatars `marble`, default palette** (see the logo ladder
  above and `marketplace-catalog-avatars.html`, kept as the record of the
  alternatives: DiceBear 16 CC0 styles, Boring Avatars 6 variants, Jdenticon,
  Minidenticons, Bauhaus, Avvvatars).

## Marketplace hard-cut notes (Pedro, 2026-09-12; ADR-005/007)

| Surface | Treatment |
| --- | --- |
| Old Web kind paths and tab adapters | Delete with redirects; canonical Browse/Installed/detail remain. |
| Old kind/grouped-search, MCP catalog install and remote skills acquisition | Remove routes/CLI/tool/schema consumers now; no translations or deprecation window. |
| Root v2 feeds and fallback | Remove; daemon, publisher and site consume only v3. |
| Existing extension packages, acquisition refs, lifecycle and installed data | Preserve. Input/digest/source capabilities remain additive. |
| Manual MCP and installed skills | Preserve definitions, auth, files and local loading; no remote acquisition dependency. |
| Extension-server management | Explicit owner plus logical name; retain automatic sticky runtime allocation and isolated auth/overrides, no requested install runtime_name. |
| Obsolete skills Marketplace config | Remove runtime support; one-time migration archives values and preserves unrelated config. |
| SQLite/provenance/auth | Lossless upgrades; only the already-approved retired derived catalog rows may be dropped. |
| Source API stability | The existing experimental label remains independent of the removed compatibility window. |

The visual HTML boards are historical design references. Their old compatibility copy does not reintroduce executable requirements superseded by the user's amendment.

## Canonical data story

| Fixture | Values |
| --- | --- |
| Compozy catalog | 20 extensions: Repository Orientation (official, installed) · Batuta (community, Francisross Soares) · herdr bridge (community, Alexandre Akira, installed 0.3.2 → 0.3.3) · the 17 `catalog/mcp.json` servers re-published (Airtable, Atlassian, Brave Search, Cal.com, Cloudflare Code Mode, Context7, GitHub, GitLab, Grafana, Linear, Notion, Playwright, Postgres, PostHog, Sentry, Stripe, Supabase) — installed: GitHub (needs authorization), Context7 (running) |
| Plugin marketplaces | `claude-plugins-official` (preset on · 6 plugins: Code Review (installed, disabled), Feature Dev, Frontend Design, Security Guidance, Commit Commands, PR Review Toolkit) · `openai-codex` (preset off) · `team-plugins` (custom, `~/Dev/team-plugins`, 4: cc-loop, claude-code-setup, claude-md-management, context7) — names are placeholders for what the documents declare |
| Installed totals | 5 · 1 update (herdr bridge) · 1 needs authorization (GitHub) |
| Contents summaries | Repository Orientation “1 skill” · herdr bridge “1 bridge · 2 skills” · GitHub / Context7 “1 MCP server” · Code Review “3 skills · 1 hook” · Batuta “2 loops · 1 skill” |
| Scope fixture | Postgres `workspace · acme-api` |
| Errors | `source_unreachable · ENOENT` · `not_a_marketplace` · catalog DNS failure sentence |

## Primitives — reuse before create

| Board element | Class (lane) | Production owner |
| --- | --- | --- |
| window head / strip / body | `.win-head` `.win-toolbar` `.win-body` (ds-shell) | OS window + `useTopbarSlot` |
| search | `.field.field-search` | `ListingToolbar.Search` |
| installed shelf | `.mkt-shelf*` | `MarketplaceInstalledShelf` (new) |
| section | `details.mkt-section > summary.mkt-section__sum` | `MarketplaceCatalogSection` (new, Collapsible) |
| grid | `.mkt-grid` | `MarketplaceGrid` |
| row card | `.mkt-card*` `.mkt-contents` `.mkt-origin` | `MarketplaceEntryCard` (new, on CatalogCard tokens) |
| logo | `.mkt-logo*` | `MarketplaceEntryLogo` (new) |
| trail | `.mkt-trail` `.mkt-status` `.mkt-installed` (ds-core) | `MarketplaceEntryTrail` (new) |
| updates line | `.mkt-updates` | `MarketplaceInstalledPage` |
| trust pills | `.pill[data-form=hollow]` | `Pill` |
| switch | `.switch` | `Switch` |
| add menu | `.menu.menu--overlay.mkt-add-menu` | `MarketplaceAddMenu` (new, DropdownMenu) |
| empties | `.empty` | `Empty` |
| stale line | `.mkt-stale` | `MarketplaceResults` |
| server section | `.railbox .rail-sec .prow` | `MarketplaceDetailExtension` (Server section) |
| settings rows | `.sgroup` `.panelbox` `details.mkt-src` | `SettingsMarketplaceSourcesSection` (new) |
| dialogs | `.dialog.dialog--sm` | `ConfirmDialog` · `ExtensionInstallDialog` · `AddMarketplaceDialog` (new) |
| notices | `.notice` success/danger | `Notice` |

## Staging that is not a VC

Overflow menu and dialogs render in-flow; production positions them as
popovers. Feed icons are stand-ins marked `data-stand-in`. Window fixture width
`--mkt-win-w: 1080px`. Section counts are the fixture, not a claim about any
marketplace. The `.tag` class is no longer used on cards.

## Chapter map

00 lab scaffold · 01 board contract · 02 window host · 03 installed shelf ·
04 catalog section · 05 grid · 06 entry card · 07 entry logo · 08 installed
view (+ updates line) · 09 add menu · 10 sources rows · 11 states · 12 detail
lede. Later runs append after the append point; never renumber.

## Board budget

| Board | Surface | Sections |
| --- | --- | --- |
| 01 browse | S1 | 8 |
| 02 card | S2 | 6 |
| 03 installed | S3 | 6 |
| 04 cut | S4 | 4 |
| 05 sources | S5 | 5 |

Iterate on these files — never regenerate a delivered board from scratch.

## Round-3 contract alignment

The amended spec and qa/peer-review-incorporation-round3.md own the execution order (01 → 05 → 02 → 03 → 04), schema/attachment preservation and complete removal inventory. Old saved Marketplace locations preserve their window state and show a normal not-found detail with Back; no redirect or hydration alias is added. Canonical listing slugs are derived independently of preserved extension acquisition refs. These are behavior/ownership clarifications; the accepted visual references remain unchanged.
