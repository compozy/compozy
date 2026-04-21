/* global React, Icons, Badge, Button, StatusDot, PipelineBar, WORKFLOWS, PROVIDERS */
// Workflows list view — card grid with pipeline bar.

function WorkflowsView({ go }) {
  const [filter, setFilter] = React.useState("all");
  const [q, setQ] = React.useState("");
  const visible = WORKFLOWS.filter(w =>
    (filter === "all" ? w.status !== "archived" : filter === "archived" ? w.status === "archived" : w.status === filter) &&
    (q ? (w.name + " " + w.title).toLowerCase().includes(q.toLowerCase()) : true)
  );
  const counts = {
    all: WORKFLOWS.filter(w => w.status !== "archived").length,
    running: WORKFLOWS.filter(w => w.status === "running").length,
    paused: WORKFLOWS.filter(w => w.status === "paused").length,
    done: WORKFLOWS.filter(w => w.status === "done").length,
    failed: WORKFLOWS.filter(w => w.status === "failed").length,
    archived: WORKFLOWS.filter(w => w.status === "archived").length,
  };
  return (
    <div style={{ padding: "24px 28px 40px", display: "flex", flexDirection: "column", gap: 20, maxWidth: 1400 }}>
      <header style={{ display: "flex", alignItems: "flex-end", justifyContent: "space-between" }}>
        <div>
          <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: 4 }}>Workspace</div>
          <h1 style={{ margin: 0, fontFamily: "var(--font-display)", fontWeight: 500, fontSize: 36, letterSpacing: "-0.02em", lineHeight: 1.02 }}>Workflows</h1>
        </div>
        <div style={{ display: "flex", gap: 6 }}>
          <Button variant="outline" size="sm" icon={<Icons.Filter/>}>Repo</Button>
          <Button variant="outline" size="sm" icon={<Icons.Sort/>}>Updated</Button>
        </div>
      </header>

      <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
        {[
          { k: "all", l: "All" },
          { k: "running", l: "Running" },
          { k: "paused", l: "Paused" },
          { k: "done", l: "Done" },
          { k: "failed", l: "Failed" },
          { k: "archived", l: "Archived" },
        ].map(f => (
          <button key={f.k} onClick={() => setFilter(f.k)} style={{
            display: "inline-flex", alignItems: "center", gap: 6,
            height: 26, padding: "0 10px", borderRadius: 5,
            border: `1px solid ${filter === f.k ? "var(--primary)" : "var(--border)"}`,
            background: filter === f.k ? "rgba(202,234,40,0.08)" : "var(--card)",
            color: filter === f.k ? "var(--primary)" : "var(--foreground)",
            fontSize: 12, fontFamily: "inherit", cursor: "pointer",
          }}>
            {f.l}
            <span style={{ fontSize: 10, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{counts[f.k]}</span>
          </button>
        ))}
        <div style={{ flex: 1 }}/>
        <div style={{ position: "relative", width: 240 }}>
          <Icons.Search size={13} style={{ position: "absolute", left: 9, top: "50%", transform: "translateY(-50%)", color: "var(--muted-foreground)", pointerEvents: "none" }}/>
          <input value={q} onChange={e => setQ(e.target.value)} placeholder="Search workflows…" style={{
            display: "block", width: "100%", height: 28, padding: "0 10px 0 28px", borderRadius: 5,
            background: "var(--card)", border: "1px solid var(--border)",
            color: "var(--foreground)", fontFamily: "inherit", fontSize: 12, outline: "none",
          }}/>
        </div>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(340px, 1fr))", gap: 14 }}>
        {visible.map(w => <WorkflowCard key={w.name} w={w} go={go}/>)}
      </div>
    </div>
  );
}

function WorkflowCard({ w, go }) {
  const [hover, setHover] = React.useState(false);
  const p = PROVIDERS[w.provider];
  const pct = Math.round((w.tasks.done / w.tasks.total) * 100);
  const statusVar = { running: "info", paused: "warning", done: "success", failed: "destructive", archived: "muted" }[w.status];
  return (
    <div onClick={() => go("spec", { workflow: w.name })}
      onMouseEnter={() => setHover(true)} onMouseLeave={() => setHover(false)}
      style={{
        background: "var(--card)", border: "1px solid var(--border)", borderRadius: 8,
        padding: 16, cursor: "pointer",
        boxShadow: hover ? "var(--shadow-md)" : "var(--shadow-sm)",
        transform: hover ? "translateY(-1px)" : "translateY(0)",
        transition: "all 140ms", display: "flex", flexDirection: "column", gap: 12,
        opacity: w.status === "archived" ? 0.7 : 1,
      }}>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 10 }}>
        <div style={{ minWidth: 0 }}>
          <div style={{ fontSize: 11, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", marginBottom: 4 }}>{w.name}</div>
          <div style={{ fontSize: 14, fontWeight: 600, color: "var(--foreground)", lineHeight: 1.3 }}>{w.title}</div>
        </div>
        <Badge variant={statusVar}>{w.status}</Badge>
      </div>

      <PipelineBar current={w.phase} done={w.phases_done} size="sm"/>

      <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: 10, alignItems: "center", marginTop: 4 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>
          <Icons.GitBranch size={11}/>{w.repo}
        </div>
        <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{w.updated}</span>
      </div>

      <div>
        <div style={{ height: 6, borderRadius: 999, background: "var(--secondary)", overflow: "hidden", display: "flex" }}>
          <div style={{ width: `${(w.tasks.done/w.tasks.total)*100}%`, background: "#10b981" }}/>
          <div style={{ width: `${(w.tasks.running/w.tasks.total)*100}%`, background: "#3b82f6" }}/>
          <div style={{ width: `${(w.tasks.failed/w.tasks.total)*100}%`, background: "#ef4444" }}/>
        </div>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 6, fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>
          <span>{w.tasks.done}/{w.tasks.total} tasks · {pct}%</span>
          <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
            {w.tasks.failed > 0 && <span style={{ color: "#fca5a5" }}>{w.tasks.failed} failed</span>}
            {w.pending_reviews > 0 && <span style={{ color: "#fcd34d" }}>{w.pending_reviews} review</span>}
          </div>
        </div>
      </div>

      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", paddingTop: 10, borderTop: "1px solid var(--border)" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <img src={p.logo} style={{ width: 14, height: 14 }} alt=""/>
          <span style={{ fontSize: 11.5, color: "var(--foreground)" }}>{p.name}</span>
          <span style={{ fontSize: 10, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{p.model}</span>
        </div>
        <div style={{ display: "flex", gap: 4 }} onClick={e => e.stopPropagation()}>
          <button onClick={() => go("tasks", { workflow: w.name })} style={miniBtn}>Tasks</button>
          <button onClick={() => go("spec", { workflow: w.name })} style={miniBtn}>Spec</button>
        </div>
      </div>
    </div>
  );
}
const miniBtn = {
  height: 24, padding: "0 8px", borderRadius: 4,
  border: "1px solid var(--border)", background: "var(--secondary)",
  fontSize: 11, color: "var(--foreground)", cursor: "pointer", fontFamily: "inherit",
};

window.WorkflowsView = WorkflowsView;
