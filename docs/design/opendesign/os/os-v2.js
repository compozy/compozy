/* ============================================================
   AGH OS v2 — shell runtime
   Same window-manager engine as os/os.js; windows now render
   the existing route components with production tokens.
   ============================================================ */
(() => {
'use strict';

const $ = (sel, el = document) => el.querySelector(sel);
const $$ = (sel, el = document) => [...el.querySelectorAll(sel)];

/* Mobile mode (<960px): windows render stacked fullscreen via CSS,
   the dock is a tab bar. These guards keep the WM from dragging,
   zooming, magnifying, or persisting fullscreen rects over the
   saved desktop layout while the media query is active. */
const mqMobile = window.matchMedia('(max-width: 959px)');
const isMobile = () => mqMobile.matches;

/* ---------------- icons ---------------- */
const ICONS = {
  dashboard: '<svg viewBox="0 0 20 20"><rect x="3" y="3" width="6" height="6" rx="1.6" fill="none" stroke="currentColor" stroke-width="1.5"/><rect x="11" y="3" width="6" height="6" rx="1.6" fill="none" stroke="currentColor" stroke-width="1.5"/><rect x="3" y="11" width="6" height="6" rx="1.6" fill="none" stroke="currentColor" stroke-width="1.5"/><rect x="11" y="11" width="6" height="6" rx="1.6" fill="none" stroke="currentColor" stroke-width="1.5"/></svg>',
  sessions: '<svg viewBox="0 0 20 20"><path d="M4 4.5h12a1.5 1.5 0 0 1 1.5 1.5v7a1.5 1.5 0 0 1-1.5 1.5H9l-3.4 2.6a.5.5 0 0 1-.8-.4v-2.2H4A1.5 1.5 0 0 1 2.5 13V6A1.5 1.5 0 0 1 4 4.5Z" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></svg>',
  session: '<svg viewBox="0 0 20 20"><path d="M3 5.5 8 10l-5 4.5M9.5 15h7" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  agents: '<svg viewBox="0 0 20 20"><rect x="4" y="6" width="12" height="9" rx="2.4" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M10 6V3.2M7.5 10.4h.01M12.5 10.4h.01" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/><path d="M7 13h6" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>',
  tasks: '<svg viewBox="0 0 20 20"><path d="m3.5 6 1.6 1.6L8 4.7M3.5 13l1.6 1.6L8 11.7M10.5 6.5H17M10.5 13.5H17" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  marketplace: '<svg viewBox="0 0 20 20"><path d="M4 7.5 5 4h10l1 3.5M4 7.5h12M4 7.5V15a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1V7.5M8 10.5h4" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  network: '<svg viewBox="0 0 20 20"><circle cx="10" cy="10" r="6.6" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M3.4 10h13.2M10 3.4c-3.6 3.8-3.6 9.4 0 13.2 3.6-3.8 3.6-9.4 0-13.2Z" fill="none" stroke="currentColor" stroke-width="1.3"/></svg>',
  vault: '<svg viewBox="0 0 20 20"><circle cx="7.5" cy="8" r="3.8" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="m10.4 10.6 5.6 5.6M13.5 13.5l1.8-1.8M15.5 15.5l1.6-1.6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>',
  settings: '<svg viewBox="0 0 20 20"><path d="M3.5 6h9M15.5 6H17M3.5 14h2M8.5 14h8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/><circle cx="14" cy="6" r="1.8" fill="none" stroke="currentColor" stroke-width="1.5"/><circle cx="6" cy="14" r="1.8" fill="none" stroke="currentColor" stroke-width="1.5"/></svg>',
  loops: '<svg viewBox="0 0 20 20"><path d="M13.5 4.5H8a4 4 0 0 0-4 4v.5M6.5 15.5H12a4 4 0 0 0 4-4V11M11 2l2.5 2.5L11 7M9 18l-2.5-2.5L9 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  jobs: '<svg viewBox="0 0 20 20"><circle cx="10" cy="10" r="6.8" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M10 6.2V10l3 1.8" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  triggers: '<svg viewBox="0 0 20 20"><path d="M11 2.5 4.5 11h4l-.9 6.5L14.5 9h-4z" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></svg>',
  bridges: '<svg viewBox="0 0 20 20"><circle cx="4.5" cy="15.5" r="2" fill="none" stroke="currentColor" stroke-width="1.5"/><circle cx="15.5" cy="4.5" r="2" fill="none" stroke="currentColor" stroke-width="1.5"/><circle cx="15.5" cy="15.5" r="2" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M6.5 15.5h7M15.5 6.5v7M6 14 14 6" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>',
  knowledge: '<svg viewBox="0 0 20 20"><path d="M4 4.5A1.5 1.5 0 0 1 5.5 3H16v13H5.5A1.5 1.5 0 0 0 4 17.5zM4 4.5v13M16 13H5.5A1.5 1.5 0 0 0 4 14.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  sandbox: '<svg viewBox="0 0 20 20"><rect x="2.5" y="11" width="6.5" height="6.5" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/><rect x="11" y="11" width="6.5" height="6.5" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/><rect x="6.75" y="2.5" width="6.5" height="6.5" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/></svg>',
  check: '<svg viewBox="0 0 16 16"><path d="m3.5 8.5 3 3 6-6.5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  spark: '<svg viewBox="0 0 16 16"><path d="M8 2.5 9.4 6l3.6 1.4-3.6 1.4L8 12.4 6.6 8.8 3 7.4 6.6 6Z" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/></svg>',
};

/* ---------------- data ---------------- */
const AGENTS = [
  { id: 'webgen', name: 'webgen', chip: 'WG', provider: 'Claude Code', model: 'claude-fable-5', status: 'working', sessions: 3 },
  { id: 'research', name: 'research', chip: 'RS', provider: 'Hermes', model: 'hermes-4', status: 'idle', sessions: 2 },
  { id: 'infra', name: 'infra', chip: 'IF', provider: 'OpenClaw', model: 'openclaw-2', status: 'working', sessions: 2 },
];
const SESSIONS = [
  { id: 's1', agent: 'webgen', title: 'Checkout flow polish', status: 'working', time: '2m' },
  { id: 's2', agent: 'webgen', title: 'Marketplace empty states', status: 'needs', time: '18m' },
  { id: 's3', agent: 'webgen', title: 'Dashboard spacing audit', status: 'done', time: '1h' },
  { id: 's4', agent: 'research', title: 'Competitor pricing scan', status: 'working', time: '3h' },
  { id: 's5', agent: 'research', title: 'ACP spec digest', status: 'done', time: '5h' },
  { id: 's6', agent: 'infra', title: 'Nightly deps sweep', status: 'working', time: '41m' },
  { id: 's7', agent: 'infra', title: 'Release dry-run', status: 'failed', time: '2d' },
];
const TASKS = [
  { id: 't1', title: 'Ship marketplace empty states', agent: 'webgen', status: 'running', meta: 'run 3 · 12m' },
  { id: 't2', title: 'QA pass — RT-037 scenarios', agent: 'infra', status: 'needs', meta: 'blocked · approval' },
  { id: 't3', title: 'Refactor session store reads', agent: 'webgen', status: 'running', meta: 'run 1 · 4m' },
  { id: 't4', title: 'Digest ACP protocol changes', agent: 'research', status: 'done', meta: 'done · 09:12' },
  { id: 't5', title: 'Rotate vault credentials', agent: 'infra', status: 'done', meta: 'done · yesterday' },
  { id: 't6', title: 'Draft docs for network budgets', agent: 'research', status: 'queued', meta: 'queued' },
];
const CATALOG = [
  { id: 'impeccable', kind: 'skill', name: 'impeccable', desc: 'Design-quality gate for UI work — critique loops and polish passes.', ver: '1.4.0', installed: true },
  { id: 'browser-use', kind: 'skill', name: 'browser-use', desc: 'Drive a real browser for QA flows, captures and scraping.', ver: '0.9.2', installed: true },
  { id: 'qa-suite', kind: 'bundle', name: 'qa-suite', desc: 'Scenario QA bundle: bootstrap, charters, reports and teardown.', ver: '2.1.0', installed: true },
  { id: 'linear-mcp', kind: 'mcp', name: 'linear', desc: 'Read and update Linear issues, projects and cycles from sessions.', ver: '1.0.3', installed: false },
  { id: 'context7', kind: 'mcp', name: 'context7', desc: 'Up-to-date library docs resolved straight into agent context.', ver: '0.8.0', installed: false },
  { id: 'git-guard', kind: 'extension', name: 'git-guard', desc: 'Blocks destructive git verbs unless the session holds approval.', ver: '0.5.1', installed: false },
  { id: 'deck-kit', kind: 'skill', name: 'deck-kit', desc: 'Slide framework with scale-to-fit canvas and print styles.', ver: '1.1.0', installed: false },
  { id: 'mesh-relay', kind: 'extension', name: 'mesh-relay', desc: 'Relay tasks to peer daemons on the network mesh.', ver: '0.3.0', installed: false },
];
const WORKSPACES = [
  { id: 'agh', name: 'agh', chip: 'AG', home: true, wallpaper: 'ember' },
  { id: 'branasio', name: 'branasio', chip: 'BR', home: false, wallpaper: 'mesh' },
  { id: 'labs', name: 'labs', chip: 'LA', home: false, wallpaper: 'carbon' },
];
let APPROVALS = [
  { id: 'a1', agent: 'webgen', text: 'Run <code>make deploy</code> in agh', detail: 'shell · outside sandbox' },
  { id: 'a2', agent: 'infra', text: 'Write outside workspace: <code>/etc/hosts</code>', detail: 'fs · elevated path' },
];
const SPARK_DATA = [11, 14, 9, 16, 12, 18, 15, 10, 17, 13, 19, 14, 16, 21];

/* Window chrome contract (pagehead-redesign): 44px unified head
   (glyph + title · status · ≤2 actions) + optional 38px tool strip. */
const MORE_BTN = '<button class="btn btn--ghost btn--icon" aria-label="More actions" data-trail="more"><svg viewBox="0 0 16 16" aria-hidden="true"><circle cx="3.5" cy="8" r="1.3" fill="currentColor"/><circle cx="8" cy="8" r="1.3" fill="currentColor"/><circle cx="12.5" cy="8" r="1.3" fill="currentColor"/></svg></button>';

const APPS = {
  dashboard: {
    name: 'Dashboard', w: 680, h: 540, x: 110, y: 56,
    status: () => '<span class="w2-status" data-tone="ok"><span class="d d--ok"></span>Connected</span>',
  },
  sessions: {
    name: 'Sessions', w: 420, h: 560, x: 56, y: 38, hoistToolbar: true,
    count: () => String(SESSIONS.length),
    actions: () => `${MORE_BTN}<button class="btn btn--primary" data-trail="new-session">New session</button>`,
  },
  session: {
    name: 'Checkout flow polish', w: 630, h: 570, x: 470, y: 26, document: true,
  },
  tasks: {
    name: 'Tasks', w: 660, h: 480, x: 200, y: 88, hoistToolbar: true,
    count: () => String(TASKS.length),
    status: () => {
      const n = TASKS.filter(t => t.status === 'running').length;
      return n ? `<span class="w2-status" data-tone="run"><span class="d d--run d--pulse"></span>${n} running</span>` : '';
    },
    actions: () => `${MORE_BTN}<button class="btn btn--primary" data-trail="new-task">New task</button>`,
  },
  agents:      { name: 'Agents',      w: 540, h: 390, x: 260, y: 118 },
  loops:       { name: 'Loops',       w: 560, h: 400, x: 240, y: 100 },
  jobs:        { name: 'Jobs',        w: 600, h: 400, x: 280, y: 118 },
  triggers:    { name: 'Triggers',    w: 620, h: 400, x: 310, y: 136 },
  marketplace: {
    name: 'Marketplace', w: 720, h: 550, x: 168, y: 52, hoistToolbar: true,
    actions: () => MORE_BTN,
  },
  bridges:     { name: 'Bridges',     w: 560, h: 400, x: 340, y: 150 },
  knowledge:   { name: 'Knowledge',   w: 580, h: 390, x: 360, y: 164 },
  sandbox:     { name: 'Sandbox',     w: 640, h: 450, x: 300, y: 126, hoistToolbar: true },
  network: {
    name: 'Network', w: 540, h: 480, x: 300, y: 98,
    status: () => '<span class="w2-status" data-tone="ok"><span class="d d--ok"></span>Healthy</span>',
  },
  vault:       { name: 'Vault',       w: 580, h: 390, x: 330, y: 128 },
  settings:    { name: 'Settings',    w: 640, h: 450, x: 280, y: 108 },
};
/* Mirrors app-sidebar.tsx groups; 'sep' = the sidebar's group breaks.
   Settings intentionally lives in the menu bar cog, not the dock. */
const DOCK_ORDER = [
  'sessions', 'dashboard',
  'agents', 'network', 'tasks', 'loops', 'jobs', 'triggers', 'sep',
  'marketplace', 'bridges', 'knowledge', 'sep',
  'sandbox', 'vault',
];
const DOCK_APPS = DOCK_ORDER.filter(id => id !== 'sep');

/* ---------------- state ---------------- */
const LS_KEY = 'agh-os-v2';
const defaultSpace = (ws) => ({ wallpaper: WORKSPACES.find(w => w.id === ws)?.wallpaper || 'ember', open: {}, minimized: [] });
let state = {
  ws: 'agh',
  spaces: { agh: defaultSpace('agh'), branasio: defaultSpace('branasio'), labs: defaultSpace('labs') },
  magnify: true,
  motion: 'full',
  sessionsView: 'recent',
  collapsed: [],
};
try {
  const saved = JSON.parse(localStorage.getItem(LS_KEY) || 'null');
  if (saved && saved.spaces) state = { ...state, ...saved };
} catch { /* fresh start */ }

let saveTimer = null;
const save = () => {
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => localStorage.setItem(LS_KEY, JSON.stringify(state)), 250);
};
const space = () => state.spaces[state.ws];

/* ---------------- els ---------------- */
const layer = $('#winLayer');
const dock = $('#dock');
let zTop = 20;
const wins = new Map();

/* ============================================================
   WINDOW MANAGER
   ============================================================ */
function clampRect(r) {
  const bw = layer.clientWidth, bh = layer.clientHeight;
  r.w = Math.min(r.w, bw - 24);
  r.h = Math.min(r.h, bh - 90);
  r.x = Math.max(8, Math.min(r.x, bw - r.w - 8));
  r.y = Math.max(4, Math.min(r.y, bh - 120));
  return r;
}

function resolveHeadField(value) {
  return typeof value === 'function' ? value() : (value || '');
}

function sessionTrailHTML(agent, { elapsed = '2m 14s', elapsedSecs = 134 } = {}) {
  return `<span class="w2-status"><span class="agent-chip mono">${agent.chip}</span>${agent.name}</span>`
    + `<span class="w2-meta mono">${agent.model}</span>`
    + `<span class="w2-vsep"></span>`
    + `<span class="w2-meta mono" data-elapsed="${elapsedSecs}" aria-label="Elapsed">${elapsed}</span>`;
}

function paintWindowHead(win, appId, opts = {}) {
  const app = APPS[appId];
  if (!app) return;
  const glyph = $('[data-win-glyph]', win);
  const titleEl = $('[data-win-title]', win);
  const countEl = $('[data-win-count]', win);
  const trail = $('[data-win-trail]', win);
  const back = $('[data-win-back]', win);
  const crumb = $('[data-win-crumb]', win);
  const sep = $('[data-win-sep]', win);
  let docDot = $('[data-win-doc-dot]', win);

  const isDoc = Boolean(app.document);
  win.dataset.doc = isDoc ? 'true' : 'false';
  win.setAttribute('aria-label', opts.title || app.name);
  titleEl.textContent = opts.title || app.name;

  if (!docDot) {
    docDot = document.createElement('span');
    docDot.setAttribute('data-win-doc-dot', '');
    docDot.setAttribute('aria-hidden', 'true');
    glyph.before(docDot);
  }

  if (isDoc) {
    glyph.hidden = true;
    docDot.hidden = false;
    docDot.className = opts.dotClass || 'd d--run d--pulse';
    countEl.hidden = true;
    back.hidden = true;
    crumb.hidden = true;
    sep.hidden = true;
    trail.innerHTML = opts.trail || '';
    return;
  }

  docDot.hidden = true;
  const depth = opts.depth;
  if (depth) {
    glyph.hidden = true;
    back.hidden = false;
    crumb.hidden = false;
    crumb.textContent = depth.parent;
    sep.hidden = false;
    titleEl.textContent = depth.child;
    countEl.hidden = true;
  } else {
    glyph.hidden = false;
    glyph.innerHTML = ICONS[appId] || ICONS.dashboard;
    back.hidden = true;
    crumb.hidden = true;
    sep.hidden = true;
    const count = opts.count !== undefined ? opts.count : resolveHeadField(app.count);
    if (count !== '' && count != null) {
      countEl.textContent = String(count);
      countEl.hidden = false;
    } else {
      countEl.hidden = true;
    }
  }

  const status = opts.status !== undefined ? opts.status : resolveHeadField(app.status);
  const actions = opts.actions !== undefined ? opts.actions : resolveHeadField(app.actions);
  const parts = [];
  if (status) parts.push(status);
  if (status && actions) parts.push('<span class="w2-vsep"></span>');
  if (actions) parts.push(`<div class="w2-actions">${actions}</div>`);
  trail.innerHTML = parts.join('');
}

function hoistToolbar(win) {
  const src = $('[data-hoist-toolbar]', win);
  const dest = $('[data-win-toolbar]', win);
  if (!src || !dest) return;
  dest.hidden = false;
  dest.appendChild(src);
}

function openWindow(appId) {
  const app = APPS[appId];
  if (!app) return;
  const existing = wins.get(appId);
  if (existing) { restoreWindow(appId); focusWindow(existing); return existing; }

  const frag = $('#tpl-window').content.cloneNode(true);
  const win = frag.querySelector('.win');
  win.dataset.app = appId;
  win.setAttribute('data-od-id', `window-${appId}`);

  const body = $('.win-body', win);
  const tpl = $(`#tpl-app-${appId}`);
  if (tpl) body.appendChild(tpl.content.cloneNode(true));

  if (app.document) {
    const a = AGENTS[0];
    paintWindowHead(win, appId, {
      title: app.name,
      trail: sessionTrailHTML(a),
    });
  } else {
    paintWindowHead(win, appId);
  }
  if (app.hoistToolbar) hoistToolbar(win);

  const saved = space().open[appId];
  const rect = saved ? { ...saved } : { x: app.x, y: app.y, w: app.w, h: app.h };
  if (!isMobile()) clampRect(rect);
  Object.assign(win.style, { left: rect.x + 'px', top: rect.y + 'px', width: rect.w + 'px', height: rect.h + 'px' });
  space().open[appId] = rect;
  space().minimized = space().minimized.filter(id => id !== appId);

  layer.appendChild(win);
  wins.set(appId, win);
  bindWindow(win, appId);
  initApp(appId, win);
  focusWindow(win);
  refreshDock(); save();
  return win;
}

function bindWindow(win, appId) {
  win.addEventListener('pointerdown', () => focusWindow(win));
  $('.wc-close', win).addEventListener('click', (e) => { e.stopPropagation(); closeWindow(appId); });
  $('.wc-min', win).addEventListener('click', (e) => { e.stopPropagation(); minimizeWindow(appId); });
  $('.wc-zoom', win).addEventListener('click', (e) => { e.stopPropagation(); zoomWindow(appId); });

  const trail = $('[data-win-trail]', win);
  trail.addEventListener('click', (e) => {
    const b = e.target.closest('[data-trail]');
    if (!b) return;
    e.stopPropagation();
    if (b.dataset.trail === 'new-session') startNewSession('webgen');
    else if (b.dataset.trail === 'new-task') toast({ title: 'New task', body: 'Task editor opens here in production.' });
    else if (b.dataset.trail === 'more') toast({ title: 'More actions', body: 'Overflow actions stay in <kbd class="mono">⌘K</kbd>.' });
  });

  const back = $('[data-win-back]', win);
  const crumb = $('[data-win-crumb]', win);
  const onBack = (e) => {
    e.stopPropagation();
    if (appId === 'sessions') setSessionsView('recent');
  };
  back.addEventListener('click', onBack);
  crumb.addEventListener('click', onBack);

  const head = $('[data-win-head]', win);
  const body = $('.win-body', win);
  body.addEventListener('scroll', () => {
    head.classList.toggle('is-scrolled', body.scrollTop > 2);
  }, { passive: true });

  head.addEventListener('pointerdown', (e) => {
    if (
      isMobile()
      || e.target.closest('.wc')
      || e.target.closest('[data-trail]')
      || e.target.closest('.w2-back')
      || e.target.closest('.w2-crumb')
      || e.target.closest('.w2-actions')
      || e.target.closest('button')
      || e.target.closest('input')
      || win.classList.contains('is-max')
    ) return;
    const startX = e.clientX, startY = e.clientY;
    const ox = win.offsetLeft, oy = win.offsetTop;
    head.setPointerCapture(e.pointerId);
    const move = (ev) => {
      const r = clampRect({ x: ox + ev.clientX - startX, y: oy + ev.clientY - startY, w: win.offsetWidth, h: win.offsetHeight });
      win.style.left = r.x + 'px'; win.style.top = r.y + 'px';
    };
    const up = () => {
      head.removeEventListener('pointermove', move);
      head.removeEventListener('pointerup', up);
      persistRect(appId, win);
    };
    head.addEventListener('pointermove', move);
    head.addEventListener('pointerup', up);
  });

  new ResizeObserver(() => { if (win.isConnected && !win.classList.contains('is-max')) persistRect(appId, win); }).observe(win);
}

function persistRect(appId, win) {
  if (isMobile()) return; /* fullscreen is CSS-forced — never overwrite the saved desktop rect */
  space().open[appId] = { x: win.offsetLeft, y: win.offsetTop, w: win.offsetWidth, h: win.offsetHeight };
  save();
}

function focusWindow(win) {
  $$('.win', layer).forEach(w => w.classList.remove('is-focused'));
  win.classList.add('is-focused');
  win.style.zIndex = ++zTop;
}

function closeWindow(appId) {
  const win = wins.get(appId);
  if (!win) return;
  win.remove(); wins.delete(appId);
  delete space().open[appId];
  space().minimized = space().minimized.filter(id => id !== appId);
  refreshDock(); save();
}

/* Minimize collapses the window into its own dock icon — the icon
   keeps a hollow indicator and a click restores it. No tray, so any
   number of minimized windows costs zero horizontal space. */
function minimizeWindow(appId) {
  const win = wins.get(appId);
  if (!win) return;
  const target = dock.querySelector(`[data-app="${appId}"]`) || dock;
  const tRect = target.getBoundingClientRect();
  const winRect = win.getBoundingClientRect();
  const dx = (tRect.left + tRect.width / 2) - (winRect.left + winRect.width / 2);
  const dy = (tRect.top + tRect.height / 2) - (winRect.top + winRect.height / 2);
  win.classList.add('is-minimizing');
  win.style.transform = `translate(${dx}px, ${dy}px) scale(.06)`;
  win.style.opacity = '0';
  const done = () => {
    win.remove(); wins.delete(appId);
    if (!space().minimized.includes(appId)) space().minimized.push(appId);
    refreshDock(); save();
  };
  document.body.dataset.motion === 'reduced' ? done() : setTimeout(done, 290);
}

function restoreWindow(appId) {
  if (space().minimized.includes(appId)) {
    space().minimized = space().minimized.filter(id => id !== appId);
  }
}

function zoomWindow(appId) {
  if (isMobile()) return; /* already fullscreen; the zoom control is hidden */
  const win = wins.get(appId);
  if (!win) return;
  if (win.classList.contains('is-max')) {
    const r = win._prevRect || APPS[appId];
    Object.assign(win.style, { left: r.x + 'px', top: r.y + 'px', width: r.w + 'px', height: r.h + 'px' });
    win.classList.remove('is-max');
  } else {
    win._prevRect = { x: win.offsetLeft, y: win.offsetTop, w: win.offsetWidth, h: win.offsetHeight };
    Object.assign(win.style, { left: '10px', top: '8px', width: (layer.clientWidth - 20) + 'px', height: (layer.clientHeight - 86) + 'px' });
    win.classList.add('is-max');
  }
  persistRect(appId, win);
}

/* ============================================================
   DOCK
   ============================================================ */
/* badge truth: sessions = sessions waiting on you; tasks = tasks needing you */
const DOCK_BADGES = () => ({
  sessions: SESSIONS.filter(s => s.status === 'needs').length,
  tasks: TASKS.filter(t => t.status === 'needs').length,
});

function buildDock() {
  dock.innerHTML = '';
  DOCK_ORDER.forEach(appId => {
    if (appId === 'sep') {
      const s = document.createElement('span');
      s.className = 'dock-sep';
      dock.appendChild(s);
      return;
    }
    const b = document.createElement('button');
    b.className = 'dock-item';
    b.dataset.app = appId;
    b.setAttribute('data-od-id', `dock-${appId}`);
    b.setAttribute('aria-label', APPS[appId].name);
    b.innerHTML = `${ICONS[appId]}<span class="run-dot"></span><span class="tip">${APPS[appId].name}</span>`;
    b.addEventListener('click', () => {
      if (isMobile()) { openWindow(appId); return; } /* tab-bar semantics: tap = switch to, never minimize */
      const win = wins.get(appId);
      if (win && win.classList.contains('is-focused')) minimizeWindow(appId);
      else openWindow(appId);
    });
    dock.appendChild(b);
  });
  refreshDock();
}

function refreshDock() {
  const badges = DOCK_BADGES();
  $$('.dock-item', dock).forEach(item => {
    const id = item.dataset.app;
    item.classList.toggle('is-open', wins.has(id) || space().minimized.includes(id));
    item.classList.toggle('is-min', !wins.has(id) && space().minimized.includes(id));
    let badge = $('.dk-badge', item);
    const n = badges[id] || 0;
    if (n > 0) {
      if (!badge) { badge = document.createElement('span'); badge.className = 'dk-badge'; item.appendChild(badge); }
      badge.textContent = n;
    } else if (badge) badge.remove();
  });
  $('#deskHint').hidden = wins.size > 0;
}

const MAG_RADIUS = 96, MAG_SCALE = .34;
dock.addEventListener('pointermove', (e) => {
  if (isMobile() || !state.magnify || document.body.dataset.motion === 'reduced') return;
  $$('.dock-item', dock).forEach(item => {
    const r = item.getBoundingClientRect();
    const d = Math.abs(e.clientX - (r.left + r.width / 2));
    const f = Math.max(0, 1 - d / MAG_RADIUS);
    item.style.transform = `translateY(${-7 * f}px) scale(${1 + MAG_SCALE * f})`;
  });
});
dock.addEventListener('pointerleave', () => {
  $$('.dock-item', dock).forEach(item => { item.style.transform = ''; });
});

/* ============================================================
   APP CONTENT (existing route components)
   ============================================================ */
const agentById = (id) => AGENTS.find(a => a.id === id);
const statusDot = (s) => ({
  working: 'd d--run d--pulse', running: 'd d--run d--pulse',
  needs: 'd d--needs', failed: 'd d--fail',
  done: 'd d--idle', queued: 'd d--idle', idle: 'd d--idle',
}[s] || 'd d--idle');

function initApp(appId, win) {
  const init = {
    dashboard: initDashboard, sessions: initSessions, session: initSession,
    tasks: initTasks, agents: initAgents, marketplace: initMarketplace,
    loops: initLoops, jobs: initJobs, triggers: initTriggers,
    bridges: initBridges, knowledge: initKnowledge, sandbox: initSandbox,
    network: initNetwork, vault: initVault, settings: initSettings,
  }[appId];
  if (init) init(win);
}

function initDashboard(win) {
  const bars = $('[data-bars]', win);
  const max = Math.max(...SPARK_DATA);
  bars.innerHTML = SPARK_DATA.map(v => `<span style="height:${Math.round(v / max * 100)}%" title="${v} runs"></span>`).join('');
  const rc = $('[data-runcards]', win);
  const running = SESSIONS.filter(s => s.status === 'working').slice(0, 3);
  rc.innerHTML = running.map(s => {
    const a = agentById(s.agent);
    return `<div class="runcard"><div><p class="runcard__title"><span class="d d--run d--pulse"></span>${s.title}<span class="kind">session</span></p><p class="runcard__sub">${a.name} · ${a.model}</p></div><span class="runcard__elapsed">${s.time}</span></div>`;
  }).join('');
}

/* ============================================================
   SESSIONS WINDOW — sidebar-sessions-02 anatomy inside WindowFrame
   ============================================================ */
const SESSION_DS = {
  working: { ds: 'running', label: 'running' },
  needs: { ds: 'waiting-for-auth', label: 'waiting' },
  failed: { ds: 'failed', label: 'failed' },
  done: { ds: 'stopped', label: 'done' },
  idle: { ds: 'idle', label: 'idle' },
};

function sessionsWin() {
  return wins.get('sessions') || null;
}

function sessionRow(s, compact) {
  const a = agentById(s.agent);
  const m = SESSION_DS[s.status] || SESSION_DS.idle;
  const sub = compact ? '' : `<span class="session-row__sub"><b>${a.name}</b><span class="session-row__state">· ${m.label}</span></span>`;
  return `<button class="session-row" data-status="${m.ds}" data-sid="${s.id}">
    <span class="status-dot"></span>
    <span class="session-row__body"><span class="session-row__title">${s.title}</span>${sub}</span>
    <span class="session-row__time">${s.time}</span>
  </button>`;
}

function renderSessionsList() {
  const win = sessionsWin();
  if (!win) return;
  const q = ($('[data-sr-filter]', win)?.value || '').trim().toLowerCase();
  const visible = SESSIONS.filter(s => !q || s.title.toLowerCase().includes(q) || agentById(s.agent).name.toLowerCase().includes(q));
  const countEl = $('[data-win-count]', win);
  if (countEl && state.sessionsView !== 'all') {
    countEl.textContent = String(visible.length);
    countEl.hidden = false;
  }

  const recent = visible.slice(0, 6);
  const recentEl = $('[data-sr-recent]', win);
  recentEl.innerHTML = (recent.length
    ? `<div class="session-list">${recent.map(s => sessionRow(s, false)).join('')}</div>`
    : '<p class="sr-empty">No sessions match.</p>')
    + `<button class="show-all" data-sr-show-all>Show all sessions
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="m6.5 3.5 4.2 4.5-4.2 4.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </button>`;

  const allEl = $('[data-sr-all]', win);
  allEl.innerHTML = AGENTS.map(a => {
    const rows = visible.filter(s => s.agent === a.id);
    if (!rows.length) return '';
    const collapsed = state.collapsed.includes(a.id);
    return `<div class="agent-group" data-collapsed="${collapsed}" data-agent="${a.id}">
      <button class="agent-group__head" aria-expanded="${!collapsed}">
        <span class="agent-avatar">${a.chip}</span>
        <span class="agent-group__name">${a.name}</span>
        <span class="agent-group__count">${rows.length}</span>
        <svg class="disclosure-icon" viewBox="0 0 16 16" aria-hidden="true"><path d="m6 3.5 4.5 4.5L6 12.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </button>
      <div class="agent-group__body"><div class="session-list">${rows.map(s => sessionRow(s, true)).join('')}</div></div>
    </div>`;
  }).join('') || '<p class="sr-empty">No sessions match.</p>';

  $$('.session-row', win).forEach(row => row.addEventListener('click', () => {
    const s = SESSIONS.find(x => x.id === row.dataset.sid);
    openSessionView(s);
  }));
  const showAll = $('[data-sr-show-all]', win);
  if (showAll) showAll.addEventListener('click', () => setSessionsView('all'));
  $$('.agent-group__head', allEl).forEach(head => head.addEventListener('click', () => {
    const group = head.closest('.agent-group');
    const id = group.dataset.agent;
    const collapsed = group.dataset.collapsed === 'true';
    group.dataset.collapsed = String(!collapsed);
    head.setAttribute('aria-expanded', String(collapsed));
    state.collapsed = collapsed ? state.collapsed.filter(x => x !== id) : [...state.collapsed, id];
    save();
  }));
}

function setSessionsView(view) {
  state.sessionsView = view;
  const win = sessionsWin();
  if (win) {
    $('[data-sr-track]', win).dataset.view = view;
    if (view === 'all') {
      paintWindowHead(win, 'sessions', {
        depth: { parent: 'Sessions', child: 'All' },
      });
    } else {
      paintWindowHead(win, 'sessions');
    }
  }
  save();
}

function initSessions(win) {
  $('[data-sr-filter]', win).addEventListener('input', renderSessionsList);
  setSessionsView(state.sessionsView);
  renderSessionsList();
}

function openSessionView(session) {
  APPS.session.name = session.title;
  const a = agentById(session.agent);
  let dotClass = 'd d--idle';
  if (session.status === 'working') dotClass = 'd d--run d--pulse';
  else if (session.status === 'failed') dotClass = 'd d--fail';
  else if (session.status === 'needs') dotClass = 'd d--needs';

  const existing = wins.get('session');
  if (existing) {
    paintWindowHead(existing, 'session', {
      title: session.title,
      dotClass,
      trail: sessionTrailHTML(a, { elapsed: '2m 14s', elapsedSecs: 134 }),
    });
    startSessionClock(existing, 134);
    restoreWindow('session');
    focusWindow(existing);
    return existing;
  }

  const win = openWindow('session');
  paintWindowHead(win, 'session', {
    title: session.title,
    dotClass,
    trail: sessionTrailHTML(a, { elapsed: '2m 14s', elapsedSecs: 134 }),
  });
  startSessionClock(win, 134);
  return win;
}

function startSessionClock(win, startSecs) {
  if (win._timer) clearInterval(win._timer);
  let secs = Number(startSecs) || 0;
  win._timer = setInterval(() => {
    if (!win.isConnected) { clearInterval(win._timer); return; }
    const el = $('[data-elapsed]', win);
    if (!el) return;
    secs += 1;
    el.dataset.elapsed = String(secs);
    el.textContent = `${Math.floor(secs / 60)}m ${String(secs % 60).padStart(2, '0')}s`;
  }, 1000);
}

function initSession(win) {
  $('[data-toolcall] .toolcall-head', win).addEventListener('click', (e) => {
    const head = e.currentTarget;
    const body = head.nextElementSibling;
    const open = head.getAttribute('aria-expanded') === 'true';
    head.setAttribute('aria-expanded', String(!open));
    body.hidden = open;
  });
  $('[data-composer]', win).addEventListener('submit', (e) => {
    e.preventDefault();
    const input = $('input', e.currentTarget);
    if (!input.value.trim()) return;
    const t = $('[data-transcript]', win);
    const div = document.createElement('div');
    div.className = 'msg msg-user';
    div.innerHTML = `<p class="msg-role">Pedro</p><p class="msg-text"></p>`;
    $('.msg-text', div).textContent = input.value;
    t.appendChild(div);
    t.scrollTop = t.scrollHeight;
    input.value = '';
  });
  const el = $('[data-elapsed]', win);
  startSessionClock(win, el ? el.dataset.elapsed : 0);
}

function initTasks(win) {
  const app = $('.app', win);
  const rows = $('[data-task-rows]', win);
  const cards = $('[data-task-cards]', win);
  let filter = 'all';
  const visible = () => TASKS.filter(t => filter === 'all' || (filter === 'running' && t.status === 'running') || (filter === 'needs' && t.status === 'needs'));
  const render = () => {
    const list = visible();
    rows.innerHTML = list.map(t => {
      const a = agentById(t.agent);
      return `<li><span class="${statusDot(t.status)}"></span><span class="row-title">${t.title}</span><span class="agent-chip mono" title="${a.name}">${a.chip}</span><span class="row-meta">${t.meta}</span></li>`;
    }).join('');
    cards.innerHTML = list.map(t => {
      const a = agentById(t.agent);
      return `<div class="task-card"><div class="card-top"><span class="${statusDot(t.status)}"></span><span class="card-sub">${t.status}</span></div><p class="card-title">${t.title}</p><div class="card-foot"><span class="agent-chip mono" title="${a.name}">${a.chip}</span><span class="card-sub">${t.meta}</span></div></div>`;
    }).join('');
  };
  $$('[data-tfilter]', win).forEach(p => p.addEventListener('click', () => {
    $$('[data-tfilter]', win).forEach(x => x.classList.remove('is-on'));
    p.classList.add('is-on');
    filter = p.dataset.tfilter;
    render();
  }));
  $$('[data-tview]', win).forEach(p => p.addEventListener('click', () => {
    $$('[data-tview]', win).forEach(x => x.classList.remove('is-on'));
    p.classList.add('is-on');
    const mode = p.dataset.tview;
    app.dataset.view = mode;
    rows.hidden = mode === 'cards';
    cards.hidden = mode === 'rows';
  }));
  render();
}

function initAgents(win) {
  $('[data-agent-rows]', win).innerHTML = AGENTS.map(a => `
    <li><span class="agent-chip mono">${a.chip}</span>
      <div style="flex:1;min-width:0"><p class="row-title">${a.name}</p><p class="row-sub">${a.provider} · ${a.model}</p></div>
      <span class="${a.status === 'working' ? 'd d--run d--pulse' : 'd d--idle'}"></span>
      <span class="row-meta">${a.sessions} sessions</span>
      <button class="btn btn--sm row-action" data-agent-new="${a.id}">Session</button>
    </li>`).join('');
  $$('[data-agent-new]', win).forEach(b => b.addEventListener('click', () => startNewSession(b.dataset.agentNew)));
}

function initMarketplace(win) {
  const grid = $('[data-mkt-grid]', win);
  const input = $('[data-mkt-filter]', win);
  let kind = 'skill', scope = 'marketplace';
  const render = () => {
    const q = input.value.trim().toLowerCase();
    const list = CATALOG.filter(c => c.kind === kind)
      .filter(c => scope === 'marketplace' ? true : c.installed)
      .filter(c => !q || c.name.includes(q) || c.desc.toLowerCase().includes(q));
    $('[data-installed-count]', win).textContent = CATALOG.filter(c => c.installed).length;
    grid.innerHTML = list.length ? list.map(c => `
      <div class="mkt-card"><div class="card-top"><span class="mkt-kind">${c.kind}</span><span class="card-sub">v${c.ver}</span></div>
        <p class="card-title mono">${c.name}</p>
        <p class="mkt-desc">${c.desc}</p>
        <div class="card-foot">${c.installed
          ? `<span class="mkt-installed">${ICONS.check} Installed</span><button class="btn btn--ghost btn--sm" data-remove="${c.id}">Remove</button>`
          : `<span></span><button class="btn btn--sm" data-install="${c.id}">Install</button>`}</div>
      </div>`).join('')
      : `<p class="mkt-empty">Nothing ${scope === 'installed' ? 'installed' : 'found'} for this kind${q ? ' and query' : ''}.</p>`;
    $$('[data-install]', grid).forEach(b => b.addEventListener('click', () => {
      const c = CATALOG.find(x => x.id === b.dataset.install);
      c.installed = true;
      toast({ title: 'Marketplace', body: `<strong>${c.name}</strong> installed to workspace <code>${state.ws}</code>.` });
      render();
    }));
    $$('[data-remove]', grid).forEach(b => b.addEventListener('click', () => {
      CATALOG.find(x => x.id === b.dataset.remove).installed = false;
      render();
    }));
  };
  $$('[data-kind]', win).forEach(p => p.addEventListener('click', () => {
    $$('[data-kind]', win).forEach(x => x.classList.remove('is-on'));
    p.classList.add('is-on');
    kind = p.dataset.kind;
    render();
  }));
  $$('[data-scope]', win).forEach(t => t.addEventListener('click', () => {
    $$('[data-scope]', win).forEach(x => { x.classList.remove('is-on'); x.setAttribute('aria-selected', 'false'); });
    t.classList.add('is-on');
    t.setAttribute('aria-selected', 'true');
    scope = t.dataset.scope;
    render();
  }));
  input.addEventListener('input', render);
  render();
}

function initLoops(win) {
  const loops = [
    { name: 'session-improvements', status: 'working', meta: 'iter 12 · 38m' },
    { name: 'qa-nightly', status: 'idle', meta: 'last run 04:00' },
    { name: 'deps-sweep', status: 'idle', meta: 'last run Mon' },
  ];
  $('[data-loop-rows]', win).innerHTML = loops.map(l => `
    <li><span class="${statusDot(l.status)}"></span>
      <span class="row-title">${l.name}</span>
      <span class="row-meta">${l.meta}</span>
    </li>`).join('');
}

function initJobs(win) {
  const jobs = [
    { name: 'nightly-verify', sched: '0 4 * * *', next: 'next in 6h', ok: true },
    { name: 'usage-rollup', sched: '*/30 * * * *', next: 'next in 12m', ok: true },
    { name: 'backup-sqlite', sched: '0 */6 * * *', next: 'next in 2h', ok: true },
  ];
  $('[data-job-rows]', win).innerHTML = jobs.map(j => `
    <li><span class="d ${j.ok ? 'd--ok' : 'd--fail'}"></span>
      <span class="row-title">${j.name}</span>
      <span class="scope-chip">${j.sched}</span>
      <span class="row-meta">${j.next}</span>
    </li>`).join('');
}

function initTriggers(win) {
  const triggers = [
    { name: 'on-push-main', src: 'github · push', target: 'task: verify branch', on: true },
    { name: 'inbox-email', src: 'gmail · label agh', target: 'session: research', on: true },
    { name: 'mesh-task-relay', src: 'network · peer', target: 'queue: infra', on: false },
  ];
  $('[data-trigger-rows]', win).innerHTML = triggers.map((t, i) => `
    <li><span class="d ${t.on ? 'd--ok' : 'd--idle'}"></span>
      <div style="flex:1;min-width:0"><p class="row-title">${t.name}</p><p class="row-sub">${t.src} → ${t.target}</p></div>
      <button class="switch ${t.on ? 'is-on' : ''}" role="switch" aria-checked="${t.on}" aria-label="Enable ${t.name}" data-trigger-i="${i}"><span class="knob"></span></button>
    </li>`).join('');
  $$('[data-trigger-i]', win).forEach(sw => sw.addEventListener('click', () => {
    const on = !sw.classList.contains('is-on');
    sw.classList.toggle('is-on', on);
    sw.setAttribute('aria-checked', String(on));
    const dot = $('.d', sw.closest('li'));
    dot.className = `d ${on ? 'd--ok' : 'd--idle'}`;
  }));
}

function initBridges(win) {
  const bridges = [
    { name: 'vscode-bridge', kind: 'sdk · ts', status: 'connected', ok: true },
    { name: 'slack-bot', kind: 'sdk · ts', status: 'connected', ok: true },
    { name: 'raycast', kind: 'uds', status: 'idle', ok: false },
  ];
  $('[data-bridge-rows]', win).innerHTML = bridges.map(b => `
    <li><span class="d ${b.ok ? 'd--ok' : 'd--idle'}"></span>
      <span class="row-title">${b.name}</span>
      <span class="scope-chip">${b.kind}</span>
      <span class="row-meta">${b.status}</span>
    </li>`).join('');
}

function initKnowledge(win) {
  const bases = [
    { name: 'agh-docs', docs: '214 docs', idx: 'indexed 1h ago' },
    { name: 'runbooks', docs: '36 docs', idx: 'indexed yesterday' },
    { name: 'competitors', docs: '58 docs', idx: 'indexed Mon' },
  ];
  $('[data-knowledge-rows]', win).innerHTML = bases.map(k => `
    <li><span class="d d--idle"></span>
      <span class="row-title">${k.name}</span>
      <span class="row-meta">${k.docs}</span>
      <span class="row-meta">${k.idx}</span>
    </li>`).join('');
}

function initSandbox(win) {
  const profiles = [
    { name: 'default', policy: 'fs: workspace · net: allowlist', used: 'used by 3 agents' },
    { name: 'locked-down', policy: 'fs: read-only · net: off', used: 'used by infra' },
    { name: 'open-dev', policy: 'fs: home · net: full', used: 'used by webgen' },
  ];
  const app = $('.app', win);
  const rows = $('[data-sb-rows]', win);
  const cards = $('[data-sb-cards]', win);
  $('[data-sb-count]', win).textContent = `${profiles.length} profiles`;
  rows.innerHTML = profiles.map(p => `
    <li><span class="row-title">${p.name}</span>
      <span class="scope-chip">${p.policy}</span>
      <span class="row-meta">${p.used}</span>
    </li>`).join('');
  cards.innerHTML = profiles.map(p => `
    <div class="task-card"><p class="card-title">${p.name}</p>
      <span class="scope-chip" style="align-self:flex-start">${p.policy}</span>
      <div class="card-foot"><span class="card-sub">${p.used}</span></div>
    </div>`).join('');
  $$('[data-sbview]', win).forEach(p => p.addEventListener('click', () => {
    $$('[data-sbview]', win).forEach(x => x.classList.remove('is-on'));
    p.classList.add('is-on');
    const mode = p.dataset.sbview;
    app.dataset.view = mode;
    rows.hidden = mode === 'cards';
    cards.hidden = mode === 'rows';
  }));
}

function initNetwork(win) {
  const peers = [
    { name: 'mbp-pedro', addr: 'uds·local', lat: '—', self: true, ok: true },
    { name: 'studio-m2', addr: '10.0.1.22:2123', lat: '2ms', ok: true },
    { name: 'vps-fra1', addr: '135.181.4.90:2123', lat: '48ms', ok: true },
    { name: 'vps-sfo2', addr: '164.92.88.10:2123', lat: '181ms', ok: true },
  ];
  $('[data-peer-rows]', win).innerHTML = peers.map(p => `
    <li><span class="d ${p.ok ? 'd--ok' : 'd--fail'}"></span>
      <span class="row-title">${p.name}${p.self ? ' <span class="row-sub">(this machine)</span>' : ''}</span>
      <span class="row-meta">${p.addr}</span><span class="row-meta">${p.lat}</span>
    </li>`).join('');
}

function initVault(win) {
  const secrets = [
    { name: 'ANTHROPIC_API_KEY', scope: 'global', used: 'used 2m ago' },
    { name: 'FAL_KEY', scope: 'ws:agh', used: 'used 1h ago' },
    { name: 'GH_TOKEN', scope: 'agent:webgen', used: 'used 09:02' },
  ];
  $('[data-secret-rows]', win).innerHTML = secrets.map(s => `
    <li><span class="row-title mono" style="font-size:11.5px">${s.name}</span>
      <span class="scope-chip">${s.scope}</span>
      <span class="secret-mask">••••••••</span>
      <span class="row-meta">${s.used}</span>
      <button class="btn btn--ghost btn--sm row-action" data-copy="${s.name}">Copy ref</button>
    </li>`).join('');
  $$('[data-copy]', win).forEach(b => b.addEventListener('click', () => {
    navigator.clipboard?.writeText(`vault://${state.ws}/${b.dataset.copy}`).catch(() => {});
    toast({ title: 'Vault', body: `Reference for <code>${b.dataset.copy}</code> copied — agents resolve it at run time.` });
  }));
}

function initSettings(win) {
  $$('.set-nav-item', win).forEach(item => item.addEventListener('click', () => {
    $$('.set-nav-item', win).forEach(x => x.classList.remove('is-on'));
    item.classList.add('is-on');
    $$('.set-pane', win).forEach(p => { p.hidden = p.dataset.paneId !== item.dataset.pane; });
  }));
  $$('.switch', win).forEach(sw => sw.addEventListener('click', () => {
    const on = !sw.classList.contains('is-on');
    sw.classList.toggle('is-on', on);
    sw.setAttribute('aria-checked', String(on));
    if (sw.dataset.set === 'magnify') { state.magnify = on; save(); }
    if (sw.dataset.set === 'motion') {
      state.motion = on ? 'reduced' : 'full';
      document.body.dataset.motion = state.motion;
      save();
    }
  }));
  const magSw = $('[data-set="magnify"]', win);
  magSw.classList.toggle('is-on', state.magnify);
  magSw.setAttribute('aria-checked', String(state.magnify));
  const motSw = $('[data-set="motion"]', win);
  motSw.classList.toggle('is-on', state.motion === 'reduced');
  motSw.setAttribute('aria-checked', String(state.motion === 'reduced'));
  const syncWp = () => $$('.wp', win).forEach(w => w.classList.toggle('is-on', w.dataset.wp === space().wallpaper));
  $$('.wp', win).forEach(w => w.addEventListener('click', () => {
    space().wallpaper = w.dataset.wp;
    document.body.dataset.wallpaper = w.dataset.wp;
    syncWp(); save();
  }));
  syncWp();
}

/* ============================================================
   WIDGETS + APPROVALS (dashboard att component)
   ============================================================ */
function approvalRow(a) {
  const ag = agentById(a.agent);
  return `<div class="att__row"><span class="d d--needs"></span>
    <div><div class="att__head"><span class="agent-chip mono">${ag.chip}</span>${ag.name}</div>
      <p class="att__text">${a.text}</p>
      <p class="att__meta">${a.detail}</p></div>
    <div class="att__actions">
      <button class="btn btn--primary btn--sm" data-approve="${a.id}">Approve</button>
      <button class="btn btn--sm" data-deny="${a.id}">Deny</button>
    </div></div>`;
}

function renderApprovals() {
  $('#bellList').innerHTML = APPROVALS.map(approvalRow).join('');
  $('#bellEmpty').hidden = APPROVALS.length > 0;
  const n = APPROVALS.length;
  const badge = $('#bellBadge');
  badge.textContent = n;
  n === 0 ? badge.setAttribute('data-zero', '') : badge.removeAttribute('data-zero');
  $$('[data-approve]').forEach(b => { b.onclick = () => resolveApproval(b.dataset.approve, true); });
  $$('[data-deny]').forEach(b => { b.onclick = () => resolveApproval(b.dataset.deny, false); });
  refreshDock();
}

function resolveApproval(id, approved) {
  const a = APPROVALS.find(x => x.id === id);
  APPROVALS = APPROVALS.filter(x => x.id !== id);
  renderApprovals();
  const ag = agentById(a.agent);
  toast({
    title: approved ? 'Approved' : 'Denied',
    body: approved ? `<strong>${ag.name}</strong> resumed — ${a.text}.` : `<strong>${ag.name}</strong> was told no. The session is waiting on a new instruction.`,
  });
}

/* ============================================================
   MENU BAR — clock · popovers · menus
   ============================================================ */
let openPop = null;
function showPop(pop, anchor, align = 'left') {
  hidePops();
  const r = anchor.getBoundingClientRect();
  pop.hidden = false;
  const pw = pop.offsetWidth;
  pop.style.top = (r.bottom + 6) + 'px';
  pop.style.left = align === 'right' ? Math.max(8, r.right - pw) + 'px' : r.left + 'px';
  openPop = pop;
  anchor.setAttribute('aria-expanded', 'true');
}
function hidePops() {
  $$('.popover').forEach(p => { p.hidden = true; });
  $$('[aria-expanded="true"]').forEach(el => el.setAttribute('aria-expanded', 'false'));
  $$('.mb-menu.is-open').forEach(m => m.classList.remove('is-open'));
  openPop = null;
}
document.addEventListener('pointerdown', (e) => {
  if (openPop && !e.target.closest('.popover') && !e.target.closest('[aria-haspopup]') && !e.target.closest('.mb-menu')) hidePops();
});

function renderWsPop() {
  $('#wsList').innerHTML = WORKSPACES.map(w => `
    <li><button class="pop-item" data-ws="${w.id}">
      <span class="ws-avatar mono">${w.chip}</span><span>${w.name}</span>
      ${w.home ? '<span class="ws-home-badge">Home</span>' : ''}
      ${w.id === state.ws ? `<span class="pi-check">${ICONS.check.replace('<svg', '<svg width="13" height="13"')}</span>` : ''}
    </button></li>`).join('');
  $$('[data-ws]', $('#wsList')).forEach(b => b.addEventListener('click', () => { hidePops(); switchWorkspace(b.dataset.ws); }));
}
$('#wsTrigger').addEventListener('click', (e) => {
  e.stopPropagation();
  if (openPop === $('#wsPop')) { hidePops(); return; }
  renderWsPop();
  showPop($('#wsPop'), $('#wsTrigger'));
});
$('#wsAdd').addEventListener('click', () => {
  hidePops();
  toast({ title: 'Workspaces', body: 'Add workspace opens the directory picker — the existing product flow.' });
});
$('#wsSpaces').addEventListener('click', () => { hidePops(); openSpaces(); });

$('#bellBtn').addEventListener('click', (e) => {
  e.stopPropagation();
  if (openPop === $('#bellPop')) { hidePops(); return; }
  showPop($('#bellPop'), $('#bellBtn'), 'right');
});

const MENUS = {
  session: [
    { label: 'New session', kbd: '⌘N', run: () => startNewSession('webgen') },
    { label: 'New task', run: () => openWindow('tasks') },
    { label: 'Open last run', run: () => openSessionView(SESSIONS[0]) },
  ],
  view: [
    { label: 'Spaces overview', kbd: '⇧⌘S', run: openSpaces },
    { label: 'Cycle wallpaper', run: cycleWallpaper },
  ],
  help: [
    { label: 'About AGH OS', run: () => toast({ title: 'AGH OS', body: 'The same routes you already use — Dashboard, Sessions, Tasks, Marketplace — now live as windows over one desktop.' }) },
    { label: 'Keyboard shortcuts', run: () => toast({ title: 'Shortcuts', body: '<code>⌘K</code> palette · <code>⌘N</code> new session · <code>⇧⌘S</code> spaces · <code>esc</code> close overlay' }) },
  ],
};
$$('.mb-menu').forEach(btn => btn.addEventListener('click', (e) => {
  e.stopPropagation();
  if (btn.classList.contains('is-open')) { hidePops(); return; }
  hidePops();
  btn.classList.add('is-open');
  const pop = $('#menuPop');
  pop.innerHTML = MENUS[btn.dataset.menu].map((m, i) =>
    `<button class="pop-item" data-mi="${i}">${m.label}${m.kbd ? `<kbd class="mono">${m.kbd}</kbd>` : ''}</button>`).join('');
  $$('[data-mi]', pop).forEach(b => b.addEventListener('click', () => {
    const item = MENUS[btn.dataset.menu][Number(b.dataset.mi)];
    hidePops(); item.run();
  }));
  showPop(pop, btn);
}));

$('#settingsBtn').addEventListener('click', () => openWindow('settings'));
$('#logoBtn').addEventListener('click', () => openWindow('dashboard'));

function cycleWallpaper() {
  const order = ['ember', 'mesh', 'carbon'];
  const next = order[(order.indexOf(space().wallpaper) + 1) % order.length];
  space().wallpaper = next;
  document.body.dataset.wallpaper = next;
  save();
}

/* ============================================================
   WORKSPACES = SPACES
   ============================================================ */
function switchWorkspace(wsId) {
  if (wsId === state.ws) return;
  wins.forEach((win, appId) => { persistRect(appId, win); win.remove(); });
  wins.clear();
  state.ws = wsId;
  if (!state.spaces[wsId]) state.spaces[wsId] = defaultSpace(wsId);
  const w = WORKSPACES.find(x => x.id === wsId);
  $('#wsName').textContent = w.name;
  $('#wsAvatar').textContent = w.chip;
  document.body.dataset.wallpaper = space().wallpaper;
  Object.keys(space().open).filter(id => !space().minimized.includes(id)).forEach(id => openWindow(id));
  refreshDock(); save();
}

function openSpaces() {
  const row = $('#spacesRow');
  row.innerHTML = WORKSPACES.map(w => {
    const sp = state.spaces[w.id] || defaultSpace(w.id);
    const openIds = Object.keys(sp.open || {}).filter(id => !(sp.minimized || []).includes(id));
    const minis = openIds.slice(0, 4).map((id) => {
      const r = sp.open[id];
      const sx = 264 / Math.max(layer.clientWidth, 1000), sy = 148 / Math.max(layer.clientHeight, 600);
      return `<span class="mini-win" style="left:${r.x * sx}px;top:${r.y * sy + 8}px;width:${r.w * sx}px;height:${r.h * sy}px"></span>`;
    }).join('');
    return `<button class="space-card ${w.id === state.ws ? 'is-active' : ''}" data-space="${w.id}">
      <span class="space-thumb" data-wp="${sp.wallpaper}">${minis}</span>
      <span class="space-foot"><span class="ws-avatar mono">${w.chip}</span><span class="space-name">${w.name}</span><span class="space-meta">${openIds.length} window${openIds.length === 1 ? '' : 's'}</span></span>
    </button>`;
  }).join('');
  $$('[data-space]', row).forEach(c => c.addEventListener('click', () => {
    closeOverlays();
    switchWorkspace(c.dataset.space);
  }));
  $('#spacesOverlay').hidden = false;
}

/* ============================================================
   COMMAND PALETTE
   ============================================================ */
function paletteItems() {
  return [
    ...[...DOCK_APPS, 'settings'].map(id => ({ group: 'Apps', icon: ICONS[id], label: `Open ${APPS[id].name}`, meta: wins.has(id) ? 'open' : '', run: () => openWindow(id) })),
    ...SESSIONS.map(s => ({ group: 'Sessions', icon: ICONS.session, label: s.title, meta: `${agentById(s.agent).name} · ${s.time}`, run: () => openSessionView(s) })),
    { group: 'Actions', icon: ICONS.spark, label: 'New session', meta: '⌘N', run: () => startNewSession('webgen') },
    { group: 'Actions', icon: ICONS.spark, label: 'Spaces overview', meta: '⇧⌘S', run: openSpaces },
    ...WORKSPACES.filter(w => w.id !== state.ws).map(w => ({ group: 'Actions', icon: ICONS.spark, label: `Switch to ${w.name}`, meta: 'space', run: () => switchWorkspace(w.id) })),
    { group: 'Actions', icon: ICONS.spark, label: 'Cycle wallpaper', meta: '', run: cycleWallpaper },
  ];
}
let palIndex = 0, palFiltered = [];
function openPalette() {
  $('#paletteOverlay').hidden = false;
  const input = $('#paletteInput');
  input.value = '';
  renderPalette('');
  input.focus();
}
function renderPalette(q) {
  const query = q.trim().toLowerCase();
  palFiltered = paletteItems().filter(i => !query || i.label.toLowerCase().includes(query) || i.meta.toLowerCase().includes(query));
  palIndex = Math.min(palIndex, Math.max(0, palFiltered.length - 1));
  const out = [];
  let lastGroup = null;
  palFiltered.forEach((item, i) => {
    if (item.group !== lastGroup) { out.push(`<p class="pal-group">${item.group}</p>`); lastGroup = item.group; }
    out.push(`<button class="pal-item ${i === palIndex ? 'is-active' : ''}" data-pi="${i}">${item.icon}<span>${item.label}</span><span class="pal-meta">${item.meta}</span></button>`);
  });
  $('#paletteResults').innerHTML = palFiltered.length ? out.join('') : '<p class="pal-none">No matches — try an app, a session title, or an action.</p>';
  $$('[data-pi]').forEach(b => b.addEventListener('click', () => runPalette(Number(b.dataset.pi))));
}
function runPalette(i) {
  const item = palFiltered[i];
  closeOverlays();
  if (item) item.run();
}
$('#paletteInput').addEventListener('input', (e) => { palIndex = 0; renderPalette(e.target.value); });
$('#paletteInput').addEventListener('keydown', (e) => {
  if (e.key === 'ArrowDown') { e.preventDefault(); palIndex = Math.min(palIndex + 1, palFiltered.length - 1); renderPalette(e.target.value); }
  if (e.key === 'ArrowUp') { e.preventDefault(); palIndex = Math.max(palIndex - 1, 0); renderPalette(e.target.value); }
  if (e.key === 'Enter') { e.preventDefault(); runPalette(palIndex); }
});
$('#cmdkBtn').addEventListener('click', openPalette);

function closeOverlays() {
  $('#paletteOverlay').hidden = true;
  $('#spacesOverlay').hidden = true;
}
$$('.overlay').forEach(ov => ov.addEventListener('pointerdown', (e) => { if (e.target === ov) closeOverlays(); }));

/* ============================================================
   TOASTS
   ============================================================ */
function toast({ title, body, actions = [], timeout = 6000 }) {
  const el = document.createElement('div');
  el.className = 'toast';
  el.innerHTML = `<div class="toast-head"><strong>${title}</strong><span class="mono" style="margin-left:auto;font-size:10px">now</span></div>
    <p class="toast-body">${body}</p>
    ${actions.length ? `<div class="toast-actions">${actions.map((a, i) => `<button class="btn btn--sm ${a.primary ? 'btn--primary' : ''}" data-ta="${i}">${a.label}</button>`).join('')}</div>` : ''}`;
  $('#toasts').appendChild(el);
  const dismiss = () => {
    el.classList.add('is-leaving');
    setTimeout(() => el.remove(), 240);
  };
  actions.forEach((a, i) => $(`[data-ta="${i}"]`, el).addEventListener('click', () => { dismiss(); a.run?.(); }));
  if (!actions.length) el.addEventListener('click', dismiss);
  setTimeout(dismiss, timeout);
}

/* demo: a session completing while you work */
setTimeout(() => {
  SESSIONS[3].status = 'done';
  renderSessionsList();
  toast({
    title: 'research · session done',
    body: '<strong>Competitor pricing scan</strong> finished — 14 sources read, summary ready.',
    timeout: 12000,
    actions: [
      { label: 'View summary', primary: true, run: () => openSessionView(SESSIONS[3]) },
      { label: 'Dismiss' },
    ],
  });
}, 14000);

/* ============================================================
   ACTIONS + KEYBOARD
   ============================================================ */
function startNewSession(agentId) {
  const a = agentById(agentId);
  toast({ title: 'New session', body: `Starting a session with <strong>${a.name}</strong> (${a.model}) in <code>${state.ws}</code>.` });
  APPS.session.name = 'New session';
  const win = openWindow('session');
  paintWindowHead(win, 'session', {
    title: 'New session',
    dotClass: 'd d--run d--pulse',
    trail: sessionTrailHTML(a, { elapsed: '0m 00s', elapsedSecs: 0 }),
  });
  startSessionClock(win, 0);
  $('input', $('[data-composer]', win)).focus();
}
$('#dockNew').addEventListener('click', () => startNewSession('webgen'));

document.addEventListener('keydown', (e) => {
  const mod = e.metaKey || e.ctrlKey;
  if (mod && e.key.toLowerCase() === 'k') { e.preventDefault(); openPalette(); }
  else if (mod && e.key.toLowerCase() === 'n') { e.preventDefault(); startNewSession('webgen'); }
  else if (mod && e.shiftKey && e.key.toLowerCase() === 's') { e.preventDefault(); openSpaces(); }
  else if (e.key === 'Escape') { closeOverlays(); hidePops(); }
});

/* ============================================================
   BOOT
   ============================================================ */
document.body.dataset.motion = state.motion;
document.body.dataset.wallpaper = space().wallpaper;
const w0 = WORKSPACES.find(x => x.id === state.ws);
$('#wsName').textContent = w0.name;
$('#wsAvatar').textContent = w0.chip;

buildDock();
renderApprovals();

const toOpen = Object.keys(space().open).filter(id => !space().minimized.includes(id));
if (toOpen.length) toOpen.forEach(id => openWindow(id));
else openSessionView(SESSIONS[0]);

})();
