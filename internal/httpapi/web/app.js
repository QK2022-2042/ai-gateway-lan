'use strict';

/* ══════════════════════════════════════════════════════════════════════════
   Helpers
   ══════════════════════════════════════════════════════════════════════════ */
const $ = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
const view = $('#view');

const state = {
  me: null,
  isAdmin: false,
  siteName: 'AI 模型中转站',
  version: '',
  page: '',
  overviewRange: '24h',
  overviewMetric: 'requests',
  overviewData: null,
  models: null,
  users: null,
  providers: null,
};

const ICONS = {
  overview: '<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>',
  logs: '<path d="M8 6h13M8 12h13M8 18h13"/><circle cx="3.5" cy="6" r=".8" fill="currentColor"/><circle cx="3.5" cy="12" r=".8" fill="currentColor"/><circle cx="3.5" cy="18" r=".8" fill="currentColor"/>',
  users: '<circle cx="9" cy="7" r="4"/><path d="M2.5 21v-1.8A5.2 5.2 0 017.7 14h2.6a5.2 5.2 0 015.2 5.2V21"/><path d="M17.5 3.5a4 4 0 010 7.6"/><path d="M19.5 14.4a5.2 5.2 0 012 4.1V21"/>',
  key: '<circle cx="7.5" cy="15.5" r="4.5"/><path d="M10.8 12.2L21 2l1.5 1.5-2 2 2 2-2 2-2-2-2.2 2.2 1.2 1.2-2.3 2.3"/>',
  providers: '<rect x="3" y="3" width="18" height="7" rx="2"/><rect x="3" y="14" width="18" height="7" rx="2"/><path d="M7 6.5h.01M7 17.5h.01"/><path d="M12 6.5h6M12 17.5h6"/>',
  models: '<path d="M21 8l-9-5-9 5 9 5 9-5z"/><path d="M3 8v8l9 5 9-5V8"/><path d="M12 13v8"/>',
  audit: '<path d="M12 3l8 3v6c0 4.6-3.4 7.7-8 9-4.6-1.3-8-4.4-8-9V6l8-3z"/><path d="M9 12l2 2 4-4"/>',
  guide: '<path d="M4 19.5A2.5 2.5 0 016.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 014 19.5v-15A2.5 2.5 0 016.5 2z"/><path d="M8 7h8M8 11h5"/>',
  settings: '<circle cx="12" cy="12" r="3.2"/><path d="M19.4 15a1.7 1.7 0 00.34 1.88l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.7 1.7 0 00-1.88-.34 1.7 1.7 0 00-1.03 1.56V21a2 2 0 11-4 0v-.09A1.7 1.7 0 009 19.4a1.7 1.7 0 00-1.88.34l-.06.06a2 2 0 11-2.83-2.83l.06-.06A1.7 1.7 0 004.6 15a1.7 1.7 0 00-1.56-1.03H3a2 2 0 110-4h.09A1.7 1.7 0 004.6 9a1.7 1.7 0 00-.34-1.88l-.06-.06a2 2 0 112.83-2.83l.06.06A1.7 1.7 0 009 4.6a1.7 1.7 0 001.03-1.56V3a2 2 0 114 0v.09A1.7 1.7 0 0015 4.6a1.7 1.7 0 001.88-.34l.06-.06a2 2 0 112.83 2.83l-.06.06A1.7 1.7 0 0019.4 9c.3.72.98 1.2 1.76 1.24H21a2 2 0 110 4h-.09A1.7 1.7 0 0019.4 15z"/>',
  logout: '<path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4"/><path d="M16 17l5-5-5-5"/><path d="M21 12H9"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/>',
  refresh: '<path d="M21 12a9 9 0 11-2.64-6.36"/><path d="M21 3v6h-6"/>',
  copy: '<rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>',
  close: '<path d="M18 6L6 18M6 6l12 12"/>',
  check: '<path d="M20 6L9 17l-5-5"/>',
  alert: '<circle cx="12" cy="12" r="9"/><path d="M12 8v4.5M12 16h.01"/>',
  trash: '<path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6M10 11v6M14 11v6"/>',
  edit: '<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 013 3L7 19l-4 1 1-4 12.5-12.5z"/>',
  play: '<path d="M6 3l14 9-14 9V3z"/>',
  download: '<path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><path d="M7 10l5 5 5-5"/><path d="M12 15V3"/>',
  arrowUp: '<path d="M12 19V5M5 12l7-7 7 7"/>',
  arrowDown: '<path d="M12 5v14M5 12l7 7 7-7"/>',
  checkCircle: '<circle cx="12" cy="12" r="9"/><path d="M8.5 12l2.5 2.5 4.5-5"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  shield: '<path d="M12 3l8 3v6c0 4.6-3.4 7.7-8 9-4.6-1.3-8-4.4-8-9V6l8-3z"/>',
  plug: '<path d="M9 2v6M15 2v6"/><path d="M6 8h12v3a6 6 0 01-12 0V8z"/><path d="M12 17v5"/>',
};

function icon(name, size = 16) {
  return `<svg viewBox="0 0 24 24" width="${size}" height="${size}" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[name] || ''}</svg>`;
}

function esc(v) {
  return String(v ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

const nf = new Intl.NumberFormat('zh-CN');
function fmtInt(n) { return nf.format(Math.round(Number(n) || 0)); }
function trimZero(s) { return s.includes('.') ? s.replace(/0+$/, '').replace(/\.$/, '') : s; }
function fmtCompact(n) {
  n = Number(n) || 0;
  const sign = n < 0 ? '-' : '';
  const a = Math.abs(n);
  if (a >= 1e8) return sign + trimZero((a / 1e8).toFixed(2)) + '亿';
  if (a >= 1e4) return sign + trimZero((a / 1e4).toFixed(1)) + '万';
  return sign + fmtInt(a);
}
function fmtMoney(n) {
  n = Number(n) || 0;
  if (n === 0) return '¥0.00';
  if (Math.abs(n) < 1) return '¥' + n.toFixed(4);
  return '¥' + n.toFixed(2);
}
function fmtPct(n) { return ((Number(n) || 0) * 100).toFixed(1) + '%'; }
function fmtMs(n) {
  n = Number(n) || 0;
  return n >= 1000 ? (n / 1000).toFixed(2) + ' s' : n + ' ms';
}
function fmtDateTime(ts) {
  if (!ts) return '—';
  const d = new Date(Number(ts) * 1000);
  const p = n => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}
function fmtShort(ts, bucket) {
  const d = new Date(Number(ts) * 1000);
  const p = n => String(n).padStart(2, '0');
  if (bucket >= 86400) return `${p(d.getMonth() + 1)}-${p(d.getDate())}`;
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:00`;
}
function relTime(ts) {
  if (!ts) return '—';
  const diff = Math.floor(Date.now() / 1000) - Number(ts);
  if (diff < 60) return '刚刚';
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`;
  if (diff < 86400 * 30) return `${Math.floor(diff / 86400)} 天前`;
  return fmtDateTime(ts).slice(0, 10);
}
function toLocalInput(ts) {
  if (!ts) return '';
  const d = new Date(Number(ts) * 1000);
  const off = d.getTimezoneOffset();
  return new Date(d.getTime() - off * 60000).toISOString().slice(0, 16);
}
function inputToUnix(v) {
  if (!v) return '';
  const t = new Date(v).getTime();
  return Number.isFinite(t) ? Math.floor(t / 1000) : '';
}
function pctWidth(v, max) { return max > 0 ? Math.max(1.5, Math.min(100, (Number(v) / max) * 100)) : 1.5; }

/* ══════════════════════════════════════════════════════════════════════════
   API / feedback
   ══════════════════════════════════════════════════════════════════════════ */
async function api(path, opts = {}) {
  const init = { credentials: 'same-origin', ...opts };
  const headers = { 'X-Requested-With': 'XMLHttpRequest', ...(opts.headers || {}) };
  if (opts.body !== undefined && typeof opts.body !== 'string') {
    init.body = JSON.stringify(opts.body);
    headers['Content-Type'] = 'application/json';
  }
  init.headers = headers;
  const res = await fetch(path, init);
  const ct = res.headers.get('content-type') || '';
  let data = null;
  if (ct.includes('application/json')) data = await res.json().catch(() => null);
  else data = await res.text().catch(() => '');
  if (!res.ok) {
    let msg = `请求失败 (${res.status})`;
    if (data && typeof data === 'object') {
      if (typeof data.error === 'string') msg = data.error;
      else if (data.error && typeof data.error === 'object' && data.error.message) msg = data.error.message;
      else if (data.message) msg = data.message;
    } else if (typeof data === 'string' && data.trim()) {
      msg = data.trim().slice(0, 200);
    }
    const err = new Error(msg);
    err.status = res.status;
    throw err;
  }
  return data;
}

function toast(msg, type = '') {
  const root = $('#toast-root');
  const el = document.createElement('div');
  el.className = 'toast ' + type;
  el.textContent = msg;
  root.appendChild(el);
  setTimeout(() => {
    el.style.opacity = '0';
    el.style.transition = 'opacity 180ms ease';
    setTimeout(() => el.remove(), 200);
  }, 3600);
}

async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
    toast('已复制到剪贴板', 'good');
  } catch {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    ta.remove();
    toast('已复制到剪贴板', 'good');
  }
}

function openModal({ title, subtitle = '', body = '', footer = '', size = '', onMount }) {
  const root = $('#modal-root');
  const overlay = document.createElement('div');
  overlay.className = 'modal-overlay';
  overlay.innerHTML = `
    <div class="modal ${size}" role="dialog" aria-modal="true" aria-label="${esc(title)}">
      <div class="modal-head">
        <div><h3>${esc(title)}</h3>${subtitle ? `<p>${esc(subtitle)}</p>` : ''}</div>
        <button class="modal-close" type="button" aria-label="关闭">${icon('close')}</button>
      </div>
      <div class="modal-body">${body}</div>
      ${footer ? `<div class="modal-foot">${footer}</div>` : ''}
    </div>`;
  root.appendChild(overlay);
  const close = () => {
    overlay.remove();
    document.removeEventListener('keydown', onKey);
  };
  const onKey = e => { if (e.key === 'Escape') close(); };
  document.addEventListener('keydown', onKey);
  overlay.addEventListener('click', e => { if (e.target === overlay) close(); });
  $('.modal-close', overlay).addEventListener('click', close);
  const focusable = overlay.querySelector('input, select, textarea, button.btn-primary');
  if (focusable) setTimeout(() => focusable.focus(), 40);
  if (onMount) onMount({ el: overlay, close, body: $('.modal-body', overlay), foot: $('.modal-foot', overlay) });
  return { el: overlay, close, body: $('.modal-body', overlay), foot: $('.modal-foot', overlay) };
}

function confirmDialog(title, message, { danger = false, confirmText = '确认' } = {}) {
  return new Promise(resolve => {
    const m = openModal({
      title,
      size: 'narrow',
      body: `<p style="color:var(--ink-2);font-size:12.5px;line-height:1.7;margin:0">${esc(message)}</p>`,
      footer: `<button class="btn" data-act="cancel" type="button">取消</button>
               <button class="btn ${danger ? 'btn-danger' : 'btn-primary'}" data-act="ok" type="button">${esc(confirmText)}</button>`,
      onMount: ({ el, close }) => {
        $('[data-act="cancel"]', el).addEventListener('click', () => { close(); resolve(false); });
        $('[data-act="ok"]', el).addEventListener('click', () => { close(); resolve(true); });
      },
    });
    m.el.addEventListener('click', e => { if (e.target === m.el) resolve(false); });
  });
}

function showSecret(title, secret, extra = '') {
  openModal({
    title,
    subtitle: '该密钥只显示这一次，请立即保存',
    body: `
      <div class="notice" style="margin-bottom:14px">${icon('alert')}
        <span>关闭此窗口后将无法再次查看完整密钥。如遗失，请使用「轮换」生成新的密钥。</span>
      </div>
      <div class="key-reveal" id="secret-value">${esc(secret)}</div>
      ${extra ? `<p class="muted" style="font-size:11px;margin-top:10px">${esc(extra)}</p>` : ''}`,
    footer: `<button class="btn" data-act="copy" type="button">${icon('copy')}复制密钥</button>
             <button class="btn btn-primary" data-act="done" type="button">我已保存</button>`,
    onMount: ({ el, close }) => {
      $('[data-act="copy"]', el).addEventListener('click', () => copyText(secret));
      $('[data-act="done"]', el).addEventListener('click', close);
    },
  });
}

/* ══════════════════════════════════════════════════════════════════════════
   Shared view pieces
   ══════════════════════════════════════════════════════════════════════════ */
function pageHead(title, desc, actions = '') {
  // The topbar already carries the page title and description; this row only
  // hosts page-level actions so the two headings never compete.
  if (!actions) return '';
  return `<div class="page-head" style="margin-top:-4px">
    <div></div><div class="page-head-actions">${actions}</div>
  </div>`;
}

function emptyState(title, desc, iconName = 'logs') {
  return `<div class="empty-state">${icon(iconName, 34)}<strong>${esc(title)}</strong><p>${esc(desc)}</p></div>`;
}

function errorBox(err) {
  return `<div class="error-box">${icon('alert')}<span>${esc(err.message || String(err))}</span></div>`;
}

function loadingCard() {
  return `<div class="card"><div class="card-body">
    <div class="skeleton" style="width:38%;margin-bottom:14px"></div>
    <div class="skeleton" style="width:100%;margin-bottom:8px"></div>
    <div class="skeleton" style="width:82%;margin-bottom:8px"></div>
    <div class="skeleton" style="width:64%"></div>
  </div></div>`;
}

function statTile(label, value, deltaHTML, note) {
  return `<div class="card stat">
    <div class="stat-label">${esc(label)}</div>
    <div class="stat-value">${value}</div>
    <div class="stat-foot">${deltaHTML}${note ? `<span class="stat-note">${esc(note)}</span>` : ''}</div>
  </div>`;
}

function deltaMarkup(current, previous, { goodWhenDown = false, pp = false } = {}) {
  const c = Number(current) || 0, p = Number(previous) || 0;
  if (c === 0 && p === 0) return `<span class="stat-delta flat">—</span>`;
  if (p === 0) return `<span class="stat-delta flat">新增</span>`;
  const diff = c - p;
  if (Math.abs(diff) < 1e-12) return `<span class="stat-delta flat">持平</span>`;
  const good = goodWhenDown ? diff < 0 : diff > 0;
  const text = pp ? `${diff > 0 ? '+' : ''}${(diff * 100).toFixed(1)} pp`
    : `${diff > 0 ? '+' : ''}${((diff / p) * 100).toFixed(1)}%`;
  return `<span class="stat-delta ${good ? '' : 'down'}">${icon(diff > 0 ? 'arrowUp' : 'arrowDown', 12)}${text}</span>`;
}

function statusPill(status) {
  const s = Number(status) || 0;
  if (s >= 200 && s < 400) return `<span class="pill good">成功</span>`;
  if (s === 429) return `<span class="pill warn">限流</span>`;
  if (s === 499) return `<span class="pill neutral">已断开</span>`;
  if (s >= 500) return `<span class="pill bad">${s === 502 ? '上游错误' : '失败'}</span>`;
  if (s >= 400) return `<span class="pill warn">客户端错误</span>`;
  return `<span class="pill neutral">${s || '未知'}</span>`;
}

function rolePill(role) {
  return role === 'admin' ? `<span class="pill info">管理员</span>` : `<span class="pill neutral">成员</span>`;
}

function enabledPill(enabled) {
  return enabled ? `<span class="pill good">启用</span>` : `<span class="pill neutral">停用</span>`;
}

function modelScope(list, inheritLabel = '全部模型') {
  if (!list || !list.length) return `<span class="muted">${esc(inheritLabel)}</span>`;
  if (list.includes('*')) return `<span class="pill info no-dot">全部</span>`;
  const shown = list.slice(0, 2).map(m => `<span class="tag">${esc(m)}</span>`).join('');
  return `<span class="chips">${shown}${list.length > 2 ? `<span class="tag">+${list.length - 2}</span>` : ''}</span>`;
}

function usageAgainst(value, limit, format = fmtInt) {
  const used = format(value || 0);
  return limit ? `${used}<span class="muted"> / ${format(limit)}</span>` : used;
}

function limitChips(k) {
  const parts = [];
  if (k.rpm_limit) parts.push(`${k.rpm_limit} RPM`);
  if (k.tpm_limit) parts.push(`${fmtCompact(k.tpm_limit)} TPM`);
  if (k.daily_request_limit) parts.push(`${k.daily_request_limit} 次/日`);
  if (k.daily_token_limit) parts.push(`${fmtCompact(k.daily_token_limit)} Tok/日`);
  if (k.monthly_token_limit) parts.push(`${fmtCompact(k.monthly_token_limit)} Tok/月`);
  if (k.concurrency_limit) parts.push(`${k.concurrency_limit} 并发`);
  if (!parts.length) return '<span class="muted">不限</span>';
  return parts.map(p => `<span class="tag">${p}</span>`).join('');
}

function pagerHTML(total, page, size) {
  const pages = Math.max(1, Math.ceil(total / size));
  return `<div class="pager">
    <span>共 ${fmtInt(total)} 条 · 第 ${page} / ${pages} 页</span>
    <div class="pager-actions">
      <button class="btn btn-sm" id="pager-prev" ${page <= 1 ? 'disabled' : ''}>上一页</button>
      <button class="btn btn-sm" id="pager-next" ${page >= pages ? 'disabled' : ''}>下一页</button>
    </div>
  </div>`;
}

function bindPager(total, page, size, go) {
  const pages = Math.max(1, Math.ceil(total / size));
  $('#pager-prev')?.addEventListener('click', () => { if (page > 1) go(page - 1); });
  $('#pager-next')?.addEventListener('click', () => { if (page < pages) go(page + 1); });
}

/* ══════════════════════════════════════════════════════════════════════════
   App shell / router
   ══════════════════════════════════════════════════════════════════════════ */
const NAV_SECTIONS = [
  {
    label: '运行',
    items: [
      { id: 'overview', label: '概览', icon: 'overview', admin: true },
      { id: 'logs', label: '调用日志', icon: 'logs' },
    ],
  },
  {
    label: '管理',
    items: [
      { id: 'keys', label: 'API 密钥', icon: 'key' },
      { id: 'users', label: '用户', icon: 'users', admin: true },
      { id: 'models', label: '模型与路由', icon: 'models', admin: true },
      { id: 'providers', label: '供应商', icon: 'providers', admin: true },
    ],
  },
  {
    label: '系统',
    items: [
      { id: 'audit', label: '审计日志', icon: 'audit', admin: true },
      { id: 'guide', label: '接入指南', icon: 'guide' },
      { id: 'settings', label: '系统设置', icon: 'settings' },
    ],
  },
];

const PAGE_META = {
  overview: ['概览', '内网 AI 模型的统一接入与用量视图'],
  logs: ['调用日志', '每一次模型调用的状态、延迟与用量'],
  keys: ['API 密钥', '分发密钥并设置模型权限与流量限额'],
  users: ['用户管理', '账号、模型权限与账号总额度'],
  models: ['模型与路由', '对外模型别名、价格与上游路由'],
  providers: ['供应商', 'OpenAI 兼容上游的连接、密钥与健康状态'],
  audit: ['审计日志', '管理员的关键操作记录'],
  guide: ['接入指南', '把内网模型接进你的应用'],
  settings: ['系统设置', '站点、日志保留与账号安全'],
};

function renderNav() {
  const nav = $('#nav');
  nav.innerHTML = NAV_SECTIONS.map(section => {
    const items = section.items.filter(it => !it.admin || state.isAdmin);
    if (!items.length) return '';
    return `<div class="nav-section">${section.label}</div>` + items.map(it => {
      const label = it.id === 'settings' && !state.isAdmin ? '账号设置' : it.label;
      return `<button class="nav-item" data-page="${it.id}" type="button">
        ${icon(it.icon)}<span>${label}</span>
      </button>`;
    }).join('');
  }).join('');
  $$('.nav-item', nav).forEach(btn => {
    btn.addEventListener('click', () => { location.hash = '#/' + btn.dataset.page; });
  });
}

function renderUserChip() {
  const u = state.me;
  if (!u) return;
  const initial = (u.display_name || u.username || '?').slice(0, 1);
  $('#user-chip').innerHTML = `
    <span class="avatar">${esc(initial)}</span>
    <span class="user-meta">
      <strong>${esc(u.display_name || u.username)}</strong>
      <span>${u.role === 'admin' ? '系统管理员' : '普通成员'}</span>
    </span>`;
}

function showApp() {
  $('#login-root').classList.add('hidden');
  $('#app').classList.remove('hidden');
  document.title = state.siteName;
  $('#brand-name').textContent = state.siteName;
  $('#foot-text').textContent = 'AI Gateway' + (state.version ? ' ' + state.version : '');
  renderNav();
  renderUserChip();
  if (!location.hash.startsWith('#/')) location.hash = '#/' + (state.isAdmin ? 'overview' : 'keys');
  else route();
}

function currentPage() {
  const raw = location.hash.replace(/^#\/?/, '');
  return raw.split('/')[0] || (state.isAdmin ? 'overview' : 'keys');
}

function route() {
  if (!state.me) return;
  const page = currentPage();
  const allowed = NAV_SECTIONS.flatMap(s => s.items)
    .find(it => it.id === page && (!it.admin || state.isAdmin));
  if (!allowed) {
    location.hash = '#/' + (state.isAdmin ? 'overview' : 'keys');
    return;
  }
  state.page = page;
  const [title, desc] = PAGE_META[page];
  const accountPage = page === 'settings' && !state.isAdmin;
  $('#page-title').textContent = accountPage ? '账号设置' : title;
  $('#page-desc').textContent = accountPage ? '修改登录密码与查看账号信息' : desc;
  $$('.nav-item').forEach(b => b.classList.toggle('active', b.dataset.page === page));
  view.innerHTML = loadingCard();
  view.focus({ preventScroll: true });
  const fn = PAGES[page];
  Promise.resolve(fn()).catch(err => {
    if (err.status === 401) { state.me = null; showLogin(); return; }
    view.innerHTML = pageHead(title, desc) + errorBox(err);
  });
}

/* ══════════════════════════════════════════════════════════════════════════
   Login
   ══════════════════════════════════════════════════════════════════════════ */
async function showLogin() {
  $('#app').classList.add('hidden');
  document.title = state.siteName + ' · 登录';
  const root = $('#login-root');
  root.classList.remove('hidden');
  root.innerHTML = `
    <div class="login-shell">
      <section class="login-hero">
        <div class="login-hero-inner">
          <div class="login-brand">
            <span class="brand-mark">
              <svg viewBox="0 0 32 32"><rect width="32" height="32" rx="8" fill="var(--series-1)"/><path d="M9 21V11l7 5 7-5v10" fill="none" stroke="#fff" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
            </span>
            <span class="brand-text"><strong>${esc(state.siteName)}</strong><small>Model Gateway</small></span>
          </div>
          <h1>一个入口，<br><span class="accent">连接所有内网模型</span></h1>
          <p class="lede">在完全离线的局域网中，统一分发模型 API、管理用户与配额，并掌握每一次调用的用量与健康状态。</p>
          <div class="login-points">
            <div class="login-point">${icon('checkCircle')}<span>OpenAI 协议兼容，现有 SDK 与客户端无需改造</span></div>
            <div class="login-point">${icon('checkCircle')}<span>按用户和密钥双层控制模型权限、RPM/TPM 与每日额度</span></div>
            <div class="login-point">${icon('checkCircle')}<span>多供应商优先级与自动故障转移，调用不中断</span></div>
            <div class="login-point">${icon('checkCircle')}<span>所有数据只存在本机 SQLite，不向任何外部服务发请求</span></div>
          </div>
          <div class="login-art" aria-hidden="true">
            <svg viewBox="0 0 520 170">
              <path class="link" d="M70 85 H200"/><path class="link" d="M200 85 H340"/><path class="link" d="M340 85 H455"/>
              <path class="link" d="M200 85 C260 30, 300 30, 340 45"/><path class="link" d="M200 85 C260 140, 300 140, 340 125"/>
              <circle class="node" cx="58" cy="85" r="24"/><circle class="node node-core" cx="200" cy="85" r="30"/>
              <circle class="node" cx="340" cy="45" r="20"/><circle class="node" cx="340" cy="85" r="20"/><circle class="node" cx="340" cy="125" r="20"/>
              <circle class="node" cx="465" cy="45" r="14"/><circle class="node" cx="465" cy="85" r="14"/><circle class="node" cx="465" cy="125" r="14"/>
              <path class="pulse" d="M82 85 H170"/><path class="pulse" d="M230 85 H320"/>
              <text x="58" y="132" text-anchor="middle" fill="var(--muted)" font-size="11">客户端</text>
              <text x="200" y="132" text-anchor="middle" fill="var(--muted)" font-size="11">AI 网关</text>
              <text x="340" y="160" text-anchor="middle" fill="var(--muted)" font-size="11">供应商</text>
            </svg>
          </div>
        </div>
      </section>
      <section class="login-panel">
        <div class="login-card">
          <h2>登录管理台</h2>
          <p class="sub">使用管理员或成员账号进入</p>
          <form class="login-form" id="login-form" autocomplete="on">
            <div class="form-error login-error" id="login-error"></div>
            <div class="field">
              <label for="login-user">用户名</label>
              <input class="input" id="login-user" name="username" autocomplete="username" required autofocus>
            </div>
            <div class="field">
              <label for="login-pass">密码</label>
              <input class="input" id="login-pass" name="password" type="password" autocomplete="current-password" required>
            </div>
            <button class="btn btn-primary" id="login-submit" type="submit">登录</button>
          </form>
          <p class="login-foot">
            离线部署 · 数据不出内网<br>
            初始管理员密码见首次启动日志或 data/initial-admin.txt
          </p>
        </div>
      </section>
    </div>`;
  try {
    const meta = await fetch('/api/meta').then(r => r.json());
    state.siteName = meta.site_name || state.siteName;
    state.version = meta.version || '';
    const strong = $('.login-brand .brand-text strong');
    if (strong) strong.textContent = state.siteName;
  } catch { /* offline shell is fine */ }

  const form = $('#login-form');
  form.addEventListener('submit', async e => {
    e.preventDefault();
    const btn = $('#login-submit');
    const errEl = $('#login-error');
    errEl.classList.remove('show');
    btn.disabled = true;
    btn.textContent = '登录中…';
    try {
      const data = await api('/api/auth/login', {
        method: 'POST',
        body: { username: $('#login-user').value.trim(), password: $('#login-pass').value },
      });
      state.me = data.user;
      state.isAdmin = !!data.is_admin;
      state.models = null;
      state.users = null;
      state.providers = null;
      showApp();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.add('show');
      btn.disabled = false;
      btn.textContent = '登录';
    }
  });
}

async function boot() {
  $('#logout-btn').innerHTML = icon('logout');
  $('#logout-btn').addEventListener('click', async () => {
    try { await api('/api/auth/logout', { method: 'POST' }); } catch { /* ignore */ }
    state.me = null;
    showLogin();
  });
  try {
    const meta = await fetch('/api/meta').then(r => r.json());
    state.siteName = meta.site_name || state.siteName;
    state.version = meta.version || '';
  } catch { /* ignore */ }
  try {
    const me = await api('/api/auth/me');
    state.me = me.user;
    state.isAdmin = !!me.is_admin;
    state.siteName = me.site_name || state.siteName;
    state.version = me.version || state.version;
    showApp();
  } catch {
    showLogin();
  }
}

/* ══════════════════════════════════════════════════════════════════════════
   Charts
   ══════════════════════════════════════════════════════════════════════════ */
const CHART = { w: 1000, h: 300, pad: { l: 58, r: 22, t: 22, b: 34 } };

function niceMax(v) {
  if (!v || v <= 0) return 1;
  const exp = Math.floor(Math.log10(v));
  const f = v / Math.pow(10, exp);
  let n;
  if (f <= 1) n = 1;
  else if (f <= 2) n = 2;
  else if (f <= 2.5) n = 2.5;
  else if (f <= 5) n = 5;
  else n = 10;
  return n * Math.pow(10, exp);
}

function renderTrend(points, bucketSec, metric) {
  const wrap = $('#trend-chart');
  if (!wrap) return;
  const fmt = metric === 'tokens' ? fmtCompact : metric === 'cost' ? fmtMoney : fmtInt;
  const values = points.map(p => Number(p[metric]) || 0);
  const total = values.reduce((a, b) => a + b, 0);
  const sub = $('#trend-sub');
  if (sub) sub.textContent = `${metricLabel(metric)} · 合计 ${fmt(total)}`;
  if (!points.length || values.every(v => v === 0)) {
    wrap.innerHTML = `<div class="chart-empty">所选周期内暂无调用数据</div>`;
    $('#trend-table').innerHTML = '';
    return;
  }
  const { w, h, pad } = CHART;
  const iw = w - pad.l - pad.r;
  const ih = h - pad.t - pad.b;
  const max = niceMax(Math.max(...values));
  const n = points.length;
  const x = i => pad.l + (n === 1 ? iw / 2 : (i / (n - 1)) * iw);
  const y = v => pad.t + ih - (Number(v) / max) * ih;

  let line = '';
  let area = '';
  points.forEach((p, i) => {
    const cmd = i === 0 ? 'M' : 'L';
    line += `${cmd}${x(i).toFixed(2)},${y(p[metric]).toFixed(2)}`;
    area += `${cmd}${x(i).toFixed(2)},${y(p[metric]).toFixed(2)}`;
  });
  area = `M${x(0).toFixed(2)},${(pad.t + ih).toFixed(2)}` + area.replace(/^M[^L]*/, '') +
    `L${x(n - 1).toFixed(2)},${(pad.t + ih).toFixed(2)}Z`;

  // Integer metrics with a tiny max (e.g. 2 requests) need fewer ticks, or
  // rounding would print the same axis label twice.
  const yTicks = metric === 'cost' ? 4 : Math.min(4, Math.max(1, Math.floor(max)));
  let grid = '';
  for (let i = 0; i <= yTicks; i++) {
    const gy = pad.t + (ih * i) / yTicks;
    const val = max * (1 - i / yTicks);
    grid += `<line class="grid-line" x1="${pad.l}" y1="${gy}" x2="${w - pad.r}" y2="${gy}"/>`;
    grid += `<text x="${pad.l - 10}" y="${gy + 3.5}" text-anchor="end">${esc(fmt(val))}</text>`;
  }
  const ticks = Math.min(6, n);
  let xlabels = '';
  const seen = new Set();
  for (let i = 0; i < ticks; i++) {
    const idx = ticks === 1 ? 0 : Math.round((i / (ticks - 1)) * (n - 1));
    if (seen.has(idx)) continue;
    seen.add(idx);
    xlabels += `<text x="${x(idx)}" y="${h - 10}" text-anchor="middle">${esc(fmtShort(points[idx].ts, bucketSec))}</text>`;
  }
  const endDot = `<circle class="hover-dot" cx="${x(n - 1)}" cy="${y(points[n - 1][metric])}" r="4"/>`;

  wrap.innerHTML = `
    <svg class="chart-svg" viewBox="0 0 ${w} ${h}" role="img"
      aria-label="${esc(metricLabel(metric))}趋势，共 ${n} 个时间点，合计 ${esc(fmt(total))}">
      ${grid}
      <line class="axis-line" x1="${pad.l}" y1="${pad.t + ih}" x2="${w - pad.r}" y2="${pad.t + ih}"/>
      ${xlabels}
      <path class="series-area" d="${area}"/>
      <path class="series-line" d="${line}" vector-effect="non-scaling-stroke"/>
      ${endDot}
      <line class="hover-line" x1="0" y1="${pad.t}" x2="0" y2="${pad.t + ih}" style="opacity:0"/>
      <circle class="hover-dot" r="4" style="opacity:0"/>
      <rect id="chart-hit" x="${pad.l}" y="${pad.t}" width="${iw}" height="${ih}" fill="transparent" style="cursor:crosshair"/>
    </svg>
    <div class="chart-tooltip" role="status"></div>`;

  const svg = $('svg', wrap);
  const tip = $('.chart-tooltip', wrap);
  const hoverLine = svg.querySelector('.hover-line');
  const hoverDot = svg.querySelectorAll('.hover-dot')[1];
  const hit = svg.querySelector('#chart-hit');

  const move = ev => {
    const rect = svg.getBoundingClientRect();
    const clientX = ev.touches ? ev.touches[0].clientX : ev.clientX;
    const px = ((clientX - rect.left) / rect.width) * w;
    let idx = Math.round(((px - pad.l) / iw) * (n - 1));
    idx = Math.max(0, Math.min(n - 1, idx));
    const cx = x(idx);
    const cy = y(points[idx][metric]);
    hoverLine.setAttribute('x1', cx); hoverLine.setAttribute('x2', cx);
    hoverLine.style.opacity = '1';
    hoverDot.setAttribute('cx', cx); hoverDot.setAttribute('cy', cy);
    hoverDot.style.opacity = '1';
    tip.innerHTML = `
      <div class="tt-time">${esc(fmtDateTime(points[idx].ts))}</div>
      <div class="tt-row"><span>${esc(metricLabel(metric))}</span><b>${esc(fmt(points[idx][metric]))}</b></div>
      <div class="tt-row"><span>请求数</span><b>${fmtInt(points[idx].requests)}</b></div>
      <div class="tt-row"><span>失败</span><b>${fmtInt(points[idx].errors)}</b></div>`;
    tip.style.left = `${(cx / w) * rect.width}px`;
    tip.style.top = `${(cy / h) * rect.height}px`;
    tip.classList.add('visible');
  };
  const leave = () => {
    tip.classList.remove('visible');
    hoverLine.style.opacity = '0';
    hoverDot.style.opacity = '0';
  };
  hit.addEventListener('mousemove', move);
  hit.addEventListener('mouseleave', leave);
  hit.addEventListener('touchstart', move, { passive: true });
  hit.addEventListener('touchmove', move, { passive: true });

  const tableRows = points.map(p => `<tr>
      <td>${esc(fmtDateTime(p.ts))}</td>
      <td class="num">${fmtInt(p.requests)}</td>
      <td class="num">${fmtCompact(p.tokens)}</td>
      <td class="num">${fmtMoney(p.cost)}</td>
      <td class="num">${fmtInt(p.errors)}</td>
    </tr>`).join('');
  $('#trend-table').innerHTML = `<details class="chart-table">
    <summary>查看数据表</summary>
    <div class="table-wrap"><table class="data-table">
      <thead><tr><th>时间</th><th class="num">请求</th><th class="num">Tokens</th><th class="num">费用</th><th class="num">失败</th></tr></thead>
      <tbody>${tableRows}</tbody>
    </table></div>
  </details>`;
}

function metricLabel(metric) {
  return metric === 'tokens' ? 'Tokens' : metric === 'cost' ? '预估费用' : '请求数';
}

function renderBars(container, rows, valueKey, formatter) {
  const el = typeof container === 'string' ? $(container) : container;
  if (!el) return;
  if (!rows.length) {
    el.innerHTML = `<div class="chart-empty" style="min-height:140px">暂无数据</div>`;
    return;
  }
  const max = Math.max(...rows.map(r => Number(r[valueKey]) || 0), 1);
  el.innerHTML = `<div class="bar-list">` + rows.map(r => `
    <div class="bar-row" title="${esc(r.name)} · ${fmtInt(r.requests)} 次 · ${fmtCompact(r.tokens)} Tokens">
      <span class="bar-label">${esc(r.name)}</span>
      <div class="bar-track"><div class="bar-fill" style="width:${pctWidth(r[valueKey], max)}%"></div></div>
      <span class="bar-value">${esc(formatter(r[valueKey]))}</span>
    </div>`).join('') + `</div>`;
}

/* ══════════════════════════════════════════════════════════════════════════
   Overview
   ══════════════════════════════════════════════════════════════════════════ */
async function pageOverview() {
  const d = await api(`/api/admin/stats/overview?range=${state.overviewRange}`);
  state.overviewData = d;
  const cur = d.current, prev = d.previous;
  const rangeLabel = { '24h': '最近 24 小时', '7d': '最近 7 天', '30d': '最近 30 天' }[d.range.key] || '最近 24 小时';

  view.innerHTML = `
    ${pageHead('概览', `${rangeLabel}的运行状态，较上一周期对比`, `
      <div class="segmented" id="range-seg">
        ${['24h', '7d', '30d'].map(k => `<button type="button" data-range="${k}" class="${k === d.range.key ? 'active' : ''}">${k === '24h' ? '24 小时' : k === '7d' ? '7 天' : '30 天'}</button>`).join('')}
      </div>
      <button class="btn" id="refresh-overview" type="button">${icon('refresh')}刷新</button>`)}
    <div class="grid-stats">
      ${statTile('请求数', fmtInt(cur.requests), deltaMarkup(cur.requests, prev.requests), '较上一周期')}
      ${statTile('Token 用量', fmtCompact(cur.tokens), deltaMarkup(cur.tokens, prev.tokens), '较上一周期')}
      ${statTile('预估费用', fmtMoney(cur.cost), deltaMarkup(cur.cost, prev.cost), '按配置单价')}
      ${statTile('成功率', fmtPct(cur.success_rate), deltaMarkup(cur.success_rate, prev.success_rate, { pp: true }), '较上一周期')}
      ${statTile('平均延迟', fmtMs(cur.avg_latency_ms), deltaMarkup(cur.avg_latency_ms, prev.avg_latency_ms, { goodWhenDown: true }), `P95 ${fmtMs(cur.p95_latency_ms)}`)}
    </div>
    <div class="dashboard-grid">
      <div class="stack">
        <div class="card">
          <div class="card-head">
            <div><div class="card-title">调用趋势</div><div class="card-sub" id="trend-sub"></div></div>
            <div class="segmented" id="metric-seg">
              ${[['requests', '请求数'], ['tokens', 'Tokens'], ['cost', '费用']].map(([k, l]) =>
    `<button type="button" data-metric="${k}" class="${k === state.overviewMetric ? 'active' : ''}">${l}</button>`).join('')}
            </div>
          </div>
          <div class="card-body"><div class="chart-wrap" id="trend-chart"></div><div id="trend-table"></div></div>
        </div>
        <div class="card">
          <div class="card-head"><div><div class="card-title">模型用量 Top 8</div><div class="card-sub">按请求数排序</div></div></div>
          <div class="card-body"><div id="model-bars"></div><div id="model-bars-table"></div></div>
        </div>
      </div>
      <div class="stack">
        <div class="card">
          <div class="card-head">
            <div><div class="card-title">供应商健康</div><div class="card-sub">最近一次探测状态</div></div>
            ${state.isAdmin ? `<a class="btn btn-sm btn-ghost" href="#/providers">管理</a>` : ''}
          </div>
          <div class="card-body tight" id="provider-health"></div>
        </div>
        <div class="card">
          <div class="card-head"><div><div class="card-title">近期错误</div><div class="card-sub">最近 8 条失败调用</div></div>
            <a class="btn btn-sm btn-ghost" href="#/logs">全部日志</a></div>
          <div class="card-body tight" id="recent-errors"></div>
        </div>
      </div>
    </div>`;

  renderTrend(d.series, d.bucket_sec, state.overviewMetric);

  const modelRows = d.by_model || [];
  renderBars('#model-bars', modelRows, 'requests', v => `${fmtInt(v)} 次`);
  $('#model-bars-table').innerHTML = modelRows.length ? `<details class="chart-table">
    <summary>查看数据表</summary>
    <div class="table-wrap"><table class="data-table">
      <thead><tr><th>模型</th><th class="num">请求</th><th class="num">Tokens</th><th class="num">费用</th><th class="num">失败</th><th class="num">平均延迟</th></tr></thead>
      <tbody>${modelRows.map(r => `<tr><td class="row-main mono">${esc(r.name)}</td><td class="num">${fmtInt(r.requests)}</td>
        <td class="num">${fmtCompact(r.tokens)}</td><td class="num">${fmtMoney(r.cost)}</td>
        <td class="num">${fmtInt(r.errors)}</td><td class="num">${fmtMs(r.avg_ms)}</td></tr>`).join('')}</tbody>
    </table></div></details>` : '';

  const providers = d.providers || [];
  $('#provider-health').innerHTML = providers.length ? providers.map(p => `
    <div style="display:flex;align-items:flex-start;gap:10px;padding:10px 2px;border-bottom:1px solid var(--grid)">
      <span class="health-dot ${p.health === 'ok' ? 'ok' : p.health === 'error' ? 'error' : 'unknown'}" style="margin-top:6px"></span>
      <div style="min-width:0;flex:1">
        <div style="display:flex;justify-content:space-between;gap:8px">
          <span style="font-size:12.5px;font-weight:600">${esc(p.name)}</span>
          <span class="pill ${p.health === 'ok' ? 'good' : p.health === 'error' ? 'bad' : 'neutral'}">${p.health === 'ok' ? '正常' : p.health === 'error' ? '异常' : '未检测'}</span>
        </div>
        <div class="muted" style="font-size:10.5px;margin-top:2px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">
          ${esc(p.health_msg || p.base_url)} · ${p.checked_at ? relTime(p.checked_at) : '尚未探测'}
        </div>
      </div>
    </div>`).join('') : `<div class="empty-state" style="padding:28px 10px">${icon('providers', 28)}<strong>还没有供应商</strong><p>先添加一个 OpenAI 兼容上游</p></div>`;

  const errs = d.recent_errors || [];
  $('#recent-errors').innerHTML = errs.length ? errs.map(l => `
    <div style="padding:9px 2px;border-bottom:1px solid var(--grid)">
      <div style="display:flex;justify-content:space-between;gap:8px;align-items:center">
        <span class="mono" style="font-size:11px;color:var(--ink)">${esc(l.model || '—')}</span>
        ${statusPill(l.status)}
      </div>
      <div class="muted" style="font-size:10.5px;margin-top:2px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap"
        title="${esc(l.error)}">${esc(l.error || '未知错误')}</div>
      <div class="muted" style="font-size:10px;margin-top:2px">${esc(l.username || '—')} · ${relTime(l.ts)}</div>
    </div>`).join('') : `<div class="empty-state" style="padding:28px 10px">${icon('checkCircle', 28)}<strong>近期没有错误</strong><p>所有调用都顺利完成</p></div>`;

  $$('#range-seg button').forEach(btn => btn.addEventListener('click', () => {
    state.overviewRange = btn.dataset.range;
    pageOverview().catch(err => { if (err.status === 401) { state.me = null; showLogin(); } else toast(err.message, 'bad'); });
  }));
  $$('#metric-seg button').forEach(btn => btn.addEventListener('click', () => {
    state.overviewMetric = btn.dataset.metric;
    $$('#metric-seg button').forEach(b => b.classList.toggle('active', b === btn));
    renderTrend(d.series, d.bucket_sec, state.overviewMetric);
  }));
  $('#refresh-overview')?.addEventListener('click', () => {
    state.overviewData = null;
    pageOverview().catch(err => toast(err.message, 'bad'));
  });
}

/* ══════════════════════════════════════════════════════════════════════════
   Logs
   ══════════════════════════════════════════════════════════════════════════ */
async function loadModelOptions() {
  if (state.models) return state.models;
  if (state.isAdmin) {
    const d = await api('/api/admin/models');
    state.models = d.items || [];
  } else {
    const d = await api('/api/my/models');
    state.models = d.items || [];
  }
  return state.models;
}

async function loadUserOptions() {
  if (!state.isAdmin) return [];
  if (state.users) return state.users;
  const d = await api('/api/admin/users');
  state.users = d.items || [];
  return state.users;
}

async function pageLogs() {
  const [models, users] = await Promise.all([loadModelOptions(), loadUserOptions()]);
  const now = Math.floor(Date.now() / 1000);
  const q = state.logQuery || { from: now - 86400, to: now };

  view.innerHTML = `
    ${pageHead('调用日志', '每一次模型调用的状态、延迟与用量', state.isAdmin
    ? `<button class="btn" id="export-logs" type="button">${icon('download')}导出 CSV</button>` : '')}
    <div class="card">
      <form class="filters" id="log-filters">
        <div class="field"><label>开始时间</label><input class="input" type="datetime-local" id="log-from" value="${toLocalInput(q.from)}"></div>
        <div class="field"><label>结束时间</label><input class="input" type="datetime-local" id="log-to" value="${toLocalInput(q.to)}"></div>
        <div class="field"><label>模型</label>
          <select class="select" id="log-model"><option value="">全部模型</option>
            ${models.map(m => `<option value="${esc(m.alias)}">${esc(m.alias)}</option>`).join('')}
          </select>
        </div>
        <div class="field"><label>状态</label>
          <select class="select" id="log-status">
            <option value="">全部</option><option value="success">成功</option><option value="error">失败</option>
          </select>
        </div>
        ${state.isAdmin ? `<div class="field"><label>用户</label>
          <select class="select" id="log-user"><option value="">全部用户</option>
            ${users.map(u => `<option value="${u.id}">${esc(u.username)}</option>`).join('')}
          </select></div>` : ''}
        <div class="field grow-field"><label>关键词</label>
          <input class="input" id="log-keyword" placeholder="请求 ID / 错误信息 / 客户端 IP">
        </div>
        <button class="btn btn-primary" type="submit">${icon('search')}查询</button>
        <button class="btn" type="button" id="log-reset">重置</button>
      </form>
      <div id="log-table">${loadingCard()}</div>
    </div>`;

  const readQuery = () => {
    const q = {
      from: inputToUnix($('#log-from').value),
      to: inputToUnix($('#log-to').value),
      model: $('#log-model')?.value || '',
      status: $('#log-status').value || '',
      keyword: $('#log-keyword').value.trim(),
    };
    if (state.isAdmin) q.user_id = $('#log-user')?.value || '';
    return q;
  };

  const load = async (page = 1) => {
    const query = readQuery();
    state.logQuery = query;
    const params = new URLSearchParams();
    Object.entries(query).forEach(([k, v]) => { if (v !== '' && v != null) params.set(k, v); });
    params.set('page', page);
    params.set('page_size', 20);
    $('#log-table').innerHTML = loadingCard();
    try {
      const d = await api('/api/logs?' + params.toString());
      renderLogTable(d);
      state.logExportQS = params.toString();
    } catch (err) {
      $('#log-table').innerHTML = errorBox(err);
    }
  };

  const renderLogTable = d => {
    const items = d.items || [];
    $('#log-table').innerHTML = items.length ? `
      <div class="table-wrap"><table class="data-table">
        <thead><tr>
          <th>时间</th><th>用户 / 密钥</th><th>模型</th><th>供应商</th><th>状态</th>
          <th class="num">Tokens</th><th class="num">费用</th><th class="num">延迟</th><th class="num">首字</th><th></th>
        </tr></thead>
        <tbody>${items.map(l => `<tr>
          <td class="nowrap mono" style="font-size:11px">${fmtDateTime(l.ts)}</td>
          <td><span class="row-main">${esc(l.username || '—')}</span><br><span class="muted" style="font-size:10.5px">${esc(l.key_name || '—')}</span></td>
          <td><span class="mono" style="font-size:11.5px;color:var(--ink)">${esc(l.model)}</span>
            ${l.retries ? `<span class="tag" title="重试次数">↻${l.retries}</span>` : ''}</td>
          <td>${esc(l.provider_name || '—')}</td>
          <td>${statusPill(l.status)}</td>
          <td class="num">${fmtCompact(l.total_tokens)}<br><span class="muted" style="font-size:10px">${fmtInt(l.prompt_tokens)} / ${fmtInt(l.completion_tokens)}</span></td>
          <td class="num">${fmtMoney(l.cost)}</td>
          <td class="num nowrap">${fmtMs(l.latency_ms)}</td>
          <td class="num nowrap">${l.first_token_ms ? fmtMs(l.first_token_ms) : '—'}</td>
          <td><button class="btn btn-sm btn-ghost" data-log-id="${l.id}" type="button">详情</button></td>
        </tr>`).join('')}</tbody>
      </table></div>
      ${pagerHTML(d.total, d.page, d.page_size)}` : emptyState('没有匹配的调用记录', '调整筛选条件或时间范围后重试');
    $$('[data-log-id]').forEach(btn => btn.addEventListener('click', () => {
      const l = items.find(x => String(x.id) === btn.dataset.logId);
      if (l) showLogDetail(l);
    }));
    bindPager(d.total, d.page, d.page_size, p => load(p));
  };

  $('#log-filters').addEventListener('submit', e => { e.preventDefault(); load(1); });
  $('#log-reset').addEventListener('click', () => {
    state.logQuery = { from: Math.floor(Date.now() / 1000) - 86400, to: Math.floor(Date.now() / 1000) };
    pageLogs();
  });
  $('#export-logs')?.addEventListener('click', () => {
    const params = new URLSearchParams();
    Object.entries(readQuery()).forEach(([k, v]) => { if (v !== '' && v != null) params.set(k, v); });
    window.location = '/api/admin/logs/export?' + params.toString();
  });

  if (state.logQuery?.model) $('#log-model').value = state.logQuery.model;
  if (state.logQuery?.status) $('#log-status').value = state.logQuery.status;
  if (state.logQuery?.keyword) $('#log-keyword').value = state.logQuery.keyword;
  if (state.isAdmin && state.logQuery?.user_id) $('#log-user').value = state.logQuery.user_id;
  await load(1);
}

function showLogDetail(l) {
  openModal({
    title: '调用详情',
    subtitle: fmtDateTime(l.ts) + ' · ' + (l.request_id || ''),
    size: 'wide',
    body: `
      ${l.error ? `<div class="error-box" style="margin-top:0;margin-bottom:14px">${icon('alert')}<span>${esc(l.error)}</span></div>` : ''}
      <dl class="kv">
        <dt>请求 ID</dt><dd class="mono">${esc(l.request_id || '—')}</dd>
        <dt>用户 / 密钥</dt><dd>${esc(l.username || '—')} / ${esc(l.key_name || '—')}</dd>
        <dt>模型</dt><dd class="mono">${esc(l.model)} → ${esc(l.upstream_model || '—')}</dd>
        <dt>供应商</dt><dd>${esc(l.provider_name || '—')}</dd>
        <dt>状态码</dt><dd>${statusPill(l.status)} <span class="mono">${l.status}</span></dd>
        <dt>Tokens</dt><dd>${fmtInt(l.prompt_tokens)} 输入 / ${fmtInt(l.completion_tokens)} 输出 / ${fmtInt(l.total_tokens)} 总计</dd>
        <dt>预估费用</dt><dd>${fmtMoney(l.cost)}</dd>
        <dt>总延迟 / 首字</dt><dd>${fmtMs(l.latency_ms)} / ${l.first_token_ms ? fmtMs(l.first_token_ms) : '—'}</dd>
        <dt>传输方式</dt><dd>${l.stream ? '流式 (SSE)' : '一次性响应'}${l.retries ? ` · 重试 ${l.retries} 次` : ''}</dd>
        <dt>客户端 IP</dt><dd class="mono">${esc(l.client_ip || '—')}</dd>
        <dt>User-Agent</dt><dd class="mono" style="font-size:11px">${esc(l.user_agent || '—')}</dd>
      </dl>`,
    footer: `<button class="btn" data-act="close" type="button">关闭</button>`,
    onMount: ({ el, close }) => $('[data-act="close"]', el).addEventListener('click', close),
  });
}

/* ══════════════════════════════════════════════════════════════════════════
   Users
   ══════════════════════════════════════════════════════════════════════════ */
async function pageUsers() {
  const d = await api('/api/admin/users');
  state.users = d.items || [];
  const items = state.users;
  view.innerHTML = `
    ${pageHead('用户管理', '账号、模型权限与账号总额度', `
      <button class="btn btn-primary" id="add-user" type="button">${icon('plus')}新建用户</button>`)}
    <div class="card">
      <div class="filters">
        <div class="field grow-field"><label>搜索</label><input class="input" id="user-search" placeholder="用户名 / 显示名 / 备注"></div>
        <button class="btn" id="user-refresh" type="button">${icon('refresh')}刷新</button>
      </div>
      <div id="user-table">${items.length ? `<div class="table-wrap"><table class="data-table">
        <thead><tr><th>用户</th><th>角色</th><th>状态</th><th>可用模型</th><th class="num">今日请求 / 上限</th><th class="num">今日 Tokens / 上限</th><th class="num">密钥</th><th>最后登录</th><th></th></tr></thead>
        <tbody>${items.map(u => `<tr>
          <td title="${esc(limitChips(u).replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim())}"><span class="row-main">${esc(u.username)}</span><br><span class="muted" style="font-size:10.5px">${esc(u.display_name || '—')}</span></td>
          <td>${rolePill(u.role)}</td>
          <td>${u.status === 'active' ? '<span class="pill good">正常</span>' : '<span class="pill bad">已禁用</span>'}</td>
          <td>${modelScope(u.allowed_models)}</td>
          <td class="num">${usageAgainst(u.usage_today?.requests, u.daily_request_limit)}</td>
          <td class="num">${usageAgainst(u.usage_today?.tokens, u.daily_token_limit, fmtCompact)}</td>
          <td class="num">${fmtInt(u.usage_today?.key_count || 0)}</td>
          <td class="nowrap">${u.last_login_at ? relTime(u.last_login_at) : '从未'}</td>
          <td><div class="actions">
            <button class="btn btn-sm" data-edit-user="${u.id}" type="button">编辑</button>
            <button class="btn btn-sm" data-reset-user="${u.id}" type="button">重置密码</button>
            <button class="btn btn-sm btn-danger" data-del-user="${u.id}" type="button">删除</button>
          </div></td>
        </tr>`).join('')}</tbody></table></div>` : emptyState('还没有用户', '创建第一个成员账号并分发密钥', 'users')}</div>
    </div>`;

  const reload = () => { state.users = null; pageUsers().catch(err => toast(err.message, 'bad')); };
  $('#add-user').addEventListener('click', () => userModal(null, reload));
  $('#user-refresh').addEventListener('click', reload);
  $('#user-search').addEventListener('input', e => {
    const kw = e.target.value.trim().toLowerCase();
    const filtered = items.filter(u => [u.username, u.display_name, u.note].join(' ').toLowerCase().includes(kw));
    const table = $('#user-table');
    if (!filtered.length) { table.innerHTML = emptyState('没有匹配的用户', '换个关键词试试', 'users'); return; }
    // Re-render only rows via the same template by replacing items.
    const html = `<div class="table-wrap"><table class="data-table"><thead><tr><th>用户</th><th>角色</th><th>状态</th><th>可用模型</th><th class="num">今日请求 / 上限</th><th class="num">今日 Tokens / 上限</th><th class="num">密钥</th><th>最后登录</th><th></th></tr></thead><tbody>${filtered.map(u => `<tr>
      <td title="${esc(limitChips(u).replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim())}"><span class="row-main">${esc(u.username)}</span><br><span class="muted" style="font-size:10.5px">${esc(u.display_name || '—')}</span></td>
      <td>${rolePill(u.role)}</td><td>${u.status === 'active' ? '<span class="pill good">正常</span>' : '<span class="pill bad">已禁用</span>'}</td>
      <td>${modelScope(u.allowed_models)}</td><td class="num">${usageAgainst(u.usage_today?.requests, u.daily_request_limit)}</td>
      <td class="num">${usageAgainst(u.usage_today?.tokens, u.daily_token_limit, fmtCompact)}</td><td class="num">${fmtInt(u.usage_today?.key_count || 0)}</td>
      <td class="nowrap">${u.last_login_at ? relTime(u.last_login_at) : '从未'}</td>
      <td><div class="actions"><button class="btn btn-sm" data-edit-user="${u.id}" type="button">编辑</button>
      <button class="btn btn-sm" data-reset-user="${u.id}" type="button">重置密码</button>
      <button class="btn btn-sm btn-danger" data-del-user="${u.id}" type="button">删除</button></div></td></tr>`).join('')}</tbody></table></div>`;
    table.innerHTML = html;
    bindRowActions(filtered, reload);
  });
  bindRowActions(items, reload);
}

function bindRowActions(items, reload) {
  $$('[data-edit-user]').forEach(btn => btn.addEventListener('click', () => {
    const u = items.find(x => String(x.id) === btn.dataset.editUser);
    if (u) userModal(u, reload);
  }));
  $$('[data-reset-user]').forEach(btn => btn.addEventListener('click', () => {
    const u = items.find(x => String(x.id) === btn.dataset.resetUser);
    if (u) resetPasswordModal(u, reload);
  }));
  $$('[data-del-user]').forEach(btn => btn.addEventListener('click', async () => {
    const u = items.find(x => String(x.id) === btn.dataset.delUser);
    if (!u) return;
    if (!await confirmDialog('删除用户', `确定删除用户「${u.username}」吗？其所有 API 密钥将同时失效，历史日志保留。`, { danger: true, confirmText: '删除' })) return;
    try {
      await api('/api/admin/users/' + u.id, { method: 'DELETE' });
      toast('用户已删除', 'good'); reload();
    } catch (err) { toast(err.message, 'bad'); }
  }));
}

function userModal(u, onDone) {
  const isEdit = !!u;
  openModal({
    title: isEdit ? '编辑用户' : '新建用户',
    subtitle: isEdit ? u.username : '创建成员账号并设置模型权限',
    size: 'wide',
    body: `<form id="user-form" class="form-grid">
      <div class="field"><label>用户名</label>
        <input class="input" name="username" value="${esc(u?.username || '')}" ${isEdit ? 'disabled' : 'required'} placeholder="字母、数字或 _ . -">
        ${isEdit ? '' : '<span class="hint">3–64 位，创建后不可修改</span>'}
      </div>
      <div class="field"><label>显示名称</label><input class="input" name="display_name" value="${esc(u?.display_name || '')}" placeholder="例如：算法组-张三"></div>
      ${isEdit ? '' : `<div class="field"><label>初始密码</label><input class="input" name="password" type="password" required placeholder="至少 8 位"><span class="hint">建议使用随机密码并通过安全渠道告知本人</span></div>`}
      <div class="field"><label>备注</label><input class="input" name="note" value="${esc(u?.note || '')}" placeholder="部门、用途等"></div>
      <div class="field"><label>角色</label>
        <select class="select" name="role">
          <option value="user" ${u?.role !== 'admin' ? 'selected' : ''}>普通成员</option>
          <option value="admin" ${u?.role === 'admin' ? 'selected' : ''}>管理员</option>
        </select>
      </div>
      <div class="field"><label>状态</label>
        <select class="select" name="status">
          <option value="active" ${u?.status !== 'disabled' ? 'selected' : ''}>正常</option>
          <option value="disabled" ${u?.status === 'disabled' ? 'selected' : ''}>禁用</option>
        </select>
      </div>
      <div class="field span-2"><label>可用模型</label>
        <input class="input mono-input" name="allowed_models" value="${esc((u?.allowed_models || ['*']).join(', '))}" placeholder="* 表示全部，或用逗号分隔多个模型别名">
        <span class="hint">支持通配符前缀，例如 qwen-*。API 密钥还可以在此范围内进一步收窄。</span>
      </div>
      <div class="form-grid-3 span-2">
        <div class="field"><label>RPM（请求/分钟）</label><input class="input" type="number" min="0" name="rpm_limit" value="${u?.rpm_limit || 0}"><span class="hint">0 为不限</span></div>
        <div class="field"><label>TPM（Token/分钟）</label><input class="input" type="number" min="0" name="tpm_limit" value="${u?.tpm_limit || 0}"><span class="hint">0 为不限</span></div>
        <div class="field"><label>并发请求数</label><input class="input" type="number" min="0" name="concurrency_limit" value="${u?.concurrency_limit || 0}"><span class="hint">0 为不限</span></div>
        <div class="field"><label>每日请求上限</label><input class="input" type="number" min="0" name="daily_request_limit" value="${u?.daily_request_limit || 0}"><span class="hint">0 为不限</span></div>
        <div class="field"><label>每日 Token 上限</label><input class="input" type="number" min="0" name="daily_token_limit" value="${u?.daily_token_limit || 0}"><span class="hint">0 为不限</span></div>
        <div class="field"><label>每月 Token 上限</label><input class="input" type="number" min="0" name="monthly_token_limit" value="${u?.monthly_token_limit || 0}"><span class="hint">0 为不限</span></div>
      </div>
      <div class="span-2"><div class="hint">用户级限额是该账号名下所有 API 密钥的合计上限；成员自助新建密钥也无法绕过。Key 级限额只能在此范围内进一步收紧。</div></div>
      <div class="span-2" id="user-error"></div>
    </form>`,
    footer: `<button class="btn" data-act="cancel" type="button">取消</button>
      <button class="btn btn-primary" data-act="save" type="button">${isEdit ? '保存' : '创建'}</button>`,
    onMount: ({ el, close }) => {
      $('[data-act="cancel"]', el).addEventListener('click', close);
      $('[data-act="save"]', el).addEventListener('click', async () => {
        const form = $('#user-form', el);
        const fd = new FormData(form);
        const body = {
          display_name: String(fd.get('display_name') || '').trim(),
          note: String(fd.get('note') || '').trim(),
          role: fd.get('role'),
          status: fd.get('status'),
          allowed_models: String(fd.get('allowed_models') || '').split(',').map(s => s.trim()).filter(Boolean),
          rpm_limit: Number(fd.get('rpm_limit')) || 0,
          tpm_limit: Number(fd.get('tpm_limit')) || 0,
          daily_request_limit: Number(fd.get('daily_request_limit')) || 0,
          daily_token_limit: Number(fd.get('daily_token_limit')) || 0,
          monthly_token_limit: Number(fd.get('monthly_token_limit')) || 0,
          concurrency_limit: Number(fd.get('concurrency_limit')) || 0,
        };
        if (!isEdit) {
          body.username = String(fd.get('username') || '').trim();
          body.password = String(fd.get('password') || '');
        }
        const errEl = $('#user-error', el);
        errEl.innerHTML = '';
        try {
          await api(isEdit ? '/api/admin/users/' + u.id : '/api/admin/users', {
            method: isEdit ? 'PATCH' : 'POST', body,
          });
          toast(isEdit ? '用户已更新' : '用户已创建', 'good');
          close(); onDone();
        } catch (err) {
          errEl.innerHTML = `<div class="form-error">${esc(err.message)}</div>`;
        }
      });
    },
  });
}

function resetPasswordModal(u, onDone) {
  openModal({
    title: '重置密码',
    subtitle: '用户 ' + u.username + ' 的所有登录会话将立即失效',
    size: 'narrow',
    body: `<form class="form-grid" id="reset-form">
      <div class="field span-2"><label>新密码</label>
        <div class="copy-row"><input class="input" name="password" id="reset-pass" type="text" required placeholder="至少 8 位">
        <button class="copy-btn" type="button" id="gen-pass">随机生成</button></div>
        <span class="hint">请通过安全渠道告知用户，并提醒其登录后修改。</span>
      </div>
      <div class="span-2" id="reset-error"></div>
    </form>`,
    footer: `<button class="btn" data-act="cancel" type="button">取消</button>
      <button class="btn btn-primary" data-act="save" type="button">重置</button>`,
    onMount: ({ el, close }) => {
      $('[data-act="cancel"]', el).addEventListener('click', close);
      $('#gen-pass', el).addEventListener('click', () => {
        const chars = 'abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789';
        let s = '';
        const buf = new Uint32Array(16);
        crypto.getRandomValues(buf);
        for (const n of buf) s += chars[n % chars.length];
        $('#reset-pass', el).value = s;
      });
      $('[data-act="save"]', el).addEventListener('click', async () => {
        const pw = $('#reset-pass', el).value;
        try {
          await api('/api/admin/users/' + u.id + '/password', { method: 'POST', body: { password: pw } });
          toast('密码已重置', 'good');
          showSecret('新密码', pw, '用户 ' + u.username + ' 下一次登录请使用此密码。');
          close(); onDone();
        } catch (err) {
          $('#reset-error', el).innerHTML = `<div class="form-error">${esc(err.message)}</div>`;
        }
      });
    },
  });
}

/* ══════════════════════════════════════════════════════════════════════════
   API keys
   ══════════════════════════════════════════════════════════════════════════ */
async function pageKeys() {
  const [users, models] = await Promise.all([loadUserOptions(), loadModelOptions()]);
  const qs = state.keyUserFilter ? '?user_id=' + encodeURIComponent(state.keyUserFilter) : '';
  const d = await api('/api/keys' + qs);
  const items = d.items || [];
  view.innerHTML = `
    ${pageHead('API 密钥', '分发密钥并设置模型权限与流量限额', `
      ${state.isAdmin ? `<select class="select" id="key-user-filter" style="width:150px">
        <option value="">全部用户</option>
        ${users.map(u => `<option value="${u.id}" ${String(u.id) === String(state.keyUserFilter || '') ? 'selected' : ''}>${esc(u.username)}</option>`).join('')}
      </select>` : ''}
      <button class="btn btn-primary" id="add-key" type="button">${icon('plus')}新建密钥</button>`)}
    <div class="card">
      <div class="filters">
        <div class="field grow-field"><label>搜索</label><input class="input" id="key-search" placeholder="名称 / 密钥前缀 / 用户名"></div>
        <button class="btn" id="key-refresh" type="button">${icon('refresh')}刷新</button>
      </div>
      <div id="key-table">${items.length ? keysTable(items) : emptyState('还没有 API 密钥', '创建密钥后即可调用 /v1 接口', 'key')}</div>
    </div>`;
  const reload = () => pageKeys().catch(err => { if (err.status === 401) { state.me = null; showLogin(); } else toast(err.message, 'bad'); });
  if (state.isAdmin) {
    $('#key-user-filter').addEventListener('change', e => {
      state.keyUserFilter = e.target.value || null;
      reload();
    });
  }
  $('#key-refresh').addEventListener('click', reload);
  $('#add-key').addEventListener('click', () => keyModal(null, users, models, reload));
  $('#key-search').addEventListener('input', e => {
    const kw = e.target.value.trim().toLowerCase();
    const filtered = items.filter(k => [k.name, k.key_prefix, k.username].join(' ').toLowerCase().includes(kw));
    $('#key-table').innerHTML = filtered.length ? keysTable(filtered) : emptyState('没有匹配的密钥', '换个关键词试试', 'key');
    bindKeyActions(filtered, users, models, reload);
  });
  bindKeyActions(items, users, models, reload);
}

function keysTable(items) {
  return `<div class="table-wrap"><table class="data-table">
    <thead><tr><th>名称</th>${state.isAdmin ? '<th>所属用户</th>' : ''}<th>密钥前缀</th><th>状态</th>
      <th class="num">今日用量</th><th>限额</th><th>模型范围</th><th>最后使用</th><th>到期</th><th></th></tr></thead>
    <tbody>${items.map(k => `<tr>
      <td><span class="row-main">${esc(k.name)}</span></td>
      ${state.isAdmin ? `<td>${esc(k.username || '—')}</td>` : ''}
      <td><span class="mono" style="font-size:11px">${esc(k.key_prefix)}</span></td>
      <td>${k.status === 'active' ? '<span class="pill good">启用</span>' : '<span class="pill neutral">禁用</span>'}</td>
      <td class="num">${fmtInt(k.usage_today?.requests || 0)} 次<br><span class="muted" style="font-size:10px">${fmtCompact(k.usage_today?.tokens || 0)} Tok</span></td>
      <td><span class="chips">${limitChips(k)}</span></td>
      <td>${modelScope(k.allowed_models, '继承用户')}</td>
      <td class="nowrap">${k.last_used_at ? relTime(k.last_used_at) : '从未'}</td>
      <td class="nowrap">${k.expires_at ? fmtDateTime(k.expires_at).slice(0, 10) : '<span class="muted">永不过期</span>'}</td>
      <td><div class="actions">
        <button class="btn btn-sm" data-edit-key="${k.id}" type="button">编辑</button>
        <button class="btn btn-sm" data-rotate-key="${k.id}" type="button">轮换</button>
        <button class="btn btn-sm btn-danger" data-del-key="${k.id}" type="button">删除</button>
      </div></td>
    </tr>`).join('')}</tbody></table></div>`;
}

function bindKeyActions(items, users, models, reload) {
  $$('[data-edit-key]').forEach(btn => btn.addEventListener('click', () => {
    const k = items.find(x => String(x.id) === btn.dataset.editKey);
    if (k) keyModal(k, users, models, reload);
  }));
  $$('[data-rotate-key]').forEach(btn => btn.addEventListener('click', async () => {
    const k = items.find(x => String(x.id) === btn.dataset.rotateKey);
    if (!k) return;
    if (!await confirmDialog('轮换密钥', `轮换「${k.name}」后，旧密钥立即失效，使用旧密钥的客户端将无法调用。`, { confirmText: '轮换' })) return;
    try {
      const res = await api(`/api/keys/${k.id}/rotate`, { method: 'POST' });
      showSecret('新密钥已生成', res.plaintext_key, '请更新所有使用该密钥的应用配置。');
      reload();
    } catch (err) { toast(err.message, 'bad'); }
  }));
  $$('[data-del-key]').forEach(btn => btn.addEventListener('click', async () => {
    const k = items.find(x => String(x.id) === btn.dataset.delKey);
    if (!k) return;
    if (!await confirmDialog('删除密钥', `确定删除「${k.name}」吗？使用该密钥的调用会立即失败。`, { danger: true, confirmText: '删除' })) return;
    try {
      await api('/api/keys/' + k.id, { method: 'DELETE' });
      toast('密钥已删除', 'good'); reload();
    } catch (err) { toast(err.message, 'bad'); }
  }));
}

function keyModal(k, users, models, onDone) {
  const isEdit = !!k;
  const presetUser = k ? k.user_id : Number(state.keyUserFilter || 0);
  const userOptions = users.map(u => `<option value="${u.id}" ${(presetUser ? u.id === presetUser : u.id === users[0]?.id) ? 'selected' : ''}>${esc(u.username)}</option>`).join('');
  openModal({
    title: isEdit ? '编辑密钥' : '新建 API 密钥',
    subtitle: isEdit ? k.name : '密钥只会在创建后显示一次',
    size: 'wide',
    body: `<form id="key-form" class="form-grid">
      ${state.isAdmin && !isEdit ? `<div class="field"><label>所属用户</label><select class="select" name="user_id">${userOptions}</select></div>` : ''}
      <div class="field"><label>名称</label><input class="input" name="name" value="${esc(k?.name || '')}" required placeholder="例如：算法组-生产环境"></div>
      <div class="field"><label>状态</label><select class="select" name="status">
        <option value="active" ${k?.status !== 'disabled' ? 'selected' : ''}>启用</option>
        <option value="disabled" ${k?.status === 'disabled' ? 'selected' : ''}>禁用</option>
      </select></div>
      <div class="field span-2"><label>可用模型</label>
        <input class="input mono-input" name="allowed_models" value="${esc((k?.allowed_models || []).join(', '))}" placeholder="留空继承用户权限；* 表示全部">
        <span class="hint">仅能访问用户权限范围内的模型。例如：qwen2.5-7b, deepseek-v3</span>
      </div>
      <div class="form-grid-3 span-2">
        <div class="field"><label>RPM（请求/分钟）</label><input class="input" type="number" min="0" name="rpm_limit" value="${k?.rpm_limit || 0}"><span class="hint">0 为不限</span></div>
        <div class="field"><label>TPM（Token/分钟）</label><input class="input" type="number" min="0" name="tpm_limit" value="${k?.tpm_limit || 0}"><span class="hint">0 为不限</span></div>
        <div class="field"><label>并发请求数</label><input class="input" type="number" min="0" name="concurrency_limit" value="${k?.concurrency_limit || 0}"><span class="hint">0 为不限</span></div>
        <div class="field"><label>每日请求上限</label><input class="input" type="number" min="0" name="daily_request_limit" value="${k?.daily_request_limit || 0}"><span class="hint">0 为不限</span></div>
        <div class="field"><label>每日 Token 上限</label><input class="input" type="number" min="0" name="daily_token_limit" value="${k?.daily_token_limit || 0}"><span class="hint">0 为不限</span></div>
        <div class="field"><label>每月 Token 上限</label><input class="input" type="number" min="0" name="monthly_token_limit" value="${k?.monthly_token_limit || 0}"><span class="hint">0 为不限</span></div>
      </div>
      <div class="span-2"><div class="hint">用户级限额同时生效，此处的 Key 级限额只能在用户额度内进一步收紧。</div></div>
      <div class="field span-2"><label>到期时间</label>
        <input class="input" type="datetime-local" name="expires_at" value="${k?.expires_at ? toLocalInput(k.expires_at) : ''}">
        <span class="hint">留空表示永不过期</span>
      </div>
      <div class="span-2" id="key-error"></div>
    </form>`,
    footer: `<button class="btn" data-act="cancel" type="button">取消</button>
      <button class="btn btn-primary" data-act="save" type="button">${isEdit ? '保存' : '创建并显示密钥'}</button>`,
    onMount: ({ el, close }) => {
      $('[data-act="cancel"]', el).addEventListener('click', close);
      $('[data-act="save"]', el).addEventListener('click', async () => {
        const fd = new FormData($('#key-form', el));
        const body = {
          name: String(fd.get('name') || '').trim(),
          status: fd.get('status'),
          allowed_models: String(fd.get('allowed_models') || '').split(',').map(s => s.trim()).filter(Boolean),
          rpm_limit: Number(fd.get('rpm_limit')) || 0,
          tpm_limit: Number(fd.get('tpm_limit')) || 0,
          daily_request_limit: Number(fd.get('daily_request_limit')) || 0,
          daily_token_limit: Number(fd.get('daily_token_limit')) || 0,
          monthly_token_limit: Number(fd.get('monthly_token_limit')) || 0,
          concurrency_limit: Number(fd.get('concurrency_limit')) || 0,
          expires_at: fd.get('expires_at') ? Number(inputToUnix(fd.get('expires_at'))) : null,
        };
        if (state.isAdmin && !isEdit) body.user_id = Number(fd.get('user_id'));
        const errEl = $('#key-error', el);
        errEl.innerHTML = '';
        try {
          const res = await api(isEdit ? '/api/keys/' + k.id : '/api/keys', {
            method: isEdit ? 'PATCH' : 'POST', body,
          });
          close();
          if (!isEdit) showSecret('API 密钥已创建', res.plaintext_key, 'Base URL 使用 ' + location.origin + '/v1');
          else toast('密钥已保存', 'good');
          onDone();
        } catch (err) {
          errEl.innerHTML = `<div class="form-error">${esc(err.message)}</div>`;
        }
      });
    },
  });
}

/* ══════════════════════════════════════════════════════════════════════════
   Providers
   ══════════════════════════════════════════════════════════════════════════ */
async function pageProviders() {
  const d = await api('/api/admin/providers');
  const items = d.items || [];
  state.providers = items;
  view.innerHTML = `
    ${pageHead('供应商', 'OpenAI 兼容上游的连接、密钥与健康状态', `
      <button class="btn" id="provider-refresh" type="button">${icon('refresh')}刷新</button>
      <button class="btn btn-primary" id="add-provider" type="button">${icon('plus')}添加供应商</button>`)}
    ${items.length ? `<div class="provider-grid" id="provider-grid">${items.map(providerCard).join('')}</div>`
      : `<div class="card">${emptyState('还没有供应商', '添加 vLLM、Ollama 或任意 OpenAI 兼容服务的内网地址', 'providers')}</div>`}`;

  const reload = () => pageProviders().catch(err => toast(err.message, 'bad'));
  $('#add-provider').addEventListener('click', () => providerModal(null, reload));
  $('#provider-refresh').addEventListener('click', reload);
  bindProviderActions(items, reload);
}

function providerCard(p) {
  const health = p.health === 'ok' ? 'good' : p.health === 'error' ? 'bad' : 'neutral';
  const healthText = p.health === 'ok' ? '正常' : p.health === 'error' ? '异常' : '未检测';
  return `<div class="card provider-card">
    <div class="provider-top">
      <div style="min-width:0">
        <div class="provider-name">${esc(p.name)}</div>
        <div class="provider-url" title="${esc(p.base_url)}">${esc(p.base_url)}</div>
      </div>
      <div style="display:flex;gap:6px;flex-wrap:wrap;justify-content:flex-end">
        ${p.status === 'enabled' ? '<span class="pill good">启用</span>' : '<span class="pill neutral">停用</span>'}
      </div>
    </div>
    <div style="display:flex;align-items:center;gap:8px">
      <span class="health-dot ${p.health === 'ok' ? 'ok' : p.health === 'error' ? 'error' : 'unknown'}"></span>
      <span class="pill ${health} no-dot" style="background:transparent;border-color:var(--border)">${healthText}</span>
      <span class="muted" style="font-size:10.5px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">
        ${esc(p.health_msg || (p.checked_at ? relTime(p.checked_at) : '尚未探测'))}
      </span>
    </div>
    <div class="provider-stats">
      <span><b>${p.keys_count}</b> 个密钥</span>
      <span><b>${p.route_count}</b> 条路由</span>
      <span>优先级 <b>${p.priority}</b></span>
      <span>权重 <b>${p.weight}</b></span>
      <span>超时 <b>${p.timeout_sec}s</b></span>
    </div>
    <div class="provider-actions">
      <button class="btn btn-sm" data-test-provider="${p.id}" type="button">${icon('play')}测试</button>
      <button class="btn btn-sm" data-sync-provider="${p.id}" type="button">${icon('refresh')}同步模型</button>
      <button class="btn btn-sm" data-edit-provider="${p.id}" type="button">编辑</button>
      <button class="btn btn-sm btn-danger" data-del-provider="${p.id}" type="button">删除</button>
    </div>
  </div>`;
}

function bindProviderActions(items, reload) {
  $$('[data-edit-provider]').forEach(btn => btn.addEventListener('click', () => {
    const p = items.find(x => String(x.id) === btn.dataset.editProvider);
    if (p) providerModal(p, reload);
  }));
  $$('[data-test-provider]').forEach(btn => btn.addEventListener('click', async () => {
    const p = items.find(x => String(x.id) === btn.dataset.testProvider);
    if (!p) return;
    btn.disabled = true;
    btn.innerHTML = `${icon('refresh', 13)}测试中…`;
    try {
      const res = await api(`/api/admin/providers/${p.id}/test`, { method: 'POST' });
      toast(res.check.ok ? `连接正常 · ${res.check.latency_ms} ms · ${res.check.model_ids?.length || 0} 个模型` : `连接失败：${res.check.message}`, res.check.ok ? 'good' : 'bad');
      reload();
    } catch (err) {
      toast(err.message, 'bad');
      btn.disabled = false;
      btn.innerHTML = `${icon('play', 13)}测试`;
    }
  }));
  $$('[data-sync-provider]').forEach(btn => btn.addEventListener('click', async () => {
    const p = items.find(x => String(x.id) === btn.dataset.syncProvider);
    if (!p) return;
    btn.disabled = true;
    btn.innerHTML = `${icon('refresh', 13)}拉取中…`;
    try {
      const res = await api(`/api/admin/providers/${p.id}/sync-models`, { method: 'POST' });
      const configured = new Set(res.configured || []);
      openModal({
        title: '上游模型列表',
        subtitle: `${p.name} · 共 ${res.models.length} 个模型`,
        size: 'wide',
        body: res.models.length ? `<div class="chips">${res.models.map(m => `<span class="tag" style="font-size:11px;padding:4px 8px">${esc(m)}${configured.has(m) ? ' ✓' : ''}</span>`).join('')}</div>
          <p class="hint" style="margin-top:14px">带 ✓ 的模型已配置路由。前往「模型与路由」把需要的模型绑定到对外别名。</p>`
          : `<div class="empty-state">上游没有返回模型列表</div>`,
        footer: `<button class="btn" data-act="copy" type="button">${icon('copy')}复制全部</button>
                 <button class="btn btn-primary" data-act="close" type="button">关闭</button>`,
        onMount: ({ el, close }) => {
          $('[data-act="close"]', el).addEventListener('click', close);
          $('[data-act="copy"]', el).addEventListener('click', () => copyText(res.models.join('\n')));
        },
      });
      reload();
    } catch (err) {
      toast(err.message, 'bad');
      btn.disabled = false;
      btn.innerHTML = `${icon('refresh', 13)}同步模型`;
    }
  }));
  $$('[data-del-provider]').forEach(btn => btn.addEventListener('click', async () => {
    const p = items.find(x => String(x.id) === btn.dataset.delProvider);
    if (!p) return;
    if (!await confirmDialog('删除供应商', `确定删除「${p.name}」吗？其下 ${p.route_count} 条模型路由将一并删除，相关模型可能暂时不可用。`, { danger: true, confirmText: '删除' })) return;
    try {
      await api('/api/admin/providers/' + p.id, { method: 'DELETE' });
      toast('供应商已删除', 'good'); reload();
    } catch (err) { toast(err.message, 'bad'); }
  }));
}

function providerModal(p, onDone) {
  const isEdit = !!p;
  const headersText = p?.extra_headers ? Object.entries(p.extra_headers).map(([k, v]) => `${k}: ${v}`).join('\n') : '';
  const keysHint = isEdit ? `当前 ${p.keys_count} 个密钥（${(p.keys_masked || []).join('、') || '无'}）` : '';
  openModal({
    title: isEdit ? '编辑供应商' : '添加供应商',
    subtitle: '支持任意 OpenAI Chat Completions 兼容服务',
    size: 'wide',
    body: `<form id="provider-form" class="form-grid">
      <div class="field"><label>名称</label><input class="input" name="name" value="${esc(p?.name || '')}" required placeholder="例如：vLLM 集群 A"></div>
      <div class="field"><label>Base URL</label>
        <input class="input mono-input" name="base_url" value="${esc(p?.base_url || '')}" required placeholder="http://10.0.0.21:8000">
        <span class="hint">可以带 /v1，也可以不带；网关会自动拼接 /v1/chat/completions</span>
      </div>
      <div class="field span-2"><label>上游 API Key</label>
        <textarea class="textarea mono-input" name="keys" rows="3" placeholder="每行一个 Key；内网无鉴权服务可留空">${esc((p?.keys_masked || []).map(() => '').join(''))}</textarea>
        <span class="hint">${isEdit ? esc(keysHint) + '。留空表示保持不变；填写则整体替换。' : '每行一个 Key，会加密存储；多个 Key 会轮换使用。'}</span>
      </div>
      <div class="form-grid-3 span-2">
        <div class="field"><label>超时（秒）</label><input class="input" type="number" min="1" max="3600" name="timeout_sec" value="${p?.timeout_sec || 120}"></div>
        <div class="field"><label>优先级</label><input class="input" type="number" min="0" max="1000" name="priority" value="${p?.priority ?? 10}"><span class="hint">数值越小越优先</span></div>
        <div class="field"><label>权重</label><input class="input" type="number" min="1" max="100" name="weight" value="${p?.weight || 1}"><span class="hint">同优先级内按权重分流</span></div>
      </div>
      <div class="field"><label>状态</label><select class="select" name="status">
        <option value="enabled" ${p?.status !== 'disabled' ? 'selected' : ''}>启用</option>
        <option value="disabled" ${p?.status === 'disabled' ? 'selected' : ''}>停用</option>
      </select></div>
      <div class="field"><label>类型</label><select class="select" name="type"><option value="openai">OpenAI 兼容</option></select></div>
      <div class="field span-2"><label>额外请求头（可选）</label>
        <textarea class="textarea mono-input" name="extra_headers" rows="2" placeholder="X-Org-Id: team-a">${esc(headersText)}</textarea>
        <span class="hint">每行一个 Header: Value，会附加到所有上游请求。</span>
      </div>
      <div class="span-2" id="provider-error"></div>
    </form>`,
    footer: `<button class="btn" data-act="cancel" type="button">取消</button>
      <button class="btn btn-primary" data-act="save" type="button">${isEdit ? '保存' : '创建'}</button>`,
    onMount: ({ el, close }) => {
      $('[data-act="cancel"]', el).addEventListener('click', close);
      $('[data-act="save"]', el).addEventListener('click', async () => {
        const fd = new FormData($('#provider-form', el));
        const rawKeys = String(fd.get('keys') || '');
        const keyLines = rawKeys.split('\n').map(s => s.trim()).filter(Boolean);
        const headers = {};
        String(fd.get('extra_headers') || '').split('\n').forEach(line => {
          const i = line.indexOf(':');
          if (i > 0) {
            const k = line.slice(0, i).trim();
            const v = line.slice(i + 1).trim();
            if (k) headers[k] = v;
          }
        });
        const body = {
          name: String(fd.get('name') || '').trim(),
          type: 'openai',
          base_url: String(fd.get('base_url') || '').trim(),
          timeout_sec: Number(fd.get('timeout_sec')) || 120,
          priority: Number(fd.get('priority')) || 0,
          weight: Number(fd.get('weight')) || 1,
          status: fd.get('status'),
          extra_headers: headers,
        };
        if (!isEdit || keyLines.length) body.keys = keyLines;
        const errEl = $('#provider-error', el);
        errEl.innerHTML = '';
        try {
          await api(isEdit ? '/api/admin/providers/' + p.id : '/api/admin/providers', {
            method: isEdit ? 'PATCH' : 'POST', body,
          });
          toast(isEdit ? '供应商已更新' : '供应商已创建', 'good');
          close(); onDone();
        } catch (err) {
          errEl.innerHTML = `<div class="form-error">${esc(err.message)}</div>`;
        }
      });
    },
  });
}

/* ══════════════════════════════════════════════════════════════════════════
   Models & routes
   ══════════════════════════════════════════════════════════════════════════ */
async function pageModels() {
  const [d, pd] = await Promise.all([api('/api/admin/models'), api('/api/admin/providers')]);
  const items = d.items || [];
  const providers = pd.items || [];
  state.models = items;
  state.providers = providers;
  view.innerHTML = `
    ${pageHead('模型与路由', '对外模型别名、价格与上游路由', `
      <button class="btn" id="model-refresh" type="button">${icon('refresh')}刷新</button>
      <button class="btn btn-primary" id="add-model" type="button">${icon('plus')}新建模型</button>`)}
    ${!providers.length ? `<div class="notice" style="margin-bottom:16px">${icon('alert')}还没有供应商，路由无法绑定。请先到 <a href="#/providers">供应商</a> 页面添加。</div>` : ''}
    <div style="display:flex;flex-direction:column;gap:14px" id="model-list">
      ${items.length ? items.map(m => modelCard(m, providers)).join('') : `<div class="card">${emptyState('还没有模型别名', '创建别名并绑定供应商后，客户端即可调用', 'models')}</div>`}
    </div>`;
  const reload = () => pageModels().catch(err => toast(err.message, 'bad'));
  $('#add-model').addEventListener('click', () => modelModal(null, reload));
  $('#model-refresh').addEventListener('click', reload);
  bindModelActions(items, providers, reload);
}

function modelCard(m, providers) {
  const routes = m.routes || [];
  return `<div class="card alias-card">
    <div class="alias-head">
      <div style="min-width:0">
        <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
          <span class="alias-name">${esc(m.alias)}</span>
          ${enabledPill(m.enabled)}
          ${routes.length ? '' : '<span class="pill warn">未配置路由</span>'}
        </div>
        <div class="alias-meta">
          <span>${esc(m.display_name || '未设置显示名')}</span>
          <span>输入 <b style="color:var(--ink-2)">¥${Number(m.input_price).toFixed(2)}</b> / 百万 tokens</span>
          <span>输出 <b style="color:var(--ink-2)">¥${Number(m.output_price).toFixed(2)}</b> / 百万 tokens</span>
          <span>${routes.length} 条路由</span>
          ${m.note ? `<span>${esc(m.note)}</span>` : ''}
        </div>
      </div>
      <div class="alias-actions">
        <button class="btn btn-sm" data-add-route="${esc(m.alias)}" type="button" ${providers.length ? '' : 'disabled'}>${icon('plus')}路由</button>
        <button class="btn btn-sm" data-edit-model="${esc(m.alias)}" type="button">编辑</button>
        <button class="btn btn-sm btn-danger" data-del-model="${esc(m.alias)}" type="button">删除</button>
      </div>
    </div>
    <div class="route-list">
      ${routes.length ? `<table class="route-table">
        <thead><tr><th>供应商</th><th>上游模型</th><th>优先级</th><th>权重</th><th>状态</th><th>操作</th></tr></thead>
        <tbody>${routes.map(r => `<tr>
          <td>${esc(r.provider_name)}</td>
          <td><span class="mono" style="font-size:11px;color:var(--ink)">${esc(r.upstream_model)}</span></td>
          <td>${r.priority}</td><td>${r.weight}</td>
          <td>${r.enabled ? '<span class="pill good">启用</span>' : '<span class="pill neutral">停用</span>'}</td>
          <td><div class="actions">
            <button class="btn btn-sm" data-edit-route="${r.id}" type="button">编辑</button>
            <button class="btn btn-sm btn-danger" data-del-route="${r.id}" type="button">删除</button>
          </div></td>
        </tr>`).join('')}</tbody>
      </table>` : `<div class="route-empty">该模型还没有上游路由，调用时会返回 503。点击右上角「路由」添加。</div>`}
    </div>
  </div>`;
}

function bindModelActions(items, providers, reload) {
  $$('[data-edit-model]').forEach(btn => btn.addEventListener('click', () => {
    const m = items.find(x => x.alias === btn.dataset.editModel);
    if (m) modelModal(m, reload);
  }));
  $$('[data-del-model]').forEach(btn => btn.addEventListener('click', async () => {
    const alias = btn.dataset.delModel;
    if (!await confirmDialog('删除模型', `确定删除模型「${alias}」吗？其所有路由与客户端访问权限将一并移除。`, { danger: true, confirmText: '删除' })) return;
    try {
      await api('/api/admin/models/' + encodeURIComponent(alias), { method: 'DELETE' });
      toast('模型已删除', 'good'); reload();
    } catch (err) { toast(err.message, 'bad'); }
  }));
  $$('[data-add-route]').forEach(btn => btn.addEventListener('click', () => {
    routeModal(null, btn.dataset.addRoute, providers, reload);
  }));
  items.forEach(m => (m.routes || []).forEach(r => {
    const editBtn = $(`[data-edit-route="${r.id}"]`);
    editBtn?.addEventListener('click', () => routeModal(r, m.alias, providers, reload));
    const delBtn = $(`[data-del-route="${r.id}"]`);
    delBtn?.addEventListener('click', async () => {
      if (!await confirmDialog('删除路由', `确定删除「${m.alias} → ${r.provider_name} / ${r.upstream_model}」这条路由吗？`, { danger: true, confirmText: '删除' })) return;
      try {
        await api('/api/admin/routes/' + r.id, { method: 'DELETE' });
        toast('路由已删除', 'good'); reload();
      } catch (err) { toast(err.message, 'bad'); }
    });
  }));
}

function modelModal(m, onDone) {
  const isEdit = !!m;
  openModal({
    title: isEdit ? '编辑模型' : '新建模型别名',
    subtitle: '客户端请求中的 model 字段使用这里的别名',
    body: `<form id="model-form" class="form-grid">
      <div class="field"><label>模型别名</label>
        <input class="input mono-input" name="alias" value="${esc(m?.alias || '')}" ${isEdit ? 'disabled' : 'required'} placeholder="例如：qwen2.5-7b">
        <span class="hint">对外暴露的模型 ID，创建后不可修改</span>
      </div>
      <div class="field"><label>显示名称</label><input class="input" name="display_name" value="${esc(m?.display_name || '')}" placeholder="例如：Qwen2.5 7B"></div>
      <div class="field"><label>输入价格（元 / 百万 tokens）</label><input class="input" type="number" min="0" step="0.01" name="input_price" value="${m?.input_price ?? 0}"></div>
      <div class="field"><label>输出价格（元 / 百万 tokens）</label><input class="input" type="number" min="0" step="0.01" name="output_price" value="${m?.output_price ?? 0}"></div>
      <div class="field"><label>启用</label><select class="select" name="enabled">
        <option value="1" ${m?.enabled !== false ? 'selected' : ''}>启用</option>
        <option value="0" ${m?.enabled === false ? 'selected' : ''}>停用</option>
      </select></div>
      <div class="field"><label>备注</label><input class="input" name="note" value="${esc(m?.note || '')}" placeholder="用途、负责人等"></div>
      <div class="span-2" id="model-error"></div>
    </form>`,
    footer: `<button class="btn" data-act="cancel" type="button">取消</button>
      <button class="btn btn-primary" data-act="save" type="button">${isEdit ? '保存' : '创建'}</button>`,
    onMount: ({ el, close }) => {
      $('[data-act="cancel"]', el).addEventListener('click', close);
      $('[data-act="save"]', el).addEventListener('click', async () => {
        const fd = new FormData($('#model-form', el));
        const body = {
          alias: String(fd.get('alias') || '').trim(),
          display_name: String(fd.get('display_name') || '').trim(),
          input_price: Number(fd.get('input_price')) || 0,
          output_price: Number(fd.get('output_price')) || 0,
          enabled: fd.get('enabled') === '1',
          note: String(fd.get('note') || '').trim(),
        };
        try {
          await api(isEdit ? '/api/admin/models/' + encodeURIComponent(m.alias) : '/api/admin/models', {
            method: isEdit ? 'PATCH' : 'POST', body,
          });
          toast(isEdit ? '模型已更新' : '模型已创建', 'good');
          close(); onDone();
        } catch (err) {
          $('#model-error', el).innerHTML = `<div class="form-error">${esc(err.message)}</div>`;
        }
      });
    },
  });
}

function routeModal(r, alias, providers, onDone) {
  const isEdit = !!r;
  openModal({
    title: isEdit ? '编辑路由' : '添加路由',
    subtitle: `模型 ${alias}`,
    body: `<form id="route-form" class="form-grid">
      <div class="field span-2"><label>供应商</label>
        <select class="select" name="provider_id" required>
          ${providers.map(p => `<option value="${p.id}" ${r?.provider_id === p.id ? 'selected' : ''}>${esc(p.name)} · ${esc(p.base_url)}</option>`).join('')}
        </select>
      </div>
      <div class="field span-2"><label>上游模型名称</label>
        <input class="input mono-input" name="upstream_model" value="${esc(r?.upstream_model || '')}" required placeholder="供应商实际接收的 model 值，例如 Qwen/Qwen2.5-7B-Instruct">
        <span class="hint">这是发送给上游的 model 字段，通常与本地推理服务加载的模型名一致。</span>
      </div>
      <div class="field"><label>优先级</label><input class="input" type="number" min="0" max="1000" name="priority" value="${r?.priority ?? 10}"><span class="hint">越小越先尝试</span></div>
      <div class="field"><label>权重</label><input class="input" type="number" min="1" max="100" name="weight" value="${r?.weight || 1}"><span class="hint">同优先级内按权重分流</span></div>
      <div class="field span-2"><label>状态</label><select class="select" name="enabled">
        <option value="1" ${r?.enabled !== false ? 'selected' : ''}>启用</option>
        <option value="0" ${r?.enabled === false ? 'selected' : ''}>停用</option>
      </select></div>
      <div class="span-2" id="route-error"></div>
    </form>`,
    footer: `<button class="btn" data-act="cancel" type="button">取消</button>
      <button class="btn btn-primary" data-act="save" type="button">${isEdit ? '保存' : '添加'}</button>`,
    onMount: ({ el, close }) => {
      $('[data-act="cancel"]', el).addEventListener('click', close);
      $('[data-act="save"]', el).addEventListener('click', async () => {
        const fd = new FormData($('#route-form', el));
        const body = {
          alias,
          provider_id: Number(fd.get('provider_id')),
          upstream_model: String(fd.get('upstream_model') || '').trim(),
          priority: Number(fd.get('priority')) || 0,
          weight: Number(fd.get('weight')) || 1,
          enabled: fd.get('enabled') === '1',
        };
        try {
          await api(isEdit ? '/api/admin/routes/' + r.id : '/api/admin/routes', {
            method: isEdit ? 'PATCH' : 'POST', body,
          });
          toast(isEdit ? '路由已更新' : '路由已添加', 'good');
          close(); onDone();
        } catch (err) {
          $('#route-error', el).innerHTML = `<div class="form-error">${esc(err.message)}</div>`;
        }
      });
    },
  });
}

/* ══════════════════════════════════════════════════════════════════════════
   Audit
   ══════════════════════════════════════════════════════════════════════════ */
async function pageAudit() {
  const load = async (page = 1) => {
    const d = await api(`/api/admin/audit?page=${page}&page_size=30`);
    const items = d.items || [];
    view.innerHTML = `
      ${pageHead('审计日志', '管理员的关键操作记录')}
      <div class="card">
        ${items.length ? `<div class="table-wrap"><table class="data-table">
          <thead><tr><th>时间</th><th>操作者</th><th>动作</th><th>对象</th><th>详情</th><th>来源 IP</th></tr></thead>
          <tbody>${items.map(a => `<tr>
            <td class="nowrap mono" style="font-size:11px">${fmtDateTime(a.ts)}</td>
            <td><span class="row-main">${esc(a.actor_name || '—')}</span></td>
            <td><span class="tag">${esc(a.action)}</span></td>
            <td class="mono" style="font-size:11px">${esc(a.target || '—')}</td>
            <td>${esc(a.detail || '—')}</td>
            <td class="mono" style="font-size:11px">${esc(a.ip || '—')}</td>
          </tr>`).join('')}</tbody>
        </table></div>${pagerHTML(d.total, d.page, d.page_size)}`
      : emptyState('还没有审计记录', '管理员操作会自动记录在这里', 'audit')}
      </div>`;
    bindPager(d.total, d.page, d.page_size, p => load(p).catch(err => toast(err.message, 'bad')));
  };
  await load(1);
}

/* ══════════════════════════════════════════════════════════════════════════
   Guide
   ══════════════════════════════════════════════════════════════════════════ */
async function pageGuide() {
  const d = await api('/api/my/models');
  const models = d.items || [];
  const base = location.origin + '/v1';
  const sampleModel = models[0]?.alias || 'your-model';
  const curl = `curl ${base}/chat/completions \\
  -H "Authorization: Bearer gw-你的密钥" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "${sampleModel}",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": false
  }'`;
  const python = `from openai import OpenAI

client = OpenAI(
    base_url="${base}",
    api_key="gw-你的密钥",
)

resp = client.chat.completions.create(
    model="${sampleModel}",
    messages=[{"role": "user", "content": "你好"}],
)
print(resp.choices[0].message.content)`;

  view.innerHTML = `
    ${pageHead('接入指南', '使用任意 OpenAI SDK 或 HTTP 客户端接入内网模型')}
    <div class="settings-grid">
      <div class="stack">
        <div class="card">
          <div class="card-head"><div><div class="card-title">接入信息</div><div class="card-sub">所有接口都挂在 /v1 下</div></div></div>
          <div class="card-body">
            <div class="section-label">Base URL</div>
            <div class="copy-row"><div class="code-block" style="padding:11px 14px">${esc(base)}</div>
              <button class="copy-btn" id="copy-base" type="button">${icon('copy')}复制</button></div>
            <div class="section-label" style="margin-top:20px">鉴权方式</div>
            <p class="ink2" style="font-size:12px;line-height:1.7">在请求头中携带 <span class="mono">Authorization: Bearer gw-...</span>。也可以使用 <span class="mono">X-Api-Key</span>。密钥在「API 密钥」页面创建，仅显示一次。</p>
            <div class="section-label" style="margin-top:20px">支持的端点</div>
            <div class="chips">
              <span class="tag">POST /v1/chat/completions</span>
              <span class="tag">POST /v1/completions</span>
              <span class="tag">POST /v1/embeddings</span>
              <span class="tag">GET /v1/models</span>
            </div>
          </div>
        </div>
        <div class="card">
          <div class="card-head"><div><div class="card-title">cURL 示例</div><div class="card-sub">适用于任何能发 HTTP 请求的环境</div></div>
            <button class="copy-btn" id="copy-curl" type="button">${icon('copy')}复制</button></div>
          <div class="card-body"><div class="code-block">${esc(curl)}</div></div>
        </div>
        <div class="card">
          <div class="card-head"><div><div class="card-title">Python（OpenAI SDK）</div><div class="card-sub">把 base_url 指向本网关即可</div></div>
            <button class="copy-btn" id="copy-python" type="button">${icon('copy')}复制</button></div>
          <div class="card-body"><div class="code-block">${esc(python)}</div></div>
        </div>
      </div>
      <div class="stack">
        <div class="card">
          <div class="card-head"><div><div class="card-title">可用模型</div><div class="card-sub">你的账号当前可以调用的模型</div></div></div>
          <div class="card-body tight">
            ${models.length ? `<div class="table-wrap"><table class="data-table">
              <thead><tr><th>模型 ID</th><th class="num">输入 / 输出</th></tr></thead>
              <tbody>${models.map(m => `<tr>
                <td><span class="mono" style="font-size:11.5px;color:var(--ink)">${esc(m.alias)}</span>
                  ${m.display_name ? `<br><span class="muted" style="font-size:10.5px">${esc(m.display_name)}</span>` : ''}</td>
                <td class="num nowrap">¥${Number(m.input_price).toFixed(2)} / ¥${Number(m.output_price).toFixed(2)}</td>
              </tr>`).join('')}</tbody>
            </table></div>` : emptyState('暂无可用模型', '请联系管理员配置模型与路由', 'models')}
          </div>
        </div>
        <div class="card">
          <div class="card-head"><div><div class="card-title">注意事项</div></div></div>
          <div class="card-body" style="font-size:12px;line-height:1.75;color:var(--ink-2)">
            <p>• 模型名使用「模型与路由」中配置的别名，而不是上游原始名称。</p>
            <p>• 支持 <span class="mono">stream: true</span> 的 SSE 流式输出，OpenAI SDK 默认即可。</p>
            <p>• 账号级与密钥级配额同时生效，达到 RPM / TPM / 每日或每月配额时返回 429，并带 <span class="mono">Retry-After</span>。</p>
            <p>• 请求 ID 会通过响应头 <span class="mono">X-Request-Id</span> 返回，可在调用日志中检索。</p>
            <p>• 网关只访问内网供应商地址，不会连接任何互联网服务。</p>
          </div>
        </div>
      </div>
    </div>`;
  $('#copy-base').addEventListener('click', () => copyText(base));
  $('#copy-curl').addEventListener('click', () => copyText(curl));
  $('#copy-python').addEventListener('click', () => copyText(python));
}

/* ══════════════════════════════════════════════════════════════════════════
   Settings
   ══════════════════════════════════════════════════════════════════════════ */
async function pageSettings() {
  if (!state.isAdmin) {
    view.innerHTML = `
      ${pageHead('账号设置', '修改当前账号的登录密码')}
      <div class="settings-grid">
        <div class="card">
          <div class="card-head"><div><div class="card-title">修改密码</div><div class="card-sub">修改后需要重新登录</div></div></div>
          <div class="card-body">
            <form class="form-grid" id="pw-form">
              <div class="field span-2"><label>当前密码</label><input class="input" type="password" name="old_password" required autocomplete="current-password"></div>
              <div class="field span-2"><label>新密码</label><input class="input" type="password" name="new_password" required minlength="8" autocomplete="new-password"><span class="hint">至少 8 位</span></div>
              <div class="field span-2"><label>确认新密码</label><input class="input" type="password" name="confirm" required autocomplete="new-password"></div>
              <div class="span-2" id="pw-error"></div>
              <div class="span-2"><button class="btn btn-primary" type="submit">保存并重新登录</button></div>
            </form>
          </div>
        </div>
        <div class="card">
          <div class="card-head"><div><div class="card-title">当前账号</div></div></div>
          <div class="card-body">
            <dl class="kv">
              <dt>用户名</dt><dd>${esc(state.me.username)}</dd>
              <dt>显示名</dt><dd>${esc(state.me.display_name || '—')}</dd>
              <dt>角色</dt><dd>${state.me.role === 'admin' ? '管理员' : '普通成员'}</dd>
              <dt>可用模型</dt><dd>${modelScope(state.me.allowed_models)}</dd>
              <dt>创建时间</dt><dd>${fmtDateTime(state.me.created_at)}</dd>
            </dl>
          </div>
        </div>
      </div>`;
    bindPasswordForm();
    return;
  }

  const [settings, sys] = await Promise.all([api('/api/admin/settings'), api('/api/admin/system')]);
  view.innerHTML = `
    ${pageHead('系统设置', '站点信息、日志保留与账号安全')}
    <div class="settings-grid">
      <div class="stack">
        <div class="card">
          <div class="card-head"><div><div class="card-title">站点</div><div class="card-sub">显示在登录页与侧边栏</div></div></div>
          <div class="card-body">
            <form class="form-grid" id="site-form">
              <div class="field"><label>站点名称</label><input class="input" name="site_name" value="${esc(settings.site_name)}" maxlength="60"></div>
              <div class="field"><label>日志保留天数</label><input class="input" type="number" min="1" max="3650" name="log_retention_days" value="${settings.log_retention_days}"><span class="hint">超期调用日志每小时清理一次；用量日汇总保留 13 个月。</span></div>
              <div class="span-2"><button class="btn btn-primary" type="submit">保存设置</button></div>
            </form>
          </div>
        </div>
        <div class="card">
          <div class="card-head"><div><div class="card-title">修改密码</div><div class="card-sub">修改后需要重新登录</div></div></div>
          <div class="card-body">
            <form class="form-grid" id="pw-form">
              <div class="field span-2"><label>当前密码</label><input class="input" type="password" name="old_password" required autocomplete="current-password"></div>
              <div class="field span-2"><label>新密码</label><input class="input" type="password" name="new_password" required minlength="8" autocomplete="new-password"></div>
              <div class="field span-2"><label>确认新密码</label><input class="input" type="password" name="confirm" required autocomplete="new-password"></div>
              <div class="span-2" id="pw-error"></div>
              <div class="span-2"><button class="btn" type="submit">修改密码</button></div>
            </form>
          </div>
        </div>
      </div>
      <div class="stack">
        <div class="card">
          <div class="card-head"><div><div class="card-title">系统信息</div><div class="card-sub">本机运行状态</div></div></div>
          <div class="card-body">
            <dl class="kv">
              <dt>版本</dt><dd class="mono">${esc(sys.version)}</dd>
              <dt>运行时长</dt><dd>${fmtDuration(sys.uptime_sec)}</dd>
              <dt>数据目录</dt><dd class="mono" style="font-size:11px">${esc(sys.data_dir)}</dd>
              <dt>用户 / 密钥</dt><dd>${fmtInt(sys.users)} / ${fmtInt(sys.keys)}</dd>
              <dt>供应商 / 模型</dt><dd>${fmtInt(sys.providers)} / ${fmtInt(sys.models)}</dd>
              <dt>日志</dt><dd>${fmtInt(sys.logs)} 条</dd>
            </dl>
          </div>
        </div>
        <div class="card">
          <div class="card-head"><div><div class="card-title">数据与备份</div></div></div>
          <div class="card-body" style="font-size:12px;line-height:1.75;color:var(--ink-2)">
            <p>• 全部数据保存在 <span class="mono">${esc(sys.data_dir)}/gateway.db</span>（SQLite）。</p>
            <p>• 上游密钥使用 <span class="mono">master.key</span> 加密；两者必须一起备份。</p>
            <p>• 停止服务后复制整个数据目录即可完成冷备份。</p>
            <p>• 网关不会向公网发起任何请求。</p>
          </div>
        </div>
      </div>
    </div>`;

  $('#site-form').addEventListener('submit', async e => {
    e.preventDefault();
    const fd = new FormData(e.target);
    try {
      await api('/api/admin/settings', {
        method: 'PATCH',
        body: {
          site_name: String(fd.get('site_name') || '').trim(),
          log_retention_days: Number(fd.get('log_retention_days')) || 90,
        },
      });
      state.siteName = String(fd.get('site_name') || '').trim();
      $('#brand-name').textContent = state.siteName;
      toast('设置已保存', 'good');
    } catch (err) { toast(err.message, 'bad'); }
  });
  bindPasswordForm();
}

function fmtDuration(sec) {
  sec = Number(sec) || 0;
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (d > 0) return `${d} 天 ${h} 小时`;
  if (h > 0) return `${h} 小时 ${m} 分钟`;
  return `${m} 分钟`;
}

function bindPasswordForm() {
  $('#pw-form')?.addEventListener('submit', async e => {
    e.preventDefault();
    const fd = new FormData(e.target);
    const errEl = $('#pw-error');
    if (errEl) errEl.innerHTML = '';
    const oldPw = String(fd.get('old_password') || '');
    const newPw = String(fd.get('new_password') || '');
    const confirm = String(fd.get('confirm') || '');
    if (newPw !== confirm) {
      if (errEl) errEl.innerHTML = '<div class="form-error">两次输入的新密码不一致</div>';
      return;
    }
    try {
      await api('/api/auth/password', {
        method: 'POST', body: { old_password: oldPw, new_password: newPw },
      });
      toast('密码已修改，请重新登录', 'good');
      state.me = null;
      setTimeout(showLogin, 600);
    } catch (err) {
      if (errEl) errEl.innerHTML = `<div class="form-error">${esc(err.message)}</div>`;
    }
  });
}

/* ══════════════════════════════════════════════════════════════════════════
   Router table & boot
   ══════════════════════════════════════════════════════════════════════════ */
const PAGES = {
  overview: pageOverview,
  logs: pageLogs,
  keys: pageKeys,
  users: pageUsers,
  models: pageModels,
  providers: pageProviders,
  audit: pageAudit,
  guide: pageGuide,
  settings: pageSettings,
};

window.addEventListener('hashchange', route);
document.addEventListener('DOMContentLoaded', boot);
