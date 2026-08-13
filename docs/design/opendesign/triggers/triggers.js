/* Shared interactions for trigger detail finals. Optional nodes are skipped. */
(function () {
  var win = document.getElementById("win");
  var toastEl = document.getElementById("toast");
  var live = document.getElementById("live");
  var toastTimer;
  var kind = document.body.getAttribute("data-target-kind") || "agent";
  var tname = document.body.getAttribute("data-target-name") || "the target";

  function toast(msg) {
    if (!toastEl) return;
    toastEl.textContent = msg;
    toastEl.classList.add("is-on");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { toastEl.classList.remove("is-on"); }, 2200);
  }

  function setEnabled(on) {
    var sw = document.getElementById("enableSwitch");
    var track = document.getElementById("enableTrack");
    var lab = document.getElementById("enableLab");
    var menuLab = document.getElementById("menuToggleLabel");
    if (sw) sw.setAttribute("aria-checked", on ? "true" : "false");
    if (track) track.classList.toggle("is-on", on);
    if (lab) lab.textContent = on ? "Enabled" : "Disabled";
    if (win) win.classList.toggle("is-paused", !on);
    if (menuLab) menuLab.textContent = on ? "Disable" : "Enable";
    if (live) {
      live.textContent = on
        ? "Trigger enabled."
        : (kind === "loop"
          ? "Trigger disabled. Matching events will not start " + tname + "."
          : "Trigger disabled. Matching events will not run " + tname + ".");
    }
    toast(on ? "Trigger enabled" : "Trigger disabled");
  }

  var sw = document.getElementById("enableSwitch");
  if (sw) {
    sw.addEventListener("click", function (e) {
      e.stopPropagation();
      if (sw.getAttribute("aria-disabled") === "true" || sw.getAttribute("aria-busy") === "true") return;
      setEnabled(sw.getAttribute("aria-checked") !== "true");
    });
  }

  var menuBtn = document.getElementById("moreBtn");
  var menu = document.getElementById("moreMenu");
  function closeMenu() {
    if (!menu || !menuBtn) return;
    menu.classList.remove("is-open");
    menuBtn.setAttribute("aria-expanded", "false");
  }
  if (menuBtn && menu) {
    menuBtn.addEventListener("click", function (e) {
      e.stopPropagation();
      var open = menu.classList.toggle("is-open");
      menuBtn.setAttribute("aria-expanded", open ? "true" : "false");
    });
    document.addEventListener("click", closeMenu);
    menu.addEventListener("click", function (e) { e.stopPropagation(); });
  }
  var menuToggle = document.getElementById("menuToggle");
  if (menuToggle) {
    menuToggle.addEventListener("click", function () {
      if (sw) setEnabled(sw.getAttribute("aria-checked") !== "true");
      closeMenu();
    });
  }

  document.querySelectorAll("[data-copy]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var v = btn.getAttribute("data-copy");
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(v).then(function () { toast("Copied " + v); closeMenu(); }, function () { toast("Copied " + v); closeMenu(); });
      } else {
        toast("Copied " + v);
        closeMenu();
      }
    });
  });

  document.querySelectorAll("[data-toast]").forEach(function (btn) {
    btn.addEventListener("click", function (e) {
      e.stopPropagation();
      toast(btn.getAttribute("data-toast"));
    });
  });

  var prompt = document.getElementById("promptPreview");
  var promptBtn = document.getElementById("promptToggle");
  if (prompt && promptBtn) {
    promptBtn.addEventListener("click", function () {
      var open = prompt.getAttribute("data-open") === "true";
      prompt.setAttribute("data-open", open ? "false" : "true");
      promptBtn.setAttribute("aria-expanded", open ? "false" : "true");
      promptBtn.textContent = open ? "Show full prompt" : "Hide prompt";
    });
  }

  document.querySelectorAll(".runrow").forEach(function (row) {
    row.addEventListener("click", function () {
      var id = row.getAttribute("aria-controls");
      var draw = document.getElementById(id);
      var open = row.getAttribute("aria-expanded") === "true";
      document.querySelectorAll(".runrow").forEach(function (r) {
        r.setAttribute("aria-expanded", "false");
        var d = document.getElementById(r.getAttribute("aria-controls"));
        if (d) d.classList.remove("is-open");
      });
      if (!open && draw) {
        row.setAttribute("aria-expanded", "true");
        draw.classList.add("is-open");
      }
    });
  });

  var sheet = document.getElementById("inspectSheet");
  var scrim = document.getElementById("scrim");
  var sheetReturn = null;
  function openSheet(open) {
    if (!sheet || !scrim) return;
    sheet.classList.toggle("is-open", open);
    scrim.classList.toggle("is-open", open);
    if (win) win.toggleAttribute("inert", open);
    if (open) {
      sheetReturn = document.activeElement;
      var closeBtn = sheet.querySelector("[data-close]");
      if (closeBtn) closeBtn.focus();
    } else if (sheetReturn && sheetReturn.focus) {
      sheetReturn.focus();
    }
  }
  var openInspect = document.getElementById("openInspect");
  if (openInspect) openInspect.addEventListener("click", function () { openSheet(true); });
  if (scrim) scrim.addEventListener("click", function () { openSheet(false); });
  if (sheet) {
    var closeBtn = sheet.querySelector("[data-close]");
    if (closeBtn) closeBtn.addEventListener("click", function () { openSheet(false); });
  }

  document.querySelectorAll("[data-insp]").forEach(function (tab) {
    tab.addEventListener("click", function () {
      document.querySelectorAll("[data-insp]").forEach(function (t) {
        t.classList.toggle("is-on", t === tab);
        t.setAttribute("aria-selected", t === tab ? "true" : "false");
      });
      var pane = tab.getAttribute("data-insp");
      var diag = document.getElementById("insp-diag");
      var env = document.getElementById("insp-env");
      if (diag) diag.hidden = pane !== "diag";
      if (env) env.hidden = pane !== "env";
    });
  });

  var delScrim = document.getElementById("deleteScrim");
  var delReturn = null;
  function openDelete(open) {
    if (!delScrim) return;
    delScrim.classList.toggle("is-open", open);
    if (win) win.toggleAttribute("inert", open);
    if (open) {
      delReturn = document.activeElement;
      var cancel = document.getElementById("delCancel");
      if (cancel) cancel.focus();
    } else if (delReturn && delReturn.focus) {
      delReturn.focus();
    }
  }
  var menuDelete = document.getElementById("menuDelete");
  if (menuDelete) menuDelete.addEventListener("click", function () { closeMenu(); openDelete(true); });
  var delCancel = document.getElementById("delCancel");
  if (delCancel) delCancel.addEventListener("click", function () { openDelete(false); });
  if (delScrim) delScrim.addEventListener("click", function (e) { if (e.target === delScrim) openDelete(false); });
  var delConfirm = document.getElementById("delConfirm");
  if (delConfirm) {
    delConfirm.addEventListener("click", function () {
      openDelete(false);
      window.location.href = delConfirm.getAttribute("data-href") || "../empty-states/triggers-empty-suggestions.html";
    });
  }

  document.addEventListener("keydown", function (e) {
    if (e.key !== "Escape") return;
    if (delScrim && delScrim.classList.contains("is-open")) { openDelete(false); return; }
    if (sheet && sheet.classList.contains("is-open")) { openSheet(false); return; }
    closeMenu();
  });
})();
