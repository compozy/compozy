/* automations.js — demo plumbing shared by the boards in opendesign/automations/.
   Exercises the real state machines: enable switch (PATCH {enabled}, no
   optimistic flip), Start views + search + Rows|Cards on the listing, run
   accordion (one open at a time), overflow menus, prompt clamp, copy, toast,
   and the StateGlyph marks on run rows ([data-sg]). */
(function () {
  const $ = (s, r = document) => r.querySelector(s);
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));

  const toastEl = document.createElement("div");
  toastEl.className = "au-toast";
  toastEl.setAttribute("role", "status");
  toastEl.setAttribute("aria-live", "polite");
  document.body.appendChild(toastEl);
  let toastTimer;
  function toast(msg) {
    toastEl.textContent = msg;
    toastEl.classList.add("is-on");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => toastEl.classList.remove("is-on"), 2200);
  }
  window.auToast = toast;

  // Enable switch: production AutomationEnableSwitch — label flips to the
  // present participle, the switch dims and only moves when the PATCH lands.
  $$("[data-au-enable]").forEach((btn) => {
    btn.addEventListener("click", (e) => {
      e.stopPropagation();
      if (btn.getAttribute("aria-busy") === "true") return;
      const on = btn.getAttribute("aria-checked") === "true";
      const lab = $(".au-enable__lab", btn);
      const name = btn.dataset.auEnable;
      btn.setAttribute("aria-busy", "true");
      if (lab) lab.textContent = on ? "Turning off…" : "Turning on…";
      setTimeout(() => {
        btn.setAttribute("aria-checked", String(!on));
        const sw = btn.classList.contains("switch") ? btn : $(".switch", btn);
        if (sw) sw.setAttribute("aria-checked", String(!on));
        btn.removeAttribute("aria-busy");
        if (lab) lab.textContent = !on ? "On" : "Off";
        const host = btn.closest("[data-au-entity]");
        if (host) host.classList.toggle("is-off", on);
        const pause = host && $(".au-lede__pause", host);
        if (pause) pause.hidden = !on;
        toast(`${!on ? "Turned on" : "Turned off"} ${name}.`);
      }, 520);
    });
  });

  // Overflow menus
  $$(".au-menu-host > [aria-haspopup]").forEach((t) => {
    t.addEventListener("click", (e) => {
      e.stopPropagation();
      const host = t.parentElement;
      const open = !host.classList.contains("is-open");
      $$(".au-menu-host.is-open").forEach((h) => h.classList.remove("is-open"));
      host.classList.toggle("is-open", open);
      t.setAttribute("aria-expanded", String(open));
    });
  });
  document.addEventListener("click", () => {
    $$(".au-menu-host.is-open").forEach((h) => {
      h.classList.remove("is-open");
      const t = $("[aria-haspopup]", h);
      if (t) t.setAttribute("aria-expanded", "false");
    });
  });
  $$("[data-toast]").forEach((b) =>
    b.addEventListener("click", (e) => {
      e.stopPropagation();
      toast(b.dataset.toast);
    })
  );

  // Run accordion: one open at a time per list
  $$(".au-run[aria-expanded]").forEach((row) => {
    row.addEventListener("click", () => {
      const list = row.closest(".au-runs");
      const open = row.getAttribute("aria-expanded") !== "true";
      $$(".au-run[aria-expanded]", list).forEach((r) => r.setAttribute("aria-expanded", "false"));
      row.setAttribute("aria-expanded", String(open));
    });
  });

  // Prompt clamp
  $$("[data-au-prompt-toggle]").forEach((b) => {
    b.addEventListener("click", () => {
      const p = document.getElementById(b.dataset.auPromptToggle);
      const open = p.dataset.open !== "true";
      p.dataset.open = String(open);
      b.textContent = open ? "Hide prompt" : "Show full prompt";
      b.setAttribute("aria-expanded", String(open));
    });
  });

  // Copy
  $$("[data-copy]").forEach((b) =>
    b.addEventListener("click", (e) => {
      e.stopPropagation();
      navigator.clipboard?.writeText(b.dataset.copy).catch(() => {});
      toast(`Copied ${b.dataset.copy}.`);
    })
  );

  // Listing: Start view + search + Rows|Cards (one window = one [data-au-listing])
  $$("[data-au-listing]").forEach((win) => {
    let view = "all";
    let q = "";
    const views = $$("[data-au-view]", win);
    const modes = $$("[data-au-mode]", win);
    const search = $("input[type=search]", win);
    const rows = $(".au-rows", win);
    const cards = $(".au-cards", win);
    const filteredEmpty = $(".au-filtered-empty", win);
    function apply() {
      let shown = 0;
      $$("[data-kind]", win).forEach((el) => {
        const kindOk = view === "all" || el.dataset.kind === view || (view === "event" && el.dataset.kind === "webhook");
        const qOk = !q || el.textContent.toLowerCase().includes(q);
        el.hidden = !(kindOk && qOk);
        if (!el.hidden && el.classList.contains("au-row")) shown++;
      });
      const mode = (rows && rows.dataset.mode) || "rows";
      if (rows) rows.hidden = mode !== "rows" || shown === 0;
      if (cards) cards.hidden = mode !== "cards" || shown === 0;
      if (filteredEmpty) filteredEmpty.hidden = shown !== 0;
    }
    views.forEach((v) =>
      v.addEventListener("click", () => {
        view = v.dataset.auView;
        views.forEach((x) => x.classList.toggle("is-on", x === v));
        views.forEach((x) => x.setAttribute("aria-pressed", String(x === v)));
        apply();
      })
    );
    modes.forEach((m) =>
      m.addEventListener("click", () => {
        const mode = m.dataset.auMode;
        modes.forEach((x) => {
          x.classList.toggle("is-on", x === m);
          x.setAttribute("aria-pressed", String(x === m));
        });
        if (rows) rows.dataset.mode = mode;
        apply();
      })
    );
    if (search)
      search.addEventListener("input", () => {
        q = search.value.trim().toLowerCase();
        apply();
      });
    $$("[data-au-clear]", win).forEach((b) =>
      b.addEventListener("click", () => {
        q = "";
        if (search) search.value = "";
        view = "all";
        views.forEach((x) => x.classList.toggle("is-on", x.dataset.auView === "all"));
        apply();
      })
    );
  });

  // Rows link to the detail board
  $$("[data-href]").forEach((el) =>
    el.addEventListener("click", (e) => {
      if (e.target.closest("button, a, [role=switch], .au-menu-host")) return;
      window.location.href = el.dataset.href;
    })
  );

  // StateGlyph — same geometry as packages/ui custom/state-glyph.tsx (16-unit box).
  const R = 6.25;
  const C = 2 * Math.PI * R;
  const SG = {
    running: `<circle cx="8" cy="8" r="${R}" opacity=".22"/><circle cx="8" cy="8" r="${R}" stroke-dasharray="${C / 4} ${C}" stroke-linecap="round"/>`,
    queued: `<circle cx="8" cy="8" r="${R}" stroke-dasharray="2.2 2.2"/>`,
    delegated: `<circle cx="8" cy="8" r="${R}"/><circle cx="8" cy="8" r="2.25" fill="currentColor" stroke="none"/>`,
    done: `<circle cx="8" cy="8" r="7" fill="currentColor" stroke="none"/><path d="M5 8.25l2 2 4-4.25" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"/>`,
    failed: `<circle cx="8" cy="8" r="${R}"/><path d="M6 6l4 4M10 6l-4 4" stroke-linecap="round"/>`,
    stopped: `<rect x="4.5" y="4.5" width="7" height="7" rx="1.5" fill="currentColor" stroke="none"/>`,
    idle: `<circle cx="8" cy="8" r="4" fill="currentColor" stroke="none"/>`,
  };
  $$("[data-sg]").forEach((el) => {
    el.classList.add("au-sg");
    el.setAttribute("aria-hidden", "true");
    el.innerHTML = `<svg viewBox="0 0 16 16">${SG[el.dataset.sg] || SG.idle}</svg>`;
  });

  if (window.lucide) window.lucide.createIcons();
})();
