/* ============================================================
   AGH OS — first-run onboarding
   Two steps, mirroring web/src/systems/onboarding:
     1. default model  → RuntimeSelector + auth mode (+ bound secret)
     2. workspaces     → /api/fs/browse directory browser + selection
   The shell renders behind, inert, and wakes when setup commits.
   Catalog + selector logic transcribed from
   _done/agents/provider-model-reasoning-selector.html.
   ============================================================ */
(() => {
'use strict';

const $ = (sel, el = document) => el.querySelector(sel);
const $$ = (sel, el = document) => [...el.querySelectorAll(sel)];
const esc = (s) => String(s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

/* ============================================================
   SHELL BACKDROP — dock mirrors app-sidebar.tsx (os-v2 parity)
   ============================================================ */
const ICONS = {
  dashboard: '<svg viewBox="0 0 20 20"><rect x="3" y="3" width="6" height="6" rx="1.6" fill="none" stroke="currentColor" stroke-width="1.5"/><rect x="11" y="3" width="6" height="6" rx="1.6" fill="none" stroke="currentColor" stroke-width="1.5"/><rect x="3" y="11" width="6" height="6" rx="1.6" fill="none" stroke="currentColor" stroke-width="1.5"/><rect x="11" y="11" width="6" height="6" rx="1.6" fill="none" stroke="currentColor" stroke-width="1.5"/></svg>',
  sessions: '<svg viewBox="0 0 20 20"><path d="M4 4.5h12a1.5 1.5 0 0 1 1.5 1.5v7a1.5 1.5 0 0 1-1.5 1.5H9l-3.4 2.6a.5.5 0 0 1-.8-.4v-2.2H4A1.5 1.5 0 0 1 2.5 13V6A1.5 1.5 0 0 1 4 4.5Z" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></svg>',
  agents: '<svg viewBox="0 0 20 20"><rect x="4" y="6" width="12" height="9" rx="2.4" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M10 6V3.2M7.5 10.4h.01M12.5 10.4h.01" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/><path d="M7 13h6" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>',
  tasks: '<svg viewBox="0 0 20 20"><path d="m3.5 6 1.6 1.6L8 4.7M3.5 13l1.6 1.6L8 11.7M10.5 6.5H17M10.5 13.5H17" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  marketplace: '<svg viewBox="0 0 20 20"><path d="M4 7.5 5 4h10l1 3.5M4 7.5h12M4 7.5V15a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1V7.5M8 10.5h4" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  network: '<svg viewBox="0 0 20 20"><circle cx="10" cy="10" r="6.6" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M3.4 10h13.2M10 3.4c-3.6 3.8-3.6 9.4 0 13.2 3.6-3.8 3.6-9.4 0-13.2Z" fill="none" stroke="currentColor" stroke-width="1.3"/></svg>',
  vault: '<svg viewBox="0 0 20 20"><circle cx="7.5" cy="8" r="3.8" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="m10.4 10.6 5.6 5.6M13.5 13.5l1.8-1.8M15.5 15.5l1.6-1.6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>',
  loops: '<svg viewBox="0 0 20 20"><path d="M13.5 4.5H8a4 4 0 0 0-4 4v.5M6.5 15.5H12a4 4 0 0 0 4-4V11M11 2l2.5 2.5L11 7M9 18l-2.5-2.5L9 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  jobs: '<svg viewBox="0 0 20 20"><circle cx="10" cy="10" r="6.8" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M10 6.2V10l3 1.8" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  triggers: '<svg viewBox="0 0 20 20"><path d="M11 2.5 4.5 11h4l-.9 6.5L14.5 9h-4z" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></svg>',
  bridges: '<svg viewBox="0 0 20 20"><circle cx="4.5" cy="15.5" r="2" fill="none" stroke="currentColor" stroke-width="1.5"/><circle cx="15.5" cy="4.5" r="2" fill="none" stroke="currentColor" stroke-width="1.5"/><circle cx="15.5" cy="15.5" r="2" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M6.5 15.5h7M15.5 6.5v7M6 14 14 6" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>',
  knowledge: '<svg viewBox="0 0 20 20"><path d="M4 4.5A1.5 1.5 0 0 1 5.5 3H16v13H5.5A1.5 1.5 0 0 0 4 17.5zM4 4.5v13M16 13H5.5A1.5 1.5 0 0 0 4 14.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  sandbox: '<svg viewBox="0 0 20 20"><rect x="2.5" y="11" width="6.5" height="6.5" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/><rect x="11" y="11" width="6.5" height="6.5" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/><rect x="6.75" y="2.5" width="6.5" height="6.5" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/></svg>',
};
const DOCK_ORDER = [
  'sessions', 'dashboard',
  'agents', 'network', 'tasks', 'loops', 'jobs', 'triggers', 'sep',
  'marketplace', 'bridges', 'knowledge', 'sep',
  'sandbox', 'vault',
];
const DOCK_LABEL = {
  sessions: 'Sessions', dashboard: 'Dashboard', agents: 'Agents', network: 'Network',
  tasks: 'Tasks', loops: 'Loops', jobs: 'Jobs', triggers: 'Triggers',
  marketplace: 'Marketplace', bridges: 'Bridges', knowledge: 'Knowledge',
  sandbox: 'Sandbox', vault: 'Vault',
};

const dockEl = $('#dock');
const dockZone = $('#dockZone');
function buildDock() {
  dockEl.innerHTML = DOCK_ORDER.map((id, i) => {
    if (id === 'sep') return '<span class="dock-sep" aria-hidden="true"></span>';
    return `<button class="dock-item" data-app="${id}" aria-label="${DOCK_LABEL[id]}" style="transition-delay:${i * 26}ms">
      ${ICONS[id]}<span class="tip">${DOCK_LABEL[id]}</span><span class="run-dot"></span>
    </button>`;
  }).join('');
}
buildDock();

/* ============================================================
   CATALOG — mirrors internal/config/provider.go + model-catalog
   ============================================================ */
const HARNESS = { acp: 'cli', pi_acp: 'api key' };
const EFFORT_ORDER = ['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'];
const EFFORT_LABEL = { none: 'None', minimal: 'Minimal', low: 'Low', medium: 'Medium', high: 'High', xhigh: 'Extra high', max: 'Max' };
const effortPos = (e) => { const i = EFFORT_ORDER.indexOf(e); return i < 0 ? 0 : i + 1; };

const PROVIDERS = [
  { id: 'claude', name: 'Claude Code', harness: 'acp', auth: 'ready' },
  { id: 'codex', name: 'Codex', harness: 'acp', auth: 'ready' },
  { id: 'gemini', name: 'Gemini CLI', harness: 'acp', auth: 'ready' },
  { id: 'opencode', name: 'OpenCode', harness: 'acp', auth: 'ready' },
  { id: 'cursor', name: 'Cursor Agent', harness: 'acp', auth: 'ready' },
  { id: 'openrouter', name: 'OpenRouter', harness: 'pi_acp', auth: 'ready' },
  { id: 'xai', name: 'xAI', harness: 'pi_acp', auth: 'ready' },
  { id: 'groq', name: 'Groq', harness: 'pi_acp', auth: 'ready' },
  { id: 'moonshot', name: 'Moonshot', harness: 'pi_acp', auth: 'ready' },
  { id: 'zai', name: 'z.ai', harness: 'pi_acp', auth: 'signin' },
];
const PROV = {}; PROVIDERS.forEach(p => { PROV[p.id] = p; });

const MODELS = [
  { id: 'opus-4.6', provider: 'claude', name: 'Opus 4.6', ctx: 200000, cin: 15, cout: 75, tools: true, reasoning: true, efforts: [], avail: 'live', fav: true, recent: true },
  { id: 'sonnet-4.6', provider: 'claude', name: 'Sonnet 4.6', ctx: 200000, cin: 3, cout: 15, tools: true, reasoning: true, efforts: [], avail: 'live', recent: true },
  { id: 'haiku-4.5', provider: 'claude', name: 'Haiku 4.5', ctx: 200000, cin: 1, cout: 5, tools: true, reasoning: false, efforts: [], avail: 'live' },
  { id: 'gpt-5.4', provider: 'codex', name: 'GPT-5.4', ctx: 400000, cin: 1.25, cout: 10, tools: true, reasoning: true, efforts: ['minimal', 'low', 'medium', 'high', 'xhigh'], def: 'medium', src: 'config', avail: 'live', fav: true, recent: true },
  { id: 'gpt-5.4-mini', provider: 'codex', name: 'GPT-5.4 Mini', ctx: 400000, cin: 0.25, cout: 2, tools: true, reasoning: true, efforts: ['minimal', 'low', 'medium', 'high', 'xhigh'], def: 'low', src: 'config', avail: 'live' },
  { id: 'gpt-5.3', provider: 'codex', name: 'GPT-5.3', ctx: 256000, cin: 1.1, cout: 9, tools: true, reasoning: true, efforts: ['low', 'medium', 'high', 'xhigh'], def: 'medium', src: 'config', avail: 'stale' },
  { id: 'gemini-2.5-pro', provider: 'gemini', name: 'Gemini 2.5 Pro', ctx: 1000000, cin: 1.25, cout: 10, tools: true, reasoning: true, efforts: [], avail: 'live' },
  { id: 'oc-gpt-5.4', provider: 'opencode', name: 'GPT-5.4', ctx: 400000, cin: null, cout: null, tools: true, reasoning: true, efforts: ['low', 'medium', 'high'], def: 'medium', src: 'acp', avail: 'live' },
  { id: 'oc-sonnet', provider: 'opencode', name: 'Sonnet 4.6', ctx: 200000, cin: null, cout: null, tools: true, reasoning: false, efforts: [], avail: 'live' },
  { id: 'cursor-auto', provider: 'cursor', name: 'Auto', ctx: null, cin: null, cout: null, tools: true, reasoning: false, efforts: [], avail: 'live' },
  { id: 'or-deepseek-r1', provider: 'openrouter', name: 'DeepSeek R1', ctx: 128000, cin: 0.55, cout: 2.2, tools: true, reasoning: true, efforts: [], avail: 'live' },
  { id: 'or-llama-3.3', provider: 'openrouter', name: 'Llama 3.3 70B', ctx: 128000, cin: 0.12, cout: 0.3, tools: true, reasoning: false, efforts: [], avail: 'live' },
  { id: 'or-qwen-2.5', provider: 'openrouter', name: 'Qwen 2.5 Coder', ctx: 128000, cin: 0.18, cout: 0.5, tools: true, reasoning: false, efforts: [], avail: 'live' },
  { id: 'grok-4', provider: 'xai', name: 'Grok 4', ctx: 256000, cin: 3, cout: 15, tools: true, reasoning: true, efforts: [], avail: 'live' },
  { id: 'groq-llama-3.3', provider: 'groq', name: 'Llama 3.3 70B', ctx: 128000, cin: 0.59, cout: 0.79, tools: true, reasoning: false, efforts: [], avail: 'live' },
  { id: 'groq-r1-distill', provider: 'groq', name: 'R1 Distill 70B', ctx: 128000, cin: 0.75, cout: 0.99, tools: true, reasoning: true, efforts: [], avail: 'live' },
  { id: 'kimi-k2', provider: 'moonshot', name: 'Kimi K2', ctx: 256000, cin: 0.6, cout: 2.5, tools: true, reasoning: false, efforts: [], avail: 'live' },
  { id: 'glm-4.6', provider: 'zai', name: 'GLM-4.6', ctx: 200000, cin: 0.6, cout: 2.2, tools: true, reasoning: false, efforts: [], avail: 'unavailable', reason: 'Sign in' },
];
const MODEL = {}; MODELS.forEach(m => { MODEL[m.id] = m; });

/* ---- provider marks (packages/ui/src/logos) ---- */
const LOGO = {
  claude: { vb: '0 0 256 257', inner: '<path fill="#D97757" d="m50.228 170.321 50.357-28.257.843-2.463-.843-1.361h-2.462l-8.426-.518-28.775-.778-24.952-1.037-24.175-1.296-6.092-1.297L0 125.796l.583-3.759 5.12-3.434 7.324.648 16.202 1.101 24.304 1.685 17.629 1.037 26.118 2.722h4.148l.583-1.685-1.426-1.037-1.101-1.037-25.147-17.045-27.22-18.017-14.258-10.37-7.713-5.25-3.888-4.925-1.685-10.758 7-7.713 9.397.649 2.398.648 9.527 7.323 20.35 15.75L94.817 91.9l3.889 3.24 1.555-1.102.195-.777-1.75-2.917-14.453-26.118-15.425-26.572-6.87-11.018-1.814-6.61c-.648-2.723-1.102-4.991-1.102-7.778l7.972-10.823L71.42 0 82.05 1.426l4.472 3.888 6.61 15.101 10.694 23.786 16.591 32.34 4.861 9.592 2.592 8.879.973 2.722h1.685v-1.556l1.36-18.211 2.528-22.36 2.463-28.776.843-8.1 4.018-9.722 7.971-5.25 6.222 2.981 5.12 7.324-.713 4.73-3.046 19.768-5.962 30.98-3.889 20.739h2.268l2.593-2.593 10.499-13.934 17.628-22.036 7.778-8.749 9.073-9.657 5.833-4.601h11.018l8.1 12.055-3.628 12.443-11.342 14.388-9.398 12.184-13.48 18.147-8.426 14.518.778 1.166 2.01-.194 30.46-6.481 16.462-2.982 19.637-3.37 8.88 4.148.971 4.213-3.5 8.62-20.998 5.184-24.628 4.926-36.682 8.685-.454.324.519.648 16.526 1.555 7.065.389h17.304l32.21 2.398 8.426 5.574 5.055 6.805-.843 5.184-12.962 6.611-17.498-4.148-40.83-9.721-14-3.5h-1.944v1.167l11.666 11.406 21.387 19.314 26.767 24.887 1.36 6.157-3.434 4.86-3.63-.518-23.526-17.693-9.073-7.972-20.545-17.304h-1.36v1.814l4.73 6.935 25.017 37.59 1.296 11.536-1.814 3.76-6.481 2.268-7.13-1.297-14.647-20.544-15.1-23.138-12.185-20.739-1.49.843-7.194 77.448-3.37 3.953-7.778 2.981-6.48-4.925-3.436-7.972 3.435-15.749 4.148-20.544 3.37-16.333 3.046-20.285 1.815-6.74-.13-.454-1.49.194-15.295 20.999-23.267 31.433-18.406 19.702-4.407 1.75-7.648-3.954.713-7.064 4.277-6.286 25.47-32.405 15.36-20.092 9.917-11.6-.065-1.686h-.583L44.07 198.125l-12.055 1.555-5.185-4.86.648-7.972 2.463-2.593 20.35-13.999-.064.065Z"/>' },
  codex: { vb: '0 0 721 721', inner: '<path fill="currentColor" d="M304.246 294.611V249.028C304.246 245.189 305.687 242.309 309.044 240.392L400.692 187.612C413.167 180.415 428.042 177.058 443.394 177.058C500.971 177.058 537.44 221.682 537.44 269.182C537.44 272.54 537.44 276.379 536.959 280.218L441.954 224.558C436.197 221.201 430.437 221.201 424.68 224.558L304.246 294.611ZM518.245 472.145V363.224C518.245 356.505 515.364 351.707 509.608 348.349L389.174 278.296L428.519 255.743C431.877 253.826 434.757 253.826 438.115 255.743L529.762 308.523C556.154 323.879 573.905 356.505 573.905 388.171C573.905 424.636 552.315 458.225 518.245 472.141V472.145ZM275.937 376.182L236.592 353.152C233.235 351.235 231.794 348.354 231.794 344.515V238.956C231.794 187.617 271.139 148.749 324.4 148.749C344.555 148.749 363.264 155.468 379.102 167.463L284.578 222.164C278.822 225.521 275.942 230.319 275.942 237.039V376.186L275.937 376.182ZM360.626 425.122L304.246 393.455V326.283L360.626 294.616L417.002 326.283V393.455L360.626 425.122ZM396.852 570.989C376.698 570.989 357.989 564.27 342.151 552.276L436.674 497.574C442.431 494.217 445.311 489.419 445.311 482.699V343.552L485.138 366.582C488.495 368.499 489.936 371.379 489.936 375.219V480.778C489.936 532.117 450.109 570.985 396.852 570.985V570.989ZM283.134 463.99L191.486 411.211C165.094 395.854 147.343 363.229 147.343 331.562C147.343 294.616 169.415 261.509 203.48 247.593V356.991C203.48 363.71 206.361 368.508 212.117 371.866L332.074 441.437L292.729 463.99C289.372 465.907 286.491 465.907 283.134 463.99ZM277.859 542.68C223.639 542.68 183.813 501.895 183.813 451.514C183.813 447.675 184.294 443.836 184.771 439.997L279.295 494.698C285.051 498.056 290.812 498.056 296.568 494.698L417.002 425.127V470.71C417.002 474.549 415.562 477.429 412.204 479.346L320.557 532.126C308.081 539.323 293.206 542.68 277.854 542.68H277.859ZM396.852 599.776C454.911 599.776 503.37 558.513 514.41 503.812C568.149 489.896 602.696 439.515 602.696 388.176C602.696 354.587 588.303 321.962 562.392 298.45C564.791 288.373 566.231 278.296 566.231 268.224C566.231 199.611 510.571 148.267 446.274 148.267C433.322 148.267 420.846 150.184 408.37 154.505C386.775 133.392 357.026 119.958 324.4 119.958C266.342 119.958 217.883 161.22 206.843 215.921C153.104 229.837 118.557 280.218 118.557 331.557C118.557 365.146 132.95 397.771 158.861 421.283C156.462 431.36 155.022 441.437 155.022 451.51C155.022 520.123 210.682 571.466 274.978 571.466C287.931 571.466 300.407 569.549 312.883 565.228C334.473 586.341 364.222 599.776 396.852 599.776Z"/>' },
  gemini: { vb: '0 0 296 298', inner: '<defs><linearGradient id="gm-g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#4285F4"/><stop offset=".45" stop-color="#9B72CB"/><stop offset=".75" stop-color="#D96570"/><stop offset="1" stop-color="#F6C013"/></linearGradient></defs><path fill="url(#gm-g)" d="M141.201 4.886c2.282-6.17 11.042-6.071 13.184.148l5.985 17.37a184.004 184.004 0 0 0 111.257 113.049l19.304 6.997c6.143 2.227 6.156 10.91.02 13.155l-19.35 7.082a184.001 184.001 0 0 0-109.495 109.385l-7.573 20.629c-2.241 6.105-10.869 6.121-13.133.025l-7.908-21.296a184 184 0 0 0-109.02-108.658l-19.698-7.239c-6.102-2.243-6.118-10.867-.025-13.132l20.083-7.467A183.998 183.998 0 0 0 133.291 26.28l7.91-21.394Z"/>' },
  cursor: { vb: '0 0 466.73 532.09', inner: '<path fill="currentColor" d="M457.43 125.94 244.42 2.96a22.127 22.127 0 0 0-22.12 0L9.3 125.94C3.55 129.26 0 135.4 0 142.05v247.99c0 6.65 3.55 12.79 9.3 16.11l213.01 122.98a22.127 22.127 0 0 0 22.12 0l213.01-122.98c5.75-3.32 9.3-9.46 9.3-16.11V142.05c0-6.65-3.55-12.79-9.3-16.11h-.01Zm-13.38 26.05L238.42 508.15c-1.39 2.4-5.06 1.42-5.06-1.36V273.58c0-4.66-2.49-8.97-6.53-11.31L24.87 145.67c-2.4-1.39-1.42-5.06 1.36-5.06h411.26c5.84 0 9.49 6.33 6.57 11.39h-.01Z"/>' },
  openrouter: { vb: '0 0 24 24', inner: '<path fill="currentColor" d="M16.778 1.844v1.919q-.569-.026-1.138-.032-.708-.008-1.415.037c-1.93.126-4.023.728-6.149 2.237-2.911 2.066-2.731 1.95-4.14 2.75-.396.223-1.342.574-2.185.798-.841.225-1.753.333-1.751.333v4.229s.768.108 1.61.333c.842.224 1.789.575 2.185.799 1.41.798 1.228.683 4.14 2.75 2.126 1.509 4.22 2.11 6.148 2.236.88.058 1.716.041 2.555.005v1.918l7.222-4.168-7.222-4.17v2.176c-.86.038-1.611.065-2.278.021-1.364-.09-2.417-.357-3.979-1.465-2.244-1.593-2.866-2.027-3.68-2.508.889-.518 1.449-.906 3.822-2.59 1.56-1.109 2.614-1.377 3.978-1.466.667-.044 1.418-.017 2.278.02v2.176L24 6.014Z"/>' },
  xai: { vb: '0 0 841.89 595.28', inner: '<path fill="currentColor" d="m557.09 211.99 8.31 326.37h66.56l8.32-445.18zM640.28 56.91H538.72L379.35 284.53l50.78 72.52zM201.61 538.36h101.56l50.79-72.52-50.79-72.53zM201.61 211.99l228.52 326.37h101.56L303.17 211.99z"/>' },
  groq: { vb: '0 0 201 201', fill: true, inner: '<path fill="#F54F35" d="M0 0h201v201H0V0Z"/><path fill="#FEFBFB" d="m128 49 1.895 1.52C136.336 56.288 140.602 64.49 142 73c.097 1.823.148 3.648.161 5.474l.03 3.247.012 3.482.017 3.613c.01 2.522.016 5.044.02 7.565.01 3.84.041 7.68.072 11.521.007 2.455.012 4.91.016 7.364l.038 3.457c-.033 11.717-3.373 21.83-11.475 30.547-4.552 4.23-9.148 7.372-14.891 9.73l-2.387 1.055c-9.275 3.355-20.3 2.397-29.379-1.13-5.016-2.38-9.156-5.17-13.234-8.925 3.678-4.526 7.41-8.394 12-12l3.063 2.375c5.572 3.958 11.135 5.211 17.937 4.625 6.96-1.384 12.455-4.502 17-10 4.174-6.784 4.59-12.222 4.531-20.094l.012-3.473c.003-2.414-.005-4.827-.022-7.241-.02-3.68 0-7.36.026-11.04-.003-2.353-.008-4.705-.016-7.058l.025-3.312c-.098-7.996-1.732-13.21-6.681-19.47-6.786-5.458-13.105-8.211-21.914-7.792-7.327 1.188-13.278 4.7-17.777 10.601C75.472 72.012 73.86 78.07 75 85c2.191 7.547 5.019 13.948 12 18 5.848 3.061 10.892 3.523 17.438 3.688l2.794.103c2.256.082 4.512.147 6.768.209v16c-16.682.673-29.615.654-42.852-10.848-8.28-8.296-13.338-19.55-13.71-31.277.394-9.87 3.93-17.894 9.562-25.875l1.688-2.563C84.698 35.563 110.05 34.436 128 49Z"/>' },
  moonshot: { vb: '0 0 1024 1024', inner: '<g transform="translate(0,1024) scale(0.1,-0.1)" fill="currentColor" stroke="none"><path d="M1595 10228 c-360 -40 -727 -206 -1000 -452 -330 -297 -541 -714 -585 -1161 -8 -78 -10 -1110 -8 -3570 4 -3236 5 -3465 21 -3545 62 -302 159 -531 324 -763 79 -111 288 -318 401 -397 241 -169 506 -277 792 -322 126 -19 7034 -19 7160 0 286 45 551 153 792 322 113 79 322 286 401 397 173 243 283 511 329 803 19 126 19 7034 0 7160 -46 292 -156 560 -329 803 -79 111 -287 317 -403 399 -112 80 -307 182 -440 231 -113 42 -288 82 -422 96 -123 14 -6915 13 -7033 -1z m1729 -2534 c14 -14 16 -120 16 -1015 l0 -999 633 0 632 0 470 1001 c258 551 475 1008 483 1015 11 12 94 14 484 12 422 -3 472 -5 481 -19 8 -13 -35 -116 -198 -470 -371 -810 -527 -1143 -562 -1198 -90 -141 -250 -266 -408 -321 -50 -17 -47 -18 305 -22 345 -5 358 -5 455 -31 121 -32 265 -100 357 -168 153 -114 288 -291 348 -458 68 -186 63 -99 67 -1375 3 -867 1 -1162 -8 -1172 -16 -19 -978 -21 -997 -2 -9 9 -12 334 -12 1400 0 763 -4 1388 -9 1388 -4 0 -16 -30 -26 -67 -35 -128 -89 -219 -189 -318 -72 -72 -108 -99 -176 -133 -164 -81 -87 -76 -1167 -80 l-963 -3 0 -1080 c0 -726 -3 -1087 -10 -1100 -10 -18 -26 -19 -498 -19 -366 0 -491 3 -500 12 -17 17 -17 5209 0 5226 9 9 133 12 494 12 425 0 484 -2 498 -16z"/></g>' },
  zai: { vb: '0 0 30 30', inner: '<path fill="currentColor" d="M15.47 7.1l-1.3 1.85c-.2.29-.54.47-.9.47h-7.1V7.09H15.47Z"/><path fill="currentColor" d="M24.3 7.1L13.14 22.91H5.7L16.86 7.1H24.3Z"/><path fill="currentColor" d="M14.53 22.91l1.31-1.86c.2-.29.54-.47.9-.47h7.09v2.33H14.53Z"/>' },
};
function glyphSVG(id) {
  const L = LOGO[id];
  if (!L) return '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>';
  return `<svg viewBox="${L.vb}" xmlns="http://www.w3.org/2000/svg">${L.inner}</svg>`;
}
const glyphTile = (cls, id) => `<span class="${cls}" data-fill="${!!(LOGO[id] && LOGO[id].fill)}">${glyphSVG(id)}</span>`;

function icon(name) {
  const p = {
    chev: '<path d="m6 9 6 6 6-6"/>',
    warn: '<path d="M10.3 3.3 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.3a2 2 0 0 0-3.4 0Z"/><path d="M12 9v4"/><path d="M12 17h.01"/>',
    tools: '<path d="M14.7 6.3a4 4 0 0 1-5.4 5.4L4 17v3h3l5.3-5.3a4 4 0 0 0 5.4-5.4l-2.3 2.3-2-2 2.3-2.3Z"/>',
    brain: '<path d="M9.5 3A2.5 2.5 0 0 0 7 5.5 2.5 2.5 0 0 0 5 8a2.5 2.5 0 0 0 .5 4.5V19a2 2 0 0 0 4 0V5.5A2.5 2.5 0 0 0 9.5 3Z"/><path d="M14.5 3A2.5 2.5 0 0 1 17 5.5 2.5 2.5 0 0 1 19 8a2.5 2.5 0 0 1-.5 4.5V19a2 2 0 0 1-4 0V5.5A2.5 2.5 0 0 1 14.5 3Z"/>',
    plus: '<path d="M12 5v14M5 12h14"/>',
    star: '<path d="m12 2 3.1 6.3 6.9 1-5 4.9 1.2 6.8L12 17.8 5.8 21l1.2-6.8-5-4.9 6.9-1L12 2Z"/>',
    gear: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1Z"/>',
    all: '<rect x="3" y="4" width="8" height="8" rx="1.5"/><rect x="13" y="4" width="8" height="8" rx="1.5"/><rect x="3" y="14" width="8" height="6" rx="1.5"/><rect x="13" y="14" width="8" height="6" rx="1.5"/>',
  }[name];
  return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">${p}</svg>`;
}
const starSvg = (fill) => `<svg viewBox="0 0 24 24" fill="${fill ? 'currentColor' : 'none'}" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="m12 2 3.1 6.3 6.9 1-5 4.9 1.2 6.8L12 17.8 5.8 21l1.2-6.8-5-4.9 6.9-1L12 2Z"/></svg>`;

const fmtCtx = (t) => t == null ? null : t >= 1e6 ? `${t / 1e6}M` : t >= 1000 ? `${Math.round(t / 1000)}k` : String(t);
const fmtCost = (m) => m.cin == null ? null : `$${m.cin}/${m.cout}`;
function imHTML(level, hollow) {
  let out = `<span class="im${hollow ? ' im--hollow' : ''}">`;
  for (let i = 1; i <= 7; i++) out += `<i class="${!hollow && i <= level ? 'on' : ''}"></i>`;
  return `${out}</span>`;
}
function reasoningStateFor(m) {
  if (m.efforts && m.efforts.length) return { mode: 'levels', levels: m.efforts, def: m.def || m.efforts[0], src: m.src };
  if (m.reasoning) return { mode: 'supported-nolevels' };
  return { mode: 'none' };
}

/* ============================================================
   REASONING RANGE SELECTOR (runtime-selector/reasoning-bar.tsx)
   ============================================================ */
function makeSlider(host, cfg) {
  const order = (cfg.levels || []).slice().sort((a, b) => effortPos(a) - effortPos(b));
  const values = [''].concat(order);
  const last = values.length - 1;
  const el = document.createElement('div');
  el.className = 'rz';
  let labels = '', stops = '';
  values.forEach((v, i) => {
    const p = last > 0 ? i / last : 0;
    const pos = `left:calc(${p}*(100% - var(--rz-h)) + var(--rz-h)/2)`;
    let lpos = pos, edge = '';
    if (i === 0) { lpos = 'left:0'; edge = ' data-edge="start"'; }
    else if (i === last) { lpos = 'right:0'; edge = ' data-edge="end"'; }
    labels += `<button type="button" class="rz__label"${edge} data-idx="${i}" style="${lpos}">${v === '' ? 'Default' : EFFORT_LABEL[v]}</button>`;
    stops += `<i class="rz__stop" style="${pos}"></i>`;
  });
  el.innerHTML = `<div class="rz__labels">${labels}</div>`
    + `<div class="rz__track" role="slider" tabindex="0" aria-orientation="horizontal"`
    + ` aria-label="Reasoning effort for ${esc(cfg.model || 'model')}" aria-valuemin="0" aria-valuemax="${last}">`
    + `<div class="rz__fill"></div>${stops}<div class="rz__thumb"></div></div>`;
  host.innerHTML = '';
  host.appendChild(el);

  const track = $('.rz__track', el);
  const labelEls = $$('.rz__label', el);
  let idx = Math.max(0, values.indexOf(cfg.value || ''));

  const markNearest = (i) => {
    el.setAttribute('data-default', i === 0 ? 'true' : 'false');
    labelEls.forEach((l, k) => l.setAttribute('data-on', k === i ? 'true' : 'false'));
  };
  const paint = () => {
    el.style.setProperty('--rz-p', last > 0 ? idx / last : 0);
    markNearest(idx);
    track.setAttribute('aria-valuenow', String(idx));
    track.setAttribute('aria-valuetext', idx === 0 ? `Provider default (${EFFORT_LABEL[cfg.def] || 'auto'})` : EFFORT_LABEL[values[idx]]);
  };
  const commit = (i) => {
    i = Math.max(0, Math.min(last, i));
    const changed = i !== idx;
    idx = i; paint();
    if (changed) cfg.onChange?.(values[idx]);
  };
  labelEls.forEach(l => l.addEventListener('click', () => { commit(parseInt(l.dataset.idx, 10)); track.focus(); }));

  const pFrom = (ev) => {
    const r = track.getBoundingClientRect(), pad = r.height / 2;
    return Math.max(0, Math.min(1, (ev.clientX - r.left - pad) / Math.max(1, r.width - 2 * pad)));
  };
  let drag = null;
  track.addEventListener('pointerdown', ev => {
    if (ev.button) return;
    drag = { sx: ev.clientX, engaged: false };
    track.setPointerCapture(ev.pointerId);
  });
  track.addEventListener('pointermove', ev => {
    if (!drag) return;
    if (!drag.engaged) {
      if (Math.abs(ev.clientX - drag.sx) < 3) return;
      drag.engaged = true; el.classList.add('is-dragging');
    }
    const p = pFrom(ev);
    el.style.setProperty('--rz-p', p);
    markNearest(Math.round(p * last));
  });
  track.addEventListener('pointerup', ev => {
    if (!drag) return;
    drag = null; el.classList.remove('is-dragging');
    commit(Math.round(pFrom(ev) * last));
    track.focus();
  });
  track.addEventListener('pointercancel', () => { if (drag) { drag = null; el.classList.remove('is-dragging'); paint(); } });
  track.addEventListener('keydown', e => {
    if (e.key === 'ArrowRight' || e.key === 'ArrowUp') { e.preventDefault(); commit(idx + 1); }
    else if (e.key === 'ArrowLeft' || e.key === 'ArrowDown') { e.preventDefault(); commit(idx - 1); }
    else if (e.key === 'Home') { e.preventDefault(); commit(0); }
    else if (e.key === 'End') { e.preventDefault(); commit(last); }
  });
  paint();
}

/* ============================================================
   ONBOARDING DRAFT (stores/use-onboarding-draft-store.ts)
   ============================================================ */
const draft = {
  step: 1,
  maxStep: 1,
  provider: 'claude',
  model: 'opus-4.6',
  reasoning: '',
  authMode: 'native_cli',
  envVar: '',
  apiKey: '',
  workspaces: [],
};
const envVarFor = (id) => `${id.toUpperCase().replace(/[^A-Z0-9]/g, '_')}_API_KEY`;
const defaultAuthFor = (id) => (PROV[id]?.harness === 'pi_acp' ? 'bound_secret' : 'native_cli');

/* ============================================================
   RUNTIME SELECTOR — trigger + popup
   ============================================================ */
const LS = { fav: 'agh:pmr:fav', recent: 'agh:pmr:recent' };
const lsLoad = (k, d) => { try { const v = localStorage.getItem(k); return v ? JSON.parse(v) : d; } catch { return d; } };
const lsSave = (k, v) => { try { localStorage.setItem(k, JSON.stringify(v)); } catch { /* private mode */ } };
let favSet = lsLoad(LS.fav, {});
let recentList = lsLoad(LS.recent, null) || MODELS.filter(m => m.recent).map(m => m.id);
MODELS.forEach(m => { if (m.fav) favSet[m.id] = true; });

const trigger = document.createElement('div');
trigger.className = 'pmr';
trigger.setAttribute('role', 'group');
trigger.setAttribute('data-open', 'false');
trigger.setAttribute('aria-labelledby', 'obRuntimeLabel');
trigger.setAttribute('data-od-id', 'pmr-trigger');
$('#obRuntimeMount').appendChild(trigger);

function renderTrigger() {
  const m = MODEL[draft.model], p = PROV[m.provider];
  const rz = reasoningStateFor(m);
  const needsAuth = p.auth !== 'ready' || m.avail === 'unavailable';
  let html = `<button class="pmr__seg" type="button" data-focus="provider" aria-label="Provider: ${esc(p.name)}" aria-haspopup="dialog">`
    + glyphTile('pmr__glyph', p.id) + `<span class="pmr__prov">${esc(p.name)}</span></button>`
    + `<button class="pmr__seg" type="button" data-focus="model" aria-label="Model: ${esc(m.name)}" aria-haspopup="dialog">`
    + `<span class="pmr__model">${esc(m.name)}</span></button>`;
  if (rz.mode === 'levels') {
    const cur = draft.reasoning || rz.def;
    const isDefault = !draft.reasoning;
    html += `<button class="pmr__seg" type="button" data-focus="reasoning" aria-label="Reasoning: ${isDefault ? 'provider default' : EFFORT_LABEL[cur]}" aria-haspopup="dialog">`
      + `<span class="pmr__rz">${imHTML(isDefault ? 0 : effortPos(cur), isDefault)}`
      + `<span class="pmr__rzlabel">${isDefault ? 'Default' : EFFORT_LABEL[cur]}</span></span></button>`;
  }
  if (needsAuth) {
    const why = m.reason || 'Needs sign in';
    html += `<span class="pmr__warn" role="img" aria-label="${esc(why)}" title="${esc(why)}">${icon('warn')}</span>`;
  }
  html += `<button class="pmr__chev" type="button" data-focus="model" aria-label="Open runtime selector" aria-haspopup="dialog">${icon('chev')}</button>`;
  trigger.innerHTML = html;
  $$('[data-focus]', trigger).forEach(seg => {
    seg.addEventListener('click', e => { e.stopPropagation(); openPopup(seg.dataset.focus); });
  });
}

function renderFacts() {
  const m = MODEL[draft.model], p = PROV[m.provider];
  const parts = [];
  const ctx = fmtCtx(m.ctx); if (ctx) parts.push(`<b>${ctx}</b> context`);
  const cost = fmtCost(m); if (cost) parts.push(`<b>${cost}</b> per Mtok`);
  if (m.tools) parts.push('tool calls');
  const rz = reasoningStateFor(m);
  if (rz.mode === 'levels') parts.push(`<b>${m.efforts.length}</b> reasoning levels`);
  else if (rz.mode === 'supported-nolevels') parts.push('reasoning on');
  parts.push(`<b>${HARNESS[p.harness]}</b> harness`);
  $('#obFacts').innerHTML = parts.join('<span class="dotsep"></span>');
}

/* ---- popup ---- */
const pop = $('#rsel');
const popList = $('#rselList');
const popRail = $('#rselRail');
const popRz = $('#rselRz');
const popSearch = $('#rselSearch');
const popRefresh = $('#rselRefresh');
let popOpen = false, railFilter = 'all', query = '', hlIndex = -1;

function openPopup(focus) {
  popOpen = true; railFilter = 'all'; query = ''; popSearch.value = ''; hlIndex = -1;
  trigger.setAttribute('data-open', 'true');
  renderRail(); renderList(); renderRz();
  positionPopup();
  pop.setAttribute('data-show', 'true');
  setTimeout(() => {
    if (focus === 'reasoning') { const t = $('.rz__track', popRz); if (t) return t.focus(); }
    if (focus === 'provider') { const a = $(`.rail__item[data-rail="${draft.provider}"]`, popRail); if (a) return a.focus(); }
    popSearch.focus();
  }, 20);
  document.addEventListener('mousedown', onDocDown, true);
}
function closePopup() {
  if (!popOpen) return;
  popOpen = false;
  trigger.setAttribute('data-open', 'false');
  pop.setAttribute('data-show', 'false');
  document.removeEventListener('mousedown', onDocDown, true);
}
function onDocDown(e) { if (!pop.contains(e.target) && !trigger.contains(e.target)) closePopup(); }

function positionPopup() {
  const r = trigger.getBoundingClientRect();
  const shown = pop.getAttribute('data-show') === 'true';
  pop.style.visibility = 'hidden'; pop.setAttribute('data-show', 'true');
  const pw = pop.offsetWidth, ph = pop.offsetHeight;
  pop.style.visibility = '';
  if (!shown) pop.setAttribute('data-show', 'false');
  let left = r.left, top = r.bottom + 6;
  left = Math.max(12, Math.min(left, window.innerWidth - pw - 12));
  if (top + ph > window.innerHeight - 12) top = Math.max(12, r.top - ph - 6);
  pop.style.left = `${left}px`;
  pop.style.top = `${top}px`;
}

const railBtn = (id, svg, title, on) => `<button class="rail__item" type="button" data-rail="${id}" data-active="${on}" title="${title}">${svg}</button>`;
function renderRail() {
  const searching = query.trim().length > 0;
  let html = railBtn('all', icon('all'), 'All models', railFilter === 'all')
    + railBtn('fav', starSvg(false), 'Favorites', railFilter === 'fav')
    + '<div class="rail__sep"></div>';
  PROVIDERS.forEach(p => {
    const dim = p.auth !== 'ready';
    html += `<button class="rail__item" type="button" data-rail="${p.id}" data-active="${railFilter === p.id}" data-dim="${dim}" title="${esc(p.name)}${dim ? ' · needs sign in' : ''}">${glyphTile('rail__logo', p.id)}</button>`;
  });
  html += `<button class="rail__item rail__gear" type="button" data-rail="__settings" title="Provider settings">${icon('gear')}</button>`;
  popRail.innerHTML = html;
  popRail.style.opacity = searching ? '.45' : '1';
  popRail.style.pointerEvents = searching ? 'none' : 'auto';
  $$('.rail__item', popRail).forEach(b => b.addEventListener('click', () => {
    const v = b.dataset.rail;
    if (v === '__settings') { b.style.color = 'var(--accent-strong)'; setTimeout(() => { b.style.color = ''; }, 500); return; }
    railFilter = v; hlIndex = -1; renderRail(); renderList(); popSearch.focus();
  }));
}

function matchQ(m, p) {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  const hay = `${m.name} ${p.name} ${m.id} ${m.provider}`.toLowerCase();
  return q.split(/\s+/).every(tok => hay.includes(tok));
}
function providerAvail(p, ms) {
  if (p.auth === 'signin') return { cls: 'av-off', label: 'Sign in' };
  if (ms.some(m => m.avail === 'stale')) return { cls: 'av-stale', label: 'Stale' };
  return { cls: 'av-live', label: 'Live' };
}
function groupHead(name, harness, avail) {
  let h = `<div class="grp__head"><span class="grp__name">${esc(name)}</span>`;
  if (harness) h += `<span class="grp__harness">${HARNESS[harness]}</span>`;
  if (avail) h += `<span class="grp__avail ${avail.cls}"><span class="dot"></span>${avail.label}</span>`;
  return `${h}</div>`;
}
function rowHTML(m) {
  const p = PROV[m.provider];
  const sel = draft.model === m.id;
  const disabled = m.avail === 'unavailable' || p.auth === 'signin';
  let chips = '';
  const ctx = fmtCtx(m.ctx); if (ctx) chips += `<span class="chip">${ctx}</span>`;
  const cost = fmtCost(m); if (cost) chips += `${chips ? '<span class="dotsep"></span>' : ''}<span class="chip">${cost}</span>`;
  if (m.tools) chips += `${chips ? '<span class="dotsep"></span>' : ''}<span class="chip">${icon('tools')}tools</span>`;
  if (m.efforts && m.efforts.length) chips += `${chips ? '<span class="dotsep"></span>' : ''}<span class="chip chip--rz">${icon('brain')}${m.efforts.length} levels</span>`;
  else if (m.reasoning) chips += `${chips ? '<span class="dotsep"></span>' : ''}<span class="chip">${icon('brain')}reasoning</span>`;
  let right = disabled ? `<span class="mrow__reason">${esc(m.reason || 'Sign in')}</span>` : '';
  right += `<span class="mrow__star" role="button" tabindex="0" data-fav="${!!favSet[m.id]}" aria-label="Toggle favorite">${starSvg(!!favSet[m.id])}</span>`;
  return `<div class="mrow" role="button" tabindex="0" data-model="${m.id}" data-sel="${sel}" data-disabled="${disabled}" data-od-id="model-${m.id}">`
    + glyphTile('mrow__glyph', m.provider)
    + `<span class="mrow__main"><span class="mrow__top"><span class="mrow__name">${esc(m.name)}</span></span>`
    + `<span class="mrow__sub"><span class="provref">${esc(p.name)}</span><span class="dotsep"></span>${chips}</span></span>`
    + `<span class="mrow__right">${right}</span></div>`;
}
function renderList() {
  const searching = query.trim().length > 0;
  let html = '', any = false;
  if (!searching && (railFilter === 'all' || railFilter === 'fav')) {
    const favModels = MODELS.filter(m => favSet[m.id]);
    const seen = new Set();
    const pinned = (railFilter === 'fav' ? favModels : recentList.map(id => MODEL[id]).filter(Boolean).concat(favModels))
      .filter(m => m && !seen.has(m.id) && seen.add(m.id));
    if (pinned.length) {
      html += groupHead(railFilter === 'fav' ? 'Favorites' : 'Recent & favorites');
      pinned.forEach(m => { html += rowHTML(m); any = true; });
    }
    if (railFilter === 'fav') return finishList(html, any);
  }
  const provs = PROVIDERS.filter(p => (railFilter === 'all' || railFilter === 'fav') ? true : p.id === railFilter);
  provs.forEach(p => {
    const ms = MODELS.filter(m => m.provider === p.id && matchQ(m, p));
    if (!ms.length) return;
    html += groupHead(p.name, p.harness, providerAvail(p, ms));
    ms.forEach(m => { html += rowHTML(m); any = true; });
  });
  if (!any) html = `<div class="rsel__empty">No models match “${esc(query.trim())}”.<br>Try a provider name, or a shorter query.</div>`;
  finishList(html, any);
}
function finishList(html, any) {
  popList.innerHTML = html;
  if (!any) return;
  $$('.mrow', popList).forEach(row => {
    const id = row.dataset.model;
    row.addEventListener('click', e => {
      if (e.target.closest('.mrow__star')) return;
      if (row.dataset.disabled === 'true') return;
      pickModel(id);
    });
    const star = $('.mrow__star', row);
    star?.addEventListener('click', e => {
      e.stopPropagation();
      if (favSet[id]) delete favSet[id]; else favSet[id] = true;
      lsSave(LS.fav, favSet); renderList();
    });
  });
  paintHighlight();
}
function renderRz() {
  const m = MODEL[draft.model];
  const rz = reasoningStateFor(m);
  if (rz.mode === 'none') {
    popRz.innerHTML = `<div class="rzbar__none">${icon('brain')}<span>This model doesn’t use reasoning effort; the provider handles it.</span></div>`;
    return;
  }
  if (rz.mode === 'supported-nolevels') {
    popRz.innerHTML = `<div class="rzbar__none">${icon('brain')}<span><b>Reasoning is on.</b> ${esc(PROV[m.provider].name)} doesn’t expose selectable effort for ${esc(m.name)}.</span></div>`;
    return;
  }
  popRz.innerHTML = `<div class="rzbar__head"><span class="rzbar__label">Reasoning effort</span>`
    + `<span class="rzbar__for">${esc(PROV[m.provider].name)} · ${esc(m.name)}</span>`
    + `<span class="rzbar__src">${rz.src === 'acp' ? 'ACP' : 'catalog'}</span></div><div class="rzbar__slider"></div>`;
  makeSlider($('.rzbar__slider', popRz), {
    levels: rz.levels, def: rz.def, value: draft.reasoning, model: m.name,
    onChange: (v) => { draft.reasoning = v || ''; renderTrigger(); renderFacts(); syncFooter(); positionPopup(); },
  });
}
function pickModel(id) {
  const m = MODEL[id];
  if (!m) return;
  const providerChanged = m.provider !== draft.provider;
  draft.provider = m.provider;
  draft.model = id;
  const rz = reasoningStateFor(m);
  if (rz.mode !== 'levels' || (draft.reasoning && !rz.levels.includes(draft.reasoning))) draft.reasoning = '';
  // provider change clears bound credentials (use-onboarding-default-model.ts)
  if (providerChanged) {
    draft.envVar = ''; draft.apiKey = '';
    draft.authMode = defaultAuthFor(draft.provider);
    syncAuth();
  }
  const seen = new Set();
  recentList = [id, ...recentList].filter(x => !seen.has(x) && seen.add(x)).slice(0, 6);
  lsSave(LS.recent, recentList);
  renderList(); renderRz(); renderTrigger(); renderFacts(); syncFooter(); positionPopup();
}
function paintHighlight() {
  const rows = $$('.mrow', popList);
  rows.forEach((r, i) => r.setAttribute('data-hl', i === hlIndex ? 'true' : 'false'));
  const r = rows[hlIndex];
  if (!r) return;
  const top = r.offsetTop, h = r.offsetHeight, st = popList.scrollTop, vh = popList.clientHeight;
  if (top < st) popList.scrollTop = top - 6;
  else if (top + h > st + vh) popList.scrollTop = top + h - vh + 6;
}
function moveHL(dir) {
  const rows = $$('.mrow', popList);
  const enabled = rows.map((r, i) => r.dataset.disabled !== 'true' ? i : -1).filter(i => i >= 0);
  if (!enabled.length) return;
  if (hlIndex < 0) hlIndex = enabled[0];
  else {
    const pos = enabled.indexOf(hlIndex);
    hlIndex = pos < 0 ? enabled[0] : enabled[(pos + dir + enabled.length) % enabled.length];
  }
  paintHighlight();
}
popSearch.addEventListener('input', () => { query = popSearch.value; hlIndex = -1; renderRail(); renderList(); });
popRefresh.addEventListener('click', () => {
  popRefresh.classList.add('spin');
  setTimeout(() => { popRefresh.classList.remove('spin'); renderList(); }, 520);
});
window.addEventListener('resize', () => { if (popOpen) positionPopup(); });

/* ============================================================
   STEP 1 — authentication mode
   ============================================================ */
const authWrap = $('#obAuth');
const keyFields = $('#obKeyFields');
const envInput = $('#obEnvVar');
const keyInput = $('#obApiKey');

function syncAuth() {
  $$('.authc', authWrap).forEach(b => b.setAttribute('aria-pressed', String(b.dataset.auth === draft.authMode)));
  keyFields.hidden = draft.authMode !== 'bound_secret';
  envInput.placeholder = envVarFor(draft.provider);
  envInput.value = draft.envVar;
  keyInput.value = draft.apiKey;
  measurePane();
}
$$('.authc', authWrap).forEach(b => b.addEventListener('click', () => {
  draft.authMode = b.dataset.auth;
  if (draft.authMode === 'native_cli') { draft.envVar = ''; draft.apiKey = ''; }
  syncAuth(); syncFooter();
  if (draft.authMode === 'bound_secret') envInput.focus();
}));
envInput.addEventListener('input', () => { draft.envVar = envInput.value; syncFooter(); });
keyInput.addEventListener('input', () => { draft.apiKey = keyInput.value; syncFooter(); });

/* ============================================================
   STEP 2 — directory browser (GET /api/fs/browse, dirs_only)
   ============================================================ */
const HOME = '/Users/pedro';
const FS = {
  '/': ['Applications', 'Library', 'System', 'Users'],
  '/Users': ['pedro', 'Shared'],
  '/Users/pedro': ['Desktop', 'Developer', 'Documents', 'Downloads', 'Projects'],
  '/Users/pedro/Desktop': [],
  '/Users/pedro/Developer': ['compozy', 'courses', 'labs', 'oss'],
  '/Users/pedro/Developer/compozy': ['agh', 'looper', 'pi', 'releasepr'],
  '/Users/pedro/Developer/compozy/agh': ['cmd', 'internal', 'packages', 'web'],
  '/Users/pedro/Developer/compozy/looper': [],
  '/Users/pedro/Developer/compozy/pi': [],
  '/Users/pedro/Developer/compozy/releasepr': [],
  '/Users/pedro/Developer/courses': ['branas'],
  '/Users/pedro/Developer/courses/branas': ['branasio'],
  '/Users/pedro/Developer/courses/branas/branasio': [],
  '/Users/pedro/Developer/labs': ['sketches'],
  '/Users/pedro/Developer/labs/sketches': [],
  '/Users/pedro/Developer/oss': [],
  '/Users/pedro/Documents': ['notes'],
  '/Users/pedro/Documents/notes': [],
  '/Users/pedro/Downloads': [],
  '/Users/pedro/Projects': ['field-notes'],
  '/Users/pedro/Projects/field-notes': [],
  '/Applications': [], '/Library': [], '/System': [], '/Users/Shared': [],
  '/Users/pedro/Developer/compozy/agh/cmd': [], '/Users/pedro/Developer/compozy/agh/internal': [],
  '/Users/pedro/Developer/compozy/agh/packages': [], '/Users/pedro/Developer/compozy/agh/web': [],
};
const parentOf = (p) => (p === '/' ? null : p.replace(/\/[^/]+$/, '') || '/');
const basename = (p) => p.replace(/\/+$/, '').split('/').pop() || p;

let cwd = HOME;
const dbPath = $('#dbPath');
const dbList = $('#dbList');
const dbUse = $('#dbUse');
const wsListEl = $('#wsList');
const wsEmpty = $('#wsEmpty');
const wsCount = $('#wsCount');

const isAdded = (p) => draft.workspaces.some(w => w.path === p);

function renderBrowser() {
  dbPath.textContent = cwd;
  dbPath.title = cwd;
  $('#dbHome').disabled = cwd === HOME;
  $('#dbUp').disabled = parentOf(cwd) === null;
  dbUse.disabled = isAdded(cwd);
  const entries = FS[cwd] || [];
  if (!entries.length) {
    dbList.innerHTML = '<p class="dirb__empty">No sub-folders here. Use this folder, or step back up.</p>';
    return;
  }
  dbList.innerHTML = entries.map(name => {
    const path = cwd === '/' ? `/${name}` : `${cwd}/${name}`;
    const added = isAdded(path);
    return `<div class="drow" data-added="${added}">
      <button class="drow__nav" type="button" data-nav="${esc(path)}">
        <svg class="drow__f" viewBox="0 0 16 16" aria-hidden="true"><path d="M2 4.6a1 1 0 0 1 1-1h3.1l1.3 1.5H13a1 1 0 0 1 1 1v5.3a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1Z" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/></svg>
        <span class="drow__n">${esc(name)}</span>
      </button>
      <button class="drow__add" type="button" data-add="${esc(path)}" aria-label="${added ? `${esc(name)} already added` : `Add ${esc(name)} as a workspace`}">
        ${added
          ? '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="m3.5 8.5 3 3 6-6.5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>'
          : '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M2 4.6a1 1 0 0 1 1-1h3.1l1.3 1.5H13a1 1 0 0 1 1 1v5.3a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1Z" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/><path d="M8 6.6v4M6 8.6h4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>'}
      </button>
    </div>`;
  }).join('');
  $$('[data-nav]', dbList).forEach(b => b.addEventListener('click', () => { cwd = b.dataset.nav; renderBrowser(); }));
  $$('[data-add]', dbList).forEach(b => b.addEventListener('click', () => addWorkspace(b.dataset.add)));
}
$('#dbHome').addEventListener('click', () => { cwd = HOME; renderBrowser(); });
$('#dbUp').addEventListener('click', () => { const p = parentOf(cwd); if (p) { cwd = p; renderBrowser(); } });
dbUse.addEventListener('click', () => addWorkspace(cwd));

function addWorkspace(path) {
  if (!path || isAdded(path)) return;
  draft.workspaces.push({ path, name: basename(path) });
  renderBrowser(); renderWorkspaces(); syncFooter(); syncShellWorkspace();
}
function removeWorkspace(path) {
  draft.workspaces = draft.workspaces.filter(w => w.path !== path);
  renderBrowser(); renderWorkspaces(); syncFooter(); syncShellWorkspace();
}
function renderWorkspaces() {
  const n = draft.workspaces.length;
  wsCount.textContent = `${n} folder${n === 1 ? '' : 's'}`;
  wsEmpty.hidden = n > 0;
  wsListEl.hidden = n === 0;
  wsListEl.innerHTML = draft.workspaces.map(w => `<li>
    <span class="wsl__ico" aria-hidden="true"><svg viewBox="0 0 16 16"><path d="M2 4.6a1 1 0 0 1 1-1h3.1l1.3 1.5H13a1 1 0 0 1 1 1v5.3a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1Z" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/></svg></span>
    <span class="wsl__main"><span class="wsl__n">${esc(w.name)}</span><span class="wsl__p mono">${esc(w.path)}</span></span>
    <button class="wsl__x" type="button" data-rm="${esc(w.path)}" aria-label="Remove ${esc(w.name)}">
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="m4.5 4.5 7 7M11.5 4.5l-7 7" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
    </button></li>`).join('');
  $$('[data-rm]', wsListEl).forEach(b => b.addEventListener('click', () => removeWorkspace(b.dataset.rm)));
}
/* the menu bar fills in behind the scrim as soon as a workspace resolves */
function syncShellWorkspace() {
  const first = draft.workspaces[0];
  const trig = $('#wsTrigger');
  $('#wsName').textContent = first ? first.name : 'No workspace';
  $('#wsAvatar').textContent = first ? first.name.slice(0, 2).toUpperCase() : '··';
  trig.dataset.empty = String(!first);
}

/* ============================================================
   WIZARD (hooks/use-onboarding-wizard.ts)
   ============================================================ */
const ob = $('#ob');
const obBody = $('#obBody');
const backBtn = $('#obBack');
const nextBtn = $('#obNext');
const sumEl = $('#obSum');
const sumLabel = $('#obSumLabel');
const sumValue = $('#obSumValue');

const paneOf = (n) => $(`.ob__pane[data-pane="${n}"]`);

function configurationError() {
  if (draft.step !== 1) return null;
  if (draft.authMode === 'bound_secret' && draft.envVar.trim() === '') {
    return 'Enter the environment variable the provider expects.';
  }
  return null;
}
function canContinue() {
  if (draft.step === 1) return configurationError() === null;
  return draft.workspaces.length > 0;
}

function syncFooter() {
  const err = configurationError();
  envInput.setAttribute('aria-invalid', String(!!err && draft.authMode === 'bound_secret'));
  if (err) {
    sumEl.dataset.tone = 'error';
    sumLabel.textContent = 'Needs an answer';
    sumValue.textContent = err;
  } else if (draft.step === 1) {
    const m = MODEL[draft.model], p = PROV[m.provider];
    const rz = reasoningStateFor(m);
    const bits = [p.name, m.name];
    if (rz.mode === 'levels') bits.push(draft.reasoning ? EFFORT_LABEL[draft.reasoning] : 'default effort');
    bits.push(draft.authMode === 'native_cli' ? 'CLI sign-in' : draft.envVar.trim() || 'bound key');
    sumEl.dataset.tone = 'ok';
    sumLabel.textContent = 'Saves as your default';
    sumValue.textContent = bits.join(' · ');
  } else {
    const n = draft.workspaces.length;
    sumEl.dataset.tone = 'ok';
    sumLabel.textContent = 'Workspaces';
    sumValue.textContent = n === 0
      ? 'None yet — add at least one folder to finish.'
      : `${n} folder${n === 1 ? '' : 's'} · ${draft.workspaces.map(w => w.name).join(', ')}`;
  }
  nextBtn.disabled = !canContinue();
}

/* The pane is stretched to the body, so its own scrollHeight can never
   report less than the current height. `.ob__inner` is the flow-sized
   content box, so its offsetHeight is the step's natural height. */
function measurePane() {
  const pane = paneOf(draft.step);
  const inner = pane && $('.ob__inner', pane);
  if (!inner) return;
  if (window.matchMedia('(max-width: 759px)').matches) { obBody.style.removeProperty('--ob-h'); return; }
  const chrome = 52 + 44 + 58;
  const max = Math.max(280, window.innerHeight - 48 - chrome);
  obBody.style.setProperty('--ob-h', `${Math.min(inner.offsetHeight, max)}px`);
}

function renderSteps() {
  $$('.ob__step', $('#obSteps')).forEach(btn => {
    const n = Number(btn.dataset.goto);
    const state = n < draft.step ? 'done' : n === draft.step ? 'on' : 'off';
    btn.dataset.state = state;
    btn.disabled = n > draft.maxStep;
    btn.setAttribute('aria-current', state === 'on' ? 'step' : 'false');
    const num = $('.ob__num', btn);
    num.innerHTML = state === 'done'
      ? '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="m3.5 8.5 3 3 6-6.5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>'
      : String(n);
  });
}

function goToStep(n) {
  if (n < 1 || n > 2 || n > draft.maxStep || n === draft.step) return;
  closePopup();
  const prev = paneOf(draft.step);
  draft.step = n;
  ob.dataset.step = String(n);
  const pane = paneOf(n);
  prev.hidden = true;
  pane.hidden = false;
  pane.dataset.enter = 'true';
  setTimeout(() => pane.removeAttribute('data-enter'), 320);
  renderSteps();
  backBtn.disabled = n === 1;
  nextBtn.textContent = n === 2 ? 'Finish setup' : 'Continue';
  measurePane();
  syncFooter();
  const landing = n === 2
    ? (dbUse.disabled ? $('.drow__nav', dbList) : dbUse)
    : $('.pmr__seg', trigger);
  landing?.focus();
}

backBtn.addEventListener('click', () => goToStep(draft.step - 1));
nextBtn.addEventListener('click', () => {
  if (!canContinue()) return;
  if (draft.step === 1) {
    draft.maxStep = 2;
    renderSteps();
    goToStep(2);
    return;
  }
  finish();
});
$$('.ob__step', $('#obSteps')).forEach(btn => btn.addEventListener('click', () => goToStep(Number(btn.dataset.goto))));

/* ============================================================
   COMMIT — POST /api/onboarding/complete, then the shell wakes
   ============================================================ */
function finish() {
  nextBtn.disabled = true;
  nextBtn.innerHTML = '<span class="spin-dot" aria-hidden="true"></span>Finishing…';
  setTimeout(unlockShell, 720);
}

function unlockShell() {
  const scrim = $('#obScrim');
  scrim.classList.add('is-leaving');
  document.body.dataset.onboarding = 'done';
  setTimeout(() => {
    scrim.remove();
    closePopup();
    pop.remove();
    $('#os').removeAttribute('inert');
    dockZone.dataset.locked = 'false';
    $('#deskHint').hidden = false;
    setTimeout(() => $$('.dock-item').forEach(el => { el.style.transitionDelay = ''; }), 700);
    const m = MODEL[draft.model], p = PROV[m.provider];
    const n = draft.workspaces.length;
    toast({
      title: `${draft.workspaces[0].name} · ready`,
      body: `Default model <strong>${esc(p.name)} / ${esc(m.name)}</strong> · ${n} workspace${n === 1 ? '' : 's'} registered. Sessions start from the dock or <code>⌘K</code>.`,
      actions: [
        { label: 'Open AGH', primary: true, run: () => { window.location.href = 'agh-os-v2.html'; } },
        { label: 'Run setup again', run: () => window.location.reload() },
      ],
    });
  }, 300);
}

/* dock + hint hand off to the running shell once setup is done */
dockEl.addEventListener('click', e => {
  if (document.body.dataset.onboarding !== 'done') return;
  if (e.target.closest('.dock-item')) window.location.href = 'agh-os-v2.html';
});
$('#dockNew').addEventListener('click', () => {
  if (document.body.dataset.onboarding === 'done') window.location.href = 'agh-os-v2.html';
});

function toast({ title, body, actions = [], timeout = 14000 }) {
  const el = document.createElement('div');
  el.className = 'toast';
  el.innerHTML = `<div class="toast-head"><span class="d d--ok"></span><strong>${title}</strong></div>
    <p class="toast-body">${body}</p>
    ${actions.length ? `<div class="toast-actions">${actions.map((a, i) => `<button class="btn btn--sm ${a.primary ? 'btn--primary' : ''}" data-ta="${i}">${a.label}</button>`).join('')}</div>` : ''}`;
  $('#toasts').appendChild(el);
  const dismiss = () => { el.classList.add('is-leaving'); setTimeout(() => el.remove(), 240); };
  actions.forEach((a, i) => $(`[data-ta="${i}"]`, el).addEventListener('click', () => { dismiss(); a.run?.(); }));
  setTimeout(dismiss, timeout);
}

/* ============================================================
   KEYBOARD + FOCUS CONTAINMENT
   ============================================================ */
document.addEventListener('keydown', e => {
  if (e.key === 'Escape' && popOpen) { e.preventDefault(); closePopup(); return; }
  if (popOpen) {
    if (e.target.closest?.('.rz')) return; // the slider owns its own keys
    if (e.key === 'ArrowDown') { e.preventDefault(); moveHL(1); return; }
    if (e.key === 'ArrowUp') { e.preventDefault(); moveHL(-1); return; }
    if (e.key === 'Enter') {
      const row = $$('.mrow', popList)[hlIndex];
      if (row && row.dataset.disabled !== 'true') { e.preventDefault(); pickModel(row.dataset.model); }
    }
    return;
  }
  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey) && document.body.dataset.onboarding === 'active') {
    e.preventDefault(); nextBtn.click();
  }
});
/* setup blocks the shell: focus never escapes the panel while it is open */
document.addEventListener('focusin', e => {
  if (document.body.dataset.onboarding !== 'active') return;
  const scrim = $('#obScrim');
  if (!scrim || scrim.contains(e.target) || pop.contains(e.target)) return;
  trigger.querySelector('.pmr__seg')?.focus();
});
window.addEventListener('resize', measurePane);

/* ============================================================
   BOOT
   ============================================================ */
draft.authMode = defaultAuthFor(draft.provider);
renderTrigger();
renderFacts();
syncAuth();
renderBrowser();
renderWorkspaces();
syncShellWorkspace();
renderSteps();
syncFooter();
measurePane();
requestAnimationFrame(measurePane);
document.fonts?.ready.then(measurePane);

})();
