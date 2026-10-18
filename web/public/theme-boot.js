// Pre-paint theme boot (D3). A blocking same-origin script, not inline: the
// daemon's CSP is `script-src 'self'`. Mirrors web/src/systems/theme
// (THEME_STORAGE_KEY, DEFAULT_THEME_PREFERENCE, applyTheme) — change together.
(function () {
  var preference = "dark";
  try {
    var stored = window.localStorage.getItem("compozy.theme");
    if (stored === "light" || stored === "dark" || stored === "system") preference = stored;
  } catch {
    // Storage unavailable: keep the default.
  }
  var theme = preference;
  if (preference === "system") {
    theme =
      typeof window.matchMedia === "function" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light";
  }
  var root = document.documentElement;
  root.dataset.theme = theme;
  root.classList.toggle("dark", theme === "dark");
  root.style.colorScheme = theme;
  var meta = document.querySelector('meta[name="theme-color"]');
  if (meta) meta.setAttribute("content", theme === "dark" ? "#0a0a0a" : "#f2f2f3");
})();
