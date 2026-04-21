/* global React, Icons, Badge, Button, StatusDot, IconBtn, REVIEW_ROUNDS, REVIEW_ISSUES, PROVIDERS */
// Review rounds list + review issue detail (diff + agent resolution).

function ReviewsView({ workflow, go }) {
  const [tab, setTab] = React.useState("open");
  const issues = REVIEW_ISSUES.filter(i =>
    tab === "open" ? i.status === "open" :
    tab === "fixed" ? i.status === "fixed" :
    tab === "invalid" ? i.status === "invalid" : true
  );
  const counts = {
    all: REVIEW_ISSUES.length,
    open: REVIEW_ISSUES.filter(i => i.status === "open").length,
    fixed: REVIEW_ISSUES.filter(i => i.status === "fixed").length,
    invalid: REVIEW_ISSUES.filter(i => i.status === "invalid").length,
  };
  return (
    <div style={{ padding: "24px 28px 40px", display: "flex", flexDirection: "column", gap: 22, maxWidth: 1400 }}>
      <header>
        <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: 4 }}>
          {workflow ? `Workflow · ${workflow}` : "Across all workflows"}
        </div>
        <h1 style={{ margin: 0, fontFamily: "var(--font-display)", fontWeight: 500, fontSize: 36, letterSpacing: "-0.02em" }}>Reviews</h1>
        {!workflow && (
          <div style={{ fontSize: 13, color: "var(--muted-foreground)", marginTop: 8, maxWidth: 680, lineHeight: 1.5 }}>
            Feedback ingested from PR reviewers across every workflow. Click a round to jump into its workflow.
          </div>
        )}
      </header>

      {/* Rounds timeline */}
      <section style={{ background: "var(--card)", border: "1px solid var(--border)", borderRadius: 8, padding: 16 }}>
        <div style={{ fontSize: 11, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: 12 }}>Rounds</div>
        <div style={{ display: "grid", gridTemplateColumns: `repeat(${REVIEW_ROUNDS.length}, 1fr)`, gap: 12 }}>
          {REVIEW_ROUNDS.map((r, i) => {
            const active = i === REVIEW_ROUNDS.length - 1;
            return (
              <div key={r.n} style={{
                padding: "14px 16px", borderRadius: 6,
                border: `1px solid ${active ? "rgba(202,234,40,0.3)" : "var(--border)"}`,
                background: active ? "rgba(202,234,40,0.04)" : "transparent",
              }}>
                <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 6 }}>
                  <span style={{ width: 14, height: 14, borderRadius: 3, background: "linear-gradient(135deg, #f97316, #fdba74)", display: "inline-flex", alignItems: "center", justifyContent: "center", fontSize: 9, fontWeight: 800, color: "#1c1917", fontFamily: "var(--font-mono)" }}>CR</span>
                  <span style={{ fontSize: 12, fontWeight: 600 }}>Round {String(r.n).padStart(3,"0")}</span>
                  {active && <Badge variant="lime">current</Badge>}
                  <span style={{ flex: 1 }}/>
                  <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{r.fetched}</span>
                </div>
                <div style={{ fontSize: 11.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", marginBottom: 10 }}>
                  {r.workflow} · PR #{r.pr}
                </div>
                <div style={{ display: "flex", gap: 10, fontSize: 11.5 }}>
                  <span><b style={{ color: "var(--foreground)" }}>{r.issues.total}</b> total</span>
                  <span style={{ color: "#fcd34d" }}>{r.issues.open} open</span>
                  <span style={{ color: "#6ee7b7" }}>{r.issues.fixed} fixed</span>
                  <span style={{ color: "var(--muted-foreground)" }}>{r.issues.invalid} invalid</span>
                </div>
              </div>
            );
          })}
        </div>
      </section>

      <div style={{ display: "flex", gap: 8 }}>
        {[{k:"open",l:"Open"},{k:"fixed",l:"Fixed"},{k:"invalid",l:"Invalid"},{k:"all",l:"All"}].map(t => (
          <button key={t.k} onClick={() => setTab(t.k)} style={{
            height: 26, padding: "0 10px", borderRadius: 5,
            border: `1px solid ${tab === t.k ? "var(--primary)" : "var(--border)"}`,
            background: tab === t.k ? "rgba(202,234,40,0.08)" : "var(--card)",
            color: tab === t.k ? "var(--primary)" : "var(--foreground)",
            fontSize: 12, fontFamily: "inherit", cursor: "pointer",
            display: "inline-flex", alignItems: "center", gap: 6,
          }}>{t.l}<span style={{ fontSize: 10, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{counts[t.k]}</span></button>
        ))}
      </div>

      <div style={{ background: "var(--card)", border: "1px solid var(--border)", borderRadius: 8, overflow: "hidden" }}>
        {issues.map(i => (
          <div key={i.id} onClick={() => go("review-detail", { workflow: "manifest-v2", round: "002", issue: i.id })} style={{
            display: "grid", gridTemplateColumns: "14px 80px 1fr 90px 100px 40px",
            padding: "12px 16px", borderBottom: "1px solid var(--border)", alignItems: "center", gap: 12, cursor: "pointer",
          }}>
            <StatusDot status={i.status === "open" ? "open" : i.status === "fixed" ? "done" : "archived"} size={11}/>
            <span style={{ fontSize: 11, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{i.id}</span>
            <div style={{ minWidth: 0 }}>
              <div style={{ fontSize: 13, color: "var(--foreground)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{i.title}</div>
              <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", marginTop: 2 }}>
                {i.file}:{i.line}
              </div>
            </div>
            <Badge variant={i.severity === "high" ? "destructive" : i.severity === "medium" ? "warning" : "muted"}>{i.severity}</Badge>
            <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{i.domain}</span>
            <Icons.ArrowRight size={14}/>
          </div>
        ))}
      </div>
    </div>
  );
}

function ReviewDetailView({ issueId = "issue_004", go }) {
  const i = REVIEW_ISSUES.find(x => x.id === issueId) || REVIEW_ISSUES[3];
  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <div style={{ padding: "22px 28px 18px", borderBottom: "1px solid var(--border)" }}>
        <div style={{ display: "flex", alignItems: "flex-end", justifyContent: "space-between", gap: 16 }}>
          <div style={{ minWidth: 0 }}>
            <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", marginBottom: 6 }}>
              manifest-v2 · round 002 · {i.id}
            </div>
            <h1 style={{ margin: 0, fontFamily: "var(--font-display)", fontWeight: 500, fontSize: 24, letterSpacing: "-0.02em", lineHeight: 1.2 }}>{i.title}</h1>
            <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 10 }}>
              <Badge variant={i.severity === "high" ? "destructive" : i.severity === "medium" ? "warning" : "muted"}>{i.severity}</Badge>
              <Badge variant={i.status === "open" ? "info" : i.status === "fixed" ? "success" : "muted"}>{i.status}</Badge>
              <span style={{ fontSize: 11.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{i.file}:{i.line}</span>
              <span style={{ color: "var(--muted-foreground)" }}>·</span>
              <span style={{ fontSize: 11.5, color: "var(--muted-foreground)" }}>via {i.author}</span>
            </div>
          </div>
          <div style={{ display: "flex", gap: 6 }}>
            <Button variant="outline" size="sm" icon={<Icons.X size={13}/>}>Mark invalid</Button>
            <Button variant="primary" size="sm" icon={<Icons.Sparkles size={13}/>}>Dispatch fix</Button>
          </div>
        </div>
      </div>

      <div style={{ flex: 1, overflow: "auto", padding: "22px 28px 40px" }}>
        <div style={{ display: "grid", gridTemplateColumns: "minmax(0,1fr) 300px", gap: 28, maxWidth: 1200 }}>
          <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
            <Section title="Reviewer comment" kicker="CodeRabbit">
              <Quote>
                The schema test passes when <code>capabilities</code> is missing, but doesn't assert behavior when it's present but empty. An empty array means "no capabilities" which should still validate successfully — right now nothing covers that and it's likely to regress. Add a test case: <code>{`{ providers: [{ id: "x", capabilities: [] }] }`}</code> → expect <code>ok</code>.
              </Quote>
            </Section>

            <Section title="Context" kicker="File referenced">
              <CodeBlock file={i.file} startLine={i.line - 2}>
{`describe("manifest schema", () => {
  it("validates well-formed manifest", () => {
    expect(parse(ok)).toMatchObject({ ok: true });
  });

  // TODO(reviewer): missing empty-capabilities case
  it("rejects missing capabilities field", () => {`}
              </CodeBlock>
            </Section>

            <Section title="Proposed patch" kicker="by Claude · dispatched on Resolve">
              <Diff lines={[
                { t: "hunk",  s: `@@ packages/providers/manifest_test.ts +${i.line} @@` },
                { t: "eq",    s: `  it("rejects missing capabilities field", () => {` },
                { t: "eq",    s: `    expect(parse(noCaps)).toMatchObject({ ok: false });` },
                { t: "eq",    s: `  });` },
                { t: "eq",    s: `` },
                { t: "plus",  s: `  it("accepts empty capabilities array", () => {` },
                { t: "plus",  s: `    const empty = { providers: [{ id: "x", capabilities: [] }] };` },
                { t: "plus",  s: `    expect(parse(empty)).toMatchObject({ ok: true });` },
                { t: "plus",  s: `    expect(parse(empty).value.providers[0].capabilities).toEqual([]);` },
                { t: "plus",  s: `  });` },
              ]}/>
            </Section>

            <Section title="Discussion">
              <ChatBubble who="CodeRabbit" ts="4h ago">
                Filed from PR #312, thread coderabbit#aa24. Considered this a medium because it silently masks regressions in the manifest schema.
              </ChatBubble>
              <ChatBubble who="Rui" ts="2h ago" me>
                Agreed, this should be covered. @compozy dispatch a fix, but keep the change scoped to the test file — don't touch the schema.
              </ChatBubble>
              <ChatBubble who="Compozy" ts="2h ago">
                Dispatching task_r2_004 to claude (Sonnet 4.5 · medium) — scope limited to <code>packages/providers/manifest_test.ts</code>. Patch above is the draft; awaiting your accept.
              </ChatBubble>
            </Section>
          </div>

          <aside style={{ display: "flex", flexDirection: "column", gap: 14 }}>
            <div style={{ background: "var(--card)", border: "1px solid var(--border)", borderRadius: 6, padding: 12, display: "flex", flexDirection: "column", gap: 10 }}>
              <MetaRow k="Severity" v={<Badge variant="warning">{i.severity}</Badge>}/>
              <MetaRow k="Status" v={<Badge variant="info">{i.status}</Badge>}/>
              <MetaRow k="Domain" v={<span style={{ fontSize: 11 }}>{i.domain}</span>}/>
              <MetaRow k="Thread" v={<span style={{ fontFamily: "var(--font-mono)", fontSize: 10.5 }}>{i.thread}</span>}/>
              <MetaRow k="PR" v={<span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>#312</span>}/>
            </div>
            <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em" }}>Auto-resolution policy</div>
            <ul style={{ margin: 0, paddingLeft: 18, fontSize: 12, color: "var(--muted-foreground)", lineHeight: 1.6 }}>
              <li>Low + copy → auto-fix</li>
              <li>Medium + test → draft patch, wait for accept</li>
              <li>High → surface as a task, never auto-dispatch</li>
            </ul>
          </aside>
        </div>
      </div>
    </div>
  );
}

function Section({ title, kicker, children }) {
  return (
    <section>
      <header style={{ display: "flex", alignItems: "baseline", gap: 10, marginBottom: 10 }}>
        <h3 style={{ margin: 0, fontFamily: "var(--font-sans)", fontWeight: 600, fontSize: 14 }}>{title}</h3>
        {kicker && <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>· {kicker}</span>}
      </header>
      <div>{children}</div>
    </section>
  );
}
function MetaRow({ k, v }) {
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
      <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em" }}>{k}</span>
      {v}
    </div>
  );
}
function Quote({ children }) {
  return (
    <blockquote style={{
      margin: 0, padding: "12px 14px",
      background: "var(--card)", border: "1px solid var(--border)", borderRadius: 6,
      fontSize: 13.5, color: "var(--foreground)", lineHeight: 1.6,
    }}>{children}</blockquote>
  );
}
function CodeBlock({ file, startLine, children }) {
  const lines = String(children).split("\n");
  return (
    <div style={{ background: "var(--card)", border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden", fontFamily: "var(--font-mono)", fontSize: 12.5 }}>
      <div style={{ padding: "6px 12px", background: "var(--secondary)", borderBottom: "1px solid var(--border)", fontSize: 11, color: "var(--muted-foreground)" }}>{file}</div>
      {lines.map((l, idx) => (
        <div key={idx} style={{ display: "grid", gridTemplateColumns: "44px 1fr", borderBottom: idx === lines.length - 1 ? "none" : "1px solid transparent" }}>
          <span style={{ padding: "2px 10px", color: "var(--muted-foreground)", textAlign: "right", background: "rgba(255,255,255,0.02)", userSelect: "none" }}>{startLine + idx}</span>
          <span style={{ padding: "2px 12px", color: "var(--foreground)", whiteSpace: "pre" }}>{l}</span>
        </div>
      ))}
    </div>
  );
}
function Diff({ lines }) {
  return (
    <div style={{ background: "var(--card)", border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden", fontFamily: "var(--font-mono)", fontSize: 12.5 }}>
      {lines.map((l, i) => {
        const style = l.t === "plus" ? { bg: "rgba(16,185,129,0.08)", fg: "#6ee7b7", prefix: "+" } :
                     l.t === "minus" ? { bg: "rgba(239,68,68,0.08)",  fg: "#fca5a5", prefix: "−" } :
                     l.t === "hunk"  ? { bg: "rgba(202,234,40,0.06)", fg: "var(--primary)", prefix: "" } :
                                       { bg: "transparent",           fg: "var(--muted-foreground)", prefix: " " };
        return (
          <div key={i} style={{ display: "grid", gridTemplateColumns: "24px 1fr", background: style.bg }}>
            <span style={{ textAlign: "center", color: style.fg, userSelect: "none", padding: "2px 0" }}>{style.prefix}</span>
            <span style={{ color: style.fg, whiteSpace: "pre", padding: "2px 12px 2px 0" }}>{l.s}</span>
          </div>
        );
      })}
    </div>
  );
}
function ChatBubble({ who, ts, avatar, me, children }) {
  return (
    <div style={{ display: "grid", gridTemplateColumns: "28px 1fr", gap: 10, padding: "10px 0", borderTop: "1px solid var(--border)" }}>
      <div style={{ width: 26, height: 26, borderRadius: 5, background: me ? "linear-gradient(135deg, #caea28, #78a10d)" : "var(--secondary)", display: "grid", placeItems: "center", color: me ? "var(--brand-950)" : "var(--muted-foreground)", fontWeight: 700, fontSize: 10, fontFamily: "var(--font-mono)" }}>
        {avatar ? <img src={avatar} style={{ width: 18, height: 18 }}/> : who.slice(0,2).toUpperCase()}
      </div>
      <div>
        <div style={{ display: "flex", alignItems: "baseline", gap: 8, marginBottom: 3 }}>
          <span style={{ fontSize: 12, fontWeight: 600 }}>{who}</span>
          <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{ts}</span>
        </div>
        <div style={{ fontSize: 13, color: "var(--foreground)", lineHeight: 1.55 }}>{children}</div>
      </div>
    </div>
  );
}

window.ReviewsView = ReviewsView;
window.ReviewDetailView = ReviewDetailView;
