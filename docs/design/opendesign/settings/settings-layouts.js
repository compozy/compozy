/* AGH · Settings › Layouts — visual window-management editor
   Every control below maps to a field that exists in the daemon contract:
   - WindowManagerConfig      (global, PATCH /api/settings/window-manager)
   - WindowManagerLayoutDoc   (workspace, validate → preview → PUT …/layout)
   - WindowManagerLayoutProfile (resource kind "window_layout")
   No invented fields. See LAYOUTS-REDESIGN-SPEC.md for the field-by-field map. */
(function () {
  "use strict";

  /* ════════════════════════ helpers ════════════════════════ */
  function el(tag, cls, txt) { var e = document.createElement(tag); if (cls) e.className = cls; if (txt != null) e.textContent = txt; return e; }
  function frag(html) { var d = document.createElement("div"); d.innerHTML = html.trim(); return d.firstElementChild; }
  function clone(v) { return JSON.parse(JSON.stringify(v)); }
  function clamp(v, lo, hi) { return v < lo ? lo : v > hi ? hi : v; }
  function q(id) { return document.getElementById(id); }
  var REDUCED = window.matchMedia("(prefers-reduced-motion:reduce)").matches;

  /* one pointer-drag primitive for every draggable affordance on this page */
  function draggable(node, spec) {
    node.addEventListener("pointerdown", function (e) {
      if (e.button !== 0) return;
      e.preventDefault(); e.stopPropagation();
      node.setPointerCapture(e.pointerId);
      node.setAttribute("data-drag", "true");
      var st = spec.start ? spec.start(e) : {};
      function mv(ev) { spec.move(ev, st); }
      function up(ev) {
        node.removeAttribute("data-drag");
        node.removeEventListener("pointermove", mv);
        node.removeEventListener("pointerup", up);
        node.removeEventListener("pointercancel", up);
        if (spec.end) spec.end(ev, st);
      }
      node.addEventListener("pointermove", mv);
      node.addEventListener("pointerup", up);
      node.addEventListener("pointercancel", up);
    });
  }

  /* magnetic detents — a ratio is a position, so snap it to positions people mean */
  var DETENTS = [0.25, 1 / 3, 0.5, 2 / 3, 0.75];
  function detent(v, tol) {
    for (var i = 0; i < DETENTS.length; i++) if (Math.abs(v - DETENTS[i]) < (tol || 0.018)) return DETENTS[i];
    return null;
  }
  function fracLabel(v) {
    var names = [[0.25, "1/4"], [1 / 3, "1/3"], [0.5, "1/2"], [2 / 3, "2/3"], [0.75, "3/4"]];
    for (var i = 0; i < names.length; i++) if (Math.abs(v - names[i][0]) < 0.004) return names[i][1];
    return Math.round(v * 100) + "%";
  }

  /* ════════════════════════ app glyphs (OS app registry) ════════════════════════ */
  var ICONS = {
    dashboard: "M3 3h7v7H3zM14 3h7v7h-7zM14 14h7v7h-7zM3 14h7v7H3z",
    tasks: "m3 17 2 2 4-4M3 7l2 2 4-4M13 6h8M13 12h8M13 18h8",
    session: "M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z",
    network: "M18 8a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM6 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM18 22a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM8.6 13.5l6.8 4M15.4 6.5l-6.8 4",
    vault: "M5 11h14v10H5zM8 11V7a4 4 0 0 1 8 0v4",
    loops: "m17 2 4 4-4 4M3 11V9a4 4 0 0 1 4-4h14M7 22l-4-4 4-4M21 13v2a4 4 0 0 1-4 4H3",
    marketplace: "M3 9h18l-1.5-5h-15zM4 9v11h16V9M9 20v-6h6v6",
    agents: "M12 8V4M8 8h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2v-8a2 2 0 0 1 2-2zM9 14h.01M15 14h.01",
    knowledge: "M4 19.5A2.5 2.5 0 0 1 6.5 17H20M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z",
    jobs: "M4 7h16v13H4zM9 7V5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2M4 13h16"
  };
  function appIcon(app, cls) {
    return '<svg class="' + (cls || "") + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="' + (ICONS[app] || ICONS.dashboard) + '"/></svg>';
  }
  var CHEV = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>';
  var CHECK = '<svg class="chk" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="m20 6-11 11-5-5"/></svg>';

  /* ════════════════════════ state — global window-manager config ════════════════════════ */
  var CFG = {
    newWindowPolicy: "floating", smallViewportPolicy: "stack", focusPolicy: "click_directional",
    focusWrap: false, focusFollowsPointer: false, raiseOnFocus: true,
    dragAwayPolicy: "window", groupMoveModifier: "alt", swapModifier: "shift",
    historyLimit: 50, desktopTransition: "slide",
    gaps: { inner: 8, top: 8, right: 10, bottom: 8, left: 10 },
    snap: { edgeBand: 32, cornerReach: 150, exitSlack: 16, repeatRatios: [0.5, 0.666667, 0.333333] },
    bindings: { topCenter: "zoom", bottomCenter: "reserved" },
    shortcuts: {}          /* only overrides live here — defaults are shipped constants */
  };
  var CFG0 = clone(CFG);

  /* ════════════════════════ state — workspace layout document ════════════════════════ */
  var WINDOWS = {
    "win-tasks": { id: "win-tasks", app: "tasks", title: "Tasks", route: "/tasks", desktopId: "d-build", placement: "tiled", minimized: false, floatingRect: { x: .2, y: .2, w: .4, h: .4 } },
    "win-session-a": { id: "win-session-a", app: "session", title: "Session · ses_8f2a", route: "/sessions/ses_8f2a", desktopId: "d-build", placement: "stacked", minimized: false, floatingRect: { x: .25, y: .25, w: .4, h: .4 } },
    "win-session-b": { id: "win-session-b", app: "session", title: "Session · ses_51c7", route: "/sessions/ses_51c7", desktopId: "d-build", placement: "stacked", minimized: false, floatingRect: { x: .3, y: .3, w: .4, h: .4 } },
    "win-dashboard": { id: "win-dashboard", app: "dashboard", title: "Dashboard", route: "/", desktopId: "d-build", placement: "tiled", minimized: false, floatingRect: { x: .3, y: .3, w: .4, h: .4 } },
    "win-network": { id: "win-network", app: "network", title: "Network", route: "/network", desktopId: "d-build", placement: "tiled", minimized: false, floatingRect: { x: .35, y: .35, w: .4, h: .4 } },
    "win-vault": { id: "win-vault", app: "vault", title: "Vault", route: "/vault", desktopId: "d-build", placement: "floating", minimized: false, floatingRect: { x: .52, y: .55, w: .3, h: .34 } },
    "win-loops": { id: "win-loops", app: "loops", title: "Loop · nightly-triage", route: "/loops/lp_ntr", desktopId: "d-review", placement: "tiled", minimized: false, floatingRect: { x: .2, y: .2, w: .4, h: .4 } },
    "win-market": { id: "win-market", app: "marketplace", title: "Marketplace", route: "/marketplace", desktopId: "d-review", placement: "tiled", minimized: false, floatingRect: { x: .25, y: .25, w: .4, h: .4 } },
    "win-jobs": { id: "win-jobs", app: "jobs", title: "Jobs", route: "/jobs", desktopId: "d-review", placement: "tiled", minimized: false, floatingRect: { x: .3, y: .3, w: .4, h: .4 } },
    "win-agents": { id: "win-agents", app: "agents", title: "Agent · planner", route: "/agents/planner", desktopId: "d-review", placement: "floating", minimized: true, floatingRect: { x: .58, y: .62, w: .28, h: .3 } },
    "win-knowledge": { id: "win-knowledge", app: "knowledge", title: "Knowledge", route: "/knowledge", desktopId: "d-focus", placement: "tiled", minimized: false, floatingRect: { x: .2, y: .2, w: .4, h: .4 } }
  };

  var DOC = {
    version: 2, workspaceId: "ws-compozy-agh", revision: 41,
    desktops: [
      {
        id: "d-build", name: "Build", order: 0, purpose: "standard", focusOwner: null,
        groups: [
          {
            id: "grp-main", frame: { x: 0, y: 0, w: 0.63, h: 1 },
            root: {
              id: "split:a1", kind: "split", axis: "vertical", weights: [0.62, 0.38], children: [
                { id: "leaf:a2", kind: "leaf", windowId: "win-tasks" },
                {
                  id: "split:a3", kind: "split", axis: "horizontal", weights: [0.5, 0.5], children: [
                    { id: "stack:a4", kind: "stack", windowIds: ["win-session-a", "win-session-b"], activeId: "win-session-a" },
                    { id: "leaf:a5", kind: "leaf", windowId: "win-dashboard" }
                  ]
                }
              ]
            }
          },
          { id: "grp-side", frame: { x: 0.63, y: 0, w: 0.37, h: 1 }, root: { id: "leaf:a6", kind: "leaf", windowId: "win-network" } }
        ],
        floating: ["win-vault"]
      },
      {
        id: "d-review", name: "Review", order: 1, purpose: "standard", focusOwner: null,
        groups: [
          {
            id: "grp-top", frame: { x: 0, y: 0, w: 1, h: 0.56 },
            root: {
              id: "split:b1", kind: "split", axis: "horizontal", weights: [0.5, 0.5], children: [
                { id: "leaf:b2", kind: "leaf", windowId: "win-loops" },
                { id: "leaf:b3", kind: "leaf", windowId: "win-market" }
              ]
            }
          },
          { id: "grp-bottom", frame: { x: 0, y: 0.56, w: 1, h: 0.44 }, root: { id: "leaf:b4", kind: "leaf", windowId: "win-jobs" } }
        ],
        floating: ["win-agents"]
      },
      {
        id: "d-focus", name: "Deep work", order: 2, purpose: "focus", focusOwner: "win-knowledge",
        groups: [{ id: "grp-focus", frame: { x: 0, y: 0, w: 1, h: 1 }, root: { id: "leaf:c1", kind: "leaf", windowId: "win-knowledge" } }],
        floating: []
      }
    ]
  };
  var DOC0 = clone(DOC);
  var WIN0 = clone(WINDOWS);
  var activeDesktop = 0;
  var SEL = null;                 /* {kind:"node"|"group"|"window", id} */
  var reviewedPrint = null, reviewState = "clean", lastPreview = null, lastDiags = [];
  var nodeSeq = 0;
  var VIEWPORT = { w: 1440, h: 900 };

  /* ════════════════════════ layout tree helpers (mirror of window-manager-layout-tree.ts) ════════════════════════ */
  function nodeWindowIds(n) {
    if (n.kind === "leaf") return [n.windowId];
    if (n.kind === "stack") return n.windowIds.slice();
    return n.children.reduce(function (a, c) { return a.concat(nodeWindowIds(c)); }, []);
  }
  function desktopWindowIds(d) {
    return Object.keys(WINDOWS).filter(function (k) { return WINDOWS[k].desktopId === d.id; });
  }
  function structuralIds(d) {
    return d.groups.reduce(function (a, g) { return a.concat(nodeWindowIds(g.root)); }, []);
  }
  function walk(n, fn, parent, idx) {
    fn(n, parent, idx);
    if (n.kind === "split") n.children.forEach(function (c, i) { walk(c, fn, n, i); });
  }
  function findNode(id) {
    var d = DOC.desktops[activeDesktop], hit = null;
    d.groups.forEach(function (g, gi) {
      walk(g.root, function (n, parent, idx) { if (n.id === id) hit = { node: n, parent: parent, index: idx, gi: gi, group: g }; });
    });
    return hit;
  }
  function replaceNode(id, next) {
    var f = findNode(id); if (!f) return;
    if (f.parent) f.parent.children[f.index] = next; else f.group.root = next;
  }
  function nid(kind) { nodeSeq += 1; return kind + ":ui-" + nodeSeq; }

  /* structure conversions — weights are emitted normalized so the daemon's
     sum-to-1 invariant (topology.split_weight_sum) can never be violated */
  function toSplit(node, axis) {
    var ids = nodeWindowIds(node);
    var w = ids.map(function () { return 1 / ids.length; });
    return { id: nid("split"), kind: "split", axis: axis, weights: w, children: ids.map(function (id) { return { id: nid("leaf"), kind: "leaf", windowId: id }; }) };
  }
  function toStack(node) {
    var ids = nodeWindowIds(node);
    return { id: nid("stack"), kind: "stack", windowIds: ids, activeId: ids[0] };
  }
  function evenWeights(node) { return node.children.map(function () { return 1 / node.children.length; }); }

  /* ════════════════════════ canvas ════════════════════════ */
  var canvas = q("canvas"), canvasQueued = false;
  function scheduleCanvas() {
    if (canvasQueued) return;
    canvasQueued = true;
    requestAnimationFrame(function () { canvasQueued = false; renderCanvas(); });
  }

  function rectsOverlap(a, b) {
    return a.x < b.x + b.w - 1e-6 && b.x < a.x + a.w - 1e-6 && a.y < b.y + b.h - 1e-6 && b.y < a.y + a.h - 1e-6;
  }
  function overlappingGroups(d) {
    var bad = {};
    for (var i = 0; i < d.groups.length; i++)
      for (var j = i + 1; j < d.groups.length; j++)
        if (rectsOverlap(d.groups[i].frame, d.groups[j].frame)) { bad[d.groups[i].id] = 1; bad[d.groups[j].id] = 1; }
    return bad;
  }

  function renderCanvas() {
    var d = DOC.desktops[activeDesktop];
    canvas.innerHTML = "";
    var cw = canvas.clientWidth || 640;
    var s = cw / VIEWPORT.w;
    var g = CFG.gaps, gap = Math.max(1, CFG.gaps.inner * s);
    var bad = overlappingGroups(d);

    /* Rectangle's gap model: a shared edge gets half a gap from each side, an
       outer edge gets the full inset. Pulling half the inner gap out of the work
       area and giving it back as group inset makes both exact. */
    var half = gap / 2;
    var work = el("div", "canvas__work");
    work.style.top = Math.max(0, g.top * s - half) + "px"; work.style.right = Math.max(0, g.right * s - half) + "px";
    work.style.bottom = Math.max(0, g.bottom * s - half) + "px"; work.style.left = Math.max(0, g.left * s - half) + "px";
    canvas.appendChild(work);

    if (!d.groups.length && !d.floating.length) {
      var empty = el("div", "canvas__empty");
      empty.appendChild(el("div", null, "No windows on this desktop"));
      empty.appendChild(el("div", null, "Open a window in the workspace to start tiling."));
      canvas.appendChild(empty); return;
    }

    d.groups.forEach(function (grp) {
      var ge = el("div", "group");
      ge.dataset.gid = grp.id;
      if (bad[grp.id]) ge.dataset.overlap = "true";
      if (SEL && SEL.kind === "group" && SEL.id === grp.id) ge.dataset.sel = "true";
      ge.style.left = (grp.frame.x * 100) + "%"; ge.style.top = (grp.frame.y * 100) + "%";
      ge.style.width = (grp.frame.w * 100) + "%"; ge.style.height = (grp.frame.h * 100) + "%";

      var root = renderNode(grp.root, gap);
      root.style.position = "absolute"; root.style.inset = half + "px";
      ge.appendChild(root);

      ["n", "s", "w", "e"].forEach(function (edge) {
        var h = el("div", "group__handle"); h.dataset.edge = edge;
        h.setAttribute("role", "separator");
        h.setAttribute("aria-label", "Resize group " + grp.id + " " + edge + " edge");
        bindGroupEdge(h, grp, edge);
        ge.appendChild(h);
      });
      work.appendChild(ge);
    });

    d.floating.forEach(function (wid) {
      var w = WINDOWS[wid]; if (!w) return;
      var fe = el("div", "float");
      fe.dataset.wid = wid;
      if (w.minimized) fe.dataset.min = "true";
      if (SEL && SEL.kind === "window" && SEL.id === wid) fe.dataset.sel = "true";
      fe.style.left = (w.floatingRect.x * 100) + "%"; fe.style.top = (w.floatingRect.y * 100) + "%";
      fe.style.width = (w.floatingRect.w * 100) + "%"; fe.style.height = (w.floatingRect.h * 100) + "%";
      fe.appendChild(frag('<div class="tile__bar">' + appIcon(w.app) + '<span class="tile__name">' + w.title + '</span></div>'));
      fe.appendChild(frag('<div class="tile__body"><span class="tile__route">' + (w.minimized ? "minimized" : w.route) + '</span></div>'));
      fe.addEventListener("click", function (e) { e.stopPropagation(); select("window", wid); });
      bindFloatDrag(fe, w);
      canvas.appendChild(fe);
    });

    canvas.onclick = function () { select(null, null); };
    renderBoardbar();
  }

  function renderNode(n, gap) {
    if (n.kind === "split") {
      var box = el("div", "nd nd--split");
      box.dataset.axis = n.axis;
      n.children.forEach(function (c, i) {
        if (i > 0) {
          var seam = el("div", "seam");
          seam.style.flex = "0 0 " + gap + "px";
          seam.appendChild(el("div", "seam__tip"));
          bindSeam(seam, n, i - 1);
          box.appendChild(seam);
        }
        var ce = renderNode(c, gap);
        ce.style.flex = n.weights[i] + " 1 0px";
        box.appendChild(ce);
      });
      return box;
    }
    var t = el("div", "tile");
    t.dataset.kind = n.kind;
    t.dataset.nid = n.id;
    if (SEL && SEL.kind === "node" && SEL.id === n.id) t.dataset.sel = "true";
    if (n.kind === "stack") {
      var tabs = el("div", "tile__tabs");
      n.windowIds.forEach(function (wid) {
        var w = WINDOWS[wid]; if (!w) return;
        var tab = el("div", "tile__tab", w.title);
        if (wid === n.activeId) tab.dataset.active = "true";
        tabs.appendChild(tab);
      });
      t.appendChild(tabs);
      var aw = WINDOWS[n.activeId];
      if (aw) {
        t.appendChild(frag('<div class="tile__bar">' + appIcon(aw.app) + '<span class="tile__name">' + aw.title + '</span></div>'));
        t.appendChild(frag('<div class="tile__body"><span class="tile__route">' + aw.route + '</span></div>'));
      }
    } else {
      var lw = WINDOWS[n.windowId];
      t.appendChild(frag('<div class="tile__bar">' + appIcon(lw ? lw.app : "dashboard") + '<span class="tile__name">' + (lw ? lw.title : n.windowId) + '</span></div>'));
      t.appendChild(frag('<div class="tile__body"><span class="tile__route">' + (lw ? lw.route : "unresolved window") + '</span></div>'));
    }
    t.addEventListener("click", function (e) { e.stopPropagation(); select("node", n.id); });
    return t;
  }

  /* seam drag → split weights. Sum stays exactly 1 by construction, so the
     daemon's topology.split_weight_sum rule can never be tripped from here. */
  function bindSeam(seam, split, i) {
    var tip = seam.querySelector(".seam__tip");
    draggable(seam, {
      start: function () {
        var box = seam.parentElement.getBoundingClientRect();
        var horiz = split.axis === "horizontal";
        var seams = split.children.length - 1;
        var gap = Math.max(1, CFG.gaps.inner * ((canvas.clientWidth || 640) / VIEWPORT.w));
        return { horiz: horiz, avail: (horiz ? box.width : box.height) - seams * gap };
      },
      move: function (ev, st) {
        if (!st.avail) return;
        var dw = (st.horiz ? ev.movementX : ev.movementY) / st.avail;
        var w = split.weights.slice(), lo = 0.08;
        var a = clamp(w[i] + dw, lo, w[i] + w[i + 1] - lo);
        w[i + 1] = w[i] + w[i + 1] - a;
        w[i] = a;
        /* snap the boundary inside the whole split, not just the adjacent pair */
        var cum = 0; for (var k = 0; k <= i; k++) cum += w[k];
        var snapped = detent(cum), isSnap = false;
        if (snapped !== null) {
          var shift = snapped - cum;
          if (w[i] + shift > lo && w[i + 1] - shift > lo) { w[i] += shift; w[i + 1] -= shift; isSnap = true; }
        }
        split.weights = w;
        tip.textContent = fracLabel(w[i]) + " · " + fracLabel(w[i + 1]);
        tip.dataset.snap = isSnap ? "true" : "false";
        /* repaint only the two affected children — a full re-render here would
           delete the seam we are dragging and drop the pointer capture */
        var kids = Array.prototype.filter.call(seam.parentElement.children, function (c) { return !c.classList.contains("seam"); });
        kids[i].style.flex = w[i] + " 1 0px";
        kids[i + 1].style.flex = w[i + 1] + " 1 0px";
        markDirty();
      },
      end: function () { renderInspector(); }
    });
    seam.addEventListener("click", function (e) { e.stopPropagation(); });
  }

  /* group edge drag → normalized frame, clamped to the daemon's rect rules */
  function bindGroupEdge(h, grp, edge) {
    draggable(h, {
      start: function () {
        return { wk: canvas.querySelector(".canvas__work").getBoundingClientRect(), f: clone(grp.frame), moved: false };
      },
      move: function (ev, st) {
        var f = grp.frame, o = st.f, MIN = 0.08, v;
        st.moved = true;
        if (edge === "w") {
          v = clamp((ev.clientX - st.wk.left) / st.wk.width, 0, o.x + o.w - MIN);
          v = detentEdge(v) !== null ? detentEdge(v) : v;
          f.w = o.x + o.w - v; f.x = v;
        } else if (edge === "e") {
          v = clamp((ev.clientX - st.wk.left) / st.wk.width, o.x + MIN, 1);
          v = detentEdge(v) !== null ? detentEdge(v) : v;
          f.w = v - o.x;
        } else if (edge === "n") {
          v = clamp((ev.clientY - st.wk.top) / st.wk.height, 0, o.y + o.h - MIN);
          v = detentEdge(v) !== null ? detentEdge(v) : v;
          f.h = o.y + o.h - v; f.y = v;
        } else {
          v = clamp((ev.clientY - st.wk.top) / st.wk.height, o.y + MIN, 1);
          v = detentEdge(v) !== null ? detentEdge(v) : v;
          f.h = v - o.y;
        }
        /* move the element in place — re-rendering would unmount this handle */
        var ge = h.parentElement;
        ge.style.left = (f.x * 100) + "%"; ge.style.top = (f.y * 100) + "%";
        ge.style.width = (f.w * 100) + "%"; ge.style.height = (f.h * 100) + "%";
        var bad = overlappingGroups(DOC.desktops[activeDesktop]);
        Array.prototype.forEach.call(canvas.querySelectorAll(".group"), function (g) {
          if (bad[g.dataset.gid]) g.dataset.overlap = "true"; else g.removeAttribute("data-overlap");
        });
        markDirty();
      },
      end: function () { select("group", grp.id); }
    });
    h.addEventListener("click", function (e) { e.stopPropagation(); });
  }
  function detentEdge(v) {
    if (Math.abs(v) < 0.014) return 0;
    if (Math.abs(v - 1) < 0.014) return 1;
    return detent(v, 0.014);
  }

  function bindFloatDrag(fe, w) {
    draggable(fe, {
      start: function () { return { c: canvas.getBoundingClientRect(), r: clone(w.floatingRect) }; },
      move: function (ev, st) {
        var dx = (ev.clientX - (st.c.left + (st.r.x + st.r.w / 2) * st.c.width)) / st.c.width;
        var dy = (ev.clientY - (st.c.top + (st.r.y + st.r.h / 2) * st.c.height)) / st.c.height;
        w.floatingRect.x = clamp(st.r.x + dx, 0, 1 - st.r.w);
        w.floatingRect.y = clamp(st.r.y + dy, 0, 1 - st.r.h);
        fe.style.left = (w.floatingRect.x * 100) + "%";
        fe.style.top = (w.floatingRect.y * 100) + "%";
        markDirty();
      },
      end: function () { select("window", w.id); }
    });
  }

  function renderBoardbar() {
    var d = DOC.desktops[activeDesktop];
    var bar = q("boardbar");
    var tiled = structuralIds(d).length;
    var floats = d.floating.filter(function (id) { return WINDOWS[id] && !WINDOWS[id].minimized; }).length;
    var mins = d.floating.filter(function (id) { return WINDOWS[id] && WINDOWS[id].minimized; }).length;
    bar.innerHTML = "";
    bar.appendChild(frag('<span class="legend"><i data-t="tiled"></i>' + tiled + ' tiled</span>'));
    bar.appendChild(frag('<span class="legend"><i data-t="float"></i>' + floats + ' floating</span>'));
    if (mins) bar.appendChild(frag('<span class="legend"><i data-t="min"></i>' + mins + ' minimized</span>'));
    bar.appendChild(el("span", "boardbar__spacer"));
    if (d.purpose === "focus" && d.focusOwner) {
      var fw = WINDOWS[d.focusOwner];
      bar.appendChild(frag('<span class="pill" data-tone="info">Focus desktop · ' + (fw ? fw.title : d.focusOwner) + '</span>'));
    }
    bar.appendChild(frag('<span class="mono">' + VIEWPORT.w + '×' + VIEWPORT.h + ' reference</span>'));
  }

  /* ════════════════════════ selection + inspector ════════════════════════ */
  function select(kind, id) { SEL = kind ? { kind: kind, id: id } : null; renderCanvas(); renderInspector(); }

  var insp = q("inspector");
  function renderInspector() {
    var d = DOC.desktops[activeDesktop];
    insp.innerHTML = "";
    var head = el("div", "insp__head"), body = el("div", "insp__body");
    insp.appendChild(head); insp.appendChild(body);

    if (!SEL) {
      head.appendChild(frag('<div class="insp__kind">' + d.name + '</div>'));
      head.appendChild(frag('<div class="insp__id">' + d.id + ' · order ' + d.order + '</div>'));
      var b0 = el("div", "insp__block");
      b0.appendChild(frag('<span class="eyebrow">Nothing selected</span>'));
      b0.appendChild(frag('<p class="insp__hint">Click a window to change what it holds or how it splits. Drag a seam to rebalance, or a group edge to repartition the desktop.</p>'));
      body.appendChild(b0);
      var kv = el("div", "insp__block");
      kv.appendChild(frag('<span class="eyebrow">Desktop</span>'));
      var k = el("div", "kv");
      k.appendChild(kvRow("Groups", String(d.groups.length)));
      k.appendChild(kvRow("Structural windows", String(structuralIds(d).length)));
      k.appendChild(kvRow("Floating", String(d.floating.length)));
      k.appendChild(kvRow("Purpose", d.purpose === "focus" ? "Focus" : "Standard"));
      kv.appendChild(k); body.appendChild(kv);
      return;
    }

    if (SEL.kind === "group") {
      var grp = null; d.groups.forEach(function (g) { if (g.id === SEL.id) grp = g; });
      if (!grp) { SEL = null; return renderInspector(); }
      head.appendChild(frag('<div class="insp__kind">Group</div>'));
      head.appendChild(frag('<div class="insp__id">' + grp.id + '</div>'));
      var gb = el("div", "insp__block");
      gb.appendChild(frag('<span class="eyebrow">Share of the desktop</span>'));
      var gk = el("div", "kv");
      gk.appendChild(kvRow("Left", fracLabel(grp.frame.x)));
      gk.appendChild(kvRow("Top", fracLabel(grp.frame.y)));
      gk.appendChild(kvRow("Width", fracLabel(grp.frame.w)));
      gk.appendChild(kvRow("Height", fracLabel(grp.frame.h)));
      gb.appendChild(gk); body.appendChild(gb);
      if (overlappingGroups(d)[grp.id]) {
        body.appendChild(frag('<p class="insp__hint" style="color:var(--danger)">This group overlaps another one. The daemon rejects overlapping groups (topology.group_overlap).</p>'));
      }
      return;
    }

    if (SEL.kind === "window") {
      var w = WINDOWS[SEL.id];
      head.appendChild(frag('<div class="insp__kind">' + appIcon(w.app) + w.title + '</div>'));
      head.appendChild(frag('<div class="insp__id">' + w.id + ' · ' + w.route + '</div>'));
      var wb = el("div", "insp__block");
      wb.appendChild(frag('<span class="eyebrow">Floating window</span>'));
      var wk = el("div", "kv");
      wk.appendChild(kvRow("Placement", w.placement));
      wk.appendChild(kvRow("Minimized", w.minimized ? "yes" : "no"));
      wk.appendChild(kvRow("Size", fracLabel(w.floatingRect.w) + " × " + fracLabel(w.floatingRect.h)));
      wb.appendChild(wk); body.appendChild(wb);
      body.appendChild(frag('<p class="insp__hint">Drag it on the canvas to reposition. Floating windows sit above every group on this desktop.</p>'));
      return;
    }

    /* node */
    var f = findNode(SEL.id);
    if (!f) { SEL = null; return renderInspector(); }
    var n = f.node, ids = nodeWindowIds(n), canStructure = ids.length >= 2;
    var kindLabel = n.kind === "split" ? (n.axis === "horizontal" ? "Columns" : "Rows") : n.kind === "stack" ? "Stack" : "Window";
    head.appendChild(frag('<div class="insp__kind">' + kindLabel + '</div>'));
    head.appendChild(frag('<div class="insp__id">' + n.id + '</div>'));

    var ab = el("div", "insp__block");
    ab.appendChild(frag('<span class="eyebrow">Arrange as</span>'));
    var row = el("div", "actrow");
    row.appendChild(structBtn("Rows", ROWS_SVG, canStructure, n.kind === "split" && n.axis === "vertical", function () {
      replaceNode(n.id, toSplit(n, "vertical")); afterStructure();
    }));
    row.appendChild(structBtn("Columns", COLS_SVG, canStructure, n.kind === "split" && n.axis === "horizontal", function () {
      replaceNode(n.id, toSplit(n, "horizontal")); afterStructure();
    }));
    row.appendChild(structBtn("Stack", STACK_SVG, canStructure, n.kind === "stack", function () {
      replaceNode(n.id, toStack(n)); afterStructure();
    }));
    ab.appendChild(row);
    if (!canStructure) ab.appendChild(frag('<p class="insp__hint" style="margin-top:7px">A single window has nothing to split. Add a second window to this branch first.</p>'));
    body.appendChild(ab);

    if (n.kind === "split") {
      var sb = el("div", "insp__block");
      sb.appendChild(frag('<span class="eyebrow">Balance</span>'));
      var sk = el("div", "kv");
      n.weights.forEach(function (w, i) {
        var r = el("div", "kv__r");
        r.appendChild(el("span", null, (n.axis === "horizontal" ? "Column " : "Row ") + (i + 1)));
        var bar = el("div", "wbar"); bar.style.flex = "1"; bar.style.maxWidth = "96px";
        var t = el("div", "wbar__t"), fl = el("div", "wbar__f"); fl.style.width = (w * 100) + "%";
        t.appendChild(fl); bar.appendChild(t);
        var b = el("b", null, fracLabel(w));
        r.appendChild(bar); r.appendChild(b);
        sk.appendChild(r);
      });
      sb.appendChild(sk);
      var evenBtn = el("button", "btn btn--xs", "Distribute evenly");
      evenBtn.style.marginTop = "8px";
      evenBtn.addEventListener("click", function () { n.weights = evenWeights(n); renderCanvas(); renderInspector(); markDirty(); });
      sb.appendChild(evenBtn);
      body.appendChild(sb);
    }

    if (n.kind === "leaf") {
      var lb = el("div", "insp__block");
      lb.appendChild(frag('<span class="eyebrow">Shows</span>'));
      var used = {};
      d.groups.forEach(function (g) {
        walk(g.root, function (m) { if (m.id !== n.id) nodeWindowIds(m).forEach(function (id) { if (m.kind !== "split") used[id] = m.id; }); });
      });
      var list = el("div", "winlist");
      desktopWindowIds(d).forEach(function (wid) {
        var w = WINDOWS[wid];
        var taken = used[wid] && used[wid] !== n.id;
        var b = el("button", "winopt");
        b.setAttribute("aria-selected", String(wid === n.windowId));
        b.innerHTML = appIcon(w.app) + '<span>' + w.title + '</span>' + (wid === n.windowId ? CHECK : '<span class="r">' + (taken ? "in use" : w.route) + '</span>');
        if (taken) { b.disabled = true; b.style.opacity = ".45"; b.style.cursor = "not-allowed"; }
        else b.addEventListener("click", function () { n.windowId = wid; renderCanvas(); renderInspector(); markDirty(); });
        list.appendChild(b);
      });
      lb.appendChild(list);
      body.appendChild(lb);
    }

    if (n.kind === "stack") {
      var kb = el("div", "insp__block");
      kb.appendChild(frag('<span class="eyebrow">On top</span>'));
      var kl = el("div", "winlist");
      n.windowIds.forEach(function (wid) {
        var w = WINDOWS[wid];
        var b = el("button", "winopt");
        b.setAttribute("aria-selected", String(wid === n.activeId));
        b.innerHTML = appIcon(w.app) + '<span>' + w.title + '</span>' + (wid === n.activeId ? CHECK : '<span class="r">' + w.route + '</span>');
        b.addEventListener("click", function () { n.activeId = wid; renderCanvas(); renderInspector(); markDirty(); });
        kl.appendChild(b);
      });
      kb.appendChild(kl);
      if (n.windowIds.length < 2) kb.appendChild(frag('<p class="insp__hint" style="margin-top:7px;color:var(--warning)">A stack needs at least two windows (topology.stack_size).</p>'));
      body.appendChild(kb);
    }
  }
  function kvRow(k, v) { var r = el("div", "kv__r"); r.appendChild(el("span", null, k)); r.appendChild(el("b", null, v)); return r; }
  function structBtn(label, svg, enabled, active, fn) {
    var b = el("button", "act");
    b.innerHTML = svg + '<span>' + label + '</span>';
    b.disabled = !enabled;
    b.setAttribute("aria-pressed", String(!!active));
    if (enabled) b.addEventListener("click", fn);
    return b;
  }
  function afterStructure() { renderCanvas(); renderInspector(); markDirty(); }

  var ROWS_SVG = '<svg viewBox="0 0 26 17" fill="none"><rect x=".7" y=".7" width="24.6" height="7" rx="1.4" fill="currentColor" fill-opacity=".22" stroke="currentColor" stroke-opacity=".7"/><rect x=".7" y="9.3" width="24.6" height="7" rx="1.4" fill="currentColor" fill-opacity=".22" stroke="currentColor" stroke-opacity=".7"/></svg>';
  var COLS_SVG = '<svg viewBox="0 0 26 17" fill="none"><rect x=".7" y=".7" width="11.6" height="15.6" rx="1.4" fill="currentColor" fill-opacity=".22" stroke="currentColor" stroke-opacity=".7"/><rect x="13.7" y=".7" width="11.6" height="15.6" rx="1.4" fill="currentColor" fill-opacity=".22" stroke="currentColor" stroke-opacity=".7"/></svg>';
  var STACK_SVG = '<svg viewBox="0 0 26 17" fill="none"><rect x=".7" y="3.7" width="19" height="12.6" rx="1.4" fill="currentColor" fill-opacity=".12" stroke="currentColor" stroke-opacity=".4"/><rect x="3.7" y="2.2" width="19" height="12.6" rx="1.4" fill="currentColor" fill-opacity=".18" stroke="currentColor" stroke-opacity=".55"/><rect x="6.3" y=".7" width="19" height="12.6" rx="1.4" fill="currentColor" fill-opacity=".26" stroke="currentColor" stroke-opacity=".85"/></svg>';

  /* ════════════════════════ desktop tabs ════════════════════════ */
  function renderTabs() {
    var wrap = q("desktop-tabs");
    wrap.innerHTML = "";
    DOC.desktops.forEach(function (d, i) {
      var b = el("button", "dtab");
      b.setAttribute("role", "tab");
      b.setAttribute("aria-selected", String(i === activeDesktop));
      b.innerHTML = '<span class="n">' + (i + 1) + '</span><span class="nm">' + d.name + '</span>' + (d.purpose === "focus" ? '<span class="fx">Focus</span>' : "");
      b.addEventListener("click", function () {
        if (i === activeDesktop) return startRename(b, d);
        activeDesktop = i; SEL = null; renderTabs(); renderCanvas(); renderInspector();
      });
      b.addEventListener("dblclick", function () { startRename(b, d); });
      wrap.appendChild(b);
    });
    wrap.appendChild(el("span", "dtab__spacer"));
    var note = el("span", null, "Desktops are created and removed in the workspace, not here");
    note.style.cssText = "font-size:11px;color:var(--faint);padding-right:4px";
    wrap.appendChild(note);
  }
  function startRename(btn, d) {
    var nm = btn.querySelector(".nm"); if (!nm) return;
    var input = el("input");
    input.value = d.name;
    nm.replaceWith(input);
    input.focus(); input.select();
    function commit() {
      var v = input.value.trim() || d.name;
      d.name = v; renderTabs(); renderInspector(); markDirty();
    }
    input.addEventListener("blur", commit);
    input.addEventListener("keydown", function (e) {
      if (e.key === "Enter") { e.preventDefault(); input.blur(); }
      if (e.key === "Escape") { e.preventDefault(); input.value = d.name; input.blur(); }
    });
  }

  /* ════════════════════════ review → preview → apply ════════════════════════ */
  function print() { return JSON.stringify({ d: DOC.desktops, w: WINDOWS }); }
  function docDirty() { return print() !== JSON.stringify({ d: DOC0.desktops, w: WIN0 }); }

  function diffCounts() {
    var out = { desktops: [], groups: [], nodes: [], windows: [] };
    DOC.desktops.forEach(function (d, i) {
      var o = DOC0.desktops[i]; if (!o) { out.desktops.push(d.id); return; }
      if (d.name !== o.name || d.purpose !== o.purpose || JSON.stringify(d.floating) !== JSON.stringify(o.floating)) out.desktops.push(d.id);
      d.groups.forEach(function (g, gi) {
        var og = o.groups[gi]; if (!og) { out.groups.push(g.id); return; }
        if (JSON.stringify(g.frame) !== JSON.stringify(og.frame) || JSON.stringify(g.root) !== JSON.stringify(og.root)) out.groups.push(g.id);
        var oldNodes = {};
        walk(og.root, function (n) { oldNodes[n.id] = JSON.stringify(n.kind === "split" ? { k: n.kind, a: n.axis, w: n.weights } : n); });
        walk(g.root, function (n) {
          var sig = JSON.stringify(n.kind === "split" ? { k: n.kind, a: n.axis, w: n.weights } : n);
          if (oldNodes[n.id] !== sig) out.nodes.push(n.id);
        });
      });
    });
    Object.keys(WINDOWS).forEach(function (k) {
      if (JSON.stringify(WINDOWS[k]) !== JSON.stringify(WIN0[k])) out.windows.push(k);
    });
    return out;
  }

  /* client-side mirror of internal/windowmanager/validate.go for the rules the editor can break */
  function validateDoc() {
    var diags = [];
    DOC.desktops.forEach(function (d) {
      var bad = overlappingGroups(d);
      Object.keys(bad).forEach(function (gid) {
        diags.push({ code: "topology.group_overlap", path: "desktops." + d.id + ".groups." + gid, message: "group frames must not overlap inside a desktop" });
      });
      d.groups.forEach(function (g) {
        var f = g.frame;
        if (f.x < -1e-6 || f.y < -1e-6 || f.w <= 0 || f.h <= 0 || f.x + f.w > 1 + 1e-6 || f.y + f.h > 1 + 1e-6)
          diags.push({ code: "topology.group_frame", path: "groups." + g.id + ".frame", message: "frame must stay inside the normalized 0–1 desktop" });
        walk(g.root, function (n) {
          if (n.kind === "split") {
            var sum = n.weights.reduce(function (a, b) { return a + b; }, 0);
            if (Math.abs(sum - 1) > 0.000001) diags.push({ code: "topology.split_weight_sum", path: "nodes." + n.id + ".weights", message: "split weights must sum to 1" });
            if (n.children.length < 2) diags.push({ code: "topology.split_size", path: "nodes." + n.id, message: "a split needs at least two children" });
          }
          if (n.kind === "stack" && n.windowIds.length < 2)
            diags.push({ code: "topology.stack_size", path: "nodes." + n.id, message: "a stack needs at least two windows" });
        });
      });
      if (d.purpose === "focus" && !d.focusOwner)
        diags.push({ code: "topology.focus_owner_required", path: "desktops." + d.id, message: "a focus desktop must name its focus owner" });
    });
    return diags;
  }

  function markDirty() {
    reviewedPrint = null;
    paintReview();
  }
  function paintReview() {
    var msg = q("rev-msg"), txt = q("rev-text");
    var dirty = docDirty(), reviewed = reviewedPrint === print();
    q("btn-reset-doc").disabled = !dirty;
    q("btn-review").disabled = !dirty || reviewState === "reviewing";
    q("btn-apply").disabled = !(reviewed && reviewState === "ok");
    q("diags").classList.toggle("is-open", reviewState === "bad" && reviewed);

    if (reviewState === "reviewing") { msg.dataset.state = "dirty"; txt.textContent = "Checking with the daemon…"; return; }
    if (reviewState === "applying") { msg.dataset.state = "ok"; txt.textContent = "Applying…"; return; }
    if (!dirty) { msg.dataset.state = "clean"; txt.textContent = "Revision " + DOC.revision + " · no unsaved layout edits"; return; }
    if (reviewed && reviewState === "bad") { msg.dataset.state = "bad"; txt.textContent = lastDiags.length + " problem" + (lastDiags.length === 1 ? "" : "s") + " to fix before this can apply"; return; }
    if (reviewed && reviewState === "ok" && lastPreview) {
      var p = lastPreview;
      msg.dataset.state = "ok";
      txt.textContent = p.changed
        ? "Validated · changes " + p.desktops + " desktop" + (p.desktops === 1 ? "" : "s") + ", " + p.groups + " group" + (p.groups === 1 ? "" : "s") + ", " + p.nodes + " node" + (p.nodes === 1 ? "" : "s") + ", " + p.windows + " window" + (p.windows === 1 ? "" : "s")
        : "Validated · this layout is identical to revision " + DOC.revision;
      return;
    }
    msg.dataset.state = "dirty";
    txt.textContent = "Unreviewed edits · the daemon must validate them before they apply";
  }

  function renderDiags() {
    var box = q("diags");
    box.innerHTML = "";
    box.appendChild(frag('<div class="diags__t">The daemon refused this layout</div>'));
    lastDiags.forEach(function (d) {
      box.appendChild(frag('<div class="diag"><b>' + d.code + '</b><span>' + (d.path ? d.path + " — " : "") + d.message + '</span></div>'));
    });
  }

  q("btn-review").addEventListener("click", function () {
    reviewState = "reviewing"; paintReview();
    setTimeout(function () {
      lastDiags = validateDoc();
      reviewedPrint = print();
      if (lastDiags.length) { reviewState = "bad"; renderDiags(); }
      else {
        var c = diffCounts();
        lastPreview = { changed: c.desktops.length + c.groups.length + c.nodes.length + c.windows.length > 0, desktops: c.desktops.length, groups: c.groups.length, nodes: c.nodes.length, windows: c.windows.length };
        reviewState = "ok";
      }
      paintReview();
    }, REDUCED ? 0 : 420);
  });
  q("btn-apply").addEventListener("click", function () {
    reviewState = "applying"; paintReview();
    setTimeout(function () {
      DOC.revision += 1;
      DOC0 = clone(DOC); WIN0 = clone(WINDOWS);
      reviewState = "clean"; reviewedPrint = null; lastPreview = null;
      q("sub-rev").textContent = String(DOC.revision);
      paintReview();
    }, REDUCED ? 0 : 520);
  });
  q("btn-reset-doc").addEventListener("click", function () {
    DOC = clone(DOC0); WINDOWS = clone(WIN0);
    SEL = null; reviewState = "clean"; reviewedPrint = null;
    renderTabs(); renderCanvas(); renderInspector(); paintReview();
  });
  q("btn-export").addEventListener("click", function () {
    var blob = new Blob([JSON.stringify({ version: 2, workspace_id: DOC.workspaceId, desktops: DOC.desktops, windows: WINDOWS, overrides: {} }, null, 2)], { type: "application/json" });
    var a = el("a"); a.href = URL.createObjectURL(blob); a.download = "layout-" + DOC.workspaceId + ".json"; a.click();
    URL.revokeObjectURL(a.href);
  });
  q("btn-import").addEventListener("click", function () {
    var i = el("input"); i.type = "file"; i.accept = "application/json,.json";
    i.addEventListener("change", function () {
      var file = i.files && i.files[0]; if (!file) return;
      var r = new FileReader();
      r.onload = function () {
        var doc;
        try { doc = JSON.parse(String(r.result)); } catch (err) { doc = null; }
        if (!doc || !Array.isArray(doc.desktops) || !doc.windows) {
          lastDiags = [{ code: "topology.unsupported_version", path: null, message: "the selected file is not a layout document" }];
          reviewState = "bad"; reviewedPrint = print(); renderDiags(); paintReview(); return;
        }
        DOC.desktops = doc.desktops;
        WINDOWS = doc.windows;
        activeDesktop = 0; SEL = null;
        /* an import lands in the draft like any other edit — it still has to pass
           validate → preview before it can be applied */
        renderTabs(); renderCanvas(); renderInspector(); markDirty();
      };
      r.readAsText(file);
    });
    i.click();
  });

  /* ════════════════════════ saved layouts (profiles) ════════════════════════ */
  var PROFILES = [
    {
      id: "launch-console", name: "Launch console", scope: "workspace", aspect: "any", overflow: "stack", windows: 4,
      doc: { kind: "split", axis: "horizontal", weights: [.62, .38], children: [{ kind: "leaf" }, { kind: "split", axis: "vertical", weights: [.5, .5], children: [{ kind: "leaf" }, { kind: "stack" }] }] }
    },
    {
      id: "two-up-review", name: "Two-up review", scope: "workspace", aspect: "landscape", overflow: "stack", windows: 2,
      doc: { kind: "split", axis: "horizontal", weights: [.5, .5], children: [{ kind: "leaf" }, { kind: "leaf" }] }
    },
    {
      id: "focus-session", name: "Focus session", scope: "global", aspect: "any", overflow: "reject", windows: 1,
      doc: { kind: "leaf" }
    },
    {
      id: "ops-grid", name: "Ops grid", scope: "global", aspect: "landscape", overflow: "stack", windows: 4,
      doc: { kind: "split", axis: "horizontal", weights: [.5, .5], children: [{ kind: "split", axis: "vertical", weights: [.5, .5], children: [{ kind: "leaf" }, { kind: "leaf" }] }, { kind: "split", axis: "vertical", weights: [.5, .5], children: [{ kind: "leaf" }, { kind: "leaf" }] }] }
    }
  ];
  var selectedProfile = null, pendingLoad = null, editing = null;

  function tileRects(n, r, out) {
    if (n.kind !== "split") { out.push({ r: r, kind: n.kind }); return; }
    var horiz = n.axis === "horizontal", g = 1.6, seams = (n.children.length - 1) * g;
    var span = (horiz ? r.w : r.h) - seams, off = horiz ? r.x : r.y;
    n.children.forEach(function (c, i) {
      var len = span * n.weights[i];
      tileRects(c, horiz ? { x: off, y: r.y, w: len, h: r.h } : { x: r.x, y: off, w: r.w, h: len }, out);
      off += len + g;
    });
  }
  function profileSvg(doc) {
    var out = []; tileRects(doc, { x: 1, y: 1, w: 98, h: 60 }, out);
    var rects = out.map(function (t) {
      var extra = t.kind === "stack"
        ? '<rect x="' + (t.r.x + 2.4) + '" y="' + (t.r.y - 1.6) + '" width="' + Math.max(0, t.r.w - 2.4) + '" height="' + t.r.h + '" rx="1.6" fill="rgba(255,255,255,.05)" stroke="rgba(255,255,255,.16)" stroke-width=".8"/>'
        : "";
      return extra + '<rect x="' + t.r.x + '" y="' + t.r.y + '" width="' + t.r.w + '" height="' + t.r.h + '" rx="1.6" fill="rgba(255,255,255,.09)" stroke="rgba(255,255,255,.24)" stroke-width=".8"/>';
    }).join("");
    return '<svg viewBox="0 0 100 62" preserveAspectRatio="none">' + rects + '</svg>';
  }

  function renderProfiles() {
    var grid = q("profile-grid");
    grid.innerHTML = "";
    PROFILES.forEach(function (p) {
      var c = el("div", "pcard");
      c.dataset.sel = String(selectedProfile === p.id);
      var hit = el("button", "pcard__hit");
      hit.setAttribute("aria-pressed", String(selectedProfile === p.id));
      hit.innerHTML =
        '<span class="pcard__viz">' + profileSvg(p.doc) + '</span>' +
        '<span class="pcard__meta">' +
        '<span class="pcard__name">' + p.name + '</span>' +
        '<span class="pcard__id">' + p.id + '</span>' +
        '<span class="pcard__chips">' +
        '<span class="pill" data-tone="' + (p.scope === "global" ? "info" : "neutral") + '">' + (p.scope === "global" ? "Global" : "Workspace") + '</span>' +
        (p.aspect !== "any" ? '<span class="pill">' + (p.aspect === "landscape" ? "Landscape" : "Portrait") + '</span>' : "") +
        '<span class="pill">' + p.windows + ' window' + (p.windows === 1 ? "" : "s") + '</span>' +
        '</span></span>';
      hit.addEventListener("click", function () { selectedProfile = selectedProfile === p.id ? null : p.id; renderProfiles(); });
      c.appendChild(hit);
      var acts = el("div", "pcard__acts");
      var load = el("button", "btn btn--xs", "Load");
      load.addEventListener("click", function (e) { e.stopPropagation(); requestLoad(p); });
      var edit = el("button", "btn btn--xs btn--ghost", "Edit");
      edit.addEventListener("click", function (e) { e.stopPropagation(); openEditor(p); });
      var sp = el("span"); sp.style.flex = "1";
      var del = el("button", "iconbtn iconbtn--danger");
      del.setAttribute("aria-label", "Delete " + p.name);
      del.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14"/></svg>';
      del.addEventListener("click", function (e) {
        e.stopPropagation();
        PROFILES = PROFILES.filter(function (x) { return x.id !== p.id; });
        if (selectedProfile === p.id) selectedProfile = null;
        renderProfiles();
      });
      acts.appendChild(load); acts.appendChild(edit); acts.appendChild(sp); acts.appendChild(del);
      c.appendChild(acts);
      grid.appendChild(c);
    });

    var add = el("button", "pcard pcard--new");
    add.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14M5 12h14"/></svg><span>Save the current layout</span>';
    add.addEventListener("click", function () { openEditor(null); });
    grid.appendChild(add);
    q("sub-profiles").textContent = String(PROFILES.length);
  }

  function requestLoad(p) {
    if (docDirty()) {
      pendingLoad = p;
      q("lc-name").textContent = p.name;
      q("load-confirm").classList.add("is-open");
      return;
    }
    doLoad(p);
  }
  function doLoad(p) {
    /* prototype: loading rewrites the active desktop's first group with the profile's tree
       shape, exactly like selectProfile() replaces the draft document today */
    var d = DOC.desktops[activeDesktop];
    var avail = desktopWindowIds(d);
    var wi = 0;
    function build(n) {
      if (n.kind === "leaf") return { id: nid("leaf"), kind: "leaf", windowId: avail[wi++ % avail.length] };
      if (n.kind === "stack") {
        var a = avail[wi++ % avail.length], b = avail[wi++ % avail.length];
        return { id: nid("stack"), kind: "stack", windowIds: [a, b], activeId: a };
      }
      return { id: nid("split"), kind: "split", axis: n.axis, weights: n.weights.slice(), children: n.children.map(build) };
    }
    d.groups = [{ id: d.groups[0] ? d.groups[0].id : "grp-loaded", frame: { x: 0, y: 0, w: 1, h: 1 }, root: build(p.doc) }];
    selectedProfile = p.id; SEL = null;
    renderProfiles(); renderCanvas(); renderInspector(); markDirty();
  }
  q("lc-cancel").addEventListener("click", function () { pendingLoad = null; q("load-confirm").classList.remove("is-open"); });
  q("lc-ok").addEventListener("click", function () { if (pendingLoad) doLoad(pendingLoad); pendingLoad = null; q("load-confirm").classList.remove("is-open"); });

  function slug(s) { return s.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, ""); }
  function openEditor(p) {
    editing = p;
    var box = q("profile-editor");
    box.classList.add("is-open");
    q("pe-name").value = p ? p.name : "";
    q("pe-id").value = p ? p.id : "";
    setSegm("pe-scope", p ? p.scope : "workspace");
    setSegm("pe-aspect", p ? p.aspect : "any");
    setOpts("pe-overflow", p ? p.overflow : "stack");
    var d = DOC.desktops[activeDesktop];
    var count = structuralIds(d).length + d.floating.length;
    q("pe-slots").textContent = p
      ? "Re-saving captures the " + count + " windows currently on " + d.name + "."
      : count + " windows on " + d.name + " will be captured into this layout.";
    q("pe-name").focus();
  }
  q("pe-name").addEventListener("input", function () {
    if (!editing && !q("pe-id").dataset.touched) q("pe-id").value = slug(q("pe-name").value);
  });
  q("pe-id").addEventListener("input", function () { q("pe-id").dataset.touched = "1"; });
  q("pe-cancel").addEventListener("click", function () { q("profile-editor").classList.remove("is-open"); editing = null; q("pe-id").removeAttribute("data-touched"); });
  q("pe-save").addEventListener("click", function () {
    var name = q("pe-name").value.trim(), id = q("pe-id").value.trim() || slug(name);
    if (!name || !id) return;
    var d = DOC.desktops[activeDesktop];
    var shape = d.groups[0] ? shapeOf(d.groups[0].root) : { kind: "leaf" };
    var rec = { id: id, name: name, scope: segmVal("pe-scope"), aspect: segmVal("pe-aspect"), overflow: optsVal("pe-overflow"), windows: structuralIds(d).length + d.floating.length, doc: shape };
    var i = -1; PROFILES.forEach(function (x, k) { if (editing && x.id === editing.id) i = k; });
    if (i >= 0 && editing.id === id) PROFILES[i] = rec; else PROFILES.push(rec);
    selectedProfile = id; editing = null;
    q("profile-editor").classList.remove("is-open");
    q("pe-id").removeAttribute("data-touched");
    renderProfiles();
  });
  function shapeOf(n) {
    if (n.kind === "split") return { kind: "split", axis: n.axis, weights: n.weights.slice(), children: n.children.map(shapeOf) };
    return { kind: n.kind };
  }
  function setSegm(id, v) {
    Array.prototype.forEach.call(q(id).children, function (b) { b.setAttribute("aria-selected", String(b.dataset.v === v)); });
    if (id === "pe-scope") q("pe-scope-hint").textContent = v === "global" ? "Available in every workspace on this daemon." : "Visible only inside compozy/agh.";
  }
  function segmVal(id) { var v = "any"; Array.prototype.forEach.call(q(id).children, function (b) { if (b.getAttribute("aria-selected") === "true") v = b.dataset.v; }); return v; }
  function setOpts(id, v) { Array.prototype.forEach.call(q(id).children, function (b) { b.setAttribute("aria-selected", String(b.dataset.v === v)); }); }
  function optsVal(id) { return segmVal(id); }
  ["pe-scope", "pe-aspect"].forEach(function (id) {
    q(id).addEventListener("click", function (e) { var b = e.target.closest("button"); if (b) setSegm(id, b.dataset.v); });
  });
  q("pe-overflow").addEventListener("click", function (e) { var b = e.target.closest(".opt"); if (b) setOpts("pe-overflow", b.dataset.v); });

  /* ════════════════════════ behavior — diagram choice cards ════════════════════════ */
  function d_(inner) { return '<svg viewBox="0 0 88 38" fill="none">' + inner + '</svg>'; }
  var R = function (x, y, w, h, o) { return '<rect x="' + x + '" y="' + y + '" width="' + w + '" height="' + h + '" rx="2" fill="currentColor" fill-opacity="' + (o || .16) + '" stroke="currentColor" stroke-opacity=".55"/>'; };
  var RA = function (x, y, w, h) { return '<rect x="' + x + '" y="' + y + '" width="' + w + '" height="' + h + '" rx="2" fill-opacity=".32" style="fill:var(--accent);stroke:var(--accent)"/>'; };

  var PICKS = [
    {
      key: "newWindowPolicy", label: "New windows", desc: "Where a window lands when it opens without a home.",
      opts: [
        { v: "floating", n: "Float on top", svg: d_(R(3, 4, 38, 30) + R(45, 4, 40, 30) + '<rect x="26" y="12" width="40" height="24" rx="2" style="fill:var(--elevated);stroke:var(--accent)"/>') },
        { v: "beside_focus", n: "Split beside focus", svg: d_(R(3, 4, 38, 30) + RA(45, 4, 40, 30)) }
      ]
    },
    {
      key: "smallViewportPolicy", label: "Not enough room", desc: "When the viewport can no longer hold every tile.",
      opts: [
        { v: "stack", n: "Fold into a stack", svg: d_(R(6, 8, 50, 26, .1) + R(14, 6, 50, 26, .16) + RA(22, 4, 50, 30)) },
        { v: "reject", n: "Refuse the layout", svg: d_('<rect x="4" y="4" width="80" height="30" rx="2" stroke="currentColor" stroke-opacity=".45" stroke-dasharray="3 3"/><path d="m36 12 16 14M52 12 36 26" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>') }
      ]
    },
    {
      key: "focusPolicy", label: "Taking focus", desc: "Whether a pointer click can move focus, or only the keyboard.",
      opts: [
        { v: "click_directional", n: "Click and keys", svg: d_(R(3, 4, 38, 30) + RA(45, 4, 40, 30) + '<path d="m58 16 10 9-4 1 2 5-2 1-2-5-3 3z" style="fill:var(--fg-strong)"/>') },
        { v: "directional", n: "Keys only", svg: d_(R(3, 4, 38, 30) + RA(45, 4, 40, 30) + '<path d="M69 19h-12M57 19l4-4M57 19l4 4" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" style="stroke:var(--fg-strong)"/>') }
      ]
    },
    {
      key: "dragAwayPolicy", label: "Dragging a tiled window", desc: "What comes along when you pull one out of its group.",
      opts: [
        { v: "window", n: "Just that window", svg: d_(R(3, 8, 26, 26) + R(33, 8, 26, 26) + '<rect x="52" y="2" width="32" height="24" rx="2" style="fill:var(--elevated);stroke:var(--accent)"/>') },
        { v: "group", n: "Its whole group", svg: d_(R(3, 8, 26, 26) + '<rect x="42" y="2" width="20" height="24" rx="2" style="fill:var(--elevated);stroke:var(--accent)"/><rect x="64" y="2" width="20" height="24" rx="2" style="fill:var(--elevated);stroke:var(--accent)"/>') }
      ]
    },
    {
      key: "desktopTransition", label: "Switching desktops", desc: "How this browser moves between desktops.",
      opts: [
        { v: "slide", n: "Slide", svg: d_(R(2, 6, 34, 26) + RA(50, 6, 34, 26) + '<path d="M40 19h8M44 15l4 4-4 4" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" style="stroke:var(--fg-strong)"/>') },
        { v: "crossfade", n: "Crossfade", svg: d_(R(12, 6, 40, 26, .08) + '<rect x="36" y="6" width="40" height="26" rx="2" fill-opacity=".2" stroke-opacity=".7" style="fill:var(--accent);stroke:var(--accent)"/>') },
        { v: "instant", n: "Instant", svg: d_(R(2, 6, 40, 26, .08) + RA(46, 6, 40, 26) + '<path d="M44 2v34" stroke-opacity=".5" stroke-width="1.2" stroke-dasharray="2 2" style="stroke:var(--fg-strong)"/>') }
      ]
    }
  ];
  var MODS = [
    { key: "groupMoveModifier", label: "Move the whole group", desc: "Hold this while dragging to take the group instead of one window." },
    { key: "swapModifier", label: "Swap two windows", desc: "Hold this while dropping onto another window to trade places." }
  ];
  var MODKEYS = [{ v: "alt", s: "⌥" }, { v: "control", s: "⌃" }, { v: "meta", s: "⌘" }, { v: "shift", s: "⇧" }, { v: "none", s: "None" }];
  var TOGGLES = [
    { key: "focusWrap", label: "Wrap directional focus", desc: "At the last window in a direction, continue from the opposite edge." },
    { key: "focusFollowsPointer", label: "Focus follows the pointer", desc: "A window takes focus as soon as the pointer enters it." },
    { key: "raiseOnFocus", label: "Raise on focus", desc: "Bring a focused floating window above its peers." }
  ];

  function renderBehavior() {
    var p = q("behavior-panel");
    p.innerHTML = "";
    PICKS.forEach(function (pk) {
      var row = el("div", "pick");
      var main = el("div", "pick__main");
      main.appendChild(el("div", "pick__label", pk.label));
      main.appendChild(el("p", "pick__desc", pk.desc));
      row.appendChild(main);
      var opts = el("div", "opts");
      pk.opts.forEach(function (o) {
        var b = el("button", "opt");
        b.dataset.v = o.v;
        b.setAttribute("aria-selected", String(CFG[pk.key] === o.v));
        b.innerHTML = o.svg + '<span class="opt__n">' + o.n + '</span>';
        b.addEventListener("click", function () { CFG[pk.key] = o.v; renderBehavior(); refreshSave(); });
        opts.appendChild(b);
      });
      row.appendChild(opts);
      p.appendChild(row);
    });
    MODS.forEach(function (m) {
      var row = el("div", "pick");
      var main = el("div", "pick__main");
      main.appendChild(el("div", "pick__label", m.label));
      main.appendChild(el("p", "pick__desc", m.desc));
      row.appendChild(main);
      var keys = el("div", "keys");
      keys.setAttribute("role", "tablist");
      keys.setAttribute("aria-label", m.label);
      MODKEYS.forEach(function (k) {
        var b = el("button", "keycap", k.s);
        b.setAttribute("role", "tab");
        b.setAttribute("aria-selected", String(CFG[m.key] === k.v));
        b.setAttribute("aria-label", k.v);
        if (k.v === "none") b.dataset.none = "true";
        b.addEventListener("click", function () { CFG[m.key] = k.v; renderBehavior(); refreshSave(); });
        keys.appendChild(b);
      });
      row.appendChild(keys);
      p.appendChild(row);
    });
    TOGGLES.forEach(function (t) {
      var row = el("div", "srow");
      row.style.borderTop = "1px solid var(--line-soft)";
      var main = el("div", "srow__main");
      main.appendChild(el("div", "srow__label", t.label));
      main.appendChild(el("p", "srow__desc", t.desc));
      row.appendChild(main);
      var ctl = el("div", "srow__ctl");
      var sw = el("button", "switch");
      sw.setAttribute("role", "switch");
      sw.setAttribute("aria-checked", String(!!CFG[t.key]));
      sw.setAttribute("aria-label", t.label);
      sw.addEventListener("click", function () { CFG[t.key] = !CFG[t.key]; sw.setAttribute("aria-checked", String(CFG[t.key])); refreshSave(); });
      ctl.appendChild(sw);
      row.appendChild(ctl);
      p.appendChild(row);
    });
  }

  /* ════════════════════════ gaps — a 1:1 box model you drag ════════════════════════
     Shown at true scale: one pixel of travel is one pixel of gap, so the value
     never needs to be typed and never needs a unit conversion in the head. */
  var gapbox = q("gapbox"), liveGap = null, gapNodes = null;

  function buildGaps() {
    gapbox.innerHTML = "";
    var work = el("div", "gapbox__work");
    for (var i = 0; i < 4; i++) work.appendChild(el("div", "gapbox__w"));
    gapbox.appendChild(work);
    var h = {};
    [["top", "y"], ["bottom", "y"], ["left", "x"], ["right", "x"]].forEach(function (s) {
      var b = el("button", "ghandle");
      b.dataset.ax = s[1];
      b.setAttribute("role", "slider");
      b.setAttribute("aria-valuemin", "0"); b.setAttribute("aria-valuemax", "64");
      bindGapHandle(b, s[0], s[1]);
      gapbox.appendChild(b); h[s[0]] = b;
    });
    ["x", "y"].forEach(function (ax) {
      var b = el("button", "ghandle");
      b.dataset.ax = ax;
      b.setAttribute("role", "slider");
      b.setAttribute("aria-valuemin", "0"); b.setAttribute("aria-valuemax", "64");
      bindInner(b, ax);
      gapbox.appendChild(b); h["inner" + ax] = b;
    });
    gapNodes = { work: work, h: h };
    paintGaps();
  }

  function paintGaps() {
    if (!gapNodes) return;
    var g = CFG.gaps, w = gapNodes.work, h = gapNodes.h;
    w.style.top = g.top + "px"; w.style.right = g.right + "px";
    w.style.bottom = g.bottom + "px"; w.style.left = g.left + "px";
    w.style.gap = g.inner + "px";
    function set(node, css, label, value) {
      node.style.cssText = "";
      Object.keys(css).forEach(function (p) { node.style[p] = css[p]; });
      node.setAttribute("aria-label", label + ", " + value + " pixels");
      node.setAttribute("aria-valuenow", String(value));
    }
    set(h.top, { top: (g.top - 5) + "px", left: "0", right: "0" }, "Top gap", g.top);
    set(h.bottom, { bottom: (g.bottom - 5) + "px", left: "0", right: "0" }, "Bottom gap", g.bottom);
    set(h.left, { left: (g.left - 5) + "px", top: "0", bottom: "0" }, "Left gap", g.left);
    set(h.right, { right: (g.right - 5) + "px", top: "0", bottom: "0" }, "Right gap", g.right);
    set(h.innerx, { left: "calc(50% - 5px)", top: (g.top + 16) + "px", bottom: (g.bottom + 16) + "px" }, "Gap between tiles", g.inner);
    set(h.innery, { top: "calc(50% - 5px)", left: (g.left + 16) + "px", right: (g.right + 16) + "px" }, "Gap between tiles", g.inner);
    h.top.dataset.ax = "y"; h.bottom.dataset.ax = "y"; h.left.dataset.ax = "x"; h.right.dataset.ax = "x";
    h.innerx.dataset.ax = "x"; h.innery.dataset.ax = "y";

    var f = q("gap-readouts");
    f.innerHTML = "";
    [["Between tiles", "inner"], ["Top", "top"], ["Right", "right"], ["Bottom", "bottom"], ["Left", "left"]].forEach(function (r, i) {
      if (i) f.appendChild(el("span", "dotsep"));
      f.appendChild(frag('<span class="readout" data-live="' + (liveGap === r[1] ? "true" : "false") + '">' + r[0] + ' <b>' + g[r[1]] + '</b>px</span>'));
    });
  }

  function bindGapHandle(h, side, ax) {
    var sign = (side === "right" || side === "bottom") ? -1 : 1;
    draggable(h, {
      start: function () { liveGap = side; return { v: CFG.gaps[side], d: 0 }; },
      move: function (ev, st) {
        st.d += (ax === "x" ? ev.movementX : ev.movementY) * sign;
        CFG.gaps[side] = clamp(Math.round(st.v + st.d), 0, 64);
        paintGaps(); scheduleCanvas(); refreshSave();
      },
      end: function () { liveGap = null; paintGaps(); }
    });
    h.addEventListener("keydown", function (e) {
      var step = e.key === "ArrowUp" || e.key === "ArrowRight" ? 1 : e.key === "ArrowDown" || e.key === "ArrowLeft" ? -1 : 0;
      if (!step) return;
      e.preventDefault();
      CFG.gaps[side] = clamp(CFG.gaps[side] + step, 0, 64);
      paintGaps(); scheduleCanvas(); refreshSave();
    });
  }
  function bindInner(h, ax) {
    draggable(h, {
      start: function () { liveGap = "inner"; return { v: CFG.gaps.inner, d: 0 }; },
      move: function (ev, st) {
        /* the seam widens from its centre, so it tracks the pointer at 1:1 */
        st.d += (ax === "x" ? ev.movementX : ev.movementY) * 2;
        CFG.gaps.inner = clamp(Math.round(st.v + st.d), 0, 64);
        paintGaps(); scheduleCanvas(); refreshSave();
      },
      end: function () { liveGap = null; paintGaps(); }
    });
    h.addEventListener("keydown", function (e) {
      var step = e.key === "ArrowUp" || e.key === "ArrowRight" ? 1 : e.key === "ArrowDown" || e.key === "ArrowLeft" ? -1 : 0;
      if (!step) return;
      e.preventDefault();
      CFG.gaps.inner = clamp(CFG.gaps.inner + step, 0, 64);
      paintGaps(); scheduleCanvas(); refreshSave();
    });
  }

  /* ════════════════════════ snap zones ════════════════════════
     Rectangle's lesson: put the control where the region is. Every zone below
     sits at the screen position it governs — no coordinate fields at all. */
  var snapmap = q("snapmap"), liveSnap = null, snapNodes = null;

  function buildSnap() {
    snapmap.innerHTML = "";
    function put(cls) { var e = el("div", cls); snapmap.appendChild(e); return e; }
    var n = {
      bands: [put("sband"), put("sband"), put("sband"), put("sband")],
      corners: [put("scorner"), put("scorner"), put("scorner"), put("scorner")],
      slack: put("sslack"), zones: {}
    };
    function grip(key, ax, lo, hi) {
      var b = el("button", "sgrip");
      b.dataset.ax = ax;
      b.setAttribute("role", "slider");
      b.setAttribute("aria-valuemin", String(lo)); b.setAttribute("aria-valuemax", String(hi));
      bindSnapGrip(b, key, ax, lo, hi);
      snapmap.appendChild(b);
      return b;
    }
    n.gBand = grip("edgeBand", "x", 4, 128);
    n.gCorner = grip("cornerReach", "y", 16, 512);
    n.gSlack = grip("exitSlack", "x", 0, 64);
    [["topCenter", "top"], ["bottomCenter", "bottom"]].forEach(function (z) {
      var b = el("button", "zone");
      b.style.left = "50%"; b.style.transform = "translateX(-50%)";
      b.setAttribute("aria-label", (z[1] === "top" ? "Top" : "Bottom") + " centre zone");
      b.addEventListener("click", function (e) { e.stopPropagation(); openZoneMenu(b, z[0]); });
      snapmap.appendChild(b);
      n.zones[z[0]] = { node: b, side: z[1] };
    });
    snapNodes = n;
    paintSnap();
  }

  function paintSnap() {
    if (!snapNodes) return;
    var s = CFG.snap, n = snapNodes;
    var k = (snapmap.clientWidth || 420) / VIEWPORT.w;
    var band = s.edgeBand * k, corner = s.cornerReach * k, slack = s.exitSlack * k;
    function css(node, o) { node.style.cssText = ""; Object.keys(o).forEach(function (p) { node.style[p] = o[p]; }); }
    css(n.bands[0], { left: "0", top: "0", bottom: "0", width: band + "px" });
    css(n.bands[1], { right: "0", top: "0", bottom: "0", width: band + "px" });
    css(n.bands[2], { left: band + "px", right: band + "px", top: "0", height: band + "px" });
    css(n.bands[3], { left: band + "px", right: band + "px", bottom: "0", height: band + "px" });
    [["left", "top"], ["right", "top"], ["left", "bottom"], ["right", "bottom"]].forEach(function (c, i) {
      var o = { width: corner + "px", height: corner + "px" }; o[c[0]] = "0"; o[c[1]] = "0";
      css(n.corners[i], o);
    });
    css(n.slack, { left: (band + slack) + "px", right: (band + slack) + "px", top: (band + slack) + "px", bottom: (band + slack) + "px" });

    css(n.gBand, { left: (band - 5) + "px", top: "calc(50% - 13px)", height: "26px" });
    n.gBand.dataset.ax = "x";
    n.gBand.setAttribute("aria-label", "Edge band, " + s.edgeBand + " pixels");
    n.gBand.setAttribute("aria-valuenow", String(s.edgeBand));
    css(n.gCorner, { top: (corner - 5) + "px", left: "5px", width: "26px" });
    n.gCorner.dataset.ax = "y";
    n.gCorner.setAttribute("aria-label", "Corner reach, " + s.cornerReach + " pixels");
    n.gCorner.setAttribute("aria-valuenow", String(s.cornerReach));
    css(n.gSlack, { left: (band + slack - 5) + "px", bottom: "12px", height: "26px" });
    n.gSlack.dataset.ax = "x";
    n.gSlack.setAttribute("aria-label", "Exit slack, " + s.exitSlack + " pixels");
    n.gSlack.setAttribute("aria-valuenow", String(s.exitSlack));

    Object.keys(n.zones).forEach(function (key) {
      var z = n.zones[key], v = CFG.bindings[key];
      z.node.dataset.v = v;
      z.node.textContent = v === "none" ? "Free" : v === "zoom" ? "Zoom" : "Reserved";
      z.node.style[z.side] = Math.max(4, band / 2 - 10) + "px";
    });

    var f = q("snap-readouts");
    f.innerHTML = "";
    [["Edge band", "edgeBand"], ["Corner reach", "cornerReach"], ["Exit slack", "exitSlack"]].forEach(function (r, i) {
      if (i) f.appendChild(el("span", "dotsep"));
      f.appendChild(frag('<span class="readout" data-live="' + (liveSnap === r[1] ? "true" : "false") + '">' + r[0] + ' <b>' + Math.round(CFG.snap[r[1]]) + '</b>px</span>'));
    });
  }

  function bindSnapGrip(h, key, ax, lo, hi) {
    draggable(h, {
      start: function () {
        liveSnap = key;
        return { v: CFG.snap[key], d: 0, inv: VIEWPORT.w / (snapmap.clientWidth || 420) };
      },
      move: function (ev, st) {
        st.d += (ax === "x" ? ev.movementX : ev.movementY) * st.inv;
        CFG.snap[key] = clamp(Math.round(st.v + st.d), lo, hi);
        paintSnap(); refreshSave();
      },
      end: function () { liveSnap = null; paintSnap(); }
    });
    h.addEventListener("keydown", function (e) {
      var step = e.key === "ArrowUp" || e.key === "ArrowRight" ? 1 : e.key === "ArrowDown" || e.key === "ArrowLeft" ? -1 : 0;
      if (!step) return;
      e.preventDefault();
      CFG.snap[key] = clamp(CFG.snap[key] + step * (e.shiftKey ? 10 : 1), lo, hi);
      paintSnap(); refreshSave();
    });
    h.addEventListener("click", function (e) { e.stopPropagation(); });
  }
  var zoneMenu = q("zone-menu");
  var ZONE_OPTS = [
    { v: "none", t: "Free", d: "The centre snaps like any other edge." },
    { v: "reserved", t: "Reserved", d: "Kept clear — nothing claims this centre." },
    { v: "zoom", t: "Zoom", d: "Dropping here zooms the window to the desktop." }
  ];
  function openZoneMenu(anchor, key) {
    zoneMenu.innerHTML = "";
    ZONE_OPTS.forEach(function (o) {
      var b = el("button");
      b.setAttribute("role", "menuitem");
      b.setAttribute("aria-selected", String(CFG.bindings[key] === o.v));
      b.innerHTML = o.t + "<span>" + o.d + "</span>";
      b.addEventListener("click", function () { CFG.bindings[key] = o.v; closeZoneMenu(); paintSnap(); refreshSave(); });
      zoneMenu.appendChild(b);
    });
    var r = anchor.getBoundingClientRect();
    zoneMenu.classList.add("is-open");
    zoneMenu.style.left = Math.max(8, Math.min(r.left, window.innerWidth - 212)) + "px";
    zoneMenu.style.top = Math.min(r.bottom + 6, window.innerHeight - 130) + "px";
  }
  function closeZoneMenu() { zoneMenu.classList.remove("is-open"); }
  document.addEventListener("click", function (e) { if (!zoneMenu.contains(e.target)) closeZoneMenu(); });

  /* ════════════════════════ repeat widths — a track of stops, not a CSV string ════════════════════════ */
  var ratio = q("ratio");
  var selStop = -1, liveStop = -1;
  function renderRatio() {
    var rs = CFG.snap.repeatRatios;
    ratio.innerHTML = "";
    var shown = rs[liveStop >= 0 ? liveStop : (selStop >= 0 ? selStop : 0)];
    var fill = el("div", "ratio__fill");
    fill.style.width = "calc((100% - 20px) * " + shown + ")";
    ratio.appendChild(fill);
    ratio.appendChild(el("div", "ratio__track"));
    ratio.appendChild(frag('<span class="ratio__hint">' + rs.length + ' of 8 stops</span>'));
    rs.forEach(function (v, i) {
      var s = el("button", "stop");
      s.style.left = "calc(10px + (100% - 20px) * " + v + ")";
      if (selStop === i) s.dataset.sel = "true";
      s.innerHTML = '<b>' + fracLabel(v) + '</b><i></i>';
      s.setAttribute("aria-label", "Repeat width " + Math.round(v * 100) + " percent");
      s.setAttribute("role", "slider");
      s.setAttribute("aria-valuemin", "10"); s.setAttribute("aria-valuemax", "90"); s.setAttribute("aria-valuenow", String(Math.round(v * 100)));
      s.addEventListener("click", function (e) { e.stopPropagation(); selStop = i; renderRatio(); });
      s.addEventListener("keydown", function (e) {
        if ((e.key === "Backspace" || e.key === "Delete") && rs.length > 1) {
          e.preventDefault(); rs.splice(i, 1); selStop = -1; renderRatio(); refreshSave(); return;
        }
        var step = e.key === "ArrowRight" ? .01 : e.key === "ArrowLeft" ? -.01 : 0;
        if (!step) return;
        e.preventDefault();
        rs[i] = clamp(Math.round((v + step) * 1000) / 1000, .1, .9);
        renderRatio(); refreshSave();
      });
      draggable(s, {
        start: function () { liveStop = i; selStop = i; return { c: ratio.getBoundingClientRect() }; },
        move: function (ev, st) {
          var t = clamp((ev.clientX - st.c.left - 10) / (st.c.width - 20), .1, .9);
          var snap = detent(t, .015); if (snap !== null) t = snap;
          rs[i] = Math.round(t * 1000000) / 1000000;
          /* move in place — a full re-render would unmount the stop being dragged */
          s.style.left = "calc(10px + (100% - 20px) * " + rs[i] + ")";
          s.querySelector("b").textContent = fracLabel(rs[i]);
          s.setAttribute("aria-valuenow", String(Math.round(rs[i] * 100)));
          fill.style.width = "calc((100% - 20px) * " + rs[i] + ")";
          paintRatioReadout(); refreshSave();
        },
        end: function () { liveStop = -1; renderRatio(); }
      });
      ratio.appendChild(s);
    });
    paintRatioReadout();
  }
  function paintRatioReadout() {
    var rs = CFG.snap.repeatRatios, f = q("ratio-readouts");
    f.innerHTML = "";
    var order = rs.slice().sort(function (a, b) { return b - a; });
    f.appendChild(frag('<span class="readout">Cycle <b>' + order.map(function (v) { return fracLabel(v); }).join(" → ") + '</b></span>'));
    if (rs.length >= 8) { f.appendChild(el("span", "dotsep")); f.appendChild(frag('<span class="readout" style="color:var(--warning)">8 stops is the maximum</span>')); }
  }
  ratio.addEventListener("click", function (e) {
    if (e.target.closest(".stop")) return;
    var rs = CFG.snap.repeatRatios;
    if (rs.length >= 8) return;
    var c = ratio.getBoundingClientRect();
    var t = clamp((e.clientX - c.left - 10) / (c.width - 20), .1, .9);
    var snap = detent(t, .015); if (snap !== null) t = snap;
    var dup = rs.some(function (v) { return Math.abs(v - t) < .01; });
    if (dup) return;
    rs.push(Math.round(t * 1000000) / 1000000);
    selStop = rs.length - 1;
    renderRatio(); refreshSave();
  });

  /* ════════════════════════ shortcuts ════════════════════════ */
  var ACTIONS = {
    "window.close": { l: "Close window", d: "meta+KeyW" },
    "window.minimize": { l: "Minimize window", d: "meta+KeyM" },
    "window.zoom": { l: "Zoom window", d: "control+alt+ArrowUp" },
    "window.toggle_floating": { l: "Toggle floating", d: "control+alt+KeyF" },
    "window.tile.left": { l: "Tile left half", d: "control+alt+ArrowLeft", p: "left" },
    "window.tile.right": { l: "Tile right half", d: "control+alt+ArrowRight", p: "right" },
    "window.tile.top": { l: "Tile top half", d: "", p: "top" },
    "window.tile.bottom": { l: "Tile bottom half", d: "", p: "bottom" },
    "window.tile.top-left": { l: "Tile top left quarter", d: "control+alt+KeyU", p: "top-left" },
    "window.tile.top-right": { l: "Tile top right quarter", d: "control+alt+KeyI", p: "top-right" },
    "window.tile.bottom-left": { l: "Tile bottom left quarter", d: "control+alt+KeyJ", p: "bottom-left" },
    "window.tile.bottom-right": { l: "Tile bottom right quarter", d: "control+alt+KeyK", p: "bottom-right" },
    "window.focus.left": { l: "Focus left", d: "control+ArrowLeft" },
    "window.focus.right": { l: "Focus right", d: "control+ArrowRight" },
    "window.focus.up": { l: "Focus up", d: "control+ArrowUp" },
    "window.focus.down": { l: "Focus down", d: "control+ArrowDown" },
    "desktop.switch.previous": { l: "Previous desktop", d: "control+alt+BracketLeft" },
    "desktop.switch.next": { l: "Next desktop", d: "control+alt+BracketRight" },
    "desktop.overview": { l: "Desktops overview", d: "meta+shift+KeyS" },
    "layout.arrange.two-up": { l: "Arrange left & right", d: "" },
    "layout.arrange.grid": { l: "Arrange in grid", d: "" },
    "layout.balance": { l: "Balance layout", d: "control+alt+KeyB" },
    "layout.undo": { l: "Undo layout", d: "meta+KeyZ" },
    "layout.redo": { l: "Redo layout", d: "meta+shift+KeyZ" }
  };
  var SC_GROUPS = [
    { t: "Window", ids: ["window.close", "window.minimize", "window.zoom", "window.toggle_floating"], open: true },
    { t: "Tiling", ids: ["window.tile.left", "window.tile.right", "window.tile.top", "window.tile.bottom", "window.tile.top-left", "window.tile.top-right", "window.tile.bottom-left", "window.tile.bottom-right"], open: true },
    { t: "Focus", ids: ["window.focus.left", "window.focus.right", "window.focus.up", "window.focus.down"], open: false },
    { t: "Desktops", ids: ["desktop.switch.previous", "desktop.switch.next", "desktop.overview"], open: false },
    { t: "Layout", ids: ["layout.arrange.two-up", "layout.arrange.grid", "layout.balance", "layout.undo", "layout.redo"], open: false }
  ];
  var PLACE = {
    left: [0, 0, 11, 16], right: [11, 0, 11, 16], top: [0, 0, 22, 8], bottom: [0, 8, 22, 8],
    "top-left": [0, 0, 11, 8], "top-right": [11, 0, 11, 8], "bottom-left": [0, 8, 11, 8], "bottom-right": [11, 8, 11, 8]
  };
  function placeSvg(p) {
    var r = PLACE[p];
    return '<svg viewBox="0 0 24 18" fill="none"><rect x="1" y="1" width="22" height="16" rx="2" stroke="currentColor" stroke-opacity=".38"/><rect x="' + (r[0] + 1) + '" y="' + (r[1] + 1) + '" width="' + r[2] + '" height="' + r[3] + '" rx="1.4" fill="currentColor" fill-opacity=".55"/></svg>';
  }
  var KEYSYM = { ArrowLeft: "←", ArrowRight: "→", ArrowUp: "↑", ArrowDown: "↓", BracketLeft: "[", BracketRight: "]", Space: "␣", Enter: "⏎", Backslash: "\\", Slash: "/", Comma: ",", Period: ".", Semicolon: ";", Quote: "'", Minus: "−", Equal: "=", Backquote: "`", Tab: "⇥", Escape: "⎋", Backspace: "⌫", Delete: "⌦", Home: "↖", End: "↘", PageUp: "⇞", PageDown: "⇟" };
  var MODSYM = { control: "⌃", alt: "⌥", shift: "⇧", meta: "⌘" };
  function codeSym(c) {
    if (KEYSYM[c]) return KEYSYM[c];
    if (/^Key[A-Z]$/.test(c)) return c.slice(3);
    if (/^Digit\d$/.test(c)) return c.slice(5);
    if (/^F\d+$/.test(c)) return c;
    return c;
  }
  function chordParts(ch) {
    if (!ch) return [];
    var ps = ch.split("+");
    var code = ps.pop();
    return ps.map(function (m) { return MODSYM[m] || m; }).concat([codeSym(code)]);
  }
  function effective(id) { return CFG.shortcuts[id] != null ? CFG.shortcuts[id] : ACTIONS[id].d; }
  var recording = null;

  function renderShortcuts() {
    var p = q("shortcut-panel");
    p.innerHTML = "";
    var byChord = {};
    Object.keys(ACTIONS).forEach(function (id) {
      var c = effective(id); if (!c) return;
      (byChord[c] = byChord[c] || []).push(id);
    });
    var clashing = {};
    Object.keys(byChord).forEach(function (c) { if (byChord[c].length > 1) byChord[c].forEach(function (id) { clashing[id] = byChord[c]; }); });

    SC_GROUPS.forEach(function (g) {
      var wrap = el("div", "kgroup");
      var head = el("button", "kghead");
      head.setAttribute("aria-expanded", String(g.open));
      var changed = g.ids.filter(function (id) { return CFG.shortcuts[id] != null; }).length;
      head.innerHTML = CHEV + '<span>' + g.t + '</span><span class="c">' + g.ids.length + (changed ? " · " + changed + " changed" : "") + '</span>';
      head.addEventListener("click", function () { g.open = !g.open; renderShortcuts(); });
      wrap.appendChild(head);
      var body = el("div", "kgbody" + (g.open ? " is-open" : ""));
      g.ids.forEach(function (id) {
        var a = ACTIONS[id], ch = effective(id), custom = CFG.shortcuts[id] != null;
        var row = el("div", "krow");
        if (custom) row.dataset.custom = "true";
        var viz = el("div", "krow__viz");
        if (a.p) viz.innerHTML = placeSvg(a.p);
        row.appendChild(viz);
        row.appendChild(el("div", "krow__n", a.l));
        row.appendChild(el("span", "krow__sp"));
        row.appendChild(frag('<span class="krow__id">' + id + '</span>'));
        var chip = el("button", "kbchip");
        var state = recording === id ? "rec" : clashing[id] ? "clash" : !ch ? "unset" : custom ? "custom" : "set";
        chip.dataset.state = state;
        chip.setAttribute("aria-label", a.l + " shortcut");
        if (state === "rec") chip.textContent = "Press keys…";
        else if (!ch) chip.textContent = "Set shortcut";
        else chip.innerHTML = chordParts(ch).map(function (k) { return "<span>" + k + "</span>"; }).join("");
        chip.addEventListener("click", function (e) { e.stopPropagation(); startRecord(id); });
        row.appendChild(chip);
        var rst = el("button", "iconbtn krow__reset");
        rst.setAttribute("aria-label", "Reset " + a.l + " to its default");
        rst.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/></svg>';
        rst.disabled = !custom;
        rst.addEventListener("click", function (e) { e.stopPropagation(); delete CFG.shortcuts[id]; renderShortcuts(); refreshSave(); });
        row.appendChild(rst);
        body.appendChild(row);
      });
      var clashIds = g.ids.filter(function (id) { return clashing[id]; });
      if (clashIds.length) {
        var msg = el("div", "kclash is-open");
        msg.textContent = clashIds.length + " shortcut" + (clashIds.length === 1 ? " is" : "s are") + " assigned to more than one action. The daemon keeps the last one it reads.";
        body.appendChild(msg);
      }
      wrap.appendChild(body);
      p.appendChild(wrap);
    });
    var n = Object.keys(CFG.shortcuts).length;
    var badge = q("sc-count");
    badge.textContent = n === 0 ? "All default" : n + " changed";
    badge.dataset.tone = n === 0 ? "neutral" : "accent";
    q("sc-reset").disabled = n === 0;
  }
  function startRecord(id) {
    recording = id;
    renderShortcuts();
    function onKey(e) {
      if (e.key === "Escape") { stop(); return; }
      if (["Control", "Alt", "Shift", "Meta"].indexOf(e.key) >= 0) return;
      e.preventDefault(); e.stopPropagation();
      var mods = [];
      if (e.ctrlKey) mods.push("control");
      if (e.altKey) mods.push("alt");
      if (e.shiftKey) mods.push("shift");
      if (e.metaKey) mods.push("meta");
      var chord = mods.concat([e.code]).join("+");
      if (chord === ACTIONS[id].d) delete CFG.shortcuts[id];
      else CFG.shortcuts[id] = chord;
      stop();
      refreshSave();
    }
    function stop() {
      window.removeEventListener("keydown", onKey, true);
      recording = null;
      renderShortcuts();
    }
    window.addEventListener("keydown", onKey, true);
  }
  q("sc-reset").addEventListener("click", function () { CFG.shortcuts = {}; renderShortcuts(); refreshSave(); });

  /* ════════════════════════ advanced + save bar ════════════════════════ */
  q("adv-toggle").addEventListener("click", function () {
    var t = q("adv-toggle"), o = t.getAttribute("aria-expanded") === "true";
    t.setAttribute("aria-expanded", String(!o));
    q("adv-body").classList.toggle("is-open", !o);
  });
  q("hist").addEventListener("input", function () {
    CFG.historyLimit = parseInt(this.value, 10);
    q("hist-v").textContent = String(CFG.historyLimit);
    refreshSave();
  });

  var savebar = q("savebar"), saveDot = q("save-dot"), saveMsg = q("save-msg"), savedTimer = null;
  function cfgDirty() { return JSON.stringify(CFG) !== JSON.stringify(CFG0); }
  function refreshSave() {
    if (savedTimer) { clearTimeout(savedTimer); savedTimer = null; }
    savebar.classList.toggle("is-open", cfgDirty());
    saveDot.removeAttribute("data-state");
    saveMsg.textContent = "Unsaved changes";
    q("save").disabled = false; q("discard").disabled = false;
  }
  q("save").addEventListener("click", function () {
    saveMsg.textContent = "Saving…"; q("save").disabled = true; q("discard").disabled = true;
    setTimeout(function () {
      CFG0 = clone(CFG);
      saveDot.setAttribute("data-state", "saved");
      saveMsg.textContent = "Saved";
      savedTimer = setTimeout(function () { savebar.classList.remove("is-open"); }, 1200);
    }, REDUCED ? 0 : 420);
  });
  q("discard").addEventListener("click", function () {
    CFG = clone(CFG0);
    q("hist").value = String(CFG.historyLimit); q("hist-v").textContent = String(CFG.historyLimit);
    renderBehavior(); paintGaps(); paintSnap(); renderRatio(); renderShortcuts(); renderCanvas();
    refreshSave(); savebar.classList.remove("is-open");
  });

  /* ════════════════════════ sidebar search ════════════════════════ */
  var navSearch = q("nav-search");
  navSearch.addEventListener("input", function () {
    var qy = navSearch.value.trim().toLowerCase();
    var navs = [].slice.call(document.querySelectorAll(".snav"));
    var labels = [].slice.call(document.querySelectorAll("[data-group]"));
    var any = false;
    navs.forEach(function (nav, i) {
      var shown = 0;
      [].slice.call(nav.querySelectorAll("a")).forEach(function (a) {
        var hay = (a.textContent + " " + (a.getAttribute("data-keywords") || "")).toLowerCase();
        var hit = !qy || hay.indexOf(qy) >= 0;
        a.classList.toggle("is-filtered-out", !hit);
        if (hit) shown++;
      });
      nav.style.display = shown ? "" : "none";
      if (labels[i]) labels[i].style.display = shown ? "" : "none";
      if (shown) any = true;
    });
    q("nav-empty-q").textContent = navSearch.value;
    q("nav-empty").classList.toggle("is-open", !any);
  });
  document.addEventListener("keydown", function (e) {
    if (e.key !== "/" || /^(INPUT|SELECT|TEXTAREA)$/.test(document.activeElement.tagName)) return;
    e.preventDefault(); navSearch.focus();
  });

  /* ════════════════════════ boot ════════════════════════ */
  renderTabs();
  renderProfiles();
  renderBehavior();
  renderShortcuts();
  requestAnimationFrame(function () {
    renderCanvas(); renderInspector(); buildGaps(); buildSnap(); renderRatio(); paintReview();
  });
  var ro = new ResizeObserver(function () {
    if (liveGap || liveSnap || liveStop >= 0) return;   /* never reflow mid-drag */
    scheduleCanvas(); paintSnap(); renderRatio();
  });
  ro.observe(canvas); ro.observe(snapmap);
})();
