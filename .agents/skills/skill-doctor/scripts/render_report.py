#!/usr/bin/env python3
"""Render a skill-doctor report.json into one shareable HTML report.

Output (next to report.json):
  report.html - scorecard, findings, and suggested skill edits in a single
                self-contained page, with a "share as png" button that draws
                a 1200x675 share image client-side and downloads it.

Python 3.9+, stdlib only. Uses system fonts so the page and the exported PNG
render the same everywhere.
"""

import argparse
import base64
import html
import json
import re
import sys
import webbrowser
from datetime import datetime, timezone
from pathlib import Path

GRADES = [
    (0.97, "A+"), (0.93, "A"), (0.90, "A-"),
    (0.87, "B+"), (0.83, "B"), (0.80, "B-"),
    (0.77, "C+"), (0.73, "C"), (0.70, "C-"),
    (0.60, "D"), (0.0, "F"),
]

DIFFS_BUNDLE_PATH = (
    Path(__file__).resolve().parent.parent / "assets" / "pierre-diffs.js"
)

# Collapsed height of a diff before the "show more" toggle takes over.
DIFF_CLAMP_PX = 320


def grade_for(score: float) -> str:
    for threshold, letter in GRADES:
        if score >= threshold:
            return letter
    return "F"


def pct(score) -> int:
    return round(float(score) * 100)


def format_generated_at(value) -> str:
    if not value:
        return ""
    raw = str(value)
    normalized = raw[:-1] + "+00:00" if raw.endswith("Z") else raw
    if re.search(r"[+-]\d{2}$", normalized):
        normalized += ":00"
    try:
        generated_at = datetime.fromisoformat(normalized)
    except ValueError:
        return raw
    suffix = ""
    if generated_at.tzinfo is not None:
        generated_at = generated_at.astimezone(timezone.utc)
        suffix = " UTC"
    time = generated_at.strftime("%I:%M %p").lstrip("0")
    return (
        f"{generated_at.strftime('%B')} {generated_at.day}, "
        f"{generated_at.year} at {time}{suffix}"
    )


def open_report(report_path: Path) -> bool:
    try:
        return bool(webbrowser.open(report_path.absolute().as_uri(), new=2))
    except (OSError, webbrowser.Error):
        return False


def esc(v) -> str:
    value = v if v is not None else ""
    return html.escape(str(value))


def render_diff(diff_text: str, proposed_path: str = "") -> str:
    if not diff_text:
        return ""
    encoded = base64.b64encode(diff_text.encode("utf-8")).decode("ascii")
    filename = Path(proposed_path).name if proposed_path else "SKILL.md"
    return (
        '<div class="diff-wrap" data-collapsed="true">'
        f'<div class="diff-view" data-pierre-diff data-diff="{encoded}" '
        f'data-filename="{esc(filename)}">'
        f'<pre class="diff-fallback">{esc(diff_text)}</pre></div>'
        '<button class="diff-toggle" type="button" hidden>show more</button>'
        "</div>"
    )


def embedded_diffs_script() -> str:
    if not DIFFS_BUNDLE_PATH.exists():
        raise RuntimeError(
            f"@pierre/diffs bundle missing: {DIFFS_BUNDLE_PATH}; "
            "restore it from warpdotdev/skill-doctor, which builds the bundle "
            "with `pnpm build:diffs`"
        )
    bundle = DIFFS_BUNDLE_PATH.read_text()
    return re.sub(r"</script", r"<\\/script", bundle, flags=re.IGNORECASE)


# Warp pixel mark (../assets/warp-pixel-icon.svg), inlined so the page stays
# self-contained. The same path data is redrawn on canvas for the share image.
WARP_VIEWBOX = (37, 35)
WARP_PATHS = [
    ("M5.3135 2L30.9247 2.00011L30.9208 3.79847L32.5185 3.79657L32.5145 5.43448L34.2294 5.44055L34.2286 28.6954H32.5239C32.507 29.1933 32.5153 29.7328 32.5106 30.2357L30.9319 30.2411C30.9297 30.4979 30.9757 31.7709 30.8834 31.8934C28.193 31.9264 25.4541 31.9005 22.7582 31.9013H5.30484L5.30653 30.2425L3.72927 30.2364L3.73053 28.6969L2 28.6933L2.0009 5.43272C2.57577 5.43872 3.15074 5.4375 3.72561 5.42899L3.73161 3.79621L5.30915 3.79222L5.3135 2Z", "#ffffff"),
    ("M32.5146 5.43457L32.5186 3.79688L30.9209 3.79883L30.9248 2H5.31348L5.30957 3.79199L3.73145 3.7959L3.72559 5.42871C3.15075 5.43722 2.57581 5.43861 2.00098 5.43262L2 28.6934L3.73047 28.6973L3.72949 30.2363L5.30664 30.2422L5.30469 31.9014H22.7578C24.7798 31.9008 26.8265 31.9149 28.8584 31.9082L30.8838 31.8936C30.976 31.7707 30.9295 30.4984 30.9316 30.2412L32.5107 30.2354C32.5154 29.7326 32.5066 29.1931 32.5234 28.6953H34.2285L34.2295 5.44043L32.5146 5.43457ZM36.2285 30.6953H34.5068L34.4922 32.2285L32.8643 32.2334C32.8528 32.2884 32.8385 32.3523 32.8184 32.4209C32.7937 32.5048 32.7066 32.7965 32.4805 33.0967L31.8896 33.8809L30.9082 33.8936C28.2026 33.9268 25.4275 33.9007 22.7588 33.9014H3.30273L3.30371 32.2344L1.72754 32.2285L1.72949 30.6924L0 30.6895L0.000976562 3.41211L1.7334 3.42969L1.73926 1.80078L3.31348 1.79785L3.31836 0H32.9287L32.9248 1.79688L34.5234 1.79395L34.5186 3.44043L36.2295 3.44727L36.2285 30.6953Z", "#000000"),
    ("M29.3721 5.42529C29.889 5.44429 30.4337 5.42268 30.96 5.43213L30.9551 7.04248C31.4775 7.03408 32.01 7.03929 32.5332 7.03857C32.4937 9.42093 32.5257 11.8903 32.5254 14.2798L32.5273 27.1108L30.9609 27.1089L30.959 28.7026L29.375 28.6987C29.3772 29.13 29.3813 29.5667 29.373 29.9976C29.3705 30.1337 29.3832 30.1651 29.3057 30.2358L6.91699 30.2378C6.89889 29.7353 6.91168 29.2118 6.91699 28.7075C6.3669 28.7025 5.8167 28.7025 5.2666 28.7075L5.26465 27.1099L3.68457 27.1089L3.68652 7.04639C4.2055 7.03529 4.7404 7.03916 5.26074 7.03564L5.2666 5.43018C5.80821 5.42385 6.35003 5.42572 6.8916 5.43506C6.88988 4.88796 6.892 4.34052 6.89746 3.79346H29.3711L29.3721 5.42529ZM9.33887 10.6978C9.18647 10.9765 9.21901 11.161 9.22461 11.4819C9.07998 11.4801 8.94005 11.4569 8.8291 11.5347C8.80072 11.622 8.80582 11.6213 8.81152 11.7144C8.68917 11.8074 8.60774 11.7932 8.44434 11.7866C8.36301 11.8515 8.30578 11.9057 8.30176 12.0259C8.28478 12.536 8.29109 13.0721 8.29102 13.5825L8.29297 21.5659C8.29326 22.3844 8.28546 23.2155 8.30371 24.0337C8.30778 24.2156 8.35142 24.2999 8.43848 24.4575C8.64083 24.5041 8.98427 24.4882 9.2041 24.4878C9.19663 24.7586 9.20523 25.128 9.30859 25.3823C9.48375 25.4631 17.0821 25.4211 17.8965 25.4204C17.9026 25.0264 17.915 24.6167 17.9082 24.2241H16.7715C15.5491 24.2241 14.2971 24.2119 13.0771 24.228C13.0791 24.0268 13.0716 23.637 13.1133 23.4565C13.2509 23.3435 13.2926 23.4911 13.3193 23.3413C13.3427 23.2103 13.2843 23.1435 13.3555 23.0181L13.501 22.9917C13.5902 22.8227 13.538 22.0611 13.5391 21.8169L13.9902 21.813C13.989 21.1612 13.9793 20.4819 13.9971 19.8325L14.5029 19.8267C14.5003 19.1758 14.5017 18.5244 14.5068 17.8735L14.9639 17.8696C14.9614 17.3226 14.861 16.4429 15.1162 16.0063C15.2178 15.9719 15.2439 15.9747 15.3477 15.9692C15.4618 15.8341 15.4034 14.2978 15.4043 14.0024L15.9121 14.0005C15.9243 13.3773 15.9407 12.7118 15.9258 12.0903L16.3555 12.0786L16.3506 10.6968C14.0515 10.6966 11.6291 10.6614 9.33887 10.6978ZM18.3584 8.38721C18.3588 8.86591 18.3663 9.36324 18.3584 9.84033L17.9102 9.84229L17.9043 11.48L17.375 11.478L17.374 14.0005L16.8447 14.0015L16.8418 15.9761L16.3652 15.981C16.3592 16.2914 16.413 17.4264 16.3115 17.5913C16.2316 17.6037 16.1517 17.6171 16.0723 17.6323C16.0557 17.7205 16.0531 17.753 16.0479 17.8394C15.9931 17.8723 15.9824 17.8775 15.9229 17.8999C15.8611 18.2051 15.899 19.4273 15.8906 19.8267L15.415 19.8335C15.4087 20.1999 15.4041 21.3175 15.3438 21.604C15.1385 21.7756 14.9409 21.8339 14.9404 22.0278C14.9396 22.3503 14.9419 22.6858 14.9414 23.0083L26.9736 23.0093C26.9722 22.6145 27.0284 22.4491 27.1084 22.0679C27.2287 21.9942 27.4175 22.067 27.4541 22.0269C27.6718 21.7854 27.5123 21.8049 27.9785 21.8228L27.9805 13.7983C27.9805 12.5388 28.0332 10.7775 27.96 9.54639C27.8386 9.54865 27.6757 9.56604 27.5723 9.51611C27.5224 9.2171 27.4479 9.21398 27.1523 9.16064C26.9526 8.99617 26.9654 8.6242 26.9736 8.38623L18.3584 8.38721Z", "#000000"),
]
WARP_MARK = (
    f'<svg class="mark" viewBox="0 0 {WARP_VIEWBOX[0]} {WARP_VIEWBOX[1]}" fill="none" '
    'aria-hidden="true" xmlns="http://www.w3.org/2000/svg">'
    + "".join(f'<path d="{d}" fill="{fill}"/>' for d, fill in WARP_PATHS)
    + "</svg>"
)

# Sticky report footer.
STAMP_NAME = "Automatically improve your skills with Warp Factories"
STAMP_SUB = "continuous scoring \u00b7 continuous skill tuning"

# Attribution shown only in the exported share image.
SHARE_STAMP_NAME = "Get your report with /skill-doctor"
SHARE_STAMP_SUB = "warp.dev/skill-doctor"

# Design tokens lifted from warp.dev/factories (factories-landing.css):
# white ground with a dot grid, Matter-Mono-ish monospace, #2a1eff accent,
# hairline rgba(13,10,61) rules, square corners, lowercase labels,
# uppercase wide-tracked meta bars.
PAGE_CSS = """
* { box-sizing: border-box; }
body {
  --mono-font: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  --fg: #1a1522; --muted: #5d5966; --muted-2: #918d9a; --accent: #2a1eff;
  --line: rgba(13, 10, 61, 0.16); --line-soft: rgba(13, 10, 61, 0.07);
  --page-bg: #fff; --surface: #fff; --bg-panel: #f6f5fb; --yellow: #eef17c;
  --button-fg: #1a1522;
  --footer-shadow: rgba(13, 10, 61, 0.12);
  font-family: var(--mono-font);
  background: radial-gradient(circle at 1px 1px, var(--line-soft) 1px, transparent 0) 0 0 / 22px 22px, var(--page-bg);
  color: var(--fg); max-width: 900px; margin: 0 auto; padding: 48px 24px;
  line-height: 1.65; font-size: 13px; color-scheme: light;
}
@media (prefers-color-scheme: dark) {
  body {
    --fg: #f4f1f8; --muted: #bbb5c2; --muted-2: #928b9b; --accent: #9188ff;
    --line: rgba(239, 235, 255, 0.2); --line-soft: rgba(239, 235, 255, 0.08);
    --page-bg: #0f0d14; --surface: #17141d; --bg-panel: #211d29;
    --footer-shadow: rgba(0, 0, 0, 0.45);
    color-scheme: dark;
  }
}
::selection { background: var(--accent); color: #fff; }
h1 { font-weight: 500; letter-spacing: -2px; font-size: 34px; margin: 4px 0 0; }
h2 { font-weight: 500; letter-spacing: -1px; font-size: 20px; margin: 40px 0 8px; }
p { color: var(--muted); font-weight: 500; }
a { color: var(--accent); }
code { background: var(--bg-panel); border: 1px solid var(--line-soft); padding: 1px 5px; }
li { margin-bottom: 10px; }
.tag { font-size: 11px; color: var(--accent); text-transform: lowercase; }
.tag::before { content: "# "; }
.muted { color: var(--muted-2); font-size: 12px; }
.stamp { display: flex; align-items: center; gap: 11px; }
.stamp .mark { width: 27px; height: 26px; flex: none; display: block; }
.stamp-name { font-size: 15px; font-weight: 600; letter-spacing: -0.03em; }
.stamp-sub { font-size: 11px; color: var(--muted-2); text-transform: lowercase; letter-spacing: 0.02em; }
.stamp-row { border: 1px solid var(--line); background: var(--surface); padding: 12px 16px; }
.factories-footer { position: sticky; bottom: 16px; z-index: 20; margin-top: 40px;
  box-shadow: 0 8px 24px var(--footer-shadow); }
.row { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.title-row { margin-top: 4px; }
.title-row h1 { margin: 0; }
.cta-button { font-family: inherit; font-size: 13px; font-weight: 600; color: var(--button-fg);
  background: var(--yellow); border: 1px solid var(--button-fg); padding: 8px 14px;
  text-decoration: none; white-space: nowrap; flex: none; cursor: pointer; }
.cta-button:hover { background: #f4f79f; }
.cta-button[disabled] { cursor: default; opacity: 0.65; }
.scorecard { display: flex; align-items: center; gap: 48px; border: 1px solid var(--line);
  background: var(--surface); padding: 26px 28px; margin-top: 20px; }
.grade-col { text-align: center; flex: none; width: 170px; }
.grade { font-size: 96px; font-weight: 600; line-height: 1; letter-spacing: -5px; color: var(--accent); }
.grade-label { font-size: 11px; color: var(--muted-2); margin-top: 8px; text-transform: uppercase; letter-spacing: 0.14em; }
.bars { flex: 1; display: flex; flex-direction: column; gap: 20px; min-width: 0; }
.bar-head { display: flex; justify-content: space-between; font-size: 13px; margin-bottom: 7px; font-weight: 500; }
.bar-name { text-transform: lowercase; }
.bar-val { font-weight: 600; font-variant-numeric: tabular-nums; }
.bar-track { height: 8px; background: var(--line-soft); box-shadow: inset 0 0 0 1px var(--line); }
.bar-fill { height: 100%; background: var(--accent);
  animation: skill-doctor-fill 700ms cubic-bezier(0.22, 1, 0.36, 1) var(--metric-delay) both;
  transform-origin: left; }
.stats { display: grid; grid-template-columns: repeat(3, 1fr); border: 1px solid var(--line);
  border-top: none; background: var(--bg-panel); }
.stat { padding: 16px 24px 14px; border-left: 1px solid var(--line); }
.stat:first-child { border-left: none; }
.stat .num { font-size: 34px; font-weight: 600; letter-spacing: -0.02em; font-variant-numeric: tabular-nums; }
.stat .lbl { font-size: 12px; color: var(--muted); margin-top: 2px; text-transform: lowercase; }
.diff-wrap { margin: 10px 0 4px; }
.diff-view { display: grid; gap: 10px; max-width: 100%;
  --diffs-font-family: var(--mono-font); --diffs-header-font-family: var(--mono-font); }
.diff-view > * { min-width: 0; }
.diff-fallback { background: var(--bg-panel); border: 1px solid var(--line); padding: 13px 16px;
  color: var(--muted); font-size: 12px; line-height: 1.7; overflow-x: auto; margin: 0; white-space: pre; }
.diff-wrap[data-overflowing="true"][data-collapsed="true"] .diff-view {
  max-height: __CLAMP__px; overflow: hidden;
  -webkit-mask-image: linear-gradient(#000 calc(100% - 72px), transparent);
  mask-image: linear-gradient(#000 calc(100% - 72px), transparent);
}
.diff-toggle { font-family: inherit; font-size: 10px; font-weight: 600; letter-spacing: 0.1em;
  text-transform: uppercase; color: var(--accent); background: var(--surface);
  border: 1px solid var(--line); padding: 5px 10px; margin-top: 6px; cursor: pointer; }
.diff-toggle:hover { border-color: var(--accent); }
@keyframes skill-doctor-fill {
  from { transform: scaleX(0); }
  to { transform: scaleX(1); }
}
@media (prefers-reduced-motion: reduce) {
  .bar-fill { animation: none; }
}
"""


def render_page(r) -> str:
    scores = r["scores"]
    stats = r.get("stats", {})
    grade = r.get("grade") or grade_for(scores["overall"])
    generated_at = format_generated_at(r.get("generated_at"))

    bars = "".join(
        f'<div class="bar-row"><div class="bar-head"><span class="bar-name">{esc(name)}</span>'
        f'<span class="bar-val">{pct(val)}</span></div>'
        f'<div class="bar-track"><div class="bar-fill" '
        f'style="width:{pct(val)}%;--metric-delay:{180 + index * 110}ms"></div></div></div>'
        for index, (name, val) in enumerate([
            ("Efficiency", scores.get("efficiency", 0)),
            ("Code Quality", scores.get("code_quality", 0)),
            ("Skill Coverage", scores.get("skill_coverage", 0)),
        ])
    )
    stat_cells = "".join(
        f'<div class="stat"><div class="num">{esc(value)}</div><div class="lbl">{esc(label)}</div></div>'
        for value, label in [
            (stats.get("sessions_analyzed", 0), "conversations scored"),
            (stats.get("skills_found", 0), "skills installed"),
            (stats.get("skills_used", 0), "skills used"),
        ]
    )
    findings = "".join(f"<li>{esc(finding)}</li>" for finding in r.get("top_findings", []))
    suggestions = "".join(
        f"""<li><b><code>{esc(s.get('skill'))}</code></b> — {esc(s.get('change'))}
        {('<div class="muted">Evidence: ' + esc(s['evidence']) + '</div>') if s.get('evidence') else ''}
        {render_diff(s.get('diff', ''), s.get('proposed_path', ''))}</li>"""
        for s in r.get("suggestions", [])
    ) or "<li>No skill change cleared the bar for this window.</li>"

    card_data = json.dumps({
        "title": r.get("title", "Agent Skill Report"),
        "eyebrow": "skill-doctor",
        "handle": r.get("handle") or "agent skill report",
        "harness": r.get("harness", "codex"),
        "grade": grade,
        "grade_label": f"overall {pct(scores['overall'])}",
        "bars": [
            ["Efficiency", pct(scores.get("efficiency", 0))],
            ["Code Quality", pct(scores.get("code_quality", 0))],
            ["Skill Coverage", pct(scores.get("skill_coverage", 0))],
        ],
        "meta": f"{stats.get('sessions_scanned', 0)} conversations found \u00b7 "
                f"last {stats.get('window_days', 45)} days",
        "stats": [
            [str(stats.get("sessions_analyzed", 0)), "conversations scored"],
            [str(stats.get("skills_found", 0)), "skills installed"],
            [str(stats.get("skills_used", 0)), "skills used"],
        ],
        "stamp": [SHARE_STAMP_NAME, SHARE_STAMP_SUB],
        "paths": [{"d": d, "fill": fill} for d, fill in WARP_PATHS],
        "viewbox": list(WARP_VIEWBOX),
    })

    return f"""<!DOCTYPE html><html><head><meta charset="utf-8">
<meta name="color-scheme" content="light dark">
<title>{esc(r.get('title', 'Agent Skill Report'))}</title>
<style>{PAGE_CSS.replace('__CLAMP__', str(DIFF_CLAMP_PX))}</style></head><body>
<div class="tag">skill-doctor</div>
<div class="row title-row">
  <h1>{esc(r.get('title', 'Agent Skill Report'))}</h1>
  <button class="cta-button" id="share-png" type="button">Share</button>
</div>
<p class="muted">Generated {esc(generated_at)} &middot; harness: {esc(r.get('harness', 'codex'))}</p>
<div class="scorecard">
  <div class="grade-col"><div class="grade">{esc(grade)}</div>
    <div class="grade-label">overall {pct(scores['overall'])}</div></div>
  <div class="bars">{bars}</div>
</div>
<div class="stats">{stat_cells}</div>
<h2>Findings</h2><ul>{findings}</ul>
<h2>Suggested skill changes</h2><ol>{suggestions}</ol>
<div class="stamp-row row factories-footer">
  <div class="stamp">{WARP_MARK}<div>
    <div class="stamp-name">{esc(STAMP_NAME)}</div>
    <div class="stamp-sub">{esc(STAMP_SUB)}</div>
  </div></div>
  <a class="cta-button" href="{esc(r.get('cta_url', 'https://warp.dev/factories/request-access'))}">Request access</a>
</div>
<script>{embedded_diffs_script()}</script>
<script>{page_script(card_data)}</script>
</body></html>"""


def page_script(card_data: str) -> str:
    """Diff collapsing plus a canvas-drawn 1200x675 share image."""
    script = r"""
(function () {
  var CARD = __CARD__;
  var CLAMP = __CLAMP__;

  // --- collapsible diffs -------------------------------------------------
  // scrollHeight is the full content height whether or not the view is
  // currently clamped, so this measures the same either way. Only diffs that
  // actually overflow get clamped, so short ones never pick up the fade.
  function syncToggle(wrap, button) {
    var view = wrap.querySelector('.diff-view');
    if (!view) return;
    var overflowing = view.scrollHeight > CLAMP + 24;
    wrap.dataset.overflowing = overflowing ? 'true' : 'false';
    button.hidden = !overflowing;
  }

  document.querySelectorAll('.diff-wrap').forEach(function (wrap) {
    var button = wrap.querySelector('.diff-toggle');
    var view = wrap.querySelector('.diff-view');
    if (!button || !view) return;
    button.addEventListener('click', function () {
      var collapsed = wrap.dataset.collapsed === 'true';
      wrap.dataset.collapsed = collapsed ? 'false' : 'true';
      button.textContent = collapsed ? 'show less' : 'show more';
      if (!collapsed) wrap.scrollIntoView({ block: 'nearest' });
    });
    syncToggle(wrap, button);
    if (window.ResizeObserver) {
      new ResizeObserver(function () { syncToggle(wrap, button); }).observe(view);
    }
  });

  // --- share image -------------------------------------------------------
  var MONO = 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace';
  var FG = '#1a1522', MUTED = '#5d5966', MUTED2 = '#918d9a', ACCENT = '#2a1eff';
  var LINE = 'rgba(13,10,61,0.16)', LINE_SOFT = 'rgba(13,10,61,0.07)';
  var PANEL = '#f6f5fb';
  var W = 1200, H = 675;

  function drawMark(c, x, y, size) {
    var scale = size / CARD.viewbox[0];
    c.save();
    c.translate(x, y);
    c.scale(scale, scale);
    CARD.paths.forEach(function (path) {
      c.fillStyle = path.fill;
      c.fill(new Path2D(path.d), 'evenodd');
    });
    c.restore();
  }

  function drawCard(scale) {
    var canvas = document.createElement('canvas');
    canvas.width = W * scale;
    canvas.height = H * scale;
    var c = canvas.getContext('2d');
    c.scale(scale, scale);

    function font(weight, size) { c.font = weight + ' ' + size + 'px ' + MONO; }
    function track(value) { try { c.letterSpacing = value; } catch (e) {} }
    function rule(x, y, w, h) { c.fillStyle = LINE; c.fillRect(x, y, w, h); }
    function text(str, x, y, align) {
      c.textAlign = align || 'left';
      c.textBaseline = 'middle';
      c.fillText(str, x, y);
      c.textAlign = 'left';
    }
    function dots(x, y, w, h, step) {
      c.save();
      c.beginPath();
      c.rect(x, y, w, h);
      c.clip();
      c.fillStyle = LINE_SOFT;
      for (var i = x; i < x + w; i += step) {
        for (var j = y; j < y + h; j += step) {
          c.beginPath();
          c.arc(i + 1, j + 1, 1, 0, Math.PI * 2);
          c.fill();
        }
      }
      c.restore();
    }

    c.fillStyle = '#fff';
    c.fillRect(0, 0, W, H);
    dots(0, 0, W, H, 22);

    var fx = 48, fy = 40, fw = 1104, fh = 595;
    c.fillStyle = '#fff';
    c.fillRect(fx, fy, fw, fh);
    rule(fx, fy, fw, 1);
    rule(fx, fy + fh - 1, fw, 1);
    rule(fx, fy, 1, fh);
    rule(fx + fw - 1, fy, 1, fh);

    // meta bar
    var barBottom = fy + 38;
    rule(fx, barBottom, fw, 1);
    font('400', 11);
    track('1.1px');
    c.fillStyle = MUTED2;
    var handle = CARD.handle.toUpperCase();
    text(handle, fx + 16, fy + 19);
    var handleEnd = fx + 16 + c.measureText(handle).width + 14;
    var harness = CARD.harness.toUpperCase();
    var harnessW = c.measureText(harness).width + 12;
    var harnessX = fx + fw - 16 - harnessW;
    c.strokeStyle = LINE;
    c.lineWidth = 1;
    c.strokeRect(harnessX + 0.5, fy + 8.5, harnessW - 1, 21);
    text(harness, harnessX + 6, fy + 19);
    var meta = CARD.meta.toUpperCase();
    var metaX = harnessX - 14 - c.measureText(meta).width;
    text(meta, metaX, fy + 19);
    rule(handleEnd, fy + 19, Math.max(0, metaX - 14 - handleEnd), 1);
    track('normal');

    // body
    dots(fx + 1, barBottom + 1, fw - 2, 404, 26);
    font('400', 11);
    track('0.4px');
    c.fillStyle = ACCENT;
    text('# ' + CARD.eyebrow, fx + 36, barBottom + 18);
    font('500', 34);
    track('-2px');
    c.fillStyle = FG;
    text(CARD.title, fx + 36, barBottom + 52);
    track('normal');

    var mainMid = barBottom + 74 + (405 - 74) / 2;

    font('600', 170);
    track('-8px');
    c.fillStyle = ACCENT;
    text(CARD.grade, fx + 186, mainMid - 15, 'center');
    track('normal');
    font('400', 11);
    track('1.5px');
    c.fillStyle = MUTED2;
    text(CARD.grade_label.toUpperCase(), fx + 186, mainMid + 88, 'center');
    track('normal');

    var bx = fx + 392;
    var bw = fx + fw - 36 - bx;
    var rowH = 35, gap = 28;
    var top = mainMid - (3 * rowH + 2 * gap) / 2;
    CARD.bars.forEach(function (bar, index) {
      var y = top + index * (rowH + gap);
      font('500', 14);
      c.fillStyle = FG;
      text(bar[0].toLowerCase(), bx, y + 9);
      font('600', 14);
      text(String(bar[1]), bx + bw, y + 9, 'right');
      c.fillStyle = LINE_SOFT;
      c.fillRect(bx, y + 27, bw, 8);
      c.strokeStyle = LINE;
      c.strokeRect(bx + 0.5, y + 27.5, bw - 1, 7);
      c.fillStyle = ACCENT;
      c.fillRect(bx, y + 27, bw * Math.max(0, Math.min(100, bar[1])) / 100, 8);
    });

    // stats
    var sy = barBottom + 405;
    c.fillStyle = PANEL;
    c.fillRect(fx + 1, sy, fw - 2, 96);
    rule(fx, sy, fw, 1);
    var colW = (fw - 2) / 3;
    CARD.stats.forEach(function (stat, index) {
      var cx = fx + 1 + index * colW;
      if (index) rule(cx, sy, 1, 96);
      font('600', 40);
      c.fillStyle = FG;
      text(stat[0], cx + 24, sy + 40);
      font('400', 12);
      c.fillStyle = MUTED;
      text(stat[1], cx + 24, sy + 72);
    });

    // footer
    var gy = sy + 96;
    c.fillStyle = '#fff';
    c.fillRect(fx + 1, gy, fw - 2, fy + fh - gy - 1);
    rule(fx, gy, fw, 1);
    drawMark(c, fx + 16, gy + 15, 27);
    font('600', 15);
    track('-0.45px');
    c.fillStyle = FG;
    text(CARD.stamp[0], fx + 54, gy + 20);
    track('normal');
    font('400', 11);
    c.fillStyle = MUTED2;
    text(CARD.stamp[1], fx + 54, gy + 37);

    return canvas;
  }

  function slug(value) {
    return (value || '').toLowerCase().replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '') || 'agent';
  }

  var button = document.getElementById('share-png');
  button.addEventListener('click', function () {
    var label = button.textContent;
    button.disabled = true;
    var ready = (document.fonts && document.fonts.ready) || Promise.resolve();
    ready.then(function () {
      drawCard(2).toBlob(function (blob) {
        var url = URL.createObjectURL(blob);
        var a = document.createElement('a');
        a.href = url;
        a.download = slug(CARD.handle) + '-skill-report.png';
        a.click();
        setTimeout(function () { URL.revokeObjectURL(url); }, 2000);
        button.disabled = false;
        button.textContent = 'saved \u2713';
        setTimeout(function () { button.textContent = label; }, 2000);
      }, 'image/png');
    });
  });
})();
"""
    return script.replace("__CARD__", card_data.replace("</", "<\\/")).replace("__CLAMP__", str(DIFF_CLAMP_PX))


def parse_args(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "report_path",
        nargs="?",
        default="./skill-doctor-report/report.json",
        help="Path to the report.json file",
    )
    parser.add_argument(
        "--open",
        action="store_true",
        dest="open_browser",
        help="Open the generated report in the default browser",
    )
    return parser.parse_args(argv)


def main(argv=None):
    args = parse_args(argv)
    report_path = Path(args.report_path).expanduser()
    if not report_path.exists():
        print(f"error: {report_path} not found", file=sys.stderr)
        sys.exit(1)
    r = json.loads(report_path.read_text())
    r.setdefault("grade", grade_for(r["scores"]["overall"]))

    out_path = report_path.parent / "report.html"
    out_path.write_text(render_page(r))
    print(f"report: {out_path.absolute().as_uri()}")
    if args.open_browser:
        if open_report(out_path):
            print("        opened in the default browser")
        else:
            print(
                "warning: could not open the report in the default browser",
                file=sys.stderr,
            )
    print('        use "share as png" for a 1200x675 share image')


if __name__ == "__main__":
    main()
