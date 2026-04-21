/* global React, Icons, IconBtn, Kbd, Badge, Button, DAEMON */
// App shell — sidebar (lean, no titlebar), main content slot.

const { useState: useStateSh } = React;

function AppShell({ view, setView, children, params }) {
  const [collapsed, setCollapsed] = useStateSh(false);
  return (
    <div style={{
      position: "fixed", inset: 0, display: "grid",
      gridTemplateColumns: `${collapsed ? 64 : 232}px 1fr`,
      background: "var(--background)", color: "var(--foreground)",
      fontFamily: "var(--font-sans)", transition: "grid-template-columns 200ms",
    }}>
      <Sidebar collapsed={collapsed} setCollapsed={setCollapsed} view={view} setView={setView} activeWorkflow={params && params.workflow}/>
      <main style={{ display: "flex", flexDirection: "column", minWidth: 0, minHeight: 0, overflow: "hidden" }}>
        <TopStrip view={view} setView={setView} params={params}/>
        <div style={{ flex: 1, minHeight: 0, overflow: "auto" }}>{children}</div>
      </main>
    </div>
  );
}

function Sidebar({ collapsed, setCollapsed, view, setView, activeWorkflow }) {
  const globals = [
    { key: "dashboard", icon: <Icons.LayoutDashboard/>, label: "Dashboard" },
  ];
  const indexes = [
    { key: "workflows", icon: <Icons.Workflow/>,        label: "Workflows", badge: "6" },
    { key: "runs",      icon: <Icons.Terminal/>,        label: "Runs",      badge: "1" },
    { key: "reviews",   icon: <Icons.MessageSquare/>,   label: "Reviews",   badge: "9" },
    { key: "memory",    icon: <Icons.Brain/>,           label: "Memory" },
  ];
  return (
    <aside style={{
      background: "var(--sidebar)", display: "flex", flexDirection: "column",
      borderRight: "1px solid var(--sidebar-border)", overflow: "hidden",
    }}>
      <div style={{ padding: "14px 14px 10px", display: "flex", alignItems: "center", gap: 10 }}>
        <div style={{
          width: 28, height: 28, flexShrink: 0, borderRadius: 5,
          background: "var(--sidebar-primary)", color: "var(--sidebar-primary-foreground)",
          display: "grid", placeItems: "center",
          fontFamily: "var(--font-display)", fontWeight: 700, fontSize: 16, letterSpacing: "-0.03em",
        }}>C</div>
        {!collapsed && (
          <div style={{ minWidth: 0, flex: 1 }}>
            <div style={{ fontSize: 13, fontWeight: 600, lineHeight: 1.15, color: "var(--foreground)", display: "flex", alignItems: "center", gap: 6 }}>
              Compozy
              <span style={{ fontSize: 9, fontFamily: "var(--font-mono)", padding: "1px 5px", borderRadius: 3, background: "rgba(202,234,40,0.12)", color: "#caea28", border: "1px solid rgba(202,234,40,0.24)", letterSpacing: "0.04em", textTransform: "uppercase" }}>daemon</span>
            </div>
            <div style={{ fontSize: 10.5, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)", lineHeight: 1.3, display: "flex", alignItems: "center", gap: 5 }}>
              <span style={{ width: 6, height: 6, borderRadius: 999, background: "#10b981", boxShadow: "0 0 6px rgba(16,185,129,0.6)" }}/>
              {DAEMON.host} · :51021
            </div>
          </div>
        )}
      </div>

      <div style={{ flex: 1, overflow: "auto", padding: "8px 8px", display: "flex", flexDirection: "column", gap: 14 }}>
        <NavGroup title={!collapsed && "Workspace"}>
          {globals.map(n => (
            <NavItem key={n.key} {...n} collapsed={collapsed} active={view === n.key} onClick={() => setView(n.key)}/>
          ))}
        </NavGroup>

        <NavGroup title={!collapsed && "Across workflows"}>
          {indexes.map(n => (
            <NavItem key={n.key} {...n} collapsed={collapsed} active={view === n.key} onClick={() => setView(n.key)}/>
          ))}
        </NavGroup>

        <NavGroup title={!collapsed && "Active workflows"}>
          {["user-auth", "manifest-v2", "multi-repo", "skill-banner"].map(w => (
            <NavItem key={w} icon={<Icons.GitBranch/>} label={w} collapsed={collapsed}
              active={activeWorkflow === w} onClick={() => setView("workflow:" + w)}
              trailing={w === "user-auth" ? <span style={{ width: 6, height: 6, borderRadius: 999, background: "#3b82f6", boxShadow: "0 0 6px rgba(59,130,246,0.6)", animation: "pulse 1.6s infinite" }}/> : null}
            />
          ))}
        </NavGroup>
      </div>

      <div style={{ borderTop: "1px solid var(--sidebar-border)", padding: "10px 12px", display: "flex", alignItems: "center", gap: 10 }}>
        <div style={{
          width: 26, height: 26, flexShrink: 0, borderRadius: 999,
          background: "linear-gradient(135deg, #caea28, #78a10d)", color: "var(--brand-950)",
          display: "grid", placeItems: "center",
          fontSize: 10.5, fontWeight: 700, fontFamily: "var(--font-mono)",
        }}>RM</div>
        {!collapsed && (
          <div style={{ minWidth: 0, flex: 1, fontSize: 12 }}>
            <div style={{ fontWeight: 500, lineHeight: 1.2, color: "var(--foreground)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>Rui Martins</div>
            <div style={{ color: "var(--muted-foreground)", lineHeight: 1.2, fontSize: 10 }}>rui@compozy.dev</div>
          </div>
        )}
        {!collapsed && <IconBtn tip="Settings" onClick={() => {}}><Icons.Settings/></IconBtn>}
      </div>
    </aside>
  );
}

function NavGroup({ title, children }) {
  return (
    <div>
      {title && (
        <div style={{
          padding: "2px 10px 6px", fontSize: 10, fontWeight: 600,
          textTransform: "uppercase", letterSpacing: "0.08em",
          color: "var(--muted-foreground)",
        }}>{title}</div>
      )}
      <div style={{ display: "flex", flexDirection: "column", gap: 1 }}>{children}</div>
    </div>
  );
}

function NavItem({ icon, label, badge, collapsed, active, onClick, trailing }) {
  const [hover, setHover] = useStateSh(false);
  const bg = active ? "var(--sidebar-accent)" : hover ? "rgba(255,255,255,0.04)" : "transparent";
  return (
    <button onClick={onClick}
      onMouseEnter={() => setHover(true)} onMouseLeave={() => setHover(false)}
      title={collapsed ? label : undefined}
      style={{
        display: "grid",
        gridTemplateColumns: collapsed ? "1fr" : "16px 1fr auto",
        justifyItems: collapsed ? "center" : "stretch",
        alignItems: "center", gap: 10,
        padding: collapsed ? "8px" : "6px 10px",
        borderRadius: 5, border: 0, background: bg,
        color: active ? "var(--foreground)" : "var(--sidebar-foreground)",
        fontSize: 13, fontWeight: active ? 500 : 400, fontFamily: "inherit",
        textAlign: "left", cursor: "pointer", width: "100%",
        position: "relative",
      }}>
      {active && !collapsed && <span style={{ position: "absolute", left: -8, top: 6, bottom: 6, width: 2, borderRadius: 2, background: "var(--primary)" }}/>}
      <span style={{ color: active ? "var(--foreground)" : "var(--muted-foreground)", display: "inline-flex" }}>{icon}</span>
      {!collapsed && <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{label}</span>}
      {!collapsed && (badge != null ? (
        <span style={{
          minWidth: 18, height: 18, padding: "0 5px",
          display: "inline-flex", alignItems: "center", justifyContent: "center",
          borderRadius: 4, background: "var(--secondary)",
          fontFamily: "var(--font-mono)", fontSize: 10, color: "var(--muted-foreground)",
        }}>{badge}</span>
      ) : trailing || null)}
    </button>
  );
}

// Top strip — breadcrumbs + command search + quick actions. No OS chrome.
function TopStrip({ view, params }) {
  const crumbs = buildCrumbs(view, params);
  return (
    <div style={{
      height: 48, flexShrink: 0, display: "flex", alignItems: "center", gap: 12,
      padding: "0 20px", borderBottom: "1px solid var(--border)",
      background: "var(--background)",
    }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, minWidth: 0 }}>
        {crumbs.map((c, i) => (
          <React.Fragment key={i}>
            {i > 0 && <span style={{ color: "var(--muted-foreground)", opacity: 0.5, fontSize: 12 }}>/</span>}
            <span style={{
              fontSize: 13, fontFamily: i === 0 ? "var(--font-sans)" : c.mono ? "var(--font-mono)" : "var(--font-sans)",
              color: i === crumbs.length - 1 ? "var(--foreground)" : "var(--muted-foreground)",
              fontWeight: i === crumbs.length - 1 ? 500 : 400,
              display: "inline-flex", alignItems: "center", gap: 6,
            }}>
              {c.icon}{c.label}
            </span>
          </React.Fragment>
        ))}
      </div>
      <div style={{ flex: 1 }}/>
      <button style={{
        display: "inline-flex", alignItems: "center", gap: 8,
        height: 28, padding: "0 10px 0 12px",
        borderRadius: 5, border: "1px solid var(--border)",
        background: "var(--card)", color: "var(--muted-foreground)",
        fontSize: 12, fontFamily: "inherit", cursor: "pointer",
      }}>
        <Icons.Search size={14}/>
        <span>Search commands, workflows, tasks…</span>
        <span style={{ display: "inline-flex", gap: 2, marginLeft: 12 }}>
          <Kbd>⌘</Kbd><Kbd>K</Kbd>
        </span>
      </button>
      <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
        <IconBtn tip="Refresh"><Icons.RefreshCw/></IconBtn>
        <Button variant="primary" size="sm" icon={<Icons.Plus/>}>New workflow</Button>
      </div>
    </div>
  );
}

function buildCrumbs(view, params) {
  const p = params || {};
  if (view === "dashboard") return [{ icon: <Icons.LayoutDashboard size={14}/>, label: "Dashboard" }];
  if (view === "workflows") return [{ icon: <Icons.Workflow size={14}/>, label: "Workflows" }];
  if (view === "runs")      return [{ icon: <Icons.Terminal size={14}/>, label: "Runs" }];
  if (view === "reviews")   return [{ icon: <Icons.MessageSquare size={14}/>, label: "Reviews" }];
  if (view === "memory")    return [{ icon: <Icons.Brain size={14}/>, label: "Memory" }];
  if (view === "spec")      return [
    { icon: <Icons.Workflow size={14}/>, label: "Workflows" },
    { label: p.workflow || "user-auth", mono: true },
    { label: "Spec" },
  ];
  if (view === "tasks") return [
    { icon: <Icons.Workflow size={14}/>, label: "Workflows" },
    { label: p.workflow || "user-auth", mono: true },
    { label: "Tasks" },
  ];
  if (view === "task-detail") return [
    { icon: <Icons.Workflow size={14}/>, label: "Workflows" },
    { label: p.workflow || "user-auth", mono: true },
    { label: "Tasks" },
    { label: p.task || "task_08", mono: true },
  ];
  if (view === "run-detail") return [
    { icon: <Icons.Terminal size={14}/>, label: "Runs" },
    { label: p.run || "run_2a9f", mono: true },
  ];
  if (view === "review-detail") return [
    { icon: <Icons.MessageSquare size={14}/>, label: "Reviews" },
    { label: p.workflow || "manifest-v2", mono: true },
    { label: "round " + (p.round || "002"), mono: true },
    { label: p.issue || "issue_004", mono: true },
  ];
  return [{ label: view }];
}

window.AppShell = AppShell;
