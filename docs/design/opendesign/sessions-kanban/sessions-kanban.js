/* Sessions Kanban board — local demo state only. Nothing here calls a product API.
   All titles, PR numbers, counts and times are ILLUSTRATIVE. */
(function () {
  "use strict";

  /* ---------- primitives ---------- */
  const ic = (name, cls = "") => `<svg class="i ${cls}" aria-hidden="true"><use href="#i-${name}"/></svg>`;
  const esc = s => String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);

  // StateGlyph — geometry copied from packages/ui/src/components/custom/state-glyph.tsx.
  // `waiting` reuses the existing parked glyphs (`delegated`, `monitoring`) — no new primitive.
  const R = 6.25, C = 2 * Math.PI * R;
  const GLYPH = {
    running: `<circle cx="8" cy="8" r="${R}" opacity=".22"/><circle class="arc" cx="8" cy="8" r="${R}" stroke-dasharray="${C / 4} ${C}" stroke-linecap="round"/>`,
    queued: `<circle cx="8" cy="8" r="${R}" stroke-dasharray="2.2 2.2"/>`,
    delegated: `<circle cx="8" cy="8" r="${R}"/><circle cx="8" cy="8" r="2.25" fill="currentColor" stroke="none"/>`,
    done: `<circle cx="8" cy="8" r="7" fill="currentColor" stroke="none"/><path d="M5 8.25l2 2 4-4.25" stroke="var(--color-canvas)" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"/>`,
    failed: `<circle cx="8" cy="8" r="${R}"/><path d="M6 6l4 4M10 6l-4 4" stroke-linecap="round"/>`,
    stopped: `<rect x="4.5" y="4.5" width="7" height="7" rx="1.5" fill="currentColor" stroke="none"/>`,
    attention: `<circle cx="8" cy="8" r="4" fill="currentColor" stroke="none"/>`,
    idle: `<circle cx="8" cy="8" r="4" fill="currentColor" stroke="none"/>`,
    monitoring: `<circle cx="8" cy="8" r="${R}"/><path d="M8 8l3.25-3.25" stroke-linecap="round"/><circle cx="8" cy="8" r="1.5" fill="currentColor" stroke="none"/>`,
  };
  const glyph = (state, sm = false, label = "") =>
    `<svg class="sg${sm ? " sg--sm" : ""}" data-state="${state}" viewBox="0 0 16 16" ${label ? `role="img" aria-label="${esc(label)}"` : 'aria-hidden="true"'}>${GLYPH[state]}</svg>`;

  // badge token → glyph state + word (web/src/systems/session/lib/session-badge.ts + `waiting`)
  const BADGE = {
    "needs-attention": ["attention", "Needs attention"],
    "waiting-for-input": ["attention", "Needs your answer"],
    "waiting-for-auth": ["attention", "Needs sign-in"],
    failed: ["failed", "Failed"],
    running: ["running", "Working"],
    // daemon badge `waiting` + `waiting_on` (dominant signal) → existing parked glyph
    "waiting:sessions": ["delegated", "Waiting on sessions"],
    "waiting:monitor": ["monitoring", "Monitoring"],
    done: ["done", "Done"],
    idle: ["idle", "Idle"],
  };

  const badgeOf = s => BADGE[s.badge === "waiting" ? `waiting:${s.waitingOn || "sessions"}` : s.badge] || BADGE.idle;

  /* ---------- pull-request-signal.ts (pure mapping, shared by chip and card) ---------- */
  // Card status (one, right-aligned): needs-you reason > checks failing > changes requested >
  // conflicts > ready > draft > merged > closed. Open with nothing to say → none.
  // Rendered as <Pill form="plain"> + <Pill.Dot>: dot + muted text, no plate (DESIGN.md §9),
  // so a board of cards stays neutral at rest and only the dots carry live state.
  function prPill(pr) {
    if (!pr) return null;
    if (pr.state === "merged") return { label: "Merged", tone: "info" };
    if (pr.state === "closed") return { label: "Closed", tone: "neutral" };
    if (pr.checks === "failing") return { label: "Checks failing", tone: "danger" };
    if (pr.review === "changes_requested") return { label: "Changes requested", tone: "danger" };
    if (pr.mergeable === "conflicting") return { label: "Conflicts", tone: "danger" };
    if (pr.ready) return { label: "Ready to merge", tone: "success" };
    if (pr.draft) return { label: "Draft", tone: "neutral" };
    return null;
  }
  const REASON = {
    approval: { label: "Approval", tone: "accent" },
    input: { label: "Input", tone: "accent" },
    auth: { label: "Auth", tone: "accent" },
    failed: { label: "Failed", tone: "danger" },
  };
  const dot = tone => `<span class="pill-dot" data-tone="${tone}" aria-hidden="true"></span>`;
  function pillHTML(p) {
    if (!p) return "";
    return `<span class="pill" data-form="plain">${dot(p.tone)}${esc(p.label)}</span>`;
  }

  /* ---------- S1 · SessionPullRequestChip ---------- */
  const PR_ICON = { open: "git-pull-request", draft: "git-pull-request-draft", merged: "git-merge", closed: "git-pull-request-closed" };
  const PR_WORD = { open: "Open", draft: "Draft", merged: "Merged", closed: "Closed" };
  function prKind(pr) { return pr.state === "open" && pr.draft ? "draft" : pr.state; }

  // ≤ 2 signals. Color only where a decision is needed (failing, changes, conflicts, ready);
  // pending/passing/approved/review-required stay muted.
  function chipSignals(pr) {
    const out = [];
    if (pr.state !== "open") return out;
    if (pr.ready) return [`<span class="spr-sig" data-tone="success" data-callout-k="ready">${dot("success")}Ready to merge</span>`];
    const c = pr.checks, n = pr.counts || {};
    if (c === "failing") out.push(`<span class="spr-sig" data-tone="danger" data-callout-k="checks">${dot("danger")}${n.failed} failing</span>`);
    else if (c === "pending") out.push(`<span class="spr-sig" data-callout-k="checks"><span class="spinner" style="width:10px;height:10px"></span>${n.pending} pending</span>`);
    else if (c === "passing") out.push(`<span class="spr-sig" data-callout-k="checks">${ic("check", "i--12")}${n.passed} passed</span>`);
    else if (c === "unknown") out.push(`<span class="spr-sig" data-callout-k="checks">Checks unknown</span>`);
    if (pr.review === "changes_requested") out.push(`<span class="spr-sig" data-tone="danger" data-callout-k="review">${dot("danger")}Changes requested</span>`);
    else if (pr.mergeable === "conflicting") out.push(`<span class="spr-sig" data-tone="danger" data-callout-k="review">${dot("danger")}Conflicts</span>`);
    else if (pr.review === "approved") out.push(`<span class="spr-sig" data-callout-k="review">Approved</span>`);
    else if (pr.review === "review_required") out.push(`<span class="spr-sig" data-callout-k="review">Review required</span>`);
    return out;
  }

  function chipTip(m) {
    const lk = m.lookup || {};
    if (!m.pr) {
      const body = {
        pending: "Looking up a pull request for this branch",
        ok: "No pull request for this branch yet",
        error: esc(lk.causeText || "GitHub not connected"),
        incomplete: "Couldn't match a PR for this branch",
      }[lk.state];
      return `<div>${body}</div>${lk.state === "error" ? `<div class="tip-foot">Retrying automatically ${esc(lk.retry || "in 2 min")}</div>` : ""}`;
    }
    const pr = m.pr, n = pr.counts || {};
    const rows = [];
    if (pr.state === "open") {
      rows.push(`<div class="tip-row"><span class="k">Checks</span><span>${pr.checks === "unknown" ? "Unknown" : pr.checks === "none" ? "None" : `${n.passed || 0} passed · ${n.failed || 0} failed · ${n.pending || 0} pending`}</span></div>`);
      rows.push(`<div class="tip-row"><span class="k">Review</span><span>${{ approved: "Approved", changes_requested: "Changes requested", review_required: "Review required", none: "Not required", unknown: "Unknown" }[pr.review]}</span></div>`);
      rows.push(`<div class="tip-row"><span class="k">Mergeable</span><span>${{ mergeable: "Yes", conflicting: "Conflicts", unknown: "Unknown" }[pr.mergeable]}</span></div>`);
    }
    const names = pr.checks === "failing" && pr.failedNames ? `<ul class="tip-list">${pr.failedNames.map(x => `<li>${esc(x)}</li>`).join("")}</ul>` : "";
    const foot = m.lookup && (m.lookup.stale || m.lookup.state === "error")
      ? `<div class="tip-foot">${m.lookup.state === "error" ? esc(m.lookup.causeText) + " · " : ""}last checked ${esc(m.lookup.observed || "14 min ago")}</div>`
      : `<div class="tip-foot">Checked ${esc((m.lookup && m.lookup.observed) || "just now")}</div>`;
    return `<div>${esc(pr.title || "")}</div>${rows.length ? `<div class="tip-rows">${rows.join("")}</div>` : ""}${names}${foot}`;
  }

  function chipHTML(m, opts = {}) {
    if (!m || !m.branch) return "";
    const lk = m.lookup || { state: "ok" };
    const stale = Boolean(m.pr && (lk.stale || lk.state === "error"));
    let inner;
    if (!m.pr) {
      const mark = { pending: `<span class="spr-mark" data-callout-k="mark">${glyph("queued", true)}</span>`, error: `<span class="spr-mark" data-callout-k="mark">${ic("circle-slash", "i--12")}</span>`, incomplete: `<span class="spr-mark" data-callout-k="mark">${ic("circle-help", "i--12")}</span>` }[lk.state] || "";
      inner = `<span class="spr-glyph" data-callout-k="glyph">${m.refreshing ? '<span class="spinner"></span>' : ic("git-branch")}</span><span class="spr-branch" data-callout-k="num">${esc(m.branch)}</span>${mark}`;
    } else {
      const k = prKind(m.pr);
      const sig = chipSignals(m.pr);
      inner = `<span class="spr-glyph" data-callout-k="glyph">${m.refreshing ? '<span class="spinner"></span>' : ic(PR_ICON[k])}</span>`
        + `<span class="spr-num" data-callout-k="num">#${m.pr.number}</span>`
        + `<span class="spr-word" data-callout-k="word">${PR_WORD[k]}</span>`
        + sig.map(s => `<span class="spr-sep" aria-hidden="true">·</span>${s}`).join("")
        + (stale ? `<span class="spr-mark" data-callout-k="stale">${ic("clock", "i--12")}</span>` : "");
    }
    const label = m.pr ? `Pull request #${m.pr.number}, ${PR_WORD[prKind(m.pr)]}` : `Branch ${m.branch}`;
    return `<span class="spr" data-stale="${stale}" data-od-id="spr-chip">`
      + `<button class="spr-btn" type="button" aria-haspopup="menu" aria-expanded="${opts.menu ? "true" : "false"}" aria-label="${esc(label)}" data-chip>${inner}${ic("chevron-down", "spr-chev")}</button>`
      + (opts.tip ? `<span class="tip" role="tooltip">${chipTip(m)}</span>` : "")
      + (opts.menu ? menuHTML(m, opts.ctx || {}) : "")
      + `</span>`;
  }

  /* ---------- S2 · chip menu (DropdownMenu) ---------- */
  const EXIT = { commit_push: ["Commit & push", "upload"], push: ["Push", "upload"], create_pr: ["Create PR", "git-pull-request-create"] };
  function menuItems(m, ctx) {
    const pr = m.pr, items = [];
    const controllable = ctx.controllable !== false;
    const canFix = controllable && pr && pr.state === "open" && pr.checks === "failing";
    const canAddress = controllable && pr && pr.state === "open" && pr.review === "changes_requested";
    let primary = null;
    if (controllable && ctx.exit) primary = { id: "exit", label: EXIT[ctx.exit][0], icon: EXIT[ctx.exit][1] };
    else if (canFix) primary = { id: "fix", label: "Fix failing checks", icon: "wrench" };
    else if (canAddress) primary = { id: "address", label: "Address review comments", icon: "message-square-text" };
    else if (pr) primary = { id: "open", label: "Open PR", icon: "external-link" };
    if (primary) items.push(primary);
    const rest = [];
    if (pr && primary.id !== "open") rest.push({ id: "open", label: "Open PR", icon: "external-link" });
    rest.push({ id: "refresh", label: "Refresh", icon: "refresh-cw", hint: (m.lookup && m.lookup.observed) || "40s ago" });
    const prompts = [];
    if (canFix && primary.id !== "fix") prompts.push({ id: "fix", label: "Fix failing checks", icon: "wrench" });
    if (canAddress && primary.id !== "address") prompts.push({ id: "address", label: "Address review comments", icon: "message-square-text" });
    return { items, rest, prompts };
  }
  const BUSY = { "Commit & push": "Committing & pushing…", Push: "Pushing…", "Create PR": "Creating PR…", Refresh: "Refreshing…" };
  function menuHTML(m, ctx = {}) {
    const { items, rest, prompts } = menuItems(m, ctx);
    const it = x => {
      const busy = ctx.busy === x.id;
      return `<button class="dm-item" role="menuitem" type="button" data-act="${x.id}" ${busy ? 'aria-busy="true"' : ""}>${busy ? '<span class="spinner" style="width:16px;height:16px"></span>' : ic(x.icon)}${esc(busy ? BUSY[x.label] || "Working…" : x.label)}${x.hint && !busy ? `<span class="hint">${esc(x.hint)}</span>` : ""}</button>`;
    };
    const sep = '<div class="dm-sep" role="separator"></div>';
    return `<div class="dm${ctx.static ? " dm--static" : ""}" role="menu" aria-label="Pull request actions">${items.map(it).join("")}${rest.length ? sep + rest.map(it).join("") : ""}${prompts.length ? sep + '<div class="dm-label eyebrow-caps">Ask the agent</div>' + prompts.map(it).join("") : ""}</div>`;
  }

  /* ---------- S5 · SessionBoardCard ---------- */
  const PROV = { claude: "#p-claude", codex: "#p-openai" };
  function cardHTML(s, opts = {}) {
    const pill = s.reason ? REASON[s.reason] : prPill(s.pr && s.pr.number ? s.pr : null);
    const br = s.branch;
    let ref = "";
    if (br) {
      ref = `${ic(s.pr && s.pr.number ? PR_ICON[prKind(s.pr)] : "git-branch")}<span class="br">${esc(br)}</span>${s.pr && s.pr.number ? `<span aria-hidden="true">·</span><span class="num">#${s.pr.number}</span>` : ""}`;
    }
    if (opts.allProjects && !(s.pr && s.pr.number)) ref = `<span class="ws">${esc(s.ws || "compozy")}</span>`;
    const meta = ref || pill ? `<div class="sbc-meta" data-callout-k="meta"><span class="sbc-ref">${ref}</span>${pill ? `<span data-callout-k="pill" style="margin-left:auto;display:inline-flex">${pillHTML(pill)}</span>` : ""}</div>` : "";
    const title = s.title || "Untitled session";
    const kids = s.kids
      ? `<span class="sbc-kids" data-callout-k="kids" title="${s.kids.needs_you} needs you · ${s.kids.working} working · ${s.kids.in_review} in review · ${s.kids.done} done">${ic("list-tree")}${s.kids.total}${s.kids.needs_you ? dot("accent") : ""}</span>`
      : "";
    const parent = s.parentTitle
      ? `<span class="sbc-parent" data-archived="${Boolean(s.parentArchived)}" data-callout-k="parent">${ic("corner-down-right")}<span>${esc(s.parentTitle)}</span></span>`
      : "";
    const wait = s.badge === "waiting" ? (([g, w]) => `<span class="sbc-wait" data-callout-k="wait">${glyph(g, true)}<span>${w === "Monitoring" ? w : "Waiting"}</span></span>`)(badgeOf(s)) : "";
    const aria = [title, pill && pill.label, s.badge === "waiting" && badgeOf(s)[1], s.time].filter(Boolean).join(", ");
    return `<button type="button" class="sbc" data-session="${s.id}" data-od-id="board-card-${s.id}" aria-label="${esc(aria)}" ${opts.current ? 'aria-current="true"' : ""} ${s.archived ? 'data-archived="true"' : ""}>`
      + meta
      + `<span class="sbc-title" data-callout-k="title">${s.unread ? '<span class="sbc-unread" aria-hidden="true"></span>' : ""}<span class="sbc-title-t" ${s.title ? "" : 'data-untitled="true"'}>${esc(title)}</span></span>`
      + `<span class="sbc-foot"><span class="sbc-who" data-callout-k="who"><svg class="sbc-prov" viewBox="0 0 24 24" aria-hidden="true"><use href="${PROV[s.provider]}"/></svg><span class="agent">${esc(s.agent)}</span></span>${kids}${parent}${wait}<span class="sbc-time" data-callout-k="time">${esc(s.time)}</span></span>`
      + `</button>`;
  }

  /* ---------- illustrative data ---------- */
  const COORD = "Sessions Kanban — coordinator";
  const P = (n, o) => ({ number: n, state: "open", draft: false, checks: "passing", review: "review_required", mergeable: "mergeable", counts: { passed: 12, failed: 0, pending: 0 }, ...o });
  const S = [
    { id: "s3", col: "needs_you", badge: "needs-attention", reason: "approval", title: "Approve migration 00153 dry-run", branch: "feat/forge-pr-cache", lookup: { state: "ok" }, provider: "claude", agent: "claude", time: "2m", unread: true },
    { id: "s2", col: "needs_you", badge: "waiting-for-input", reason: "input", title: "Which token scope does the GraphQL provider need?", branch: "feat/github-graphql", pr: P(943, { title: "GitHub GraphQL forge status", checks: "failing", counts: { passed: 9, failed: 2, pending: 0 }, failedNames: ["go-test (extensions/forge)", "golangci-lint"] }), provider: "codex", agent: "codex", time: "6m", parentTitle: COORD, child: true },
    { id: "s10", col: "needs_you", badge: "failed", reason: "failed", title: "Rebase attachment lifecycle onto main", branch: "fix/attachment-rebase", lookup: { state: "ok" }, provider: "codex", agent: "codex", time: "18m" },
    { id: "s1", col: "working", badge: "waiting", waitingOn: "sessions", title: COORD, provider: "claude", agent: "orchestrator", time: "4m", kids: { total: 4, needs_you: 1, working: 2, in_review: 1, done: 0 } },
    { id: "s19", col: "working", badge: "waiting", waitingOn: "monitor", title: "Merge train: #919–#923", provider: "claude", agent: "general", time: "7m" },
    { id: "s4", col: "working", badge: "running", title: "Topbar PR chip — web slice", branch: "feat/session-pr-chip", pr: P(944, { title: "Session PR chip", draft: true, checks: "pending", counts: { passed: 4, failed: 0, pending: 3 } }), provider: "claude", agent: "designer", time: "now", parentTitle: COORD, child: true, exit: "commit_push", managed: true },
    { id: "s5", col: "working", badge: "running", title: "Waiting badge: settle-time revalidation", branch: "feat/waiting-badge", lookup: { state: "pending" }, provider: "codex", agent: "codex", time: "1m", parentTitle: COORD, child: true, managed: true, exit: "create_pr" },
    { id: "s6", col: "working", badge: "running", title: "GraphQL pagination for terminal PRs", branch: "feat/forge-graphql-pages", pr: P(949, { title: "Paginate terminal PR lookup", checks: "failing", counts: { passed: 10, failed: 1, pending: 0 }, failedNames: ["go-test (extensions/forge/github)"] }), provider: "claude", agent: "claude", time: "3m" },
    { id: "s9", col: "in_review", badge: "done", title: "Board column SQL expression + equivalence test", branch: "feat/board-column-sql", pr: P(945, { title: "Board column SQL expression", review: "approved", ready: true }), provider: "codex", agent: "codex", time: "22m", parentTitle: COORD, child: true, unread: true },
    { id: "s7", col: "in_review", badge: "done", title: "Forge status optional fields", branch: "feat/forge-status-fields", pr: P(942, { title: "forge/status optional fields", review: "changes_requested" }), provider: "claude", agent: "claude", time: "1h" },
    { id: "s8", col: "in_review", badge: "idle", title: "CLI: session list --column and session pr", branch: "feat/cli-session-pr", pr: P(946, { title: "session pr CLI", checks: "failing", counts: { passed: 14, failed: 1, pending: 0 }, failedNames: ["cli-docs-drift"] }), provider: "codex", agent: "codex", time: "2h" },
    { id: "s12", col: "in_review", badge: "done", title: "Archive bar disclosure", branch: "feat/board-archive-bar", pr: P(948, { title: "Archive bar", mergeable: "conflicting" }), provider: "codex", agent: "codex", time: "5h", unread: true },
    { id: "s11", col: "in_review", badge: "done", title: "Docs: the sessions board, step by step", branch: "docs/sessions-board", pr: P(947, { title: "Sessions board guide", draft: true, checks: "none", counts: {} }), provider: "claude", agent: "writer", time: "3h" },
    { id: "s13", col: "in_review", badge: "idle", title: "Hide child sessions filter", branch: "feat/children-filter", pr: P(950, { title: "children=exclude", checks: "pending", counts: { passed: 8, failed: 0, pending: 2 } }), provider: "claude", agent: "claude", time: "6h" },
    { id: "s14", col: "done", badge: "done", title: "Retain unsent composer attachments across sessions", branch: "fix/composer-attachments", pr: P(928, { state: "merged" }), provider: "claude", agent: "claude", time: "1d" },
    { id: "s15", col: "done", badge: "done", title: "Unify child sessions and subagent indicators", branch: "feat/child-indicators", pr: P(927, { state: "merged" }), provider: "codex", agent: "codex", time: "1d" },
    { id: "s16", col: "done", badge: "done", title: "Spike: REST vs GraphQL request budget", provider: "claude", agent: "claude", time: "2d" },
    { id: "s17", col: "done", badge: "done", title: "Explore Synara kanban model", branch: "spike/synara-kanban", pr: P(930, { state: "closed" }), provider: "codex", agent: "codex", time: "3d" },
    { id: "s18", col: "done", badge: "idle", title: "", provider: "claude", agent: "claude", time: "4d" },
  ];
  const ARCHIVED = [
    { id: "a1", archived: true, badge: "done", title: "Tasks removal — slice 3", branch: "spike/tasks-removal", pr: P(901, { state: "closed" }), provider: "codex", agent: "codex", time: "Oct 21" },
    { id: "a2", archived: true, badge: "done", title: "Agent orchestrator competitor read", provider: "claude", agent: "claude", time: "Oct 21" },
    { id: "a3", archived: true, badge: "done", title: "Provider usage collection", branch: "feat/provider-usage", pr: P(871, { state: "merged" }), provider: "codex", agent: "codex", time: "Oct 21" },
    { id: "a4", archived: true, badge: "done", title: "Monitor wake on autonomous updates", branch: "fix/monitor-wake", pr: P(860, { state: "merged" }), provider: "claude", agent: "claude", time: "Oct 20" },
  ];
  const prOf = id => S.find(x => x.id === id).pr;
  const FACETS = { needs_you: 3, working: 5, in_review: 6, done: 128 };
  const FACETS_NO_KIDS = { needs_you: 2, working: 3, in_review: 5, done: 128 };
  const COLS = [
    { id: "needs_you", name: "Needs you", glyph: "attention" },
    { id: "working", name: "Working", glyph: "running" },
    { id: "in_review", name: "In review", icon: "git-pull-request" },
    { id: "done", name: "Done", glyph: "done" },
  ];
  const chipModel = s => s && s.branch ? { branch: s.branch, pr: s.pr || null, lookup: s.lookup || { state: "ok", observed: "40s ago" } } : null;

  /* ---------- S4 · board ---------- */
  function columnHTML(col, cards, o = {}) {
    const count = o.count ?? cards.length;
    const loaded = cards.length;
    const collapsed = o.collapsed;
    const label = `${col.name}, ${count} ${count === 1 ? "session" : "sessions"}`;
    let body;
    if (o.state === "loading") body = [0, 1, 2].map(() => `<div class="sbc sbc--skel" aria-hidden="true"><div class="skel" style="width:55%"></div><div class="skel" style="width:90%;height:12px"></div><div class="skel" style="width:40%"></div></div>`).join("");
    else if (o.state === "error") body = `<div class="sb-col-err" role="alert">${ic("circle-slash")}<span>Couldn't load ${col.name.toLowerCase()} sessions.</span><button type="button" class="btn btn--sm">Retry</button></div>`;
    else if (!cards.length) body = `<div class="sb-col-empty">${{ needs_you: "Nothing needs you", working: "Nothing running", in_review: "No open pull requests", done: "Nothing finished yet" }[col.id]}</div>`;
    else body = cards.map(c => cardHTML(c, { current: o.current === c.id, allProjects: o.allProjects })).join("")
      + (count > loaded ? `<button type="button" class="btn btn--sm btn--quiet sb-more" data-more="${col.id}">Load more <span class="n">${loaded} of ${count}</span></button>` : "");
    const disc = col.id === "done" ? `<button type="button" class="btn btn--quiet btn--icon-xs" aria-expanded="${!collapsed}" aria-controls="sbcol-${col.id}" aria-label="${collapsed ? "Expand" : "Collapse"} Done" data-collapse="done" data-callout-k="disc">${ic("chevron-right")}</button>` : "";
    return `<section class="sb-col" data-column="${col.id}" ${collapsed ? 'data-collapsed="true"' : ""} ${count === 0 && o.state !== "loading" ? "data-empty" : ""} role="region" aria-label="${esc(label)}" data-od-id="board-col-${col.id}">`
      + `<header class="sb-col-head" data-callout-k="head">${col.icon ? `<svg class="i i--14" style="color:var(--color-info)" aria-hidden="true"><use href="#i-${col.icon}"/></svg>` : glyph(col.glyph)}<h2 class="sb-col-name">${col.name}</h2><span class="sb-col-count" data-callout-k="count">${o.state === "loading" ? "" : count}</span>${disc}</header>`
      + `<div class="sb-col-body" id="sbcol-${col.id}">${body}</div></section>`;
  }

  function boardHTML(st) {
    const hide = st.hideChildren;
    const facets = st.empty ? { needs_you: 0, working: 0, in_review: 0, done: 0 } : hide ? FACETS_NO_KIDS : FACETS;
    const by = id => st.empty ? [] : S.filter(s => s.col === id && !(hide && s.child));
    const doneShown = by("done").slice(0, st.doneLoaded || 5);
    const cols = COLS.map(c => columnHTML(c, c.id === "done" ? doneShown : by(c.id), {
      count: facets[c.id], collapsed: c.id === "done" && st.doneCollapsed, current: st.current,
      state: st.colState && st.colState[c.id],
    })).join("");
    const arcN = st.empty ? 0 : 37;
    const arc = arcN ? `<div class="sb-arc" data-od-id="board-archive"><button type="button" class="sb-arc-btn" aria-expanded="${Boolean(st.archiveOpen)}" aria-controls="sb-arc-body" data-archive>${ic("chevron-right", "chev")}${ic("archive")}<span>Archive</span><span class="n">${arcN}</span></button>${st.archiveOpen ? `<div class="sb-arc-body" id="sb-arc-body">${ARCHIVED.map(a => cardHTML(a)).join("")}<button type="button" class="btn btn--sm btn--quiet sb-more" style="align-self:center">Load more <span class="n">4 of 37</span></button></div>` : ""}</div>` : "";
    return `<div class="sb" data-od-id="sessions-board"><div class="sb-scroll"><div class="sb-row">${cols}</div></div>${arc}</div>`;
  }

  /* ---------- S3 · toggle ---------- */
  function toggleHTML(mode, focusDemo) {
    return `<div class="svt" role="group" aria-label="View" data-od-id="session-view-toggle">`
      + `<button type="button" class="svt-item${focusDemo === "list" ? " is-focus-demo" : ""}" aria-pressed="${mode === "list"}" tabindex="${mode === "list" ? 0 : -1}" data-mode="list">${ic("list")}List</button>`
      + `<button type="button" class="svt-item${focusDemo === "kanban" ? " is-focus-demo" : ""}" aria-pressed="${mode === "kanban"}" tabindex="${mode === "kanban" ? 0 : -1}" data-mode="kanban">${ic("square-kanban")}Kanban</button></div>`;
  }

  /* ---------- topbars ---------- */
  const CTL = `<div class="tb-ctl" aria-hidden="true"><span>${ic("minus", "i--14")}</span><span>${ic("square", "i--12")}</span><span>${ic("x", "i--14")}</span></div>`;
  function sessionTopbar(s, st) {
    const [g] = badgeOf(s);
    const m = st.chipOverride || chipModel(s);
    const wt = s.managed ? `<span class="wt-chip">${ic("folder-git-2")}${esc(s.branch.replace(/^\w+\//, ""))}</span>` : "";
    const running = s.badge === "running";
    return `<header class="tb" data-od-id="session-topbar">`
      + `<div class="tb-id" data-callout-k="id">${glyph(g, true)}<h1 class="tb-title">${esc(s.title || "Untitled session")}</h1><button class="tb-icon" type="button" aria-label="More actions">${ic("ellipsis")}</button>${toggleHTML("list")}</div>`
      + `<div class="tb-status" data-callout-k="status">${chipHTML(m, { menu: st.menuOpen, ctx: { exit: s.managed ? s.exit : null, controllable: true, busy: st.busy } })}${wt}</div><span class="tb-vsep" aria-hidden="true"></span>`
      + `<div class="tb-actions">${running ? `<button type="button" class="btn btn--sm">${ic("square")}Stop</button>` : ""}<button class="tb-icon" type="button" aria-label="Sessions sidebar" aria-pressed="true">${ic("panel-left")}</button><button class="tb-icon" type="button" aria-label="Inspector">${ic("panel-right")}</button></div>${CTL}</header>`;
  }
  function boardTopbar(st) {
    return `<header class="tb" data-od-id="board-topbar">`
      + `<div class="tb-id"><span class="tb-well">${ic("square-kanban", "i--14")}</span><h1 class="tb-title">Sessions</h1>${toggleHTML("kanban")}</div>`
      + `<div class="tb-status"></div>`
      + `<div class="tb-actions"><button type="button" class="btn btn--sm btn--quiet" aria-pressed="${Boolean(st.hideChildren)}" data-hide-kids>${ic("list-tree")}Hide child sessions</button><button type="button" class="btn btn--sm">${ic("plus")}New session</button></div>${CTL}</header>`;
  }

  /* ---------- list body ---------- */
  function listHTML(st) {
    const cur = S.find(s => s.id === st.current) || S[0];
    const rows = S.map(s => {
      const [g, word] = badgeOf(s);
      const attn = ["needs-attention", "waiting-for-input", "failed"].includes(s.badge);
      return `<button type="button" class="ls-row${s.child ? " ls-row--child" : ""}" data-session="${s.id}" aria-current="${s.id === cur.id}" data-attn="${attn}">${glyph(g)}<span class="t">${esc(s.title || "Untitled session")}</span><span class="tm">${esc(s.time)}</span><span class="sub"><span class="w">${esc(s.agent)}</span>${attn || s.badge === "waiting" ? ` · ${word}` : ""}</span></button>`;
    }).join("");
    const transcript = cur.badge === "waiting" && cur.waitingOn !== "monitor"
      ? `<div class="ls-msg ls-msg--user">Split the Kanban spec into slices and run them in parallel.</div><div class="ls-tool">${glyph("done", true)} session_spawn × 4</div><div class="ls-msg">Four children are running. I'll pick up when they report back — nothing for you to do yet.</div>`
      : `<div class="ls-msg ls-msg--user">${esc(cur.title || "Untitled session")}</div><div class="ls-tool">${glyph("done", true)} read web/src/systems/session/hooks/use-session-topbar-slot.tsx</div><div class="ls-tool">${glyph(cur.badge === "running" ? "running" : "done", true)} bun run --filter web test</div><div class="ls-msg">Illustrative transcript — the window body is unchanged by this spec.</div>`;
    return `<div class="ls"><nav class="ls-side" aria-label="Sessions"><div class="ls-head eyebrow">compozy</div>${rows}</nav><div class="ls-main"><div class="ls-transcript">${transcript}</div><div class="ls-composer"><span>Message ${esc(cur.agent)}…</span>${ic("send")}</div></div></div>`;
  }

  /* ---------- interactive demo window ---------- */
  const demo = { mode: "list", current: "s2", hideChildren: true, doneCollapsed: false, archiveOpen: false, doneLoaded: 5, menuOpen: false, busy: null, chipOverride: null, toast: null };
  const PRESETS = {
    live: null,
    nobranch: { branch: "" },
    pending: { branch: "feat/session-pr-chip", pr: null, lookup: { state: "pending" } },
    nopr: { branch: "feat/session-pr-chip", pr: null, lookup: { state: "ok" } },
    error: { branch: "feat/session-pr-chip", pr: null, lookup: { state: "error", causeText: "GitHub not connected" } },
    pendingChecks: { branch: "feat/session-pr-chip", pr: P(944, { title: "Session PR chip", checks: "pending", counts: { passed: 4, failed: 0, pending: 3 } }) },
    failing: { branch: "feat/github-graphql", pr: prOf("s2") },
    changes: { branch: "feat/forge-status-fields", pr: prOf("s7") },
    ready: { branch: "feat/board-column-sql", pr: prOf("s9") },
    stale: { branch: "feat/forge-status-fields", pr: prOf("s7"), lookup: { state: "error", causeText: "GitHub rate limited", observed: "14 min ago", stale: true } },
    merged: { branch: "fix/composer-attachments", pr: prOf("s14") },
  };

  function renderDemo() {
    const win = document.getElementById("demo-win");
    if (!win) return;
    const cur = S.find(s => s.id === demo.current);
    win.innerHTML = demo.mode === "list"
      ? sessionTopbar(cur, demo) + listHTML(demo)
      : boardTopbar(demo) + boardHTML(demo);
    if (demo.toast) win.insertAdjacentHTML("beforeend", `<div style="position:absolute;right:16px;bottom:16px;z-index:50" role="status">${demo.toast}</div>`);
    bindToggle(win);
  }
  function toast(title, body, tone = "info") {
    demo.toast = `<div class="toast"><span class="toast-mark">${glyph(tone === "success" ? "done" : "delegated")}</span><span class="toast-body"><span class="toast-title">${esc(title)}</span>${body ? `<span class="toast-text">${body}</span>` : ""}</span></div>`;
    renderDemo();
    clearTimeout(toast.t);
    toast.t = setTimeout(() => { demo.toast = null; renderDemo(); }, 3200);
  }
  function bindToggle(root) {
    root.querySelectorAll(".svt").forEach(g => g.addEventListener("keydown", e => {
      if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
      const items = [...g.querySelectorAll(".svt-item")];
      const i = items.indexOf(document.activeElement);
      const next = items[(i + (e.key === "ArrowRight" ? 1 : items.length - 1)) % items.length];
      items.forEach(x => x.tabIndex = -1); next.tabIndex = 0; next.focus(); e.preventDefault();
    }));
  }

  document.addEventListener("click", e => {
    const win = document.getElementById("demo-win");
    if (!win || !win.contains(e.target)) {
      const th = e.target.closest("[data-theme-set]");
      if (th) {
        document.documentElement.dataset.theme = th.dataset.themeSet;
        document.querySelectorAll("[data-theme-set]").forEach(x => x.setAttribute("aria-pressed", String(x === th)));
        return;
      }
      const p = e.target.closest("[data-preset]");
      if (p) {
        document.querySelectorAll("[data-preset]").forEach(b => b.setAttribute("aria-pressed", String(b === p)));
        demo.chipOverride = PRESETS[p.dataset.preset]; demo.mode = "list"; demo.menuOpen = false; renderDemo();
      }
      return;
    }
    const t = e.target;
    const mode = t.closest("[data-mode]");
    if (mode) { demo.mode = mode.dataset.mode; demo.menuOpen = false; renderDemo(); return; }
    const card = t.closest(".sbc[data-session]");
    if (card && !card.dataset.archived) { demo.current = card.dataset.session; demo.mode = "list"; demo.chipOverride = null; resetPresets(); renderDemo(); return; }
    const row = t.closest(".ls-row[data-session]");
    if (row) { demo.current = row.dataset.session; demo.chipOverride = null; demo.menuOpen = false; resetPresets(); renderDemo(); return; }
    if (t.closest("[data-chip]")) { demo.menuOpen = !demo.menuOpen; renderDemo(); const f = win.querySelector(".dm .dm-item"); if (f) f.focus(); return; }
    const act = t.closest("[data-act]");
    if (act) {
      const a = act.dataset.act, cur = S.find(s => s.id === demo.current), m = demo.chipOverride || chipModel(cur);
      if (a === "refresh" || a === "exit") {
        demo.busy = a; renderDemo();
        setTimeout(() => {
          demo.busy = null; demo.menuOpen = false;
          if (a === "exit") toast(cur.exit === "create_pr" ? "Pull request created" : "Pushed to origin", `<code>${esc(m.branch)}</code>`, "success");
          else renderDemo();
        }, 1100);
        return;
      }
      demo.menuOpen = false;
      if (a === "open") toast("Opens on GitHub", `<code>github.com/compozy/compozy/pull/${m.pr.number}</code> — new browser tab (not followed in this board).`);
      if (a === "fix") toast("Prompt queued", `Fix the failing checks on #${m.pr.number} — sent to this session.`);
      if (a === "address") toast("Prompt queued", `Address the review comments on #${m.pr.number} — sent to this session.`);
      return;
    }
    if (t.closest("[data-hide-kids]")) { demo.hideChildren = !demo.hideChildren; renderDemo(); return; }
    if (t.closest("[data-collapse]")) { demo.doneCollapsed = !demo.doneCollapsed; renderDemo(); return; }
    if (t.closest("[data-archive]")) { demo.archiveOpen = !demo.archiveOpen; renderDemo(); return; }
    if (t.closest("[data-more]")) { demo.doneLoaded = 10; renderDemo(); return; }
    if (demo.menuOpen && !t.closest(".dm")) { demo.menuOpen = false; renderDemo(); }
  });
  document.addEventListener("keydown", e => {
    if (e.key === "Escape" && demo.menuOpen) { demo.menuOpen = false; renderDemo(); const c = document.querySelector("#demo-win [data-chip]"); if (c) c.focus(); }
  });
  function resetPresets() { document.querySelectorAll("[data-preset]").forEach(b => b.setAttribute("aria-pressed", String(b.dataset.preset === "live"))); }

  /* ---------- static specimens ---------- */
  // <div data-render="chip" data-model="pending" data-tip data-callouts="glyph:1,num:2,mark:3">
  function placeCallouts(stage) {
    const spec = stage.dataset.callouts;
    if (!spec) return;
    const sr = stage.getBoundingClientRect();
    spec.split(",").forEach(pair => {
      const [k, n, side = "t"] = pair.split(":");
      const el = stage.querySelector(`[data-callout-k="${k}"]`);
      if (!el) return;
      const r = el.getBoundingClientRect();
      const b = document.createElement("i");
      b.className = "lab-n"; b.textContent = n; b.setAttribute("aria-hidden", "true");
      const x = r.left - sr.left, y = r.top - sr.top;
      const pos = { t: [x + r.width / 2 - 8, y - 19], l: [x - 21, y + r.height / 2 - 8], r: [x + r.width + 5, y + r.height / 2 - 8], b: [x + Math.min(r.width / 2, 40) - 8, y + r.height + 3] }[side];
      b.style.left = `${pos[0]}px`; b.style.top = `${pos[1]}px`;
      stage.appendChild(b);
    });
  }
  const MODELS = {
    ...PRESETS,
    draft: { branch: "feat/session-pr-chip", pr: prOf("s4") },
    conflicts: { branch: "feat/board-archive-bar", pr: prOf("s12") },
    incomplete: { branch: "feat/session-pr-chip", pr: null, lookup: { state: "incomplete" } },
    closed: { branch: "spike/synara-kanban", pr: prOf("s17") },
    unknownChecks: { branch: "feat/forge-status-fields", pr: P(942, { title: "forge/status optional fields", checks: "unknown", review: "unknown", mergeable: "unknown" }) },
    refreshing: { branch: "feat/github-graphql", pr: prOf("s2"), refreshing: true },
    passing: { branch: "feat/forge-status-fields", pr: P(942, { title: "forge/status optional fields", review: "approved", mergeable: "unknown", counts: { passed: 12 } }) },
  };
  const byId = id => [...S, ...ARCHIVED].find(s => s.id === id);

  function renderSpecimens() {
    document.querySelectorAll("[data-render]").forEach(el => {
      const kind = el.dataset.render;
      if (kind === "chip") {
        const m = MODELS[el.dataset.model];
        const html = chipHTML(m, { tip: el.hasAttribute("data-tip") });
        el.innerHTML = html || `<span class="lab-tag">— no chip rendered —</span>`;
      } else if (kind === "menu") {
        const m = MODELS[el.dataset.model];
        el.innerHTML = menuHTML(m, { static: true, exit: el.dataset.exit || null, controllable: el.dataset.controllable !== "false", busy: el.dataset.busy || null });
      } else if (kind === "status") {
        const m = MODELS[el.dataset.model];
        const wt = el.dataset.wt ? `<span class="wt-chip">${ic("folder-git-2")}${esc(el.dataset.wt)}</span>` : "";
        const origin = el.dataset.origin ? `<span class="origin-pill">${ic("corner-down-right", "i--12")}Continued from ${esc(el.dataset.origin)}</span>` : "";
        el.innerHTML = `<header class="tb" style="border:0;padding:0;height:auto;width:100%"><div class="tb-id">${glyph(el.dataset.glyph || "running", true)}<h1 class="tb-title" style="max-width:190px">${esc(el.dataset.title || "Topbar PR chip — web slice")}</h1><button class="tb-icon" type="button" aria-label="More actions">${ic("ellipsis")}</button>${toggleHTML(el.dataset.mode || "list", el.dataset.focus)}</div><div class="tb-status">${chipHTML(m)}${origin}${wt}</div></header>`;
      } else if (kind === "card") {
        const s = { ...byId(el.dataset.session) };
        if (el.dataset.patch) Object.assign(s, JSON.parse(el.dataset.patch));
        el.innerHTML = cardHTML(s, { current: el.hasAttribute("data-current"), allProjects: el.hasAttribute("data-all") });
      } else if (kind === "column") {
        const col = COLS.find(c => c.id === el.dataset.column);
        const ids = (el.dataset.ids || "").split(",").filter(Boolean);
        el.innerHTML = columnHTML(col, ids.map(byId), { count: Number(el.dataset.count || ids.length), state: el.dataset.state, collapsed: el.hasAttribute("data-collapsed") });
      } else if (kind === "board") {
        el.innerHTML = boardTopbar({ hideChildren: el.hasAttribute("data-hide") }) + boardHTML({ empty: el.hasAttribute("data-empty"), hideChildren: el.hasAttribute("data-hide"), doneCollapsed: el.hasAttribute("data-collapsed"), archiveOpen: el.hasAttribute("data-archive"), doneLoaded: 2, colState: el.dataset.loading ? { needs_you: "loading", working: "loading", in_review: "loading", done: "loading" } : null });
      } else if (kind === "glyph") {
        el.innerHTML = glyph(el.dataset.state, el.hasAttribute("data-sm"));
      }
    });
    requestAnimationFrame(() => document.querySelectorAll("[data-callouts]").forEach(placeCallouts));
  }

  window.addEventListener("DOMContentLoaded", () => { renderSpecimens(); renderDemo(); });
})();
