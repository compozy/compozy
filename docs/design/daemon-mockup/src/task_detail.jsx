/* global React, Icons, Badge, Button, StatusDot, IconBtn, Markdown, TASKS_USERAUTH, MEMORY_TASK, PROVIDERS, WORKFLOWS, SAMPLE_LOG */
// Task detail — objective, files, memory, and a live log tail.

function TaskDetailView({ workflow = "user-auth", taskId = "task_08", go }) {
  const w = WORKFLOWS.find(x => x.name === workflow) || WORKFLOWS[0];
  const task = TASKS_USERAUTH.find(t => t.id === taskId) || TASKS_USERAUTH[7];
  const running = task.status === "running";
  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <div style={{ padding: "22px 28px 18px", borderBottom: "1px solid var(--border)" }}>
        <div style={{ display: "flex", alignItems: "flex-end", justifyContent: "space-between", gap: 16 }}>
          <div style={{ minWidth: 0 }}>
            <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", marginBottom: 6, display: "flex", alignItems: "center", gap: 6 }}>
              <button onClick={() => go("tasks", { workflow: w.name })} style={{ background: "transparent", border: 0, color: "var(--muted-foreground)", fontFamily: "inherit", fontSize: "inherit", cursor: "pointer", padding: 0 }}>tasks</button>
              <span>/</span><span>{task.id}</span>
            </div>
            <h1 style={{ margin: 0, fontFamily: "var(--font-display)", fontWeight: 500, fontSize: 26, letterSpacing: "-0.02em", lineHeight: 1.15 }}>{task.title}</h1>
            <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 10 }}>
              <StatusDot status={task.status} size={12}/>
              <span style={{ fontSize: 12, color: "var(--foreground)", textTransform: "capitalize" }}>{task.status}</span>
              <span style={{ color: "var(--muted-foreground)", fontSize: 12 }}>·</span>
              <span style={{ fontSize: 11.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{task.duration}</span>
              <span style={{ color: "var(--muted-foreground)", fontSize: 12 }}>·</span>
              <span style={{ fontSize: 11.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{task.files} files touched</span>
              {task.provider && <>
                <span style={{ color: "var(--muted-foreground)", fontSize: 12 }}>·</span>
                <span style={{ display: "inline-flex", alignItems: "center", gap: 5, fontSize: 11.5 }}>
                  <img src={PROVIDERS[task.provider].logo} style={{ width: 12, height: 12 }}/>{PROVIDERS[task.provider].name}
                </span>
              </>}
            </div>
          </div>
          <div style={{ display: "flex", gap: 6 }}>
            {running && <Button variant="outline" size="sm" icon={<Icons.Pause size={13}/>}>Pause</Button>}
            {task.status === "failed" && <Button variant="primary" size="sm" icon={<Icons.RotateCcw size={13}/>}>Retry</Button>}
            <Button variant="outline" size="sm" icon={<Icons.Terminal size={13}/>} onClick={() => go("run-detail", { run: "run_2a9f" })}>Open run</Button>
          </div>
        </div>
      </div>

      <div style={{ flex: 1, display: "grid", gridTemplateColumns: "1fr 420px", minHeight: 0 }}>
        <div style={{ overflow: "auto", padding: "22px 28px 40px", borderRight: "1px solid var(--border)" }}>
          <SectionHead kicker="memory" title={`${task.id} · notebook`}/>
          <div style={{ maxWidth: 760 }}><Markdown text={MEMORY_TASK}/></div>
        </div>
        <div style={{ overflow: "hidden", display: "flex", flexDirection: "column", minHeight: 0, background: "var(--card)" }}>
          <LogTail running={running}/>
        </div>
      </div>
    </div>
  );
}

function SectionHead({ kicker, title }) {
  return (
    <div style={{ marginBottom: 14 }}>
      <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: 4 }}>{kicker}</div>
      <h2 style={{ margin: 0, fontFamily: "var(--font-sans)", fontWeight: 600, fontSize: 17, letterSpacing: "-0.005em" }}>{title}</h2>
    </div>
  );
}

function LogTail({ running }) {
  const [tick, setTick] = React.useState(0);
  React.useEffect(() => {
    if (!running) return;
    const i = setInterval(() => setTick(t => t + 1), 2500);
    return () => clearInterval(i);
  }, [running]);
  const ref = React.useRef(null);
  React.useEffect(() => {
    if (ref.current) ref.current.scrollTop = ref.current.scrollHeight;
  }, [tick]);

  const extra = running ? [
    { t: "12:04:" + String(40 + tick).padStart(2, "0"), lv: "claude", msg: "Evidence collected: code diffs, test output, memory patch. Summary written to task_08.md." },
  ] : [];
  const lines = [...SAMPLE_LOG, ...extra.slice(0, tick)];

  const lvColor = {
    info:   "var(--foreground)",
    debug:  "var(--muted-foreground)",
    claude: "#fcd34d",
    tool:   "#93c5fd",
    error:  "#fca5a5",
  };
  return (
    <>
      <header style={{ display: "flex", alignItems: "center", gap: 10, padding: "12px 16px", borderBottom: "1px solid var(--border)", background: "var(--background)" }}>
        <Icons.Terminal size={14}/>
        <span style={{ fontSize: 12, fontWeight: 600 }}>Live log</span>
        <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{lines.length} lines</span>
        <span style={{ flex: 1 }}/>
        {running ? (
          <span style={{ display: "inline-flex", alignItems: "center", gap: 5, fontSize: 10.5, fontFamily: "var(--font-mono)", color: "#10b981" }}>
            <span style={{ width: 6, height: 6, borderRadius: 999, background: "#10b981", boxShadow: "0 0 6px rgba(16,185,129,0.7)", animation: "pulse 1.4s infinite" }}/>
            streaming
          </span>
        ) : (
          <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>ended</span>
        )}
        <IconBtn tip="Copy"><Icons.Copy/></IconBtn>
      </header>
      <div ref={ref} style={{ flex: 1, overflow: "auto", padding: "8px 0", fontFamily: "var(--font-mono)", fontSize: 11.5, lineHeight: 1.5 }}>
        {lines.map((l, i) => (
          <div key={i} style={{ display: "grid", gridTemplateColumns: "66px 52px 1fr", gap: 10, padding: "2px 16px" }}>
            <span style={{ color: "var(--muted-foreground)" }}>{l.t}</span>
            <span style={{
              color: lvColor[l.lv], fontWeight: 600, textTransform: "uppercase", fontSize: 9.5, letterSpacing: "0.04em", paddingTop: 2,
            }}>{l.lv}</span>
            <span style={{ color: lvColor[l.lv] || "var(--foreground)", whiteSpace: "pre-wrap", wordBreak: "break-word" }}>{l.msg}</span>
          </div>
        ))}
        {running && (
          <div style={{ padding: "4px 16px 10px", display: "flex", alignItems: "center", gap: 8, color: "var(--muted-foreground)" }}>
            <span style={{ width: 8, height: 14, background: "currentColor", animation: "blink 1s steps(1) infinite", opacity: 0.7 }}/>
          </div>
        )}
      </div>
    </>
  );
}

window.TaskDetailView = TaskDetailView;
