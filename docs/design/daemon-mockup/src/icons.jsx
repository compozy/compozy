/* global React */
// Icons + shared primitives for Compozy daemon web app.

const I = ({ d, size = 16, fill = "none", strokeWidth = 1.5, children, style, ...rest }) => (
  <svg width={size} height={size} viewBox="0 0 24 24"
    fill={fill} stroke="currentColor" strokeWidth={strokeWidth}
    strokeLinecap="round" strokeLinejoin="round"
    style={{ flexShrink: 0, display: "inline-block", verticalAlign: "middle", ...style }}
    {...rest}>
    {d ? <path d={d}/> : children}
  </svg>
);

const Icons = {
  ChevronL: (p) => <I {...p}><polyline points="15 18 9 12 15 6"/></I>,
  ChevronR: (p) => <I {...p}><polyline points="9 18 15 12 9 6"/></I>,
  ChevronDown: (p = {}) => <I size={12} {...p}><polyline points="6 9 12 15 18 9"/></I>,
  ChevronUp: (p = {}) => <I size={12} {...p}><polyline points="18 15 12 9 6 15"/></I>,
  ArrowRight: (p) => <I {...p}><line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/></I>,
  Plus: (p) => <I {...p}><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></I>,
  Search: (p) => <I {...p}><circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/></I>,
  Settings: (p) => <I {...p}><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9c.36.14.68.36.94.64"/></I>,
  Activity: (p) => <I {...p}><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></I>,
  LayoutDashboard: (p) => <I {...p}><rect x="3" y="3" width="7" height="9"/><rect x="14" y="3" width="7" height="5"/><rect x="14" y="12" width="7" height="9"/><rect x="3" y="16" width="7" height="5"/></I>,
  Workflow: (p) => <I {...p}><rect x="3" y="3" width="6" height="6" rx="1"/><rect x="15" y="15" width="6" height="6" rx="1"/><path d="M9 6h4a2 2 0 0 1 2 2v8"/></I>,
  FileText: (p) => <I {...p}><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="8" y1="13" x2="16" y2="13"/><line x1="8" y1="17" x2="13" y2="17"/></I>,
  ListTodo: (p) => <I {...p}><rect x="3" y="5" width="6" height="6" rx="1"/><path d="m3 17 2 2 4-4"/><line x1="13" y1="6" x2="21" y2="6"/><line x1="13" y1="12" x2="21" y2="12"/><line x1="13" y1="18" x2="21" y2="18"/></I>,
  Play: (p = {}) => <I size={14} {...p}><polygon points="5 3 19 12 5 21 5 3" fill="currentColor"/></I>,
  Terminal: (p) => <I {...p}><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></I>,
  MessageSquare: (p) => <I {...p}><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></I>,
  Brain: (p) => <I {...p}><path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96.44 2.5 2.5 0 0 1-2.96-3.08 3 3 0 0 1-.34-5.58 2.5 2.5 0 0 1 1.32-4.24 2.5 2.5 0 0 1 1.98-3A2.5 2.5 0 0 1 9.5 2Z"/><path d="M14.5 2a2.5 2.5 0 0 0-2.5 2.5v15a2.5 2.5 0 0 0 4.96.44 2.5 2.5 0 0 0 2.96-3.08 3 3 0 0 0 .34-5.58 2.5 2.5 0 0 0-1.32-4.24 2.5 2.5 0 0 0-1.98-3A2.5 2.5 0 0 0 14.5 2Z"/></I>,
  GitBranch: (p) => <I {...p}><line x1="6" y1="3" x2="6" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/></I>,
  More: (p) => <I {...p}><circle cx="5" cy="12" r="1" fill="currentColor"/><circle cx="12" cy="12" r="1" fill="currentColor"/><circle cx="19" cy="12" r="1" fill="currentColor"/></I>,
  X: (p = {}) => <I size={14} {...p}><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></I>,
  Filter: (p = {}) => <I size={14} {...p}><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/></I>,
  Sort: (p = {}) => <I size={14} {...p}><line x1="3" y1="6" x2="21" y2="6"/><line x1="6" y1="12" x2="18" y2="12"/><line x1="10" y1="18" x2="14" y2="18"/></I>,
  Check: (p) => <I {...p}><polyline points="20 6 9 17 4 12"/></I>,
  AlertTriangle: (p) => <I {...p}><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></I>,
  AlertCircle: (p) => <I {...p}><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></I>,
  Clock: (p) => <I {...p}><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></I>,
  Zap: (p) => <I {...p}><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></I>,
  Cpu: (p) => <I {...p}><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M9 2v2M15 2v2M9 20v2M15 20v2M2 9h2M2 15h2M20 9h2M20 15h2"/></I>,
  Package: (p) => <I {...p}><path d="m7.5 4.27 9 5.15"/><path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/><path d="M3.3 7 12 12l8.7-5"/><path d="M12 22V12"/></I>,
  Archive: (p) => <I {...p}><rect x="2" y="4" width="20" height="5" rx="1"/><path d="M4 9v9a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9"/><line x1="10" y1="13" x2="14" y2="13"/></I>,
  RefreshCw: (p) => <I {...p}><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></I>,
  Circle: (p) => <I {...p}><circle cx="12" cy="12" r="9"/></I>,
  CheckCircle: (p) => <I {...p}><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></I>,
  XCircle: (p) => <I {...p}><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></I>,
  PlayCircle: (p) => <I {...p}><circle cx="12" cy="12" r="10"/><polygon points="10 8 16 12 10 16 10 8" fill="currentColor" strokeWidth="0"/></I>,
  PauseCircle: (p) => <I {...p}><circle cx="12" cy="12" r="10"/><line x1="10" y1="15" x2="10" y2="9"/><line x1="14" y1="15" x2="14" y2="9"/></I>,
  MoreHorizontal: (p) => <I {...p}><circle cx="12" cy="12" r="1" fill="currentColor"/><circle cx="19" cy="12" r="1" fill="currentColor"/><circle cx="5" cy="12" r="1" fill="currentColor"/></I>,
  FolderOpen: (p) => <I {...p}><path d="m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2"/></I>,
  Hash: (p) => <I {...p}><line x1="4" y1="9" x2="20" y2="9"/><line x1="4" y1="15" x2="20" y2="15"/><line x1="10" y1="3" x2="8" y2="21"/><line x1="16" y1="3" x2="14" y2="21"/></I>,
  Bug: (p) => <I {...p}><rect x="8" y="6" width="8" height="14" rx="4"/><path d="m19 7-3 2M5 7l3 2M19 19l-3-2M5 19l3-2M20 13h-4M8 13H4M15 6a3 3 0 0 0-6 0"/></I>,
  Send: (p = {}) => <I size={14} {...p}><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></I>,
  ArrowLeft: (p = {}) => <I size={14} {...p}><line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/></I>,
  ExternalLink: (p = {}) => <I size={12} {...p}><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/></I>,
  Server: (p) => <I {...p}><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></I>,
  Eye: (p = {}) => <I size={14} {...p}><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></I>,
  Layers: (p) => <I {...p}><polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/></I>,
  Copy: (p = {}) => <I size={13} {...p}><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></I>,
  MoreH: (p) => <I {...p}><circle cx="12" cy="12" r="1" fill="currentColor"/><circle cx="19" cy="12" r="1" fill="currentColor"/><circle cx="5" cy="12" r="1" fill="currentColor"/></I>,
  Pause: (p = {}) => <I size={14} {...p}><rect x="6" y="4" width="4" height="16" fill="currentColor" strokeWidth="0"/><rect x="14" y="4" width="4" height="16" fill="currentColor" strokeWidth="0"/></I>,
  Square: (p = {}) => <I size={14} {...p}><rect x="6" y="6" width="12" height="12" fill="currentColor" strokeWidth="0"/></I>,
  RotateCcw: (p = {}) => <I size={14} {...p}><polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/></I>,
  Sparkles: (p = {}) => <I size={14} {...p}><path d="M12 3v4M12 17v4M3 12h4M17 12h4M5.6 5.6l2.8 2.8M15.6 15.6l2.8 2.8M5.6 18.4l2.8-2.8M15.6 8.4l2.8-2.8"/></I>,
};

// ── Kbd ──
const Kbd = ({ children }) => (
  <span style={{
    display: "inline-flex", alignItems: "center", justifyContent: "center",
    minWidth: 16, height: 16, padding: "0 4px", borderRadius: 3,
    background: "var(--secondary)", border: "1px solid var(--border)",
    fontFamily: "var(--font-mono)", fontSize: 10, color: "var(--foreground)",
  }}>{children}</span>
);

// ── Badge ──
const Badge = ({ variant = "secondary", children, icon, mono }) => {
  const styles = {
    secondary:   { bg: "var(--secondary)", fg: "var(--foreground)", bd: "transparent" },
    outline:     { bg: "transparent",      fg: "var(--foreground)", bd: "var(--border)" },
    primary:     { bg: "var(--primary)",   fg: "var(--primary-foreground)", bd: "transparent" },
    lime:        { bg: "rgba(202,234,40,0.12)", fg: "#caea28", bd: "rgba(202,234,40,0.28)" },
    destructive: { bg: "rgba(239,68,68,0.12)",  fg: "#fca5a5", bd: "rgba(239,68,68,0.32)" },
    success:     { bg: "rgba(16,185,129,0.12)", fg: "#6ee7b7", bd: "rgba(16,185,129,0.32)" },
    warning:     { bg: "rgba(245,158,11,0.12)", fg: "#fcd34d", bd: "rgba(245,158,11,0.32)" },
    info:        { bg: "rgba(59,130,246,0.12)", fg: "#93c5fd", bd: "rgba(59,130,246,0.32)" },
    violet:      { bg: "rgba(139,92,246,0.12)", fg: "#c4b5fd", bd: "rgba(139,92,246,0.32)" },
    muted:       { bg: "transparent",           fg: "var(--muted-foreground)", bd: "var(--border)" },
  }[variant];
  return (
    <span style={{
      display: "inline-flex", alignItems: "center", gap: 5,
      padding: "0 8px", height: 18, borderRadius: 999,
      background: styles.bg, color: styles.fg,
      border: `1px solid ${styles.bd}`,
      fontFamily: mono ? "var(--font-mono)" : "var(--font-sans)",
      fontSize: mono ? 10 : 10.5, fontWeight: 500,
      whiteSpace: "nowrap", letterSpacing: mono ? "0.02em" : 0,
    }}>
      {icon}{children}
    </span>
  );
};

// ── Button ──
const Button = ({ variant = "secondary", size = "md", children, icon, onClick, style = {} }) => {
  const [hover, setHover] = React.useState(false);
  const V = {
    primary:   { bg: "var(--primary)",   fg: "var(--primary-foreground)", bd: "var(--primary)", shadow: "0 3px 8px rgba(202,234,40,0.24), inset 0 1px 0 rgba(255,255,255,0.18)", hover: "#bedc22" },
    secondary: { bg: "var(--secondary)", fg: "var(--foreground)",         bd: "transparent",    shadow: "none", hover: "rgba(255,255,255,0.1)" },
    outline:   { bg: "var(--card)",      fg: "var(--foreground)",         bd: "var(--border)",  shadow: "none", hover: "rgba(255,255,255,0.04)" },
    ghost:     { bg: "transparent",      fg: "var(--foreground)",         bd: "transparent",    shadow: "none", hover: "var(--accent)" },
    destructive:{bg: "rgba(239,68,68,0.1)", fg: "#fca5a5", bd: "rgba(239,68,68,0.28)", shadow: "none", hover: "rgba(239,68,68,0.18)" },
  }[variant];
  const S = { xs: { h: 22, pad: "0 8px", fs: 11.5 }, sm: { h: 26, pad: "0 10px", fs: 12 }, md: { h: 30, pad: "0 12px", fs: 13 } }[size];
  return (
    <button onClick={onClick}
      onMouseEnter={() => setHover(true)} onMouseLeave={() => setHover(false)}
      style={{
        display: "inline-flex", alignItems: "center", gap: 6,
        height: S.h, padding: S.pad,
        borderRadius: 4, border: `1px solid ${V.bd}`,
        background: hover ? V.hover : V.bg,
        color: V.fg, boxShadow: V.shadow,
        fontFamily: "var(--font-sans)", fontSize: S.fs, fontWeight: 500,
        cursor: "pointer", transition: "background 120ms",
        ...style,
      }}>
      {icon}{children}
    </button>
  );
};

// ── IconBtn (sq icon button) ──
const IconBtn = ({ children, tip, onClick, active }) => {
  const [hover, setHover] = React.useState(false);
  return (
    <button onClick={onClick} title={tip}
      onMouseEnter={() => setHover(true)} onMouseLeave={() => setHover(false)}
      style={{
        display: "inline-flex", alignItems: "center", justifyContent: "center",
        width: 26, height: 26, borderRadius: 4,
        background: active ? "var(--accent)" : hover ? "var(--accent)" : "transparent",
        border: 0, padding: 0,
        color: "var(--foreground)", cursor: "pointer",
      }}>{children}</button>
  );
};

// ── StatusDot ──
const STATUS_META = {
  "pending":     { label: "Pending",     color: "#857e77", dash: "2 2" },
  "queued":      { label: "Queued",      color: "#857e77", dash: "2 2" },
  "running":     { label: "Running",     color: "#3b82f6", pulse: true },
  "in-progress": { label: "In progress", color: "#3b82f6", pulse: true },
  "in-review":   { label: "In review",   color: "#8b5cf6" },
  "done":        { label: "Done",        color: "#10b981", fill: true },
  "completed":   { label: "Completed",   color: "#10b981", fill: true },
  "failed":      { label: "Failed",      color: "#ef4444", fill: true },
  "error":       { label: "Error",       color: "#ef4444", fill: true },
  "stale":       { label: "Stale",       color: "#f59e0b" },
  "blocked":     { label: "Blocked",     color: "#f59e0b", fill: true },
  "resolved":    { label: "Resolved",    color: "#10b981", fill: true },
  "open":        { label: "Open",        color: "#ef4444" },
  "triaged":     { label: "Triaged",     color: "#8b5cf6" },
  "fixed":       { label: "Fixed",       color: "#10b981", fill: true },
  "archived":    { label: "Archived",    color: "#78716c", fill: true },
};
const StatusDot = ({ status, size = 14 }) => {
  const m = STATUS_META[status] || STATUS_META.pending;
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" style={{ flexShrink: 0 }}>
      {m.pulse && <circle cx="12" cy="12" r="11" fill={m.color} opacity="0.2">
        <animate attributeName="r" values="8;11;8" dur="1.8s" repeatCount="indefinite"/>
        <animate attributeName="opacity" values="0.35;0.05;0.35" dur="1.8s" repeatCount="indefinite"/>
      </circle>}
      <circle cx="12" cy="12" r="8" fill={m.fill ? m.color : "none"}
        stroke={m.color} strokeWidth="2.5" strokeDasharray={m.dash || ""}/>
      {m.fill && status !== "failed" && status !== "error" && status !== "blocked" && status !== "archived" && (
        <path d="M8.5 12l2.5 2.5 4.5-5" fill="none" stroke="#0c0a09" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
      )}
      {(status === "failed" || status === "error") && (
        <path d="M9 9l6 6M15 9l-6 6" fill="none" stroke="#0c0a09" strokeWidth="2" strokeLinecap="round"/>
      )}
    </svg>
  );
};

// ── Sparkline (svg poly) ──
const Sparkline = ({ data, w = 120, h = 28, color = "var(--primary)" }) => {
  const max = Math.max(...data, 1), min = Math.min(...data, 0);
  const r = max - min || 1;
  const pts = data.map((v, i) => `${(i/(data.length-1))*w},${h - ((v - min)/r)*(h-2) - 1}`).join(" ");
  const area = `0,${h} ${pts} ${w},${h}`;
  return (
    <svg width={w} height={h} style={{ display: "block" }}>
      <polygon points={area} fill={color} opacity="0.12"/>
      <polyline points={pts} fill="none" stroke={color} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
    </svg>
  );
};

// ── PipelinePhase (small dot w/ connector showing PRD→TechSpec→Tasks→Exec→Review) ──
const PHASES = [
  { key: "prd",      label: "PRD" },
  { key: "techspec", label: "TechSpec" },
  { key: "tasks",    label: "Tasks" },
  { key: "exec",     label: "Execution" },
  { key: "review",   label: "Review" },
];
const PipelineBar = ({ current, done = [], size = "md" }) => {
  const doneSet = new Set(done);
  const dotR = size === "sm" ? 4 : 5;
  const gap = size === "sm" ? 36 : 56;
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 0 }}>
      {PHASES.map((p, i) => {
        const isDone = doneSet.has(p.key);
        const isCur = current === p.key;
        const color = isDone ? "#10b981" : isCur ? "var(--primary)" : "var(--border)";
        return (
          <React.Fragment key={p.key}>
            <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 4 }}>
              <div style={{
                width: dotR*2, height: dotR*2, borderRadius: 999,
                background: isCur || isDone ? color : "transparent",
                border: `2px solid ${color}`,
                boxShadow: isCur ? "0 0 0 3px rgba(202,234,40,0.18)" : "none",
              }}/>
              {size !== "sm" && (
                <span style={{
                  fontSize: 9.5, fontFamily: "var(--font-mono)",
                  color: isCur || isDone ? "var(--foreground)" : "var(--muted-foreground)",
                  textTransform: "uppercase", letterSpacing: "0.06em",
                }}>{p.label}</span>
              )}
            </div>
            {i < PHASES.length - 1 && (
              <div style={{ width: gap, height: 2, background: doneSet.has(PHASES[i+1].key) || (isDone && isCur) ? "#10b981" : isDone ? "linear-gradient(90deg, #10b981, var(--border))" : "var(--border)", marginBottom: size === "sm" ? 0 : 18 }}/>
            )}
          </React.Fragment>
        );
      })}
    </div>
  );
};

Object.assign(window, { Icons, Kbd, Badge, Button, IconBtn, StatusDot, STATUS_META, Sparkline, PipelineBar, PHASES });
