/* agent-collaboration.js — demo plumbing for the boards in opendesign/agent-collaboration/.
   Loaded AFTER ../subagents/subagents.js, which already owns: StateGlyph drawing ([data-sg]),
   the HoverCard timing, the toast, and [data-sa-open] link clicks.
   This file adds the rules _uiux.md defines for S1–S4:
   - the operator bubble's clamp on session message bodies (176px + mask, Show more / Show less);
   - the sender flow: Sending… → Waiting for reply → the "Reply from" card lands and the sent
     card resolves its reply state by matching reply_watch_id ↔ synthetic.wake_event_id;
   - queue cancel: a session-message row leaves in one 180ms fade; when it carried a reply
     watch, the sender transcript receives a "Dropped" reply card. Edit / Steer never exist. */
(function () {
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
  const icons = (root) => { if (window.lucide) window.lucide.createIcons({ nodes: [root] }); };

  /* ---------- clamp ---------- */
  const CLAMP = 176, SLACK = 8;
  function wireClamp(root = document) {
    $$("[data-ac-clamp]", root).forEach((body) => {
      if (body.dataset.acWired) return;
      body.dataset.acWired = "1";
      const btn = body.parentElement.querySelector(".session-message__more");
      if (!btn) return;
      const clampable = body.scrollHeight > CLAMP + SLACK;
      btn.hidden = !clampable;
      if (!clampable) return;
      body.dataset.clamped = "true";
      btn.addEventListener("click", () => {
        const open = body.dataset.clamped === "true";
        body.dataset.clamped = open ? "false" : "true";
        btn.textContent = open ? "Show less" : "Show more";
        btn.setAttribute("aria-expanded", String(open));
      });
    });
  }

  /* ---------- sender flow (S3 → S2) ---------- */
  const STATES = {
    sending: '<span class="session-reply-state"><svg class="spin" data-lucide="loader-circle"></svg>Sending…</span>',
    waiting: '<span class="session-reply-state" data-tone="success"><span class="status-dot status-dot--success status-dot--pulse"></span>Waiting for reply</span>',
    replied: '<span class="session-reply-state" data-tone="success"><span class="status-dot status-dot--success"></span>Replied</span>',
  };
  function setFlow(flow, step) {
    const card = flow.querySelector(".session-sent-card");
    const slot = card.querySelector("[data-ac-reply-slot]");
    const title = card.querySelector("[data-ac-sent-verb]");
    const reply = flow.querySelector("[data-ac-reply]");
    const turn = flow.querySelector("[data-ac-after]");
    const btn = flow.querySelector("[data-ac-advance]");
    flow.dataset.step = step;
    card.dataset.state = step;
    title.textContent = step === "sending" ? "Sending to" : "Sent to";
    slot.innerHTML = STATES[step];
    reply.hidden = step !== "replied";
    if (turn) turn.hidden = step === "sending";
    btn.textContent = step === "sending" ? "Admit the send →" : step === "waiting" ? "Target turn ends →" : "Reset";
    $$(".ac-flow__step", flow).forEach((s) => { s.querySelector("b") && s.querySelector("b").remove(); });
    const label = flow.querySelector(`[data-ac-step="${step}"]`);
    if (label) label.insertAdjacentHTML("afterbegin", "<b>● </b>");
    icons(slot);
  }
  // Session links are real links in production (Enter / ⌘-click); here they only announce.
  // PR links open the forge URL externally — never the session navigation.
  document.addEventListener("click", (e) => {
    const a = e.target.closest("a[data-sa-open]");
    if (!a) return;
    e.preventDefault();
    if (!a.dataset.saOpen.startsWith("PR #")) return;
    e.stopPropagation();
    const t = document.querySelector(".sa-toast");
    if (!t) return;
    t.innerHTML = `<span>Opens ${a.dataset.saOpen.split(" ·")[0]} in your browser</span>`;
    t.classList.add("is-on");
    clearTimeout(t._acT);
    t._acT = setTimeout(() => t.classList.remove("is-on"), 2200);
  }, true);
  document.addEventListener("click", (e) => {
    const adv = e.target.closest("[data-ac-advance]");
    if (adv) {
      const flow = adv.closest("[data-ac-flow]");
      const next = { sending: "waiting", waiting: "replied", replied: "sending" }[flow.dataset.step];
      setFlow(flow, next);
      return;
    }
    const rm = e.target.closest("[data-ac-remove]");
    if (rm) { removeQueueRow(rm); return; }
    const reset = e.target.closest("[data-ac-queue-reset]");
    if (reset) { location.reload(); }
  });

  /* ---------- queue cancel (S4) ---------- */
  function removeQueueRow(btn) {
    const row = btn.closest(".session-queue-row");
    const strip = row.closest(".session-queue-strip");
    row.classList.add("is-leaving");
    setTimeout(() => {
      row.remove();
      const rows = $$(".session-queue-row", strip);
      rows.forEach((r, i) => { const p = r.querySelector(".session-queue-row__pos"); if (p) p.textContent = `#${i + 1}`; });
      const n = strip.querySelector(".session-queue-strip__head .n");
      if (n) n.textContent = String(rows.length);
      if (rows.length === 0) strip.hidden = true;
      const watch = row.dataset.acWatch;
      if (watch) {
        const target = document.getElementById(watch);
        const tpl = document.getElementById(`${watch}-tpl`);
        if (target && tpl) {
          target.innerHTML = tpl.innerHTML;
          icons(target);
        }
      }
    }, 180);
  }

  $$("[data-ac-flow]").forEach((f) => setFlow(f, f.dataset.step || "sending"));
  wireClamp();
})();
