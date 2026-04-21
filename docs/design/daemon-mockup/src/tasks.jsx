/* global React, Icons, Badge, Button, StatusDot, IconBtn, TASKS_USERAUTH, WORKFLOWS, PROVIDERS */
// Tasks view — kanban by status with effort + domain tags.

const { useState: useStateT } = React;

const DOMAIN_COLORS = {
  db:       { bg: "rgba(59,130,246,0.12)", fg: "#93c5fd" },
  backend:  { bg: "rgba(168,85,247,0.12)", fg: "#c4b5fd" },
  api:      { bg: "rgba(16,185,129,0.12)", fg: "#6ee7b7" },
  frontend: { bg: "rgba(236,72,153,0.12)", fg: "#f9a8d4" },
  test:     { bg: "rgba(234,179,8,0.14)",  fg: "#fcd34d" },
};

const COLS = [
  { k: "pending", l: "Pending" },
  { k: "queued",  l: "Queued"  },
  { k: "running", l: "Running" },
  { k: "review",  l: "Review"  },
  { k: "done",    l: "Done"    },
  { k: "failed",  l: "Failed"  },
];

function TasksView({ workflow = "user-auth", go }) {
  const w = WORKFLOWS.find(x => x.name === workflow) || WORKFLOWS[0];
  const tasks = TASKS_USERAUTH;
  const byCol = {};
  COLS.forEach(c => byCol[c.k] = tasks.filter(t => t.status === c.k));
  // squeeze empty cols
  const visibleCols = COLS.filter(c => byCol[c.k].length > 0);

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <div style={{ padding: "22px 28px 16px", borderBottom: "1px solid var(--border)" }}>
        <div style={{ display: "flex", alignItems: "flex-end", justifyContent: "space-between", gap: 16 }}>
          <div>
            <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: 4 }}>
              Workflow · {w.name}
            </div>
            <h1 style={{ margin: 0, fontFamily: "var(--font-display)", fontWeight: 500, fontSize: 28, letterSpacing: "-0.02em" }}>
              Tasks <span style={{ color: "var(--muted-foreground)", fontWeight: 400 }}>{tasks.length}</span>
            </h1>
          </div>
          <div style={{ display: "flex", gap: 6 }}>
            <Button variant="outline" size="sm" icon={<Icons.FileText size={13}/>} onClick={() => go("spec", { workflow: w.name })}>Spec</Button>
            <Button variant="outline" size="sm" icon={<Icons.Brain size={13}/>} onClick={() => go("memory", { workflow: w.name })}>Memory</Button>
            <Button variant="primary" size="sm" icon={<Icons.Plus/>}>New task</Button>
          </div>
        </div>
        <div style={{ display: "flex", gap: 14, marginTop: 14, fontSize: 11.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>
          <span>{byCol.done.length} done</span>
          <span>·</span>
          <span style={{ color: "#93c5fd" }}>{byCol.running.length} running</span>
          <span>·</span>
          <span style={{ color: "#fca5a5" }}>{byCol.failed.length} failed</span>
          <span>·</span>
          <span>{byCol.pending.length} pending</span>
        </div>
      </div>

      <div style={{ flex: 1, overflow: "auto", padding: 20 }}>
        <div style={{ display: "grid", gridTemplateColumns: `repeat(${visibleCols.length}, minmax(260px, 1fr))`, gap: 14, minWidth: "fit-content" }}>
          {visibleCols.map(c => (
            <KanbanColumn key={c.k} col={c} tasks={byCol[c.k]} go={go} workflow={w.name}/>
          ))}
        </div>
      </div>
    </div>
  );
}

function KanbanColumn({ col, tasks, go, workflow }) {
  const colorMap = {
    pending: "#857e77", queued: "#857e77",
    running: "#3b82f6", review: "#f59e0b", done: "#10b981", failed: "#ef4444",
  };
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10, minHeight: 120 }}>
      <header style={{
        display: "flex", alignItems: "center", gap: 8,
        padding: "6px 10px", borderRadius: 5, background: "var(--card)", border: "1px solid var(--border)",
      }}>
        <span style={{ width: 8, height: 8, borderRadius: 2, background: colorMap[col.k] }}/>
        <span style={{ fontSize: 12, fontWeight: 600, color: "var(--foreground)" }}>{col.l}</span>
        <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{tasks.length}</span>
        <span style={{ flex: 1 }}/>
        <IconBtn tip="Column actions"><Icons.MoreH/></IconBtn>
      </header>
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {tasks.map(t => <TaskCard key={t.id} task={t} go={go} workflow={workflow}/>)}
      </div>
    </div>
  );
}

function TaskCard({ task, go, workflow }) {
  const [hover, setHover] = useStateT(false);
  const running = task.status === "running";
  const failed = task.status === "failed";
  const dom = DOMAIN_COLORS[task.domain] || { bg: "var(--secondary)", fg: "var(--muted-foreground)" };
  return (
    <div onClick={() => go("task-detail", { workflow, task: task.id })}
      onMouseEnter={() => setHover(true)} onMouseLeave={() => setHover(false)}
      style={{
        background: "var(--card)",
        border: `1px solid ${failed ? "rgba(239,68,68,0.35)" : running ? "rgba(59,130,246,0.35)" : "var(--border)"}`,
        borderRadius: 6, padding: 12, cursor: "pointer",
        boxShadow: hover ? "var(--shadow-md)" : "var(--shadow-sm)",
        transition: "all 120ms",
        display: "flex", flexDirection: "column", gap: 10,
        position: "relative", overflow: "hidden",
      }}>
      {running && (
        <div style={{ position: "absolute", top: 0, left: 0, right: 0, height: 2, background: "linear-gradient(90deg, transparent, #3b82f6, transparent)", backgroundSize: "200% 100%", animation: "shimmer 1.6s linear infinite" }}/>
      )}
      <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
        <span style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{task.id}</span>
        <span style={{ flex: 1 }}/>
        <span style={{
          fontSize: 9.5, fontFamily: "var(--font-mono)", fontWeight: 600,
          padding: "1px 5px", borderRadius: 3,
          background: dom.bg, color: dom.fg,
          textTransform: "lowercase",
        }}>{task.domain}</span>
        <span style={{
          fontSize: 9.5, fontFamily: "var(--font-mono)", fontWeight: 700,
          width: 18, height: 16, borderRadius: 3,
          background: "var(--secondary)", color: "var(--muted-foreground)",
          display: "inline-flex", alignItems: "center", justifyContent: "center",
        }}>{task.effort}</span>
      </div>
      <div style={{ fontSize: 13, lineHeight: 1.4, color: "var(--foreground)" }}>{task.title}</div>
      {failed && (
        <div style={{ fontSize: 11, color: "#fca5a5", padding: "6px 8px", borderRadius: 4, background: "rgba(239,68,68,0.08)", border: "1px solid rgba(239,68,68,0.2)", fontFamily: "var(--font-mono)" }}>
          {task.error}
        </div>
      )}
      <div style={{ display: "flex", alignItems: "center", gap: 10, fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>
        <span style={{ display: "inline-flex", alignItems: "center", gap: 3 }}><Icons.FileText size={10}/>{task.files}</span>
        <span>·</span>
        <span>{task.duration}</span>
        {task.provider && (
          <>
            <span style={{ flex: 1 }}/>
            <img src={PROVIDERS[task.provider].logo} style={{ width: 12, height: 12 }}/>
          </>
        )}
      </div>
    </div>
  );
}

window.TasksView = TasksView;
