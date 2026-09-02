/* landing.js: landing v3 prototype behaviors (2026-09-01).
   Reveal-on-scroll (once), ARIA tabs (hero demo · features · install), hero demo
   auto-advance with per-tab plate pans, OS auto-detect for the download CTAs,
   copy buttons, the DIY-stack collapse (the page's one authored motion), install
   step numbering, and an opt-in annotation mode (?annotate) that labels every
   stand-in asset. Plain JS, no framework, no scroll listeners. */
(function () {
  "use strict";

  var root = document.documentElement;
  var reduceMotion =
    window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  if (reduceMotion) root.classList.add("reduce-motion");
  if (/[?#]annotate/.test(window.location.href)) root.classList.add("is-annotated");

  /* ---------- reveal on scroll (once) ---------- */
  var revealables = Array.prototype.slice.call(document.querySelectorAll("[data-reveal]"));
  function showAll(nodes) {
    nodes.forEach(function (n) {
      n.classList.add("is-in");
    });
  }
  if (reduceMotion || !("IntersectionObserver" in window)) {
    showAll(revealables);
  } else {
    var revealIO = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (en) {
          if (en.isIntersecting) {
            en.target.classList.add("is-in");
            revealIO.unobserve(en.target);
          }
        });
      },
      { threshold: 0.12, rootMargin: "0px 0px -6% 0px" }
    );
    revealables.forEach(function (n) {
      revealIO.observe(n);
    });
  }

  /* ---------- OS auto-detect → download CTA label + href ---------- */
  function detectOS() {
    var ua = navigator.userAgent || "";
    var plat =
      (navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || "";
    if (/Android|iPhone|iPad|iPod/i.test(ua)) return "unknown";
    if (/Mac/i.test(plat) || /Mac OS X/i.test(ua)) return "macOS";
    if (/Win/i.test(plat)) return "Windows";
    if (/Linux/i.test(plat) || /Linux/i.test(ua)) return "Linux";
    return "unknown";
  }
  var os = detectOS();
  var desktopOS = os === "macOS" || os === "Linux";
  document.querySelectorAll("[data-download]").forEach(function (el) {
    var kind = el.getAttribute("data-download");
    var label = el.querySelector("[data-label]");
    var text = "Download";
    if (kind === "final") text = "Download CompozyOS";
    else if (desktopOS) text = "Download for " + os;
    if (label) label.textContent = text;
    // Windows / unknown → the trust page; macOS / Linux → direct artifact (real URL at execution).
    el.setAttribute("href", desktopOS ? "#download-" + os.toLowerCase() : "#download");
    el.setAttribute("data-os", os);
  });
  document.querySelectorAll("[data-os-note]").forEach(function (el) {
    el.textContent = os === "unknown" ? "" : "Detected " + os + ".";
  });

  /* ---------- ARIA tabs ---------- */
  function initTabs(list) {
    var tabs = Array.prototype.slice.call(list.querySelectorAll('[role="tab"]'));
    var single = list.hasAttribute("data-single");
    function select(tab, focus, auto) {
      tabs.forEach(function (t) {
        var on = t === tab;
        t.setAttribute("aria-selected", on ? "true" : "false");
        t.tabIndex = on ? 0 : -1;
        var panel = document.getElementById(t.getAttribute("aria-controls"));
        if (!panel) return;
        if (single) {
          if (on) panel.setAttribute("aria-labelledby", t.id);
        } else {
          panel.hidden = !on;
          if (on) showAll(Array.prototype.slice.call(panel.querySelectorAll("[data-reveal]")));
        }
      });
      if (focus) tab.focus();
      list.dispatchEvent(
        new CustomEvent("tabchange", { detail: { tab: tab, index: tabs.indexOf(tab), auto: !!auto } })
      );
    }
    tabs.forEach(function (t) {
      t.addEventListener("click", function () {
        select(t, false, false);
      });
      t.addEventListener("keydown", function (e) {
        var i = tabs.indexOf(t);
        var n = null;
        if (e.key === "ArrowRight") n = tabs[(i + 1) % tabs.length];
        else if (e.key === "ArrowLeft") n = tabs[(i - 1 + tabs.length) % tabs.length];
        else if (e.key === "Home") n = tabs[0];
        else if (e.key === "End") n = tabs[tabs.length - 1];
        if (n) {
          e.preventDefault();
          select(n, true, false);
        }
      });
    });
    return { list: list, tabs: tabs, select: select };
  }
  var tablists = {};
  document.querySelectorAll('[role="tablist"]').forEach(function (list) {
    tablists[list.getAttribute("aria-label")] = initTabs(list);
  });

  /* ---------- hero demo: six clips, auto-advance, paused on interaction ---------- */
  var demo = document.querySelector(".demo");
  var demoTabs = tablists["Product demos"];
  if (demo && demoTabs) {
    var CLIP_MS = 7000; // stands in for each clip's real duration (≤40s)
    var playing = false;
    var startedAt = 0;
    var elapsed = 0;
    var raf = 0;
    var wasPlaying = false;
    var current = demoTabs.tabs[0];
    var $ = function (sel) {
      return demo.querySelector(sel);
    };
    var plateImg = $(".plate__img");

    function apply(tab) {
      current = tab;
      $("[data-demo-route]").textContent = tab.getAttribute("data-route");
      $("[data-demo-caption]").textContent = tab.getAttribute("data-caption");
      if (plateImg) {
        plateImg.style.setProperty("--pos", tab.getAttribute("data-pos") || "50% 0%");
        plateImg.style.setProperty("--zoom", tab.getAttribute("data-zoom") || "1");
      }
      demoTabs.tabs.forEach(function (t) {
        t.style.setProperty("--p", "0");
      });
    }
    function setPlaying(v) {
      playing = v;
      demo.setAttribute("data-playing", v ? "true" : "false");
      demo.querySelectorAll("[data-demo-toggle]").forEach(function (b) {
        b.setAttribute("aria-label", v ? "Pause demo" : "Play demo");
      });
      var l = $("[data-demo-toggle-label]");
      if (l) l.textContent = v ? "Pause" : "Play";
      cancelAnimationFrame(raf);
      if (v) {
        startedAt = performance.now() - elapsed;
        raf = requestAnimationFrame(tick);
      }
    }
    function tick(now) {
      if (!playing) return;
      elapsed = now - startedAt;
      if (elapsed >= CLIP_MS) {
        elapsed = 0;
        startedAt = now;
        var i = demoTabs.tabs.indexOf(current);
        demoTabs.select(demoTabs.tabs[(i + 1) % demoTabs.tabs.length], false, true);
      }
      current.style.setProperty("--p", String(Math.min(1, elapsed / CLIP_MS)));
      raf = requestAnimationFrame(tick);
    }
    demoTabs.list.addEventListener("tabchange", function (e) {
      elapsed = 0;
      startedAt = performance.now();
      apply(e.detail.tab);
      if (!e.detail.auto && playing) setPlaying(false); // interaction pauses auto-advance
    });
    demo.querySelectorAll("[data-demo-toggle]").forEach(function (b) {
      b.addEventListener("click", function () {
        setPlaying(!playing);
      });
    });
    $("[data-demo-replay]").addEventListener("click", function () {
      elapsed = 0;
      demoTabs.select(demoTabs.tabs[0], false, true);
      setPlaying(true);
    });
    var plate = $(".plate");
    plate.addEventListener("pointerenter", function () {
      wasPlaying = playing;
      if (playing) setPlaying(false);
    });
    plate.addEventListener("pointerleave", function () {
      if (wasPlaying && !playing) setPlaying(true);
    });
    document.addEventListener("visibilitychange", function () {
      if (document.hidden && playing) {
        wasPlaying = true;
        setPlaying(false);
      }
    });
    if (!reduceMotion && "IntersectionObserver" in window) {
      var seen = false;
      var io = new IntersectionObserver(
        function (entries) {
          entries.forEach(function (en) {
            if (en.isIntersecting && !seen) {
              seen = true;
              setPlaying(true);
            } else if (!en.isIntersecting && playing) {
              wasPlaying = true;
              setPlaying(false);
            } else if (en.isIntersecting && seen && wasPlaying && !playing) {
              setPlaying(true);
            }
          });
        },
        { threshold: 0.45 }
      );
      io.observe(plate);
    }
    apply(current);
  }

  /* ---------- copy buttons ---------- */
  document.querySelectorAll("[data-copy]").forEach(function (btn) {
    var label = btn.querySelector("[data-copy-label]");
    var timer = 0;
    function done(state) {
      btn.setAttribute("data-state", state);
      if (label) label.textContent = state === "copied" ? "Copied" : "Copy failed";
      clearTimeout(timer);
      timer = setTimeout(function () {
        btn.removeAttribute("data-state");
        if (label) label.textContent = "Copy";
      }, 1500);
    }
    btn.addEventListener("click", function () {
      var text = btn.getAttribute("data-copy");
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(
          function () {
            done("copied");
          },
          function () {
            done("failed");
          }
        );
      } else {
        done("failed");
      }
    });
  });

  /* ---------- DIY stack → CompozyOS collapse (the one authored motion) ---------- */
  var stage = document.querySelector("[data-pain]");
  if (stage) {
    var parts = Array.prototype.slice.call(stage.querySelectorAll(".part"));
    if (reduceMotion || !("IntersectionObserver" in window)) {
      stage.classList.add("pain__stage--static");
    } else {
      var measure = function () {
        var s = stage.getBoundingClientRect();
        var cx = s.left + s.width / 2;
        var cy = s.top + s.height / 2;
        parts.forEach(function (c) {
          var r = c.getBoundingClientRect();
          c.style.setProperty("--dx", cx - (r.left + r.width / 2) + "px");
          c.style.setProperty("--dy", cy - (r.top + r.height / 2) + "px");
        });
      };
      var collapse = function () {
        measure();
        stage.classList.add("is-collapsed");
      };
      var replay = function () {
        stage.classList.remove("is-collapsed");
        requestAnimationFrame(function () {
          requestAnimationFrame(function () {
            setTimeout(collapse, 800);
          });
        });
      };
      var painIO = new IntersectionObserver(
        function (entries) {
          entries.forEach(function (en) {
            if (en.isIntersecting) {
              setTimeout(collapse, 900);
              painIO.disconnect();
            }
          });
        },
        { threshold: 0.55 }
      );
      painIO.observe(stage);
      var replayBtn = stage.querySelector("[data-pain-replay]");
      if (replayBtn) replayBtn.addEventListener("click", replay);
    }
  }

  /* ---------- install: npm / Go add the bootstrap step ---------- */
  var installTabs = tablists["Install methods"];
  if (installTabs) {
    var bootstrap = document.querySelector('[data-step="bootstrap"]');
    var numbered = document.querySelectorAll("[data-step-n]");
    installTabs.list.addEventListener("tabchange", function (e) {
      var needsBootstrap = e.detail.tab.id === "it-npm" || e.detail.tab.id === "it-go";
      if (bootstrap) bootstrap.hidden = !needsBootstrap;
      numbered.forEach(function (n, i) {
        n.textContent = String(i + (needsBootstrap ? 2 : 1));
      });
    });
  }
})();
