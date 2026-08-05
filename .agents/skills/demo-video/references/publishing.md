# Publishing — packaging per destination

All encodes reuse the cached frames — `encode.mjs` with different env is cheap; never re-record for a format change.

## Pull request (evidence of behavior)

GitHub renders an inline player for video *attachments* (mp4/mov/webm, H.264 recommended). Limits: 10MB video on free plans, 100MB on paid; images (GIF) 10MB everywhere. Raw-URL links to files in the repo do **not** render a player.

1. Encode to budget: `TARGET_MB=<publish.pr.max_mb> OUT_W=1920 node encode.mjs --dir <workdir> --out demo-pr.mp4`.
2. Upload: `bash <skill-dir>/scripts/attach-github.sh <workdir>/demo-pr.mp4 <publish.pr.repo>` — prints the comment markdown (bare URL on its own line → inline player). It uses the endpoint behind the web UI's drag-and-drop, which is **undocumented** (island94.org, 2026-08; verified working) and may break without notice.
3. Compose the comment: one line naming the flow demonstrated (and the QA scenario id when the demo walks one), then the markdown from step 2. Post with `gh pr comment`.
4. **Fallback when the upload fails:** print a large "🚨 YOUR ATTENTION IS REQUIRED 🚨" banner asking the user to drag-and-drop the mp4 (give its absolute path) into the PR comment box — that is the only supported route.

A GIF (`gif.mjs`, ≤10MB) is a secondary form: use it when the demo must be visible with zero clicks in *any* markdown surface (README, issue body). Prefer the mp4 player in PR comments — smaller, sharper, scrubs.

## Social / marketing (X, launch posts)

X standard accounts: H.264 + AAC (silent ok), ≤140s, ≤512MB, up to 1080p (premium unlocks higher). Encode: `OUT_W=1920 FPS=60 CRF=16 node encode.mjs --dir <workdir> --out demo-social.mp4`. Vertical cuts re-encode from the same frames with `OUT_W`/`OUT_H` swapped only if the capture composed for it — otherwise record a vertical-composed flow.

Quality bar is higher than PR evidence: walk the full self-eval in choreography.md, and prefer re-choreographing over shipping a beat that reads mechanical. For post-production beyond this pipeline (music, narration, multi-clip edits), hand `demo-social.mp4` to a video-editing skill/tool rather than growing this one.

## QA evidence (when the config sets `publish.evidence_dir`)

When the demo walks a QA scenario, copy the mp4 into `<evidence_dir>/<date>-<scope>/` and cite it by repo-relative path from the scenario/report per the project's QA contract. The demo then serves as replayable evidence, not just decoration on the PR.
