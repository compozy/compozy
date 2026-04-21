/* global React, Icons, Button, Badge, Markdown, WORKFLOWS, MEMORY_SHARED, MEMORY_TASK, PROVIDERS */
// Memory — global index + per-workflow viewer.

function MemoryIndexView({ go }) {
  // Global index: all workflows that have memory
  const rows = WORKFLOWS.filter(w => w.status !== "archived").map(w => ({
    workflow: w,
    files: 9 + (w.name.length % 4), // synth
    size: (3 + (w.name.length % 5)) + "." + (w.name.length % 9) + " KB",
    updated: w.updated,
    lastTask: w.status === "running" ? "task_08.md" : "task_06.md",
    active: w.status === "running",
  }));
  return (
    <div style={{ padding: "24px 28px 40px", display: "flex", flexDirection: "column", gap: 22, maxWidth: 1400 }}>
      <header>
        <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: 4 }}>Across workspace</div>
        <h1 style={{ margin: 0, fontFamily: "var(--font-display)", fontWeight: 500, fontSize: 36, letterSpacing: "-0.02em" }}>Memory</h1>
        <div style={{ fontSize: 13, color: "var(--muted-foreground)", marginTop: 8, maxWidth: 680, lineHeight: 1.5 }}>
          Each workflow keeps its own memory store — a shared <code style={{ fontFamily: "var(--font-mono)", fontSize: 12, padding: "1px 5px", background: "var(--secondary)", borderRadius: 3 }}>MEMORY.md</code> and per-task notebooks that agents write after every task.
        </div>
      </header>

      <div style={{ background: "var(--card)", border: "1px solid var(--border)", borderRadius: 8, overflow: "hidden" }}>
        <div style={{ display: "grid", gridTemplateColumns: "24px 1fr 90px 90px 180px 110px 40px", padding: "10px 16px", background: "var(--secondary)", borderBottom: "1px solid var(--border)", fontSize: 10, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em" }}>
          <span/><span>Workflow</span><span>Files</span><span>Size</span><span>Last task patch</span><span>Updated</span><span/>
        </div>
        {rows.map(({ workflow: w, files, size, updated, lastTask, active }) => (
          <div key={w.name} onClick={() => go("memory", { workflow: w.name })} style={{
            display: "grid", gridTemplateColumns: "24px 1fr 90px 90px 180px 110px 40px",
            padding: "12px 16px", borderBottom: "1px solid var(--border)",
            alignItems: "center", cursor: "pointer", gap: 10,
          }}>
            <Icons.Brain size={14} style={{ color: active ? "var(--primary)" : "var(--muted-foreground)" }}/>
            <div style={{ display: "flex", alignItems: "center", gap: 8, minWidth: 0 }}>
              <Icons.GitBranch size={12} style={{ color: "var(--muted-foreground)" }}/>
              <span style={{ fontSize: 12.5, fontFamily: "var(--font-mono)", color: "var(--foreground)" }}>{w.name}</span>
              {active && <Badge variant="info">live</Badge>}
              <span style={{ fontSize: 11, color: "var(--muted-foreground)" }}>· {w.title}</span>
            </div>
            <span style={{ fontSize: 11.5, fontFamily: "var(--font-mono)", color: "var(--foreground)" }}>{files}</span>
            <span style={{ fontSize: 11.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{size}</span>
            <span style={{ fontSize: 11.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{lastTask}</span>
            <span style={{ fontSize: 11, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{updated}</span>
            <Icons.ArrowRight size={14}/>
          </div>
        ))}
      </div>
    </div>
  );
}

function MemoryView({ workflow = "user-auth", go }) {
  const w = WORKFLOWS.find(x => x.name === workflow) || WORKFLOWS[0];
  const [sel, setSel] = React.useState("MEMORY.md");
  const files = [
    { name: "MEMORY.md",  size: "4.1 KB", updated: "42s ago", kind: "shared" },
    { name: "task_01.md", size: "1.2 KB", updated: "3d ago",  kind: "task" },
    { name: "task_04.md", size: "1.4 KB", updated: "2d ago",  kind: "task" },
    { name: "task_07.md", size: "2.8 KB", updated: "2d ago",  kind: "task" },
    { name: "task_08.md", size: "3.3 KB", updated: "42s ago", kind: "task", active: true },
    { name: "task_09.md", size: "0.9 KB", updated: "1m ago",  kind: "task", active: true },
    { name: "task_10.md", size: "0.6 KB", updated: "15m ago", kind: "task", failed: true },
  ];
  const text = sel === "MEMORY.md" ? MEMORY_SHARED : MEMORY_TASK;
  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <div style={{ padding: "22px 28px 16px", borderBottom: "1px solid var(--border)" }}>
        <div style={{ display: "flex", alignItems: "flex-end", justifyContent: "space-between", gap: 16 }}>
          <div>
            <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: 4, display: "flex", alignItems: "center", gap: 6 }}>
              <button onClick={() => go("memory-index")} style={{ background: "transparent", border: 0, color: "var(--muted-foreground)", fontFamily: "inherit", fontSize: "inherit", cursor: "pointer", padding: 0, textTransform: "inherit", letterSpacing: "inherit" }}>Memory</button>
              <span>/</span><span>{w.name}</span>
            </div>
            <h1 style={{ margin: 0, fontFamily: "var(--font-display)", fontWeight: 500, fontSize: 26, letterSpacing: "-0.02em" }}>{w.title}</h1>
            <div style={{ fontSize: 12, color: "var(--muted-foreground)", marginTop: 8, maxWidth: 640, lineHeight: 1.5 }}>
              Shared memory + per-task notebooks for <code style={{ fontFamily: "var(--font-mono)" }}>{w.name}</code>.
            </div>
          </div>
          <div style={{ display: "flex", gap: 6 }}>
            <Button variant="outline" size="sm" icon={<Icons.FileText size={13}/>} onClick={() => go("spec", { workflow: w.name })}>Spec</Button>
            <Button variant="outline" size="sm" icon={<Icons.ListTodo size={13}/>} onClick={() => go("tasks", { workflow: w.name })}>Tasks</Button>
          </div>
        </div>
      </div>

      <div style={{ flex: 1, display: "grid", gridTemplateColumns: "280px 1fr", minHeight: 0 }}>
        <aside style={{ borderRight: "1px solid var(--border)", overflow: "auto", padding: "12px 10px", background: "var(--card)" }}>
          <div style={{ padding: "4px 10px 10px", fontSize: 10, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em" }}>.compozy/memory/{w.name}</div>
          <div style={{ padding: "0 8px 6px 8px", fontSize: 10, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em" }}>Shared</div>
          {files.filter(f => f.kind === "shared").map(f => (
            <FileItem key={f.name} f={f} sel={sel === f.name} onClick={() => setSel(f.name)}/>
          ))}
          <div style={{ padding: "14px 8px 6px", fontSize: 10, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", textTransform: "uppercase", letterSpacing: "0.08em" }}>Per-task notebooks</div>
          {files.filter(f => f.kind === "task").map(f => (
            <FileItem key={f.name} f={f} sel={sel === f.name} onClick={() => setSel(f.name)}/>
          ))}
        </aside>

        <main style={{ overflow: "auto" }}>
          <div style={{ padding: "18px 28px 8px", borderBottom: "1px solid var(--border)", display: "flex", alignItems: "center", gap: 10 }}>
            <Icons.FileText size={15}/>
            <span style={{ fontFamily: "var(--font-mono)", fontSize: 12.5, color: "var(--foreground)" }}>.compozy/memory/{w.name}/{sel}</span>
            <span style={{ flex: 1 }}/>
            {sel === "MEMORY.md" && <Badge variant="lime">shared</Badge>}
          </div>
          <article style={{ padding: "20px 28px 40px", maxWidth: 780 }}>
            <Markdown text={text}/>
          </article>
        </main>
      </div>
    </div>
  );
}

function FileItem({ f, sel, onClick }) {
  return (
    <button onClick={onClick} style={{
      width: "100%", textAlign: "left", border: 0, background: sel ? "var(--sidebar-accent)" : "transparent",
      padding: "7px 10px", borderRadius: 5, cursor: "pointer", fontFamily: "inherit",
      display: "grid", gridTemplateColumns: "14px 1fr auto", alignItems: "center", gap: 8, marginBottom: 1,
    }}>
      <Icons.FileText size={12} style={{ color: "var(--muted-foreground)" }}/>
      <div style={{ minWidth: 0 }}>
        <div style={{ fontSize: 12, fontFamily: "var(--font-mono)", color: "var(--foreground)", display: "flex", alignItems: "center", gap: 6 }}>
          {f.name}
          {f.active && <span style={{ width: 5, height: 5, borderRadius: 999, background: "#3b82f6", boxShadow: "0 0 5px rgba(59,130,246,0.6)" }}/>}
          {f.failed && <span style={{ width: 5, height: 5, borderRadius: 999, background: "#ef4444" }}/>}
        </div>
        <div style={{ fontSize: 10, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" }}>{f.size} · {f.updated}</div>
      </div>
    </button>
  );
}

window.MemoryIndexView = MemoryIndexView;
window.MemoryView = MemoryView;
