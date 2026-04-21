/* global React, Icons, Badge, Button, Sparkline, PipelineBar, StatusDot, DAEMON, WORKFLOWS, RUNS, REVIEW_ISSUES, PROVIDERS */
// Dashboard — daemon overview: status, active runs, queue, pending reviews.

const { useState: useStateD } = React;

function DashboardView({ go }) {
  const active = WORKFLOWS.filter(w => w.status === "running");
  const queued = WORKFLOWS.filter(w => w.status === "running" || w.status === "paused").flatMap(w =>
    Array.from({ length: w.tasks.pending }, (_, i) => ({ workflow: w.name, idx: w.tasks.done + w.tasks.running + i + 1, total: w.tasks.total }))
  );
  const pendingReviews = REVIEW_ISSUES.filter(i => i.status === "open");

  return (
    <div style={{ padding: "24px 28px 40px", display: "flex", flexDirection: "column", gap: 28, maxWidth: 1400 }}>

      {/* Page header */}
      <header style={{ display: "flex", alignItems: "flex-end", justifyContent: "space-between", gap: 16 }}>
        <div>
          <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: 4 }}>Overview</div>
          <h1 style={{ margin: 0, fontFamily: "var(--font-display)", fontWeight: 500, fontSize: 38, letterSpacing: "-0.02em", lineHeight: 1.02, color: "var(--foreground)" }}>
            1 run in flight · <span style={{ color: "var(--muted-foreground)" }}>6 workflows total</span>
          </h1>
        </div>
        <div style={{ display: "flex", gap: 6 }}>
          <Button variant="outline" size="sm" icon={<Icons.Archive size={14}/>}>Archive cleaned</Button>
          <Button variant="outline" size="sm" icon={<Icons.RefreshCw size={14}/>}>Sync all</Button>
        </div>
      </header>

      {/* Daemon status strip */}
      <DaemonStatusStrip/>

      {/* Row: active runs + queue */}
      <div style={{ display: "grid", gridTemplateColumns: "1.4fr 1fr", gap: 16, alignItems: "stretch" }}>
        <Card title="Active runs" kicker={`${active.length} running`} action={<button onClick={() => go("runs")} style={linkStyle}>All runs <Icons.ArrowRight size={12}/></button>}>
          <ActiveRunsList go={go}/>
        </Card>
        <Card title="Queue" kicker={`${queued.length} tasks pending`} action={<span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>concurrent · 2</span>}>
          <QueuePanel queued={queued} go={go}/>
        </Card>
      </div>

      {/* Row: reviews + activity */}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1.2fr", gap: 16, alignItems: "stretch" }}>
        <Card title="Pending reviews" kicker={`${pendingReviews.length} open across 1 workflow`} action={<button onClick={() => go("reviews")} style={linkStyle}>Open reviews <Icons.ArrowRight size={12}/></button>}>
          <PendingReviewsPanel issues={pendingReviews} go={go}/>
        </Card>
        <Card title="Activity" kicker="last 2 hours">
          <ActivityTimeline/>
        </Card>
      </div>
    </div>
  );
}

const linkStyle = {
  display: "inline-flex", alignItems: "center", gap: 4,
  background: "transparent", border: 0,
  fontSize: 11.5, color: "var(--muted-foreground)", cursor: "pointer",
  fontFamily: "var(--font-sans)",
};

function Card({ title, kicker, action, children }) {
  return (
    <section style={{
      background: "var(--card)", border: "1px solid var(--border)", borderRadius: 8,
      boxShadow: "var(--shadow-sm)", overflow: "hidden",
      display: "flex", flexDirection: "column",
    }}>
      <header style={{
        display: "flex", alignItems: "center", justifyContent: "space-between",
        padding: "12px 16px", borderBottom: "1px solid var(--border)",
      }}>
        <div style={{ display: "flex", alignItems: "baseline", gap: 10, minWidth: 0 }}>
          <h3 style={{ margin: 0, fontSize: 13, fontWeight: 600, color: "var(--foreground)", letterSpacing: "-0.005em" }}>{title}</h3>
          {kicker && <span style={{ fontSize: 11, color: "var(--muted-foreground)" }}>{kicker}</span>}
        </div>
        {action}
      </header>
      <div style={{ flex: 1 }}>{children}</div>
    </section>
  );
}

function DaemonStatusStrip() {
  return (
    <div style={{
      border: "1px solid var(--border)", borderRadius: 8,
      background: "linear-gradient(180deg, rgba(202,234,40,0.04), transparent)",
      padding: "14px 18px", display: "grid",
      gridTemplateColumns: "auto 1fr repeat(5, max-content)", alignItems: "center", gap: 22,
    }}>
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <div style={{ position: "relative", width: 36, height: 36, borderRadius: 8, background: "rgba(16,185,129,0.08)", display: "grid", placeItems: "center", border: "1px solid rgba(16,185,129,0.28)" }}>
          <Icons.Server size={18}/>
          <span style={{ position: "absolute", top: 4, right: 4, width: 6, height: 6, borderRadius: 999, background: "#10b981", boxShadow: "0 0 6px rgba(16,185,129,0.6)" }}/>
        </div>
        <div>
          <div style={{ fontSize: 12.5, fontWeight: 600, color: "var(--foreground)", display: "flex", alignItems: "center", gap: 8 }}>
            Daemon healthy
            <Badge variant="muted" mono>v{DAEMON.version}</Badge>
          </div>
          <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>
            pid {DAEMON.pid} · {DAEMON.api}
          </div>
        </div>
      </div>
      <div/>
      <Metric label="Uptime" value={DAEMON.uptime}/>
      <Metric label="Active runs" value={DAEMON.active_runs} accent={DAEMON.active_runs > 0 ? "var(--primary)" : null}/>
      <Metric label="Agents" value={DAEMON.agents_running} suffix="running"/>
      <Metric label="Queue" value={DAEMON.queue_depth} suffix="tasks"/>
      <div style={{ display: "flex", flexDirection: "column", gap: 2, alignItems: "flex-end" }}>
        <div style={{ fontSize: 9.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em" }}>Tokens today</div>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <Sparkline data={DAEMON.tokens_series} w={92} h={24}/>
          <span style={{ fontFamily: "var(--font-mono)", fontSize: 12, color: "var(--foreground)" }}>
            {DAEMON.tokens_today.in}<span style={{ color: "var(--muted-foreground)" }}> in</span>
          </span>
        </div>
      </div>
    </div>
  );
}

function Metric({ label, value, suffix, accent }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <div style={{ fontSize: 9.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em" }}>{label}</div>
      <div style={{ display: "flex", alignItems: "baseline", gap: 5 }}>
        <span style={{ fontSize: 18, fontFamily: "var(--font-display)", fontWeight: 500, color: accent || "var(--foreground)", letterSpacing: "-0.01em" }}>{value}</span>
        {suffix && <span style={{ fontSize: 10.5, color: "var(--muted-foreground)" }}>{suffix}</span>}
      </div>
    </div>
  );
}

function ActiveRunsList({ go }) {
  const active = RUNS.filter(r => r.status === "running");
  return (
    <div>
      {active.map(r => {
        const w = WORKFLOWS.find(x => x.name === r.workflow);
        const pct = Math.round((r.jobs_done / r.jobs_total) * 100);
        return (
          <div key={r.id} onClick={() => go("run-detail", { run: r.id })} style={{
            padding: "14px 16px", borderBottom: "1px solid var(--border)", cursor: "pointer",
            display: "grid", gridTemplateColumns: "auto 1fr auto", alignItems: "center", gap: 14,
          }}>
            <img src={PROVIDERS[r.provider].logo} style={{ width: 28, height: 28, borderRadius: 5, background: "var(--secondary)", padding: 4, border: "1px solid var(--border)" }} alt=""/>
            <div style={{ minWidth: 0 }}>
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <span style={{ fontSize: 13, fontWeight: 600, color: "var(--foreground)" }}>{w.title}</span>
                <Badge variant="muted" mono>{r.id}</Badge>
              </div>
              <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 6, fontSize: 11, color: "var(--muted-foreground)", fontFamily: "var(--font-mono)" }}>
                <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}><Icons.GitBranch size={11}/>{r.workflow}</span>
                <span>·</span><span>{r.duration}</span>
                <span>·</span><span>{r.tokens_in} in</span>
              </div>
              <div style={{ marginTop: 8, display: "grid", gridTemplateColumns: "1fr auto", gap: 10, alignItems: "center" }}>
                <ProgressBar total={r.jobs_total} done={r.jobs_done} running={r.jobs_running} failed={r.jobs_failed}/>
                <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", whiteSpace: "nowrap" }}>
                  <span style={{ color: "var(--foreground)", fontWeight: 600 }}>{r.jobs_done}</span>/{r.jobs_total} · {pct}%
                </span>
              </div>
            </div>
            <Icons.ArrowRight size={14}/>
          </div>
        );
      })}
      {active.length === 0 && (
        <div style={{ padding: "32px 16px", textAlign: "center", color: "var(--muted-foreground)", fontSize: 12 }}>No runs in flight.</div>
      )}
    </div>
  );
}

function ProgressBar({ total, done, running, failed }) {
  const seg = (n) => `${(n/total)*100}%`;
  return (
    <div style={{ height: 6, borderRadius: 999, background: "var(--secondary)", overflow: "hidden", display: "flex" }}>
      <div style={{ width: seg(done), background: "#10b981" }}/>
      <div style={{ width: seg(running), background: "#3b82f6", backgroundImage: "linear-gradient(90deg, rgba(255,255,255,0.18) 25%, transparent 25%, transparent 50%, rgba(255,255,255,0.18) 50%, rgba(255,255,255,0.18) 75%, transparent 75%)", backgroundSize: "12px 12px", animation: "barber 0.8s linear infinite" }}/>
      <div style={{ width: seg(failed), background: "#ef4444" }}/>
    </div>
  );
}

function QueuePanel({ queued, go }) {
  // group by workflow
  const groups = queued.reduce((acc, q) => { (acc[q.workflow] ||= []).push(q); return acc; }, {});
  return (
    <div style={{ padding: "8px 0" }}>
      {Object.entries(groups).map(([wf, items]) => {
        const w = WORKFLOWS.find(x => x.name === wf);
        return (
          <div key={wf} onClick={() => go("tasks", { workflow: wf })} style={{
            padding: "10px 16px", borderBottom: "1px solid var(--border)", cursor: "pointer",
            display: "grid", gridTemplateColumns: "1fr auto", gap: 10, alignItems: "center",
          }}>
            <div style={{ minWidth: 0 }}>
              <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 5 }}>
                <Icons.GitBranch size={12}/>
                <span style={{ fontSize: 12.5, fontFamily: "var(--font-mono)", color: "var(--foreground)" }}>{wf}</span>
                <Badge variant={w.status === "paused" ? "warning" : "info"}>{w.status}</Badge>
              </div>
              <div style={{ display: "flex", gap: 3, flexWrap: "wrap" }}>
                {items.slice(0, 12).map((q, i) => (
                  <span key={i} style={{
                    fontSize: 9, fontFamily: "var(--font-mono)",
                    padding: "2px 6px", borderRadius: 3,
                    background: "var(--secondary)", color: "var(--muted-foreground)",
                    border: "1px dashed var(--border)",
                  }}>task_{String(q.idx).padStart(2, "0")}</span>
                ))}
              </div>
            </div>
            <span style={{ fontSize: 11, color: "var(--muted-foreground)", fontFamily: "var(--font-mono)", whiteSpace: "nowrap" }}>{items.length} pending</span>
          </div>
        );
      })}
    </div>
  );
}

function PendingReviewsPanel({ issues, go }) {
  const sev = { high: "destructive", medium: "warning", low: "muted" };
  return (
    <div>
      {issues.map(i => (
        <div key={i.id} onClick={() => go("review-detail", { workflow: "manifest-v2", round: "002", issue: i.id })} style={{
          padding: "10px 16px", borderBottom: "1px solid var(--border)", cursor: "pointer",
          display: "grid", gridTemplateColumns: "14px 1fr auto", gap: 10, alignItems: "center",
        }}>
          <StatusDot status="open" size={12}/>
          <div style={{ minWidth: 0 }}>
            <div style={{ fontSize: 12.5, color: "var(--foreground)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{i.title}</div>
            <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", marginTop: 2, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {i.file}:{i.line}
            </div>
          </div>
          <Badge variant={sev[i.severity]}>{i.severity}</Badge>
        </div>
      ))}
    </div>
  );
}

function ActivityTimeline() {
  const kindIcon = {
    task:    <Icons.ListTodo size={12}/>,
    run:     <Icons.Terminal size={12}/>,
    review:  <Icons.MessageSquare size={12}/>,
    archive: <Icons.Archive size={12}/>,
  };
  return (
    <div style={{ padding: "6px 0" }}>
      {DAEMON.events.map((ev, i) => (
        <div key={i} style={{ display: "grid", gridTemplateColumns: "72px 14px 1fr", gap: 10, padding: "8px 16px", fontSize: 12, position: "relative" }}>
          <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textAlign: "right" }}>{ev.t}</span>
          <span style={{ position: "relative", display: "flex", alignItems: "flex-start", justifyContent: "center", color: "var(--muted-foreground)", paddingTop: 2 }}>
            {i < DAEMON.events.length - 1 && <span style={{ position: "absolute", top: 16, left: "50%", bottom: -10, width: 1, background: "var(--border)" }}/>}
            <span style={{ position: "relative", width: 14, height: 14, borderRadius: 999, background: "var(--card)", border: "1px solid var(--border)", display: "grid", placeItems: "center", zIndex: 1 }}>
              {kindIcon[ev.kind]}
            </span>
          </span>
          <div style={{ minWidth: 0 }}>
            <div style={{ color: "var(--foreground)", fontSize: 12.5 }}>{ev.msg}</div>
            <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", marginTop: 1 }}>
              <span>{ev.workflow}</span>
              {ev.kind && <> · {ev.kind}</>}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

window.DashboardView = DashboardView;
