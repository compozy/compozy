# Concurrent-session browser measurements

This is a controlled real-daemon/Chromium experiment, not production telemetry or INP.
The fixture uses isolated homes, real SQLite/HTTP/SSE, the normal built Web application
and deterministic ACP subprocesses. It never reads an operator's provider credentials.

Each run starts 32 sessions in one workspace, streams 600 chunks at 100 ms intervals
from eight sessions, and seeds 512 done plus 128 archived sessions. Chromium uses a
1500 × 1050 viewport. The harness selects the last 16 sessions in four passes, verifies
`aria-current=true` and visible chat, then waits two animation frames. The first pass
warms navigation. The remaining 48 elapsed intervals include Playwright actionability,
assertion polling and animation-frame scheduling. They are not a direct click-to-paint
measurement. Full-stream completion, page errors, requests and cleanup are also checked.

## Results

| Run | Source | Completed selections | Warm median | Warm p95 | Post-settlement JS heap | Page errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Baseline 1 | `de5ab2f4d` | 64 | 604.80 ms | 1,337.26 ms | 81,542,136 B | 0 |
| Candidate 1 | Before final lint-only source cleanup | 64 | 515.77 ms | 1,104.06 ms | 79,795,528 B | 0 |
| Candidate 2 | Before final lint-only source cleanup | 64 | 681.32 ms | 1,514.59 ms | 82,415,280 B | 0 |
| Baseline 2 | `de5ab2f4d` | Incomplete | Not accepted | Not accepted | Not accepted | Selection assertion failed |
| Initial published candidate | `14730a705771c0a3eb6e92947eddede632dff666` | 64 | 691.54 ms | 3,307.73 ms | 82,780,800 B | 0 |

The published candidate's daemon binary records that exact VCS revision with `vcs.modified=false`.
Its build SHA-256 and every accepted warm timing sample are in
[the sanitized measurement artifact](evidence/browser-measurements.json). Every listed
run's browser/runtime teardown succeeded, including the failed baseline attempt.
The artifact recomputes the conventional median for the even-sized samples; early
console receipts selected the upper middle sample. The source harness now computes
the conventional median directly.

**No stable overall navigation-latency or browser-heap reduction is established.**
The first comparison improved, but later timings overlap or worsen under shared-host
variation. The final p95 is materially worse than the first baseline observation and
must not be hidden by the component-level gains. Three candidate runs completed all
64 selections without a page error. This validates that workload's functionality;
it does not establish a latency SLO or explain every existing slow click.

The directly affected network boundary is visible in the app: full `/api/status`
reads were 13 in baseline 1 and two in each candidate, with 11–13 lightweight identity
reads replacing them. Total request counts were 1,663 / 1,474 / 1,695 / 1,744 for the
four completed runs. Those totals depend on duration and event interleaving; they
are not a fixed-budget request-reduction benchmark. Controlled owner fixtures carry
the exact burst-count claims. Post-GC settled heaps are similar, so the transcript
and terminal retained-state measurements must not be described as whole-browser savings.

## Baseline selection failure

Baseline 2 clicked Performance 27 in the second pass, but that row did not become
current within the existing five-second assertion timeout. The screenshot remained
on Performance 26. Another visible row's hover styling does not establish that it
became selected. No assertion, timeout or retry was relaxed to accept this run.

A separate diagnostic copy adds passive pointer/focus/scroll and whitelisted
window-command timing capture, without authorization headers or tokens. Its timing
has instrumentation overhead and is excluded from the comparison above. Causal
findings and limits are recorded in the navigation review; an unexplained baseline
failure is not attributed to a candidate fix.

## Isolated navigation correction

The [navigation investigation](review-session-navigation.md) subsequently reproduced
stale conversation identity on the published candidate. The correct trusted click,
window command and target detail read succeeded, and the shell changed identity after
6.3 ms, but the departing conversation remained interactive for over 5.25 seconds.
The content hook deferred the authoritative route identity to allow a crossfade.

Consuming the pathname and drawer flag directly removes that deferral while retaining
the keyed provider replacement and scope guards. The same instrumented harness, unchanged
assertions/timeouts, daemon binary and fixture then completed **64/64 selections** and
all eight streams. All **48 warm revisits** reached the selected row and new chat DOM
in median **166.15 ms**, maximum **191.50 ms**, measured on the browser clock from the
trusted click. The formerly failing revisit completed in **178.40 ms**. Teardown succeeded.

This is one causal diagnostic pair, with added observer overhead. It supports the
specific identity correction; it is not a stable population latency claim and is not
mixed into the non-instrumented table above. The original baseline failure did not
capture identity milestones, so its exact cause remains unproved.

## Reproduction and evidence

Build each checkout's SDK/Web through root Turbo and its daemon with the same Go
toolchain. The baseline is a frozen archive of `de5ab2f4d`, with dependencies installed
in that checkout. Supply its matching binary and source together:

```sh
bun web/e2e/scripts/benchmark-concurrent-sessions.ts \
  <checkout> <matching-daemon-binary> <output-directory> 32
```

Run under the repository's machine-wide verification lock and a two-CPU local budget,
one benchmark at a time. `measurements.json`, `sessions.png`, `teardown.json`, and
failure artifacts are written to the selected output directory. Bun 1.4.0 and Chromium
156.0.8078.4 were used for the accepted runs. The controlled ACP fixture does not model
provider network latency or a production model catalog of thousands of entries; the
closed-selector SSR experiment measures that separate population.

Baseline screenshot: [real app after completed streams](evidence/browser-baseline.png).
Initial published candidate screenshot: [same workload after completed streams](evidence/browser-candidate.png).

## Main-integration correctness walk

After composing main `8c025ff8f` with performance parent `e62862f48`, the unchanged
non-instrumented harness completed **64/64 selections**, all eight 600-chunk streams,
and both browser/runtime teardown operations. It used 32 active plus 640 retained
sessions, Bun 1.4.0 and Chromium 156.0.8078.4. No browser or stream error was reported.
The [integrated screenshot](evidence/browser-main-integration.png) and an additional
run in [the measurement artifact](evidence/browser-measurements.json) retain the result.

The 48 warm selections had median **296.68 ms** and p95 **375.76 ms** including Playwright
actionability, assertions and two animation frames. This is one correctness walk of the
combined UI, not a paired speed comparison against new main, and it does not replace the
earlier causal diagnostic. The broader Stays-on UI differs from the original baseline.

Raw receipts are `.cache/performance/main-integration-browser.log` and
`browser-main-integration/{measurements.json,sessions.png,teardown.json}`. The exact binary
SHA-256 is `747aa4d830af0098d57b156955d2325574c2bca2b4c860c67418ceda593232a7`;
`main-integration-source-freeze.json` records the 13,518 tracked source/config input hashes.
This walk preceded the final internal capability-helper extraction needed for the merged
function-size lint limit. That extraction retains the snapshot semantics and is checked by
the canonical Info race test and benchmark reruns. Final current-head CI, including runtime
and Web E2E, owns acceptance of the committed source after that extraction.
