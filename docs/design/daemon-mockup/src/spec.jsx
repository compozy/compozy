/* global React, Icons, Badge, Button, PipelineBar, StatusDot, SPEC_USERAUTH, WORKFLOWS, PROVIDERS */
// Spec view — PRD + TechSpec + ADRs in 3 tabs.

function SpecView({ workflow = "user-auth", go }) {
  const w = WORKFLOWS.find(x => x.name === workflow) || WORKFLOWS[0];
  const s = SPEC_USERAUTH; // only one fully fleshed spec; real app would switch by workflow.
  const [tab, setTab] = React.useState("prd");
  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <div style={{ padding: "22px 28px 18px", borderBottom: "1px solid var(--border)", display: "flex", flexDirection: "column", gap: 14 }}>
        <div style={{ display: "flex", alignItems: "flex-end", justifyContent: "space-between", gap: 16 }}>
          <div>
            <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: 4 }}>
              Workflow · {w.name}
            </div>
            <h1 style={{ margin: 0, fontFamily: "var(--font-display)", fontWeight: 500, fontSize: 30, letterSpacing: "-0.02em", lineHeight: 1.05 }}>
              {w.title}
            </h1>
            <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 10, fontSize: 11.5, color: "var(--muted-foreground)", fontFamily: "var(--font-mono)" }}>
              <Icons.GitBranch size={12}/>{w.repo} · {w.branch} · updated {w.updated}
            </div>
          </div>
          <div style={{ display: "flex", gap: 6 }}>
            <Button variant="outline" size="sm" icon={<Icons.ListTodo size={13}/>} onClick={() => go("tasks", { workflow: w.name })}>Tasks</Button>
            <Button variant="outline" size="sm" icon={<Icons.Brain size={13}/>} onClick={() => go("memory", { workflow: w.name })}>Memory</Button>
            <Button variant="primary" size="sm" icon={<Icons.Play/>}>Resume run</Button>
          </div>
        </div>
        <PipelineBar current={w.phase} done={w.phases_done}/>
        <div style={{ display: "flex", gap: 2, borderBottom: 0, marginTop: 6 }}>
          {[{k:"prd",l:"PRD"},{k:"techspec",l:"TechSpec"},{k:"adrs",l:"ADRs",count:s.adrs.length}].map(t => (
            <button key={t.k} onClick={() => setTab(t.k)} style={{
              padding: "8px 14px", border: 0, background: "transparent",
              borderBottom: `2px solid ${tab === t.k ? "var(--primary)" : "transparent"}`,
              color: tab === t.k ? "var(--foreground)" : "var(--muted-foreground)",
              fontSize: 13, fontWeight: tab === t.k ? 600 : 500, fontFamily: "inherit",
              cursor: "pointer", display: "inline-flex", alignItems: "center", gap: 6,
            }}>
              {t.l}
              {t.count != null && <span style={{ fontSize: 10, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{t.count}</span>}
            </button>
          ))}
        </div>
      </div>

      <div style={{ flex: 1, overflow: "auto", padding: "24px 28px 40px" }}>
        <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 1fr) 260px", gap: 28, maxWidth: 1100 }}>
          <article>
            {tab === "prd" && <Markdown text={s.prd}/>}
            {tab === "techspec" && <Markdown text={s.techspec}/>}
            {tab === "adrs" && <AdrsList adrs={s.adrs}/>}
          </article>
          <aside style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <SpecMetaCard w={w}/>
            <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em" }}>File</div>
            <div style={{ fontSize: 11.5, fontFamily: "var(--font-mono)", color: "var(--foreground)", padding: 10, background: "var(--card)", border: "1px solid var(--border)", borderRadius: 6, wordBreak: "break-all" }}>
              .compozy/tasks/{w.name}/{tab === "prd" ? "_prd.md" : tab === "techspec" ? "_techspec.md" : "adrs/"}
            </div>
          </aside>
        </div>
      </div>
    </div>
  );
}

function SpecMetaCard({ w }) {
  const p = PROVIDERS[w.provider];
  return (
    <div style={{ background: "var(--card)", border: "1px solid var(--border)", borderRadius: 6, padding: 12, display: "flex", flexDirection: "column", gap: 10 }}>
      <Row k="Status" v={<Badge variant={w.status === "running" ? "info" : w.status === "done" ? "success" : "muted"}>{w.status}</Badge>}/>
      <Row k="Phase" v={<Badge variant="lime">{w.phase}</Badge>}/>
      <Row k="Agent" v={<span style={{ display: "inline-flex", alignItems: "center", gap: 5, fontSize: 12 }}><img src={p.logo} style={{ width: 12, height: 12 }}/>{p.name}</span>}/>
      <Row k="Model" v={<span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{p.model}</span>}/>
      <Row k="Tasks" v={<span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{w.tasks.done}/{w.tasks.total}</span>}/>
      <Row k="Owner" v="Rui"/>
      <Row k="Created" v={<span style={{ fontSize: 11 }}>{w.created}</span>}/>
    </div>
  );
}
function Row({ k, v }) {
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10 }}>
      <span style={{ fontSize: 11, color: "var(--muted-foreground)", fontFamily: "var(--font-mono)", textTransform: "uppercase", letterSpacing: "0.06em" }}>{k}</span>
      <span style={{ textAlign: "right" }}>{v}</span>
    </div>
  );
}

function AdrsList({ adrs }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      {adrs.map(a => (
        <div key={a.n} style={{ border: "1px solid var(--border)", borderRadius: 6, background: "var(--card)", padding: 16 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 8 }}>
            <span style={{ fontFamily: "var(--font-mono)", fontSize: 11, color: "var(--muted-foreground)" }}>ADR-{a.n}</span>
            <Badge variant={a.decision === "accepted" ? "success" : a.decision === "deferred" ? "warning" : "muted"}>{a.decision}</Badge>
            <span style={{ flex: 1 }}/>
            <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{a.date}</span>
          </div>
          <div style={{ fontSize: 15, fontWeight: 600, color: "var(--foreground)", marginBottom: 6 }}>{a.title}</div>
          <div style={{ fontSize: 13, color: "var(--muted-foreground)", lineHeight: 1.55 }}>{a.summary}</div>
        </div>
      ))}
    </div>
  );
}

// Tiny markdown renderer — handles #, ##, ###, code fences, inline `code`, **b**, lists.
function Markdown({ text }) {
  const lines = text.split("\n");
  const out = [];
  let i = 0;
  const inline = (s) => {
    const parts = [];
    let rest = s;
    while (rest.length) {
      const m = rest.match(/`([^`]+)`|\*\*([^*]+)\*\*/);
      if (!m) { parts.push(rest); break; }
      if (m.index > 0) parts.push(rest.slice(0, m.index));
      if (m[1]) parts.push(<code key={parts.length} style={codeInline}>{m[1]}</code>);
      else parts.push(<strong key={parts.length}>{m[2]}</strong>);
      rest = rest.slice(m.index + m[0].length);
    }
    return parts;
  };
  while (i < lines.length) {
    const l = lines[i];
    if (l.startsWith("```")) {
      const buf = [];
      i++;
      while (i < lines.length && !lines[i].startsWith("```")) { buf.push(lines[i]); i++; }
      i++;
      out.push(<pre key={out.length} style={codeBlock}>{buf.join("\n")}</pre>);
      continue;
    }
    if (l.startsWith("### ")) out.push(<h3 key={out.length} style={h3}>{inline(l.slice(4))}</h3>);
    else if (l.startsWith("## ")) out.push(<h2 key={out.length} style={h2}>{inline(l.slice(3))}</h2>);
    else if (l.startsWith("# ")) out.push(<h1 key={out.length} style={h1}>{inline(l.slice(2))}</h1>);
    else if (l.startsWith("- ")) {
      const buf = [];
      while (i < lines.length && lines[i].startsWith("- ")) { buf.push(lines[i].slice(2)); i++; }
      out.push(<ul key={out.length} style={ul}>{buf.map((b, k) => <li key={k} style={li}>{inline(b)}</li>)}</ul>);
      continue;
    }
    else if (/^\d+\. /.test(l)) {
      const buf = [];
      while (i < lines.length && /^\d+\. /.test(lines[i])) { buf.push(lines[i].replace(/^\d+\. /, "")); i++; }
      out.push(<ol key={out.length} style={ul}>{buf.map((b, k) => <li key={k} style={li}>{inline(b)}</li>)}</ol>);
      continue;
    }
    else if (l.trim() === "") out.push(<div key={out.length} style={{ height: 8 }}/>);
    else out.push(<p key={out.length} style={p}>{inline(l)}</p>);
    i++;
  }
  return <div style={{ color: "var(--foreground)", fontSize: 14, lineHeight: 1.62, fontFamily: "var(--font-sans)" }}>{out}</div>;
}
const h1 = { fontFamily: "var(--font-display)", fontWeight: 500, fontSize: 30, margin: "18px 0 12px", letterSpacing: "-0.02em", lineHeight: 1.1 };
const h2 = { fontFamily: "var(--font-sans)", fontWeight: 600, fontSize: 18, margin: "22px 0 8px", letterSpacing: "-0.005em" };
const h3 = { fontFamily: "var(--font-sans)", fontWeight: 600, fontSize: 14, margin: "14px 0 6px", textTransform: "uppercase", letterSpacing: "0.06em", color: "var(--muted-foreground)" };
const p  = { margin: "8px 0" };
const ul = { margin: "6px 0 10px", paddingLeft: 20 };
const li = { marginBottom: 4 };
const codeInline = { fontFamily: "var(--font-mono)", fontSize: 12.5, padding: "1px 5px", borderRadius: 3, background: "var(--secondary)", border: "1px solid var(--border)" };
const codeBlock = { fontFamily: "var(--font-mono)", fontSize: 12.5, background: "var(--card)", border: "1px solid var(--border)", borderRadius: 6, padding: "12px 14px", margin: "10px 0", overflow: "auto", lineHeight: 1.55 };

window.SpecView = SpecView;
window.Markdown = Markdown;
