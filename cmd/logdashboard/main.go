package main

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/scopweb/mcp-filesystem-go-light/internal/dashboardapi"
	"github.com/scopweb/mcp-filesystem-go-light/internal/logview"
)

type dashboardData struct {
	Tool       string
	RequestID  string
	Addr       string
	Sort       string
	Limit      int
	RecentOff  int
	ErrorsOff  int
	RequestOff int
}

type requestDetailData struct {
	Tool       string
	RequestID  string
	Addr       string
	Sort       string
	Limit      int
	RequestOff int
	Page       dashboardapi.RequestPage
	PrevOffset int
	NextOffset int
	HasPrev    bool
	HasNext    bool
}

var dashboardTemplate = template.Must(template.New("dashboard").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MCP Log Dashboard</title>
  <style>
    :root { color-scheme: dark; --bg:#0f1117; --panel:#1a1d27; --panel-strong:#1f2330; --ink:#e1e4ed; --muted:#8b8fa3; --accent:#6c8aff; --accent-soft:rgba(108,138,255,.15); --error:#f87171; --error-soft:rgba(248,113,113,.15); --ok:#4ade80; --ok-soft:rgba(74,222,128,.15); --line:#2a2d3a; --shadow:0 12px 32px rgba(0,0,0,.28); }
    body { margin:0; font-family: "Segoe UI", Inter, system-ui, sans-serif; background:var(--bg); color:var(--ink); }
    main { width:min(100%, 1600px); margin:0 auto; padding:32px 24px 48px; box-sizing:border-box; }
    h1 { margin:0 0 8px; font-size:2.2rem; letter-spacing:-.03em; }
    h2 { margin:0 0 14px; font-size:1.15rem; letter-spacing:-.02em; }
    p { color:var(--muted); }
    .controls, .cards, .panel-stack, .request-search { display:grid; gap:16px; }
    .controls { grid-template-columns: minmax(280px, 1fr) minmax(140px, 180px) minmax(140px, 180px) auto; align-items:end; margin:24px 0 16px; }
    .request-search { grid-template-columns: minmax(280px, 1fr) auto; align-items:end; margin-bottom:16px; }
    .cards { grid-template-columns: repeat(auto-fit,minmax(180px,1fr)); margin-bottom:16px; }
    .panel-stack { grid-template-columns: 1fr; }
    .card, .panel { background:var(--panel); border:1px solid var(--line); border-radius:18px; box-shadow:var(--shadow); }
    .card { padding:18px; }
    .card span { display:block; font-size:11px; color:var(--muted); text-transform:uppercase; letter-spacing:.08em; }
    .card strong { display:block; font-size:1.95rem; margin-top:10px; font-variant-numeric:tabular-nums; }
    .panel { padding:18px; }
    label { display:block; font-size:.9rem; color:var(--muted); margin-bottom:8px; }
    input, select { width:100%; box-sizing:border-box; padding:12px 14px; border-radius:12px; border:1px solid var(--line); background:var(--bg); color:var(--ink); font:inherit; }
    input:focus, select:focus { outline:none; border-color:var(--accent); box-shadow:0 0 0 4px rgba(47,111,237,.12); }
    button { padding:12px 18px; border:0; border-radius:999px; background:var(--accent); color:#fff; font:inherit; font-weight:600; cursor:pointer; }
    button:disabled { opacity:.45; cursor:not-allowed; }
    .table-wrap { width:100%; overflow-x:auto; }
    table { width:100%; border-collapse:collapse; font-size:.95rem; min-width:1200px; }
    th, td { text-align:left; padding:12px 10px; border-bottom:1px solid var(--line); vertical-align:top; }
    th { color:var(--muted); font-size:11px; font-weight:700; text-transform:uppercase; letter-spacing:.08em; }
    tbody tr:hover td { background:rgba(108,138,255,.05); }
    .error { color:var(--error); }
    .mono { font-family: "Cascadia Mono", Consolas, monospace; font-size:.9rem; }
    .subops { color:var(--muted); font-size:.88rem; margin-top:6px; }
    .toolbar, .pager { display:flex; gap:12px; align-items:center; flex-wrap:wrap; }
    .pager button { padding:8px 14px; }
    .link { color:var(--accent); text-decoration:none; }
    .meta { color:var(--muted); font-size:.9rem; margin:10px 0 0; }
    .hero { margin-bottom:20px; }
    .lede { max-width:900px; margin:0; }
    .controls-panel { background:var(--panel-strong); border:1px solid var(--line); border-radius:18px; padding:18px; box-shadow:var(--shadow); margin-bottom:16px; }
    .section-head { display:flex; justify-content:space-between; align-items:baseline; gap:12px; margin-bottom:10px; }
    .section-head p { margin:0; font-size:.92rem; }
    .tabs { display:flex; gap:8px; flex-wrap:wrap; margin-bottom:18px; }
    .tab-button { padding:10px 14px; border-radius:999px; background:transparent; color:var(--muted); border:1px solid var(--line); }
    .tab-button.active { background:var(--accent-soft); color:var(--accent); border-color:rgba(108,138,255,.25); }
    .tab-panel { display:none; }
    .tab-panel.active { display:block; }
    .badge { display:inline-flex; align-items:center; gap:6px; padding:4px 10px; border-radius:999px; font-size:11px; font-weight:700; text-transform:uppercase; letter-spacing:.08em; white-space:nowrap; }
    .badge.ok { background:var(--ok-soft); color:var(--ok); }
    .badge.error { background:var(--error-soft); color:var(--error); }
    .badge.tool { background:var(--accent-soft); color:var(--accent); }
    .entry-stack { display:flex; flex-direction:column; gap:4px; }
    .path-cell { max-width:0; width:100%; }
    .path-pill { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
    .kv-list, .op-list { display:flex; flex-wrap:wrap; gap:6px; }
    .param-stack { display:grid; gap:8px; }
    .param-block { display:grid; gap:6px; }
    .param-label { color:var(--muted); font-size:11px; font-weight:700; text-transform:uppercase; letter-spacing:.08em; }
    .kv { display:inline-flex; gap:6px; align-items:center; max-width:320px; padding:4px 8px; border-radius:999px; background:var(--bg); border:1px solid var(--line); font-size:12px; }
    .kv .k { color:var(--muted); }
    .kv .v { color:var(--ink); font-family:"Cascadia Mono", Consolas, monospace; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
    .op-pill { display:inline-flex; align-items:center; padding:4px 8px; border-radius:999px; background:var(--accent-soft); color:var(--accent); font-size:12px; font-weight:600; }
    #request-trace { min-height: 120px; display:grid; gap:12px; }
    .trace-group { display:grid; gap:10px; }
    .trace-entry { border:1px solid var(--line); border-radius:14px; background:var(--panel-strong); padding:14px; }
    .trace-entry-head { display:flex; justify-content:space-between; gap:12px; align-items:flex-start; margin-bottom:8px; }
    .trace-entry-meta { display:flex; gap:8px; flex-wrap:wrap; align-items:center; }
    .trace-entry-title { font-weight:700; }
    .trace-entry-path { color:var(--muted); font-size:.92rem; word-break:break-word; }
    .trace-empty { color:var(--muted); padding:18px 2px; }
    .btn-detail { padding:6px 12px; border-radius:999px; background:var(--accent-soft); color:var(--accent); border:1px solid rgba(108,138,255,.25); font-size:12px; font-weight:700; }
    .modal-backdrop { position:fixed; inset:0; background:rgba(3,6,12,.72); display:none; align-items:center; justify-content:center; padding:24px; z-index:1000; }
    .modal-backdrop.open { display:flex; }
    .modal { width:min(980px, 100%); max-height:min(88vh, 900px); overflow:auto; background:var(--panel); border:1px solid var(--line); border-radius:20px; box-shadow:var(--shadow); }
    .modal-header { display:flex; justify-content:space-between; gap:16px; align-items:flex-start; padding:18px 20px; border-bottom:1px solid var(--line); position:sticky; top:0; background:var(--panel); }
    .modal-body { display:grid; gap:18px; padding:20px; }
    .modal-title { display:grid; gap:8px; }
    .modal-close { background:transparent; color:var(--muted); border:1px solid var(--line); padding:8px 12px; }
    .detail-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(220px,1fr)); gap:12px; }
    .detail-card { border:1px solid var(--line); border-radius:14px; background:var(--panel-strong); padding:14px; }
    .detail-card h3 { margin:0 0 10px; font-size:12px; color:var(--muted); text-transform:uppercase; letter-spacing:.08em; }
    .detail-full { color:var(--ink); font-family:"Cascadia Mono", Consolas, monospace; font-size:13px; word-break:break-word; white-space:pre-wrap; }
    @media (max-width: 960px) { .controls { grid-template-columns: 1fr 1fr; } }
    @media (max-width: 720px) { .controls, .request-search { grid-template-columns: 1fr; } main { padding:24px 16px 40px; } }
  </style>
</head>
<body>
  <main>
    <div class="hero">
      <h1>MCP Log Dashboard</h1>
      <p class="lede">Focused log view over operations.jsonl, metrics.json and optional proxy.jsonl. Inspired by the denser visual hierarchy of ultra, but kept intentionally smaller.</p>
    </div>
    <div class="controls-panel">
      <div class="controls">
        <div>
          <label for="tool">Tool filter</label>
          <input id="tool" placeholder="read_file, write_file, search..." value="{{.Tool}}">
        </div>
        <div>
          <label for="sort">Sort</label>
          <select id="sort">
            <option value="desc" {{if eq .Sort "desc"}}selected{{end}}>Newest first</option>
            <option value="asc" {{if eq .Sort "asc"}}selected{{end}}>Oldest first</option>
          </select>
        </div>
        <div>
          <label for="limit">Page size</label>
          <input id="limit" type="number" min="1" max="100" value="{{.Limit}}">
        </div>
        <button id="refresh">Refresh</button>
      </div>
      <div class="request-search">
        <div>
          <label for="request-id">Request ID lookup</label>
	          <input id="request-id" placeholder="req-42" value="{{.RequestID}}">
        </div>
        <button id="find-request">Find Request</button>
      </div>
    </div>
    <section class="cards" id="cards"></section>
    <section class="panel-stack">
      <div class="panel">
        <div class="tabs" role="tablist" aria-label="Log views">
          <button class="tab-button active" data-tab="recent" role="tab" aria-selected="true">Recent Operations</button>
          <button class="tab-button" data-tab="errors" role="tab" aria-selected="false">Recent Errors</button>
          <button class="tab-button" data-tab="request" role="tab" aria-selected="false">Request Trace</button>
        </div>

        <div id="tab-recent" class="tab-panel active" role="tabpanel">
          <div class="section-head"><h2>Recent Operations</h2><p>Latest tool calls with duration, normalized params and detail modal.</p></div>
          <div class="table-wrap">
            <table>
              <thead><tr><th>Time</th><th>Tool</th><th>Request Params</th><th>Executed</th><th>Status</th><th>Duration</th><th>View</th></tr></thead>
              <tbody id="recent"></tbody>
            </table>
          </div>
          <div id="recent-meta" class="meta"></div>
          <div class="pager">
            <button id="recent-prev">Previous</button>
            <button id="recent-next">Next</button>
          </div>
        </div>

        <div id="tab-errors" class="tab-panel" role="tabpanel">
          <div class="section-head"><h2>Recent Errors</h2><p>Only failed operations, kept separate for fast scanning.</p></div>
          <div class="table-wrap">
            <table>
              <thead><tr><th>Time</th><th>Tool</th><th>Error</th></tr></thead>
              <tbody id="errors"></tbody>
            </table>
          </div>
          <div id="errors-meta" class="meta"></div>
          <div class="pager">
            <button id="errors-prev">Previous</button>
            <button id="errors-next">Next</button>
          </div>
        </div>

        <div id="tab-request" class="tab-panel" role="tabpanel">
          <div class="section-head"><h2>Request Trace</h2><p>Correlated request entries rendered as individual events.</p></div>
          <div id="request-trace" class="mono">Enter a request_id to inspect correlated proxy/server entries.</div>
          <div id="request-meta" class="meta"></div>
          <div class="pager">
            <button id="request-prev">Previous</button>
            <button id="request-next">Next</button>
          </div>
          <p><a id="request-detail-link" class="link" href="#">Open detail page</a></p>
        </div>
      </div>
    </section>
    <div id="detail-modal-backdrop" class="modal-backdrop" aria-hidden="true">
      <div class="modal" role="dialog" aria-modal="true" aria-labelledby="detail-modal-title">
        <div class="modal-header">
          <div class="modal-title">
            <h2 id="detail-modal-title">Operation Detail</h2>
            <div id="detail-modal-subtitle" class="meta"></div>
          </div>
          <button id="detail-modal-close" class="modal-close">Close</button>
        </div>
        <div id="detail-modal-body" class="modal-body"></div>
      </div>
    </div>
  </main>
  <script>
    const toolInput = document.getElementById('tool');
    const sortInput = document.getElementById('sort');
    const limitInput = document.getElementById('limit');
    const refreshButton = document.getElementById('refresh');
    const requestInput = document.getElementById('request-id');
    const findRequestButton = document.getElementById('find-request');
    const requestDetailLink = document.getElementById('request-detail-link');
    const recentPrevButton = document.getElementById('recent-prev');
    const recentNextButton = document.getElementById('recent-next');
    const errorsPrevButton = document.getElementById('errors-prev');
    const errorsNextButton = document.getElementById('errors-next');
    const recentMeta = document.getElementById('recent-meta');
    const errorsMeta = document.getElementById('errors-meta');
    const requestMeta = document.getElementById('request-meta');
    const requestPrevButton = document.getElementById('request-prev');
    const requestNextButton = document.getElementById('request-next');
    const tabButtons = document.querySelectorAll('.tab-button');
    const tabPanels = document.querySelectorAll('.tab-panel');
    const detailModalBackdrop = document.getElementById('detail-modal-backdrop');
    const detailModalClose = document.getElementById('detail-modal-close');
    const detailModalBody = document.getElementById('detail-modal-body');
    const detailModalSubtitle = document.getElementById('detail-modal-subtitle');

    let recentOffset = {{.RecentOff}};
    let errorsOffset = {{.ErrorsOff}};
    let requestOffset = {{.RequestOff}};
    let recentEntries = [];

    function escapeHtml(value) {
      return String(value || '')
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
    }

    function formatTimestamp(value) {
      if (!value) {
        return '';
      }
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) {
        return value;
      }
      return date.toLocaleString();
    }

    function toolBadge(tool, kind) {
      const label = escapeHtml(tool || kind || 'unknown');
      return '<span class="badge tool">' + label + '</span>';
    }

    function statusBadge(status) {
      const normalized = (status || '').toLowerCase() === 'ok' ? 'ok' : 'error';
      return '<span class="badge ' + normalized + '">' + escapeHtml(status || normalized) + '</span>';
    }

    function renderPath(path) {
      if (!path) {
        return '<span class="path-pill">-</span>';
      }
      return '<span class="path-pill" title="' + escapeHtml(path) + '">' + escapeHtml(compactArgValue(path)) + '</span>';
    }

    function formatArgValue(value) {
      if (value === null || value === undefined) {
        return '-';
      }
      if (Array.isArray(value)) {
        return '[' + value.map(formatArgValue).join(', ') + ']';
      }
      if (typeof value === 'object') {
        return JSON.stringify(value);
      }
      return String(value);
    }

    function compactArgValue(value) {
      const text = formatArgValue(value);
      if (text.length <= 32) {
        return text;
      }
      return '...' + text.slice(-29);
    }

    function stableStringify(value) {
      if (Array.isArray(value)) {
        return '[' + value.map(stableStringify).join(',') + ']';
      }
      if (value && typeof value === 'object') {
        return '{' + Object.keys(value).sort().map(key => JSON.stringify(key) + ':' + stableStringify(value[key])).join(',') + '}';
      }
      return JSON.stringify(value);
    }

    function argsDiffer(rawArgs, normalizedArgs) {
      return stableStringify(rawArgs || {}) !== stableStringify(normalizedArgs || {});
    }

    function renderArgList(args) {
      const entries = Object.entries(args || {});
      if (!entries.length) {
        return '<span class="trace-empty">No parameters</span>';
      }
      return '<div class="kv-list">' + entries.map(([key, value]) => {
        const fullValue = formatArgValue(value);
        const compactValue = compactArgValue(value);
        return '<span class="kv" title="' + escapeHtml(key + ': ' + fullValue) + '"><span class="k">' + escapeHtml(key) + '</span><span class="v">' + escapeHtml(compactValue) + '</span></span>';
      }).join('') + '</div>';
    }

    function renderArgListFull(args) {
      const entries = Object.entries(args || {});
      if (!entries.length) {
        return '<span class="trace-empty">No parameters</span>';
      }
      return '<div class="kv-list">' + entries.map(([key, value]) => {
        const fullValue = formatArgValue(value);
        return '<span class="kv"><span class="k">' + escapeHtml(key) + '</span><span class="v">' + escapeHtml(fullValue) + '</span></span>';
      }).join('') + '</div>';
    }

    function renderRequestParams(entry) {
      const rawArgs = entry.args_raw || {};
      const normalizedArgs = entry.args_normalized || {};
      const hasRaw = Object.keys(rawArgs).length > 0;
      const hasNormalized = Object.keys(normalizedArgs).length > 0;

      if (!hasRaw && !hasNormalized) {
        return '<span class="trace-empty">No parameters</span>';
      }

      if (!hasRaw || !argsDiffer(rawArgs, normalizedArgs)) {
        return renderArgList(hasNormalized ? normalizedArgs : rawArgs);
      }

      let html = '<div class="param-stack">';
      html += '<div class="param-block"><span class="param-label">Raw</span>' + renderArgList(rawArgs) + '</div>';
      html += '<div class="param-block"><span class="param-label">Normalized</span>' + renderArgList(normalizedArgs) + '</div>';
      html += '</div>';
      return html;
    }

    function renderExecuted(entry) {
      const items = [];
      if (entry.internal_action) {
        items.push('<span class="op-pill">' + escapeHtml(entry.internal_action) + '</span>');
      }
      (entry.sub_operations || []).forEach(step => {
        items.push('<span class="op-pill">' + escapeHtml(step) + '</span>');
      });
      if (!items.length) {
        return '<span class="trace-empty">No internal ops</span>';
      }
      return '<div class="op-list">' + items.join('') + '</div>';
    }

    function renderExecutedFull(entry) {
      const items = [];
      if (entry.internal_action) {
        items.push('<span class="op-pill">' + escapeHtml(entry.internal_action) + '</span>');
      }
      (entry.sub_operations || []).forEach(step => {
        items.push('<span class="op-pill">' + escapeHtml(step) + '</span>');
      });
      if (!items.length) {
        return '<span class="trace-empty">No internal ops</span>';
      }
      return '<div class="op-list">' + items.join('') + '</div>';
    }

    function openDetailModal(index) {
      const entry = recentEntries[index];
      if (!entry) {
        return;
      }

      detailModalSubtitle.innerHTML = toolBadge(entry.tool, entry.kind) + ' ' + statusBadge(entry.status) + ' <span class="mono">' + escapeHtml(formatTimestamp(entry.ts)) + '</span>';

      let body = '<div class="detail-grid">';
      body += '<section class="detail-card"><h3>Request ID</h3><div class="detail-full">' + escapeHtml(entry.request_id || '-') + '</div></section>';
      body += '<section class="detail-card"><h3>Duration</h3><div class="detail-full">' + escapeHtml(String(entry.duration_ms || 0)) + 'ms</div></section>';
      body += '<section class="detail-card"><h3>Path</h3><div class="detail-full">' + escapeHtml(entry.path || '-') + '</div></section>';
      body += '<section class="detail-card"><h3>Error</h3><div class="detail-full">' + escapeHtml(entry.error || '-') + '</div></section>';
      body += '</div>';

      body += '<section class="detail-card"><h3>Executed</h3>' + renderExecutedFull(entry) + '</section>';

      const rawArgs = entry.args_raw || {};
      const normalizedArgs = entry.args_normalized || {};
      if (Object.keys(rawArgs).length || Object.keys(normalizedArgs).length) {
        body += '<div class="detail-grid">';
        if (Object.keys(rawArgs).length) {
          body += '<section class="detail-card"><h3>Raw Params</h3>' + renderArgListFull(rawArgs) + '</section>';
        }
        if (Object.keys(normalizedArgs).length) {
          body += '<section class="detail-card"><h3>Normalized Params</h3>' + renderArgListFull(normalizedArgs) + '</section>';
        }
        body += '</div>';
      }

      detailModalBody.innerHTML = body;
      detailModalBackdrop.classList.add('open');
      detailModalBackdrop.setAttribute('aria-hidden', 'false');
    }

    function closeDetailModal() {
      detailModalBackdrop.classList.remove('open');
      detailModalBackdrop.setAttribute('aria-hidden', 'true');
    }

    function activateTab(name) {
      tabButtons.forEach(button => {
        const isActive = button.dataset.tab === name;
        button.classList.toggle('active', isActive);
        button.setAttribute('aria-selected', isActive ? 'true' : 'false');
      });
      tabPanels.forEach(panel => {
        panel.classList.toggle('active', panel.id === 'tab-' + name);
      });
    }

    function renderTraceEntry(entry, source) {
      let html = '<article class="trace-entry">';
      html += '<div class="trace-entry-head">';
      html += '<div><div class="trace-entry-title">' + toolBadge(entry.tool, entry.kind) + '</div><div class="trace-entry-path">' + renderPath(entry.path || '') + '</div></div>';
      html += '<div class="trace-entry-meta">' + statusBadge(entry.status) + '<span class="mono">' + escapeHtml(formatTimestamp(entry.ts)) + '</span><span class="mono">' + escapeHtml(String(entry.duration_ms || 0)) + 'ms</span><span class="badge tool">' + escapeHtml(source) + '</span></div>';
      html += '</div>';
      if (entry.internal_action) {
        html += '<div>action=' + escapeHtml(entry.internal_action) + '</div>';
      }
      if (entry.sub_operations && entry.sub_operations.length) {
        html += '<div class="subops">sub_operations: ' + escapeHtml(entry.sub_operations.join(', ')) + '</div>';
      }
      if (entry.error) {
        html += '<div class="error">' + escapeHtml(entry.error) + '</div>';
      }
      html += '</article>';
      return html;
    }

    function updateUrlState() {
      const params = new URLSearchParams(window.location.search);
      const tool = toolInput.value.trim();
      const requestID = requestInput.value.trim();
      const sort = sortInput.value;
      const limit = limitInput.value.trim();
      if (tool) {
        params.set('tool', tool);
      } else {
        params.delete('tool');
      }
      if (requestID) {
        params.set('request_id', requestID);
      } else {
        params.delete('request_id');
      }
      params.set('sort', sort || 'desc');
      if (limit) {
        params.set('limit', limit);
      } else {
        params.delete('limit');
      }
      params.set('recent_offset', String(recentOffset));
      params.set('errors_offset', String(errorsOffset));
      params.set('request_offset', String(requestOffset));
      const query = params.toString();
      const nextUrl = query ? ('?' + query) : window.location.pathname;
      window.history.replaceState({}, '', nextUrl);
    }

    function currentParams() {
      return {
        tool: encodeURIComponent(toolInput.value.trim()),
        sort: encodeURIComponent(sortInput.value || 'desc'),
        limit: encodeURIComponent(limitInput.value.trim() || '12'),
      };
    }

    async function load() {
      updateUrlState();
      const params = currentParams();
      const summary = await fetch('/api/summary?tool=' + params.tool).then(r => r.json());
      const recent = await fetch('/api/recent?limit=' + params.limit + '&offset=' + recentOffset + '&sort=' + params.sort + '&tool=' + params.tool).then(r => r.json());
      const errors = await fetch('/api/errors?limit=' + params.limit + '&offset=' + errorsOffset + '&sort=' + params.sort + '&tool=' + params.tool).then(r => r.json());

      const cards = document.getElementById('cards');
      cards.innerHTML = '';
      const metrics = summary.metrics || {};
      [
        ['Total', metrics.ops_total || 0],
        ['OK', metrics.ops_ok || 0],
        ['Errors', metrics.ops_error || 0],
        ['Avg ms', (metrics.avg_duration_ms || 0).toFixed(2)],
      ].forEach(([label, value]) => {
        const div = document.createElement('div');
        div.className = 'card';
        div.innerHTML = '<span>' + label + '</span><strong>' + value + '</strong>';
        cards.appendChild(div);
      });

      const recentBody = document.getElementById('recent');
      recentBody.innerHTML = '';
      recentEntries = recent.items || [];
      (recent.items || []).forEach(entry => {
        const row = document.createElement('tr');
        const detailLink = entry.request_id ? '<a class="link" href="/request?id=' + encodeURIComponent(entry.request_id) + '&tool=' + encodeURIComponent(toolInput.value.trim()) + '">open</a>' : '';
        const index = recentEntries.indexOf(entry);
        row.innerHTML = '<td class="mono" title="' + escapeHtml(entry.ts || '') + '">' + escapeHtml(formatTimestamp(entry.ts)) + '</td><td><div class="entry-stack">' + toolBadge(entry.tool, entry.kind) + '<span>' + detailLink + '</span></div></td><td>' + renderRequestParams(entry) + '</td><td>' + renderExecuted(entry) + '</td><td>' + statusBadge(entry.status) + '</td><td class="mono">' + escapeHtml(String(entry.duration_ms || 0)) + 'ms</td><td><button class="btn-detail" data-index="' + index + '">View</button></td>';
        recentBody.appendChild(row);
      });
      updatePager(recentMeta, recentPrevButton, recentNextButton, recent);

      const errorsBody = document.getElementById('errors');
      errorsBody.innerHTML = '';
      (errors.items || []).forEach(entry => {
        const row = document.createElement('tr');
        const detailLink = entry.request_id ? '<a class="link" href="/request?id=' + encodeURIComponent(entry.request_id) + '&tool=' + encodeURIComponent(toolInput.value.trim()) + '">open</a>' : '';
        row.innerHTML = '<td class="mono" title="' + escapeHtml(entry.ts || '') + '">' + escapeHtml(formatTimestamp(entry.ts)) + '</td><td><div class="entry-stack">' + toolBadge(entry.tool, entry.kind) + '<span>' + detailLink + '</span></div></td><td class="error">' + escapeHtml(entry.error || '') + '</td>';
        errorsBody.appendChild(row);
      });
      updatePager(errorsMeta, errorsPrevButton, errorsNextButton, errors);

      if (requestInput.value.trim()) {
        await loadRequest();
      }
    }

    function updatePager(metaNode, prevButton, nextButton, page) {
      const total = page.total || 0;
      const offset = page.offset || 0;
      const limit = page.limit || Number(limitInput.value || 12);
      const items = page.items || [];
      const start = total === 0 ? 0 : offset + 1;
      const end = total === 0 ? 0 : offset + items.length;
      const pageNumber = total === 0 ? 0 : Math.floor(offset / limit) + 1;
      const totalPages = total === 0 ? 0 : Math.ceil(total / limit);
      metaNode.textContent = 'Showing ' + start + '-' + end + ' of ' + total + ' · page ' + pageNumber + ' of ' + totalPages;
      prevButton.disabled = offset <= 0;
      nextButton.disabled = offset + items.length >= total;
    }

    async function loadRequest() {
      const requestID = requestInput.value.trim();
      const trace = document.getElementById('request-trace');
      updateUrlState();
      requestDetailLink.href = requestID ? ('/request?id=' + encodeURIComponent(requestID) + '&tool=' + encodeURIComponent(toolInput.value.trim())) : '#';
      if (!requestID) {
	      trace.innerHTML = '<div class="trace-empty">Enter a request_id to inspect correlated proxy/server entries.</div>';
	      requestMeta.textContent = '';
	      requestPrevButton.disabled = true;
	      requestNextButton.disabled = true;
        return;
      }

      const params = currentParams();
      const payload = await fetch('/api/request?id=' + encodeURIComponent(requestID) + '&tool=' + params.tool + '&limit=' + params.limit + '&offset=' + requestOffset + '&sort=' + params.sort).then(r => r.json());
      const proxy = (payload.proxy && payload.proxy.items) || [];
      const server = (payload.server && payload.server.items) || [];

      if (!proxy.length && !server.length) {
	      trace.innerHTML = '<div class="trace-empty">No entries found for request_id ' + escapeHtml(requestID) + '</div>';
	      updateRequestPager(payload);
        return;
      }

      let html = '<div class="trace-group"><div><strong>request_id:</strong> ' + escapeHtml(payload.request_id) + '</div>';
      if (proxy.length) {
        html += '<h3>Proxy</h3>';
        proxy.forEach(entry => {
          html += renderTraceEntry(entry, 'proxy');
        });
      }
      if (server.length) {
        html += '<h3>Server</h3>';
        server.forEach(entry => {
          html += renderTraceEntry(entry, 'server');
        });
      }
      html += '</div>';
      trace.innerHTML = html;
      updateRequestPager(payload);
    }

    function updateRequestPager(payload) {
	    const server = payload.server || { total: 0, offset: 0, limit: Number(limitInput.value || 12), items: [] };
	    const proxy = payload.proxy || { total: 0, offset: 0, limit: Number(limitInput.value || 12), items: [] };
	    const total = Math.max(server.total || 0, proxy.total || 0);
	    const offset = Math.max(server.offset || 0, proxy.offset || 0);
	    const limit = Math.max(server.limit || 0, proxy.limit || 0, Number(limitInput.value || 12));
	    const currentServerEnd = (server.offset || 0) + ((server.items || []).length);
	    const currentProxyEnd = (proxy.offset || 0) + ((proxy.items || []).length);
	    const pageNumber = total === 0 ? 0 : Math.floor(offset / limit) + 1;
	    const totalPages = total === 0 ? 0 : Math.ceil(total / limit);
	    requestMeta.textContent = 'Server ' + ((server.total || 0) === 0 ? '0-0' : ((server.offset || 0) + 1) + '-' + currentServerEnd) + ' of ' + (server.total || 0) + ' · Proxy ' + ((proxy.total || 0) === 0 ? '0-0' : ((proxy.offset || 0) + 1) + '-' + currentProxyEnd) + ' of ' + (proxy.total || 0) + ' · page ' + pageNumber + ' of ' + totalPages;
	    requestPrevButton.disabled = offset <= 0;
	    requestNextButton.disabled = (offset + limit) >= total;
	  }

    refreshButton.addEventListener('click', load);
    sortInput.addEventListener('change', () => { recentOffset = 0; errorsOffset = 0; load(); });
    limitInput.addEventListener('change', () => { recentOffset = 0; errorsOffset = 0; requestOffset = 0; load(); });
    toolInput.addEventListener('keydown', event => { if (event.key === 'Enter') load(); });
    findRequestButton.addEventListener('click', loadRequest);
    requestInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadRequest(); });
    recentPrevButton.addEventListener('click', () => { recentOffset = Math.max(0, recentOffset - Number(limitInput.value || 12)); load(); });
    recentNextButton.addEventListener('click', () => { recentOffset += Number(limitInput.value || 12); load(); });
    errorsPrevButton.addEventListener('click', () => { errorsOffset = Math.max(0, errorsOffset - Number(limitInput.value || 12)); load(); });
    errorsNextButton.addEventListener('click', () => { errorsOffset += Number(limitInput.value || 12); load(); });
    requestPrevButton.addEventListener('click', () => { requestOffset = Math.max(0, requestOffset - Number(limitInput.value || 12)); loadRequest(); });
    requestNextButton.addEventListener('click', () => { requestOffset += Number(limitInput.value || 12); loadRequest(); });
    tabButtons.forEach(button => {
      button.addEventListener('click', () => activateTab(button.dataset.tab));
    });
    detailModalClose.addEventListener('click', closeDetailModal);
    detailModalBackdrop.addEventListener('click', event => { if (event.target === detailModalBackdrop) closeDetailModal(); });
    document.addEventListener('click', event => {
      const button = event.target.closest('.btn-detail[data-index]');
      if (!button) {
        return;
      }
      openDetailModal(Number(button.dataset.index));
    });
    document.addEventListener('keydown', event => { if (event.key === 'Escape') closeDetailModal(); });
    load();
  </script>
</body>
</html>`))

var requestTemplate = template.Must(template.New("request").Funcs(template.FuncMap{
	"requestSummary": requestSummary,
}).Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Request Detail</title>
  <style>
    :root { color-scheme: dark; --bg:#0f1117; --panel:#1a1d27; --panel-strong:#1f2330; --ink:#e1e4ed; --muted:#8b8fa3; --accent:#6c8aff; --accent-soft:rgba(108,138,255,.15); --error:#f87171; --error-soft:rgba(248,113,113,.15); --ok:#4ade80; --ok-soft:rgba(74,222,128,.15); --line:#2a2d3a; --shadow:0 12px 32px rgba(0,0,0,.28); }
    body { margin:0; font-family: "Segoe UI", Inter, system-ui, sans-serif; background:var(--bg); color:var(--ink); }
    main { max-width:1200px; margin:0 auto; padding:32px 20px 48px; }
    .panel { background:var(--panel); border:1px solid var(--line); border-radius:18px; box-shadow:var(--shadow); padding:18px; margin-bottom:16px; }
    .mono { font-family: "Cascadia Mono", Consolas, monospace; font-size:.9rem; }
    .error { color:var(--error); }
    a { color:var(--accent); text-decoration:none; }
    .meta { color:var(--muted); font-size:.9rem; margin:10px 0 0; }
    .pager { display:flex; gap:12px; align-items:center; flex-wrap:wrap; margin-top:12px; }
    .pager a { padding:8px 14px; border-radius:999px; background:var(--accent); color:#fff; }
    .pager .disabled { opacity:.45; pointer-events:none; }
    .entry { border:1px solid var(--line); border-radius:14px; background:var(--panel-strong); padding:14px; margin-top:12px; }
    .entry-head { display:flex; justify-content:space-between; gap:12px; flex-wrap:wrap; margin-bottom:8px; }
    .badge { display:inline-flex; align-items:center; gap:6px; padding:4px 10px; border-radius:999px; font-size:11px; font-weight:700; text-transform:uppercase; letter-spacing:.08em; white-space:nowrap; }
    .badge.ok { background:var(--ok-soft); color:var(--ok); }
    .badge.error { background:var(--error-soft); color:var(--error); }
    .badge.tool { background:var(--accent-soft); color:var(--accent); }
    .path { color:var(--muted); word-break:break-word; }
  </style>
</head>
<body>
  <main>
    <p><a href="/?tool={{.Tool}}&request_id={{.RequestID}}&sort={{.Sort}}&limit={{.Limit}}&request_offset={{.RequestOff}}">Back to dashboard</a></p>
    <h1>Request Detail</h1>
    <p class="mono">request_id={{.RequestID}}</p>
    <div class="panel">
      <h2>Proxy</h2>
      <div class="meta">{{requestSummary .Page.Proxy.Total .Page.Proxy.Offset .Page.Proxy.Limit .Page.Proxy.Items}}</div>
      {{if .Page.Proxy.Items}}{{range .Page.Proxy.Items}}
        <div class="entry">
          <div class="entry-head">
            <span class="badge tool">{{.Tool}}</span>
            <span class="badge {{if eq .Status "ok"}}ok{{else}}error{{end}}">{{.Status}}</span>
          </div>
          <div class="mono">{{.Timestamp}} · {{.DurationMs}}ms</div>
          <div class="path">{{.Path}}</div>
          {{if .Error}}<div class="error">{{.Error}}</div>{{end}}
        </div>
      {{end}}{{else}}<div class="mono">No proxy entries</div>{{end}}
    </div>
    <div class="panel">
      <h2>Server</h2>
      <div class="meta">{{requestSummary .Page.Server.Total .Page.Server.Offset .Page.Server.Limit .Page.Server.Items}}</div>
      {{if .Page.Server.Items}}{{range .Page.Server.Items}}
        <div class="entry">
          <div class="entry-head">
            <span class="badge tool">{{if .Tool}}{{.Tool}}{{else}}{{.Kind}}{{end}}</span>
            <span class="badge {{if eq .Status "ok"}}ok{{else}}error{{end}}">{{.Status}}</span>
          </div>
          <div class="mono">{{.Timestamp}} · {{.DurationMs}}ms</div>
          <div class="path">{{.Path}}</div>
          {{if .InternalAction}}<div>action={{.InternalAction}}</div>{{end}}
          {{if .SubOperations}}<div class="mono">sub_operations={{range $i, $v := .SubOperations}}{{if $i}}, {{end}}{{$v}}{{end}}</div>{{end}}
          {{if .Error}}<div class="error">{{.Error}}</div>{{end}}
        </div>
      {{end}}{{else}}<div class="mono">No server entries</div>{{end}}
      <div class="pager">
        <a class="{{if not .HasPrev}}disabled{{end}}" href="/request?id={{.RequestID}}&tool={{.Tool}}&sort={{.Sort}}&limit={{.Limit}}&offset={{.PrevOffset}}">Previous</a>
        <a class="{{if not .HasNext}}disabled{{end}}" href="/request?id={{.RequestID}}&tool={{.Tool}}&sort={{.Sort}}&limit={{.Limit}}&offset={{.NextOffset}}">Next</a>
      </div>
    </div>
  </main>
</body>
</html>`))

func main() {
	logDir := flag.String("log-dir", "", "Directory containing operations.jsonl and metrics.json")
	proxyLogDir := flag.String("proxy-log-dir", "", "Directory containing proxy.jsonl (optional)")
	addr := flag.String("addr", ":8091", "HTTP listen address")
	flag.Parse()

	if strings.TrimSpace(*logDir) == "" {
		panic("--log-dir is required")
	}

	handler := newDashboardHandler(*logDir, *proxyLogDir, *addr)

	fmt.Printf("dashboard listening on http://127.0.0.1%s\n", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		panic(err)
	}
}

func newDashboardHandler(logDir, proxyLogDir, addr string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		limit := dashboardapi.ReadLimit(r, 12)
		_ = dashboardTemplate.Execute(w, dashboardData{
			Tool:       strings.TrimSpace(r.URL.Query().Get("tool")),
			RequestID:  strings.TrimSpace(r.URL.Query().Get("request_id")),
			Addr:       addr,
			Sort:       dashboardapi.ReadSort(r),
			Limit:      limit,
			RecentOff:  dashboardapi.ReadOffset(r, "recent_offset"),
			ErrorsOff:  dashboardapi.ReadOffset(r, "errors_offset"),
			RequestOff: dashboardapi.ReadOffset(r, "request_offset"),
		})
	})
	mux.HandleFunc("/request", func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.URL.Query().Get("id"))
		tool := strings.TrimSpace(r.URL.Query().Get("tool"))
		limit := dashboardapi.ReadLimit(r, 12)
		offset := dashboardapi.ReadOffset(r, "offset")
		sortOrder := dashboardapi.ReadSort(r)
		entries := loadEntries(logDir, tool)
		proxyEntries, _ := logview.ReadProxyEntries(proxyLogDir)
		proxyEntries = logview.FilterProxyEntries(proxyEntries, tool)
		page := dashboardapi.BuildRequestPage(requestID, dashboardapi.FilterRequestEntries(entries, requestID), dashboardapi.FilterProxyRequestEntries(proxyEntries, requestID), sortOrder, offset, limit)
		total := page.Server.Total
		if page.Proxy.Total > total {
			total = page.Proxy.Total
		}
		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		nextOffset := offset + limit
		_ = requestTemplate.Execute(w, requestDetailData{
			Tool:       tool,
			RequestID:  requestID,
			Addr:       addr,
			Sort:       sortOrder,
			Limit:      limit,
			RequestOff: offset,
			Page:       page,
			PrevOffset: prevOffset,
			NextOffset: nextOffset,
			HasPrev:    offset > 0,
			HasNext:    nextOffset < total,
		})
	})
	mux.HandleFunc("/api/summary", func(w http.ResponseWriter, r *http.Request) {
		dashboardapi.RespondJSON(w, buildSummary(logDir, strings.TrimSpace(r.URL.Query().Get("tool"))))
	})
	mux.HandleFunc("/api/recent", func(w http.ResponseWriter, r *http.Request) {
		entries := loadEntries(logDir, strings.TrimSpace(r.URL.Query().Get("tool")))
		dashboardapi.RespondJSON(w, dashboardapi.BuildAuditPage(entries, dashboardapi.ReadSort(r), dashboardapi.ReadOffset(r, "offset"), dashboardapi.ReadLimit(r, 12)))
	})
	mux.HandleFunc("/api/errors", func(w http.ResponseWriter, r *http.Request) {
		entries := dashboardapi.FilterErrorEntries(loadEntries(logDir, strings.TrimSpace(r.URL.Query().Get("tool"))))
		dashboardapi.RespondJSON(w, dashboardapi.BuildAuditPage(entries, dashboardapi.ReadSort(r), dashboardapi.ReadOffset(r, "offset"), dashboardapi.ReadLimit(r, 12)))
	})
	mux.HandleFunc("/api/request", func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.URL.Query().Get("id"))
		tool := strings.TrimSpace(r.URL.Query().Get("tool"))
		limit := dashboardapi.ReadLimit(r, 12)
		offset := dashboardapi.ReadOffset(r, "offset")
		sortOrder := dashboardapi.ReadSort(r)
		entries := loadEntries(logDir, tool)
		proxyEntries, _ := logview.ReadProxyEntries(proxyLogDir)
		proxyEntries = logview.FilterProxyEntries(proxyEntries, tool)
		dashboardapi.RespondJSON(w, dashboardapi.BuildRequestPage(requestID, dashboardapi.FilterRequestEntries(entries, requestID), dashboardapi.FilterProxyRequestEntries(proxyEntries, requestID), sortOrder, offset, limit))
	})
	return mux
}

func buildSummary(logDir, tool string) map[string]interface{} {
	entries := loadEntries(logDir, tool)
	metrics := logview.BuildMetricsFromEntries(entries)
	if strings.TrimSpace(tool) == "" {
		if snapshot, err := logview.ReadMetrics(filepath.Join(logDir, "metrics.json")); err == nil {
			metrics = snapshot
		}
	}
	return map[string]interface{}{
		"metrics": metrics,
		"recent":  limitEntries(entries, 10),
	}
}

func loadEntries(logDir, tool string) []logview.AuditEntry {
	entries, _ := logview.ReadAuditEntries(filepath.Join(logDir, "operations.jsonl"))
	return logview.FilterAuditEntries(entries, tool)
}

func limitEntries(entries []logview.AuditEntry, limit int) []logview.AuditEntry {
	if limit < len(entries) {
		return entries[:limit]
	}
	return entries
}

func requestSummary(total, offset, limit int, items interface{}) string {
	count := 0
	switch value := items.(type) {
	case []logview.AuditEntry:
		count = len(value)
	case []logview.ProxyEntry:
		count = len(value)
	}
	return dashboardapi.PageSummary(total, offset, limit, count)
}
