/* global React, Icons, Badge, Button, StatusDot, IconBtn, RUNS, WORKFLOWS, PROVIDERS, SAMPLE_LOG */
// Runs list + Run detail (split view with live log).

function RunsView({ workflow, go }) {
  const [filter, setFilter] = React.useState("all");
  const base = workflow ? RUNS.filter(r => r.workflow === workflow) : RUNS;
  const visible = base.filter(r => filter === "all" ? true : r.status === filter);
  const counts = {
    all: base.length,
    running: base.filter(r => r.status === "running").length,
    done: base.filter(r => r.status === "done").length,
    failed: base.filter(r => r.status === "failed").length,
    paused: base.filter(r => r.status === "paused").length,
  };
  return (
    <div style={{ padding: "24px 28px 40px", display: "flex", flexDirection: "column", gap: 20, maxWidth: 1400 }}>
      <header style={{ display: "flex", alignItems: "flex-end", justifyContent: "space-between" }}>
        <div>
          <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: 4 }}>
            {workflow ? `Workflow · ${workflow}` : "Across all workflows"}
          </div>
          <h1 style={{ margin: 0, fontFamily: "var(--font-display)", fontWeight: 500, fontSize: 36, letterSpacing: "-0.02em" }}>Runs</h1>
          {!workflow && (
            <div style={{ fontSize: 13, color: "var(--muted-foreground)", marginTop: 8, maxWidth: 680, lineHeight: 1.5 }}>
              Every execution the daemon has dispatched. Click a run to see its jobs and logs; click the workflow name to open it.
            </div>
          )}
        </div>
        <Button variant="primary" size="sm" icon={<Icons.Play/>}>Start run</Button>
      </header>

      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        {[{k:"all",l:"All"},{k:"running",l:"Running"},{k:"done",l:"Done"},{k:"failed",l:"Failed"},{k:"paused",l:"Paused"}].map(f => (
          <button key={f.k} onClick={() => setFilter(f.k)} style={{
            height: 26, padding: "0 10px", borderRadius: 5,
            border: `1px solid ${filter === f.k ? "var(--primary)" : "var(--border)"}`,
            background: filter === f.k ? "rgba(202,234,40,0.08)" : "var(--card)",
            color: filter === f.k ? "var(--primary)" : "var(--foreground)",
            fontSize: 12, fontFamily: "inherit", cursor: "pointer",
            display: "inline-flex", alignItems: "center", gap: 6,
          }}>
            {f.l}<span style={{ fontSize: 10, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{counts[f.k]}</span>
          </button>
        ))}
      </div>

      <div style={{ background: "var(--card)", border: "1px solid var(--border)", borderRadius: 8, overflow: "hidden" }}>
        <div style={{
          display: "grid", gridTemplateColumns: "160px 1fr 110px 170px 110px 120px 40px",
          padding: "10px 16px", background: "var(--secondary)", borderBottom: "1px solid var(--border)",
          fontSize: 10, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em",
        }}>
          <span>Run</span><span>Workflow</span><span>Status</span><span>Progress</span><span>Duration</span><span>Tokens</span><span/>
        </div>
        {visible.map(r => {
          const pct = Math.round((r.jobs_done / r.jobs_total) * 100);
          const sv = { running: "info", done: "success", failed: "destructive", paused: "warning" }[r.status];
          return (
            <div key={r.id} onClick={() => go("run-detail", { run: r.id })} style={{
              display: "grid", gridTemplateColumns: "160px 1fr 110px 170px 110px 120px 40px",
              padding: "12px 16px", borderBottom: "1px solid var(--border)",
              alignItems: "center", cursor: "pointer", gap: 12,
            }}>
              <div style={{ display: "flex", alignItems: "center", gap: 8, minWidth: 0 }}>
                <img src={PROVIDERS[r.provider].logo} style={{ width: 18, height: 18 }}/>
                <span style={{ fontSize: 12, fontFamily: "var(--font-mono)", color: "var(--foreground)" }}>{r.id}</span>
              </div>
              <span style={{ fontSize: 12.5, color: "var(--foreground)" }}>{r.workflow}</span>
              <Badge variant={sv}>{r.status}</Badge>
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <div style={{ flex: 1, height: 4, borderRadius: 2, background: "var(--secondary)", overflow: "hidden" }}>
                  <div style={{ width: `${pct}%`, height: "100%", background: r.status === "failed" ? "#ef4444" : r.status === "done" ? "#10b981" : "#3b82f6" }}/>
                </div>
                <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", minWidth: 44, textAlign: "right" }}>{r.jobs_done}/{r.jobs_total}</span>
              </div>
              <span style={{ fontSize: 11, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{r.duration}</span>
              <span style={{ fontSize: 11, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{r.tokens_in}</span>
              <Icons.ArrowRight size={14}/>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function RunDetailView({ runId = "run_2a9f", go }) {
  const r = RUNS.find(x => x.id === runId) || RUNS[0];
  const w = WORKFLOWS.find(x => x.name === r.workflow) || WORKFLOWS[0];
  const p = PROVIDERS[r.provider];
  const running = r.status === "running";

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <div style={{ padding: "22px 28px 18px", borderBottom: "1px solid var(--border)" }}>
        <div style={{ display: "flex", alignItems: "flex-end", justifyContent: "space-between", gap: 16 }}>
          <div>
            <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", marginBottom: 4 }}>Run · {r.id}</div>
            <h1 style={{ margin: 0, fontFamily: "var(--font-display)", fontWeight: 500, fontSize: 28, letterSpacing: "-0.02em" }}>{w.title}</h1>
            <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 10 }}>
              <StatusDot status={r.status} size={12}/>
              <span style={{ fontSize: 12, textTransform: "capitalize" }}>{r.status}</span>
              <span style={{ color: "var(--muted-foreground)" }}>·</span>
              <span style={{ fontSize: 11.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>started {r.started}</span>
              <span style={{ color: "var(--muted-foreground)" }}>·</span>
              <span style={{ fontSize: 11.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{r.duration}</span>
            </div>
          </div>
          <div style={{ display: "flex", gap: 6 }}>
            {running && <>
              <Button variant="outline" size="sm" icon={<Icons.Pause size={13}/>}>Pause</Button>
              <Button variant="outline" size="sm" icon={<Icons.Square size={13}/>}>Stop</Button>
            </>}
            {r.status === "failed" && <Button variant="primary" size="sm" icon={<Icons.RotateCcw size={13}/>}>Retry failed</Button>}
            {r.status === "paused" && <Button variant="primary" size="sm" icon={<Icons.Play/>}>Resume</Button>}
          </div>
        </div>

        <div style={{
          marginTop: 18, display: "grid", gridTemplateColumns: "repeat(5, 1fr)",
          gap: 14, padding: "14px 16px", background: "var(--card)", border: "1px solid var(--border)", borderRadius: 6,
        }}>
          <Metric2 label="Agent" value={<span style={{ display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13 }}><img src={p.logo} style={{ width: 14, height: 14 }}/>{p.name}</span>} sub={r.model}/>
          <Metric2 label="Reasoning" value={r.reasoning} sub="effort"/>
          <Metric2 label="Concurrent" value={r.concurrent} sub={`batch ${r.batch}`}/>
          <Metric2 label="Tokens in" value={r.tokens_in} sub="prompt"/>
          <Metric2 label="Tokens out" value={r.tokens_out} sub="response"/>
        </div>

        <div style={{
          marginTop: 14, padding: "10px 14px", background: "#0c0b09", border: "1px solid var(--border)", borderRadius: 6,
          fontFamily: "var(--font-mono)", fontSize: 11.5, display: "flex", alignItems: "center", gap: 10,
        }}>
          <span style={{ color: "var(--primary)", fontWeight: 600 }}>$</span>
          <span style={{ color: "var(--foreground)", userSelect: "all" }}>{r.command}</span>
          <span style={{ flex: 1 }}/>
          <IconBtn tip="Copy"><Icons.Copy/></IconBtn>
        </div>
      </div>

      <div style={{ flex: 1, display: "grid", gridTemplateColumns: "1fr 1fr", minHeight: 0 }}>
        <RunJobs r={r} go={go}/>
        <div style={{ display: "flex", flexDirection: "column", minHeight: 0, borderLeft: "1px solid var(--border)", background: "var(--card)" }}>
          <LogTail running={running}/>
        </div>
      </div>
    </div>
  );
}

function Metric2({ label, value, sub }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <span style={{ fontSize: 9.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em" }}>{label}</span>
      <span style={{ fontSize: 15, fontFamily: "var(--font-sans)", fontWeight: 600, color: "var(--foreground)" }}>{value}</span>
      {sub && <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{sub}</span>}
    </div>
  );
}

function RunJobs({ r, go }) {
  // synth list of jobs based on counts
  const jobs = [];
  for (let i = 0; i < r.jobs_done; i++) jobs.push({ id: `task_${String(i+1).padStart(2,"0")}`, status: "done", duration: `${3 + (i%6)}m ${(i*13)%60}s` });
  for (let i = 0; i < r.jobs_running; i++) jobs.push({ id: `task_${String(r.jobs_done+i+1).padStart(2,"0")}`, status: "running", duration: `${i+1}m · running` });
  for (let i = 0; i < r.jobs_failed; i++) jobs.push({ id: `task_${String(r.jobs_done+r.jobs_running+i+1).padStart(2,"0")}`, status: "failed", duration: "timeout" });
  for (let i = 0; i < r.jobs_pending; i++) jobs.push({ id: `task_${String(r.jobs_done+r.jobs_running+r.jobs_failed+i+1).padStart(2,"0")}`, status: "pending", duration: "—" });

  return (
    <div style={{ display: "flex", flexDirection: "column", minHeight: 0 }}>
      <header style={{ padding: "12px 16px", borderBottom: "1px solid var(--border)", display: "flex", alignItems: "center", gap: 8 }}>
        <Icons.ListTodo size={14}/>
        <span style={{ fontSize: 12, fontWeight: 600 }}>Jobs</span>
        <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{jobs.length}</span>
      </header>
      <div style={{ flex: 1, overflow: "auto" }}>
        {jobs.map((j, i) => (
          <div key={i} onClick={() => go("task-detail", { workflow: r.workflow, task: j.id })} style={{
            display: "grid", gridTemplateColumns: "16px 90px 1fr 90px",
            padding: "8px 16px", borderBottom: "1px solid var(--border)",
            alignItems: "center", cursor: "pointer", gap: 10,
          }}>
            <StatusDot status={j.status} size={11}/>
            <span style={{ fontSize: 11.5, fontFamily: "var(--font-mono)", color: "var(--foreground)" }}>{j.id}</span>
            <span style={{ fontSize: 11.5, color: "var(--muted-foreground)", textTransform: "capitalize" }}>{j.status}</span>
            <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textAlign: "right" }}>{j.duration}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// Reuse LogTail from task_detail
function LogTail({ running }) {
  const [tick, setTick] = React.useState(0);
  React.useEffect(() => {
    if (!running) return;
    const i = setInterval(() => setTick(t => t + 1), 2500);
    return () => clearInterval(i);
  }, [running]);
  const ref = React.useRef(null);
  React.useEffect(() => { if (ref.current) ref.current.scrollTop = ref.current.scrollHeight; }, [tick]);
  const lvColor = { info: "var(--foreground)", debug: "var(--muted-foreground)", claude: "#fcd34d", tool: "#93c5fd", error: "#fca5a5" };
  return (
    <>
      <header style={{ display: "flex", alignItems: "center", gap: 10, padding: "12px 16px", borderBottom: "1px solid var(--border)", background: "var(--background)" }}>
        <Icons.Terminal size={14}/>
        <span style={{ fontSize: 12, fontWeight: 600 }}>Live log</span>
        <span style={{ flex: 1 }}/>
        {running && (
          <span style={{ display: "inline-flex", alignItems: "center", gap: 5, fontSize: 10.5, fontFamily: "var(--font-mono)", color: "#10b981" }}>
            <span style={{ width: 6, height: 6, borderRadius: 999, background: "#10b981", boxShadow: "0 0 6px rgba(16,185,129,0.7)", animation: "pulse 1.4s infinite" }}/>
            streaming
          </span>
        )}
      </header>
      <div ref={ref} style={{ flex: 1, overflow: "auto", padding: "8px 0", fontFamily: "var(--font-mono)", fontSize: 11.5, lineHeight: 1.5 }}>
        {SAMPLE_LOG.map((l, i) => (
          <div key={i} style={{ display: "grid", gridTemplateColumns: "66px 52px 1fr", gap: 10, padding: "2px 16px" }}>
            <span style={{ color: "var(--muted-foreground)" }}>{l.t}</span>
            <span style={{ color: lvColor[l.lv], fontWeight: 600, textTransform: "uppercase", fontSize: 9.5, letterSpacing: "0.04em", paddingTop: 2 }}>{l.lv}</span>
            <span style={{ color: lvColor[l.lv] || "var(--foreground)", whiteSpace: "pre-wrap", wordBreak: "break-word" }}>{l.msg}</span>
          </div>
        ))}
      </div>
    </>
  );
}

window.RunsView = RunsView;
window.RunDetailView = RunDetailView;
