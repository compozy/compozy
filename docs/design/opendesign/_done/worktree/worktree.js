/* Worktree side submenu — mirrors WorktreeHoverSubmenu (120ms close delay).
   Hover, focus, or ArrowRight opens the panel; pointer click on the workspace
   still selects. Static flies (data-wt-static) stay pinned for state galleries.
   The panel is a sibling of the host (popover or one overview card group) and
   aligns to that host — never to the whole overlay. */
(() => {
  const DELAY = 120;

  const place = (fly, host, sub) => {
    const flyBox = fly.getBoundingClientRect();
    const hostBox = host.getBoundingClientRect();
    sub.style.marginTop = `${Math.max(0, hostBox.top - flyBox.top)}px`;
  };

  document.querySelectorAll("[data-wt-fly]").forEach(fly => {
    const hosts = [...fly.querySelectorAll("[data-wt-host]")];
    const subs = [...fly.querySelectorAll("[data-wt-sub]")];
    const pinned = fly.hasAttribute("data-wt-static");
    let timer = 0;

    const open = id => {
      window.clearTimeout(timer);
      timer = 0;
      hosts.forEach(host => {
        host.setAttribute("aria-expanded", host.getAttribute("data-wt-host") === id ? "true" : "false");
      });
      subs.forEach(sub => {
        const on = sub.getAttribute("data-wt-sub") === id;
        sub.hidden = !on;
        if (on) {
          const host = hosts.find(h => h.getAttribute("data-wt-host") === id);
          if (host) place(fly, host, sub);
        }
      });
    };

    const close = () => {
      hosts.forEach(host => host.setAttribute("aria-expanded", "false"));
      subs.forEach(sub => {
        sub.hidden = true;
      });
    };

    const scheduleClose = () => {
      if (pinned) return;
      window.clearTimeout(timer);
      timer = window.setTimeout(close, DELAY);
    };

    const expanded = hosts.find(host => host.getAttribute("aria-expanded") === "true");
    if (expanded) open(expanded.getAttribute("data-wt-host"));
    else if (!pinned) close();

    if (pinned) return;

    fly.addEventListener("mouseenter", () => {
      window.clearTimeout(timer);
      timer = 0;
    });
    fly.addEventListener("mouseleave", scheduleClose);
    fly.addEventListener("focusout", event => {
      const next = event.relatedTarget;
      if (next instanceof Node && fly.contains(next)) return;
      scheduleClose();
    });

    hosts.forEach(host => {
      const id = host.getAttribute("data-wt-host");
      host.addEventListener("mouseenter", () => open(id));
      host.addEventListener("focus", () => open(id));
      host.addEventListener("keydown", event => {
        if (event.key === "ArrowRight") {
          event.preventDefault();
          open(id);
          const sub = subs.find(s => s.getAttribute("data-wt-sub") === id);
          const first = sub && sub.querySelector("button, [href], [tabindex]:not([tabindex='-1'])");
          if (first instanceof HTMLElement) first.focus();
        }
        if (event.key === "ArrowLeft" || event.key === "Escape") {
          event.preventDefault();
          close();
          host.focus();
        }
      });
    });
  });
})();
