/* subagents.js — demo plumbing shared by the boards in opendesign/subagents/.
   Exercises the real rules from _uiux.md:
   - StateGlyph marks drawn from [data-sg] (production geometry, 16-unit box);
   - one shared 1 s ticker that writes textContent of live elapsed nodes
     (no per-node timers; settled and stale nodes are frozen);
   - HoverCard: 200 ms hover delay, opens on focus, closes on leave/blur/Escape;
   - group disclosure + provider-native inline expansion (aria-expanded);
   - Stop flows (banner + roster rows) with "Stopping…" and the failure toast;
   - inspector paging "Show 12 more" and the Previous subagents collapsible. */
(function () {
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));

  /* ---------- StateGlyph ---------- */
  const R = 6.25, C = 2 * Math.PI * R;
  const MARK = {
    running: `<circle cx="8" cy="8" r="${R}" opacity=".22"/><circle cx="8" cy="8" r="${R}" stroke-dasharray="${C / 4} ${C}" stroke-linecap="round"/>`,
    queued: `<circle cx="8" cy="8" r="${R}" stroke-dasharray="2.2 2.2"/>`,
    delegated: `<circle cx="8" cy="8" r="${R}"/><circle cx="8" cy="8" r="2.25" fill="currentColor" stroke="none"/>`,
    done: `<circle cx="8" cy="8" r="7" fill="currentColor" stroke="none"/><path class="ck" d="M5 8.25l2 2 4-4.25" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"/>`,
    failed: `<circle cx="8" cy="8" r="${R}"/><path d="M6 6l4 4M10 6l-4 4" stroke-linecap="round"/>`,
    stopped: `<rect x="4.5" y="4.5" width="7" height="7" rx="1.5" fill="currentColor" stroke="none"/>`,
    attention: `<circle cx="8" cy="8" r="4" fill="currentColor" stroke="none"/>`,
    idle: `<circle cx="8" cy="8" r="4" fill="currentColor" stroke="none"/>`,
  };
  function drawGlyphs(root = document) {
    $$("[data-sg]", root).forEach((el) => {
      const state = el.dataset.sg;
      const spin = state === "running" && el.dataset.still === undefined;
      const svg = `<svg class="state-glyph" data-state="${state}" data-size="${el.dataset.size || "md"}"${spin ? ' data-spinning="true"' : ""}${el.dataset.pulse !== undefined ? ' data-pulse="true"' : ""} viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"${el.dataset.label ? ` role="img" aria-label="${el.dataset.label}"` : ' aria-hidden="true"'}>${MARK[state] || ""}</svg>`;
      el.outerHTML = svg;
    });
  }

  /* ---------- toast ---------- */
  const toastEl = document.createElement("div");
  toastEl.className = "sa-toast";
  toastEl.setAttribute("role", "status");
  toastEl.setAttribute("aria-live", "polite");
  document.body.appendChild(toastEl);
  let toastTimer;
  function toast(msg, danger) {
    toastEl.innerHTML = (danger ? '<svg data-lucide="circle-alert"></svg>' : "") + `<span>${msg}</span>`;
    if (window.lucide) window.lucide.createIcons({ nodes: [toastEl] });
    toastEl.classList.add("is-on");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => toastEl.classList.remove("is-on"), 2600);
  }

  /* ---------- elapsed: one shared ticker ---------- */
  function fmt(s) {
    s = Math.max(0, Math.floor(s));
    if (s < 60) return `${s}s`;
    const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), r = s % 60;
    if (h > 0) return `${h}h ${String(m).padStart(2, "0")}m`;
    return `${m}m ${String(r).padStart(2, "0")}s`;
  }
  const t0 = Date.now();
  function tick() {
    const now = Date.now();
    $$("[data-elapsed]").forEach((el) => {
      const base = Number(el.dataset.elapsed);
      const live = el.dataset.live === "true";
      el.textContent = fmt(live ? base + (now - t0) / 1000 : base);
    });
  }

  /* ---------- HoverCard ---------- */
  function wireHoverCards() {
    $$(".hc-host").forEach((host) => {
      const trigger = host.querySelector("[data-hc-trigger]");
      const card = host.querySelector(":scope > .hover-card");
      if (!trigger || !card) return;
      let openT, closeT;
      const open = () => { clearTimeout(closeT); card.classList.add("is-open"); trigger.classList.add("is-open"); };
      const close = () => { clearTimeout(openT); card.classList.remove("is-open"); trigger.classList.remove("is-open"); };
      host.addEventListener("mouseenter", () => { clearTimeout(closeT); openT = setTimeout(open, 200); });
      host.addEventListener("mouseleave", () => { clearTimeout(openT); closeT = setTimeout(close, 120); });
      trigger.addEventListener("focus", open);
      trigger.addEventListener("blur", () => { closeT = setTimeout(close, 80); });
      trigger.addEventListener("keydown", (e) => { if (e.key === "Escape") close(); });
    });
  }

  /* ---------- disclosures ---------- */
  document.addEventListener("click", (e) => {
    const g = e.target.closest("[data-sa-disclosure]");
    if (g) {
      const panel = document.getElementById(g.getAttribute("aria-controls"));
      const open = g.getAttribute("aria-expanded") !== "true";
      g.setAttribute("aria-expanded", String(open));
      if (panel) panel.hidden = !open;
      const grp = g.closest(".subagent-group");
      if (grp && grp.dataset.settled === "true") grp.dataset.dim = String(!open);
      return;
    }
    const nav = e.target.closest("[data-sa-open]");
    if (nav && !e.target.closest("[data-sa-stop]")) {
      toast(`${e.metaKey || e.ctrlKey ? "Opens in a new window" : "Switches this window to"} · ${nav.dataset.saOpen}`);
      return;
    }
    const stop = e.target.closest("[data-sa-stop]");
    if (stop) { e.stopPropagation(); runStop(stop); return; }
    const more = e.target.closest("[data-sa-more]");
    if (more) {
      const list = document.getElementById(more.dataset.saMore);
      const hidden = $$("[data-page-hidden]", list).slice(0, 12);
      hidden.forEach((r) => { r.hidden = false; r.removeAttribute("data-page-hidden"); });
      const left = $$("[data-page-hidden]", list).length;
      if (left === 0) more.remove(); else more.textContent = `Show ${Math.min(12, left)} more`;
      return;
    }
  });

  function runStop(btn) {
    const mode = btn.dataset.saStop; // "banner" | "row" | "row-fail" | "banner-fail"
    if (mode.startsWith("banner")) {
      const label = btn.querySelector("span") || btn;
      btn.disabled = true;
      label.textContent = "Stopping…";
      setTimeout(() => {
        if (mode === "banner-fail") {
          btn.disabled = false; label.textContent = "Stop";
          toast("Could not stop subagents.", true);
        } else {
          const banner = btn.closest(".subagent-waiting-banner");
          if (banner) banner.hidden = true;
          const restore = document.querySelector(`[data-sa-restore="${banner && banner.id}"]`);
          if (restore) restore.hidden = false;
        }
      }, 1300);
      return;
    }
    const row = btn.closest(".subagent-roster__row");
    row.dataset.stopping = "true";
    btn.setAttribute("aria-label", "Stopping subagent");
    btn.innerHTML = '<span data-sg="running" data-size="sm"></span>';
    drawGlyphs(btn);
    setTimeout(() => {
      row.removeAttribute("data-stopping");
      if (mode === "row-fail") {
        btn.innerHTML = '<svg data-lucide="square"></svg>';
        btn.setAttribute("aria-label", "Stop subagent");
        if (window.lucide) window.lucide.createIcons({ nodes: [btn] });
        toast("Could not stop subagent", true);
      } else {
        row.dataset.live = "false";
        const av = row.querySelector(".status-dot");
        if (av) av.className = "status-dot status-dot--faint";
        const end = row.querySelector(".subagent-roster__end");
        end.innerHTML = '<span class="subagent-roster__w">Canceled</span>';
      }
    }, 1300);
  }

  /* ---------- reset / restore demo controls ---------- */
  document.addEventListener("click", (e) => {
    const r = e.target.closest("[data-sa-reset]");
    if (!r) return;
    const banner = document.getElementById(r.dataset.saReset);
    banner.hidden = false;
    const b = banner.querySelector("[data-sa-stop]");
    b.disabled = false; (b.querySelector("span") || b).textContent = "Stop";
    r.closest("[data-sa-restore]").hidden = true;
  });

  drawGlyphs();
  if (window.lucide) window.lucide.createIcons();
  wireHoverCards();
  tick();
  setInterval(tick, 1000);
})();
