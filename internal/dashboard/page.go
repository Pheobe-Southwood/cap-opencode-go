// Package dashboard renders the plugin page embedded in the CLIProxyAPI
// management centre.
//
// The page is served from an unauthenticated resource route, so it never
// renders API keys: configuration lives behind the host management API. The
// page reads the credential the management centre already stored for this
// browser, so the operator is never asked for a password; without it the page
// stays read-only.
//
// Layout notes: the plugin page is rendered in an iframe that the management
// centre places under its own fixed header, therefore
//   - the page fills the iframe with a flex column: a top action bar plus one
//     scrollable region. There is deliberately no bottom bar: on phones the
//     management centre can leave a strip of its own background under the
//     iframe, and a fixed bottom bar would float above that strip;
//   - the header height is measured from the parent document and applied as a
//     top inset, so the host's floating header never covers plugin content.
package dashboard

import (
	"strings"
)

// Page returns the dashboard document for a plugin id and role.
func Page(pluginID, role string) []byte {
	document := strings.ReplaceAll(pageTemplate, "__PLUGIN_ID__", jsString(pluginID))
	document = strings.ReplaceAll(document, "__ROLE__", jsString(role))
	return []byte(document)
}

// jsString escapes a value for embedding inside a JavaScript string literal.
func jsString(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "<", `\u003c`, ">", `\u003e`, "&", `\u0026`)
	return replacer.Replace(value)
}

const pageTemplate = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<title>OpenCode Go</title>
<style id="initial-theme">
  html{background:#faf9f5;color-scheme:light}
  html[data-theme='dark']{background:#151412;color-scheme:dark}
  html[data-theme='white']{background:#fff;color-scheme:light}
  @media (prefers-color-scheme: dark){
    html:not([data-theme]){background:#151412;color-scheme:dark}
  }
</style>
<style>
  /* Design tokens mirror the management centre so the page looks native.
     Light values live on :root, dark values on [data-theme='dark']; the theme
     attribute is copied from the parent document at runtime and kept in sync. */
  :root{
    --bg-primary:#f0eee8; --bg-secondary:#faf9f5; --bg-tertiary:#e9e6df; --bg-hover:#e9e6df;
    --text-primary:#2d2a26; --text-secondary:#6d6760; --text-tertiary:#a29c95;
    --border-color:#e3e1db; --border-primary:#d5d2cb;
    --primary-color:#8b8680; --primary-contrast:#fff;
    --success-color:#10b981; --warning-color:#c65746; --error-color:#c65746; --amber-color:#d97706;
    --radius-sm:6px; --radius-md:8px; --radius-lg:12px;
    --shadow:0 1px 2px 0 #00000014; --shadow-lg:0 10px 18px -3px #0000001a;
    --dur:0.18s;
    --host-header:0px;
  }
  html[data-theme='dark']{
    --bg-primary:#1d1b18; --bg-secondary:#151412; --bg-tertiary:#262320; --bg-hover:#2e2a26;
    --text-primary:#f6f4f1; --text-secondary:#c9c3bb; --text-tertiary:#9c958d;
    --border-color:#3a3530; --border-primary:#4a453f;
    --primary-color:#8b8680; --primary-contrast:#151412;
    --success-color:#10b981; --warning-color:#ff8a7a; --error-color:#ff8a7a; --amber-color:#ffd862;
    --shadow:0 1px 2px 0 #00000040; --shadow-lg:0 10px 18px -3px #00000059;
  }
  @media (prefers-color-scheme: dark){
    html:not([data-theme]){
      --bg-primary:#1d1b18; --bg-secondary:#151412; --bg-tertiary:#262320; --bg-hover:#2e2a26;
      --text-primary:#f6f4f1; --text-secondary:#c9c3bb; --text-tertiary:#9c958d;
      --border-color:#3a3530; --border-primary:#4a453f;
      --primary-contrast:#151412;
      --success-color:#10b981; --warning-color:#ff8a7a; --error-color:#ff8a7a; --amber-color:#ffd862;
      --shadow:0 1px 2px 0 #00000040; --shadow-lg:0 10px 18px -3px #00000059;
    }
  }

  *{box-sizing:border-box}
  html,body{margin:0;height:100%;overflow:hidden}
  body{
    background:var(--bg-secondary); color:var(--text-primary);
    font:13px/1.55 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;
    -webkit-text-size-adjust:100%;
  }
  /* The page fills the iframe: action bar on top, one scrolling body below and
     nothing anchored to the bottom edge. */
  .app{display:flex;flex-direction:column;height:100%;padding-top:var(--host-header,0px)}
  .toolbar{flex:0 0 auto;display:flex;gap:10px;align-items:center;flex-wrap:wrap;
    padding:9px clamp(12px,2.4vw,32px);border-bottom:1px solid var(--border-color);background:var(--bg-secondary)}
  .toolbar h1{font-size:16px;margin:0;font-weight:700;white-space:nowrap}
  .toolbar .grow{flex:1 1 auto;min-width:0}
  .toolbar-actions{display:flex;gap:8px;align-items:center;flex-wrap:wrap}
  .scroll{flex:1 1 auto;min-height:0;overflow-y:auto;-webkit-overflow-scrolling:touch;
    padding:12px clamp(12px,2.4vw,32px) calc(20px + env(safe-area-inset-bottom))}
  h2{font-size:14px;margin:0;font-weight:700}
  .sub{color:var(--text-secondary);font-size:12px}
  .hint{color:var(--text-tertiary);font-size:12px}
  .card{background:var(--bg-primary);border:1px solid var(--border-color);border-radius:var(--radius-lg);
    padding:14px;margin-top:12px;box-shadow:var(--shadow)}
  .card:first-child{margin-top:0}
  .card-head{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-bottom:12px;flex-wrap:wrap}
  details.card > summary{cursor:pointer;font-weight:700;list-style:none;display:flex;gap:8px;align-items:center}
  details.card > summary::-webkit-details-marker{display:none}
  details.card > summary::before{content:'▸';color:var(--text-tertiary);font-size:11px}
  details.card[open] > summary::before{content:'▾'}
  details.card[open] > summary{margin-bottom:12px}
  .grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(190px,1fr));gap:12px}
  .kv{display:flex;flex-direction:column;gap:3px;min-width:0}
  .kv .k{color:var(--text-tertiary);font-size:11px;text-transform:uppercase;letter-spacing:.06em}
  .kv .v{font-weight:600;word-break:break-all}
  button{font:inherit;cursor:pointer;border-radius:var(--radius-md);border:1px solid var(--border-primary);
    background:var(--bg-secondary);color:var(--text-primary);padding:8px 12px;font-weight:600;transition:background var(--dur)}
  button:hover{background:var(--bg-hover)}
  button.primary{background:var(--primary-color);border-color:var(--primary-color);color:var(--primary-contrast)}
  button.danger{color:var(--error-color);border-color:color-mix(in srgb,var(--error-color) 45%,transparent)}
  button.ghost{border-color:transparent;background:transparent;padding:6px 8px}
  button.small{padding:6px 10px;font-size:12px}
  button:disabled{opacity:.5;cursor:not-allowed}
  input,select{font:inherit;border-radius:var(--radius-md);border:1px solid var(--border-color);
    background:var(--bg-secondary);color:var(--text-primary);padding:8px 9px;min-width:0;max-width:100%}
  input[type=text],input[type=search],input[type=number],input[type=password]{width:100%}
  input:focus,select:focus,button:focus-visible{outline:2px solid color-mix(in srgb,var(--primary-color) 55%,transparent);outline-offset:1px}
  .pill{display:inline-block;padding:2px 8px;border-radius:999px;border:1px solid var(--border-color);font-size:11px;
    color:var(--text-secondary);white-space:nowrap}
  .pill.ok{color:var(--success-color);border-color:color-mix(in srgb,var(--success-color) 45%,transparent)}
  .pill.warn{color:var(--amber-color);border-color:color-mix(in srgb,var(--amber-color) 45%,transparent)}
  .pill.err{color:var(--error-color);border-color:color-mix(in srgb,var(--error-color) 45%,transparent)}
  .pill.accent{color:var(--text-primary);border-color:var(--border-primary);background:var(--bg-tertiary)}
  .switch{display:inline-flex;align-items:center;gap:7px;cursor:pointer;user-select:none;white-space:nowrap}
  .switch input{appearance:none;width:38px;height:22px;border-radius:999px;background:var(--bg-tertiary);
    position:relative;transition:background var(--dur);cursor:pointer;border:1px solid var(--border-color);padding:0;flex:0 0 auto}
  .switch input:checked{background:var(--success-color);border-color:var(--success-color)}
  .switch input::after{content:'';position:absolute;top:2px;left:2px;width:16px;height:16px;border-radius:50%;
    background:#fff;transition:left var(--dur)}
  .switch input:checked::after{left:18px}
  .bar{height:8px;border-radius:999px;background:var(--bg-tertiary);overflow:hidden;margin:6px 0 2px}
  .bar > span{display:block;height:100%;background:var(--success-color);transition:width var(--dur)}
  .bar.warn > span{background:var(--amber-color)}
  .bar.err > span{background:var(--error-color)}
  .quota-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(230px,1fr));gap:14px}
  .quota-key{border:1px solid var(--border-color);border-radius:var(--radius-md);padding:10px 12px;background:var(--bg-secondary)}
  .toolbar-group{display:flex;gap:8px;align-items:center;flex-wrap:wrap}
  .notice{border:1px solid color-mix(in srgb,var(--amber-color) 45%,transparent);background:color-mix(in srgb,var(--amber-color) 10%,transparent);
    border-radius:var(--radius-md);padding:10px 12px;margin-top:12px;color:var(--text-secondary);font-size:12px}
  .notice:first-child{margin-top:0}
  .err-text{color:var(--error-color)}
  .warn-text{color:var(--amber-color)}

  table{width:100%;border-collapse:collapse}
  th,td{text-align:left;padding:8px;border-bottom:1px solid var(--border-color);vertical-align:middle}
  th{color:var(--text-tertiary);font-size:11px;text-transform:uppercase;letter-spacing:.06em;font-weight:700}
  tbody tr:last-child td{border-bottom:0}
  code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px}

  /* ---- model list ---- */
  .filters{position:sticky;top:0;z-index:3;background:var(--bg-primary);padding:2px 0 10px;
    display:grid;grid-template-columns:minmax(0,2fr) repeat(2,minmax(0,1fr)) auto;gap:8px;align-items:center;
    border-bottom:1px solid var(--border-color);margin-bottom:10px}
  .filters .search{position:relative}
  .filters .bulk{display:flex;gap:8px;flex-wrap:wrap;justify-content:flex-end}
  .model-list{display:grid;gap:8px}
  .model-card{border:1px solid var(--border-color);border-radius:var(--radius-md);background:var(--bg-secondary);padding:9px 11px}
  .model-card.off{opacity:.62}
  .model-top{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
  .model-name{font-weight:700;word-break:break-all}
  .model-id{color:var(--text-tertiary);font-size:12px;word-break:break-all;margin-top:2px}
  .model-top .switch{margin-left:auto}
  .model-badges{display:flex;gap:6px;flex-wrap:wrap}
  .settings-panel{margin-top:10px;padding-top:10px;border-top:1px dashed var(--border-color);
    display:grid;grid-template-columns:minmax(0,2fr) repeat(2,minmax(0,1fr));gap:10px;align-items:end}
  .settings-panel .actions{display:flex;gap:8px;flex-wrap:wrap;align-items:center}
  .field{display:flex;flex-direction:column;gap:4px;min-width:0}
  .field > span{font-size:11px;color:var(--text-tertiary);text-transform:uppercase;letter-spacing:.05em}
  .model-note{margin-top:8px}
  .probe-result{margin-top:8px;display:flex;gap:8px;flex-wrap:wrap;align-items:center}
  .empty-state{border:1px dashed var(--border-color);border-radius:var(--radius-md);padding:18px;color:var(--text-secondary)}
  .sibling-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(230px,1fr));gap:10px}
  .sibling{border:1px solid var(--border-color);border-radius:var(--radius-md);padding:10px 12px;background:var(--bg-secondary)}
  .key-form{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,2fr) 90px auto;gap:8px;align-items:end;margin-bottom:12px}

  @media (max-width:900px){
    .filters{grid-template-columns:minmax(0,1fr) minmax(0,1fr);}
    .filters .search{grid-column:1/-1}
    .filters .bulk{grid-column:1/-1;justify-content:flex-start}
    .settings-panel{grid-template-columns:minmax(0,1fr) minmax(0,1fr)}
    .key-form{grid-template-columns:minmax(0,1fr) minmax(0,1fr)}
  }
  @media (max-width:640px){
    .toolbar{padding:8px 12px;gap:8px}
    .toolbar h1{font-size:15px}
    .toolbar .grow{order:3;flex:1 1 100%}
    .toolbar-actions{flex:1 1 100%;justify-content:space-between}
    .toolbar-actions button{flex:1 1 0}
    .scroll{padding:10px 12px calc(18px + env(safe-area-inset-bottom))}
    .card{padding:12px;border-radius:10px}
    .grid{grid-template-columns:minmax(0,1fr)}
    .model-top .switch{margin-left:0;order:9}
    .model-card .settings-toggle{margin-left:auto}
    .settings-panel{grid-template-columns:minmax(0,1fr)}
    .quota-grid{grid-template-columns:minmax(0,1fr)}
    .key-form{grid-template-columns:minmax(0,1fr)}
    table.responsive thead{display:none}
    table.responsive,table.responsive tbody,table.responsive tr,table.responsive td{display:block;width:100%}
    table.responsive tr{border:1px solid var(--border-color);border-radius:var(--radius-md);
      padding:10px 12px;margin-bottom:10px;background:var(--bg-secondary)}
    table.responsive td{border:0;padding:3px 0;display:flex;gap:10px;align-items:baseline}
    table.responsive td::before{content:attr(data-label);flex:0 0 74px;color:var(--text-tertiary);
      font-size:11px;text-transform:uppercase;letter-spacing:.05em}
  }
</style>
</head>
<body>
<div class="app">
  <div class="toolbar">
    <h1>OpenCode Go</h1>
    <span class="pill accent" id="saveState">加载中…</span>
    <span class="grow"></span>
    <div class="toolbar-actions">
      <button id="reload" type="button">重载</button>
      <button id="save" class="primary" type="button" disabled>保存</button>
    </div>
  </div>

  <div class="scroll" id="scrollRegion">
    <div id="credNotice" class="notice" style="display:none"></div>

    <details class="card" id="statusCard">
      <summary>运行状态<span class="hint" id="statusSummary"></span></summary>
      <div class="grid" id="statusGrid"></div>
      <div id="configError" class="err-text" style="margin-top:10px;display:none"></div>
    </details>

    <div class="card">
      <div class="card-head">
        <h2>模型与别名</h2>
        <div class="toolbar-group">
          <span class="hint" id="modelHint"></span>
          <button id="refreshModels" class="small" type="button">从上游刷新</button>
          <button id="collapseModels" class="small" type="button">收起全部设置</button>
        </div>
      </div>
      <div class="filters">
        <label class="search field"><input type="search" id="modelFilter" placeholder="搜索模型 ID / 名称 / 别名…" autocomplete="off"></label>
        <select id="familyFilter" aria-label="协议筛选">
          <option value="">全部协议</option>
          <option value="chat">chat/completions</option>
          <option value="responses">responses</option>
          <option value="messages">messages</option>
          <option value="gemini">gemini</option>
        </select>
        <select id="stateFilter" aria-label="状态筛选">
          <option value="">全部状态</option>
          <option value="served">本插件正在提供</option>
          <option value="disabled">已关闭</option>
          <option value="discovered">上游新增（未启用）</option>
          <option value="missing">上游已下线</option>
          <option value="alias">仅别名</option>
          <option value="inferred">协议为推测</option>
        </select>
        <div class="bulk">
          <button id="enableFiltered" class="small" type="button">启用筛选结果</button>
          <button id="disableFiltered" class="small" type="button">停用筛选结果</button>
        </div>
      </div>
      <div id="modelList" class="model-list"></div>
    </div>

    <div class="card">
      <div class="card-head">
        <h2>额度余量</h2>
        <div class="toolbar-group">
          <span class="hint" id="quotaHint"></span>
          <button id="refreshQuota" class="small" type="button">刷新</button>
        </div>
      </div>
      <div id="quotaBox" class="hint">加载中…</div>
    </div>

    <div class="card">
      <div class="card-head"><h2>Key 池（多账号）</h2></div>
      <div class="key-form">
        <label class="field"><span>名称</span><input type="text" id="keyName" placeholder="key-1" autocomplete="off"></label>
        <label class="field"><span>API Key</span><input type="password" id="keySecret" placeholder="oc_sk_…" autocomplete="off"></label>
        <label class="field"><span>权重</span><input type="number" id="keyWeight" min="1" value="1"></label>
        <button id="addKey" class="small" type="button">添加</button>
      </div>
      <table class="responsive">
        <thead><tr><th>名称</th><th>Key</th><th>权重</th><th>状态</th><th>健康</th><th></th></tr></thead>
        <tbody id="keyRows"></tbody>
      </table>
    </div>

    <div class="card">
      <div class="card-head"><h2>插件实例</h2></div>
      <div class="sibling-grid" id="siblingGrid"></div>
      <div class="hint" style="margin-top:8px">四个协议族各由一个独立插件实例承载；只有主实例（cap-opencode-go）有侧边栏页面与配置块。别名与协议归属只在主实例配置里维护。</div>
    </div>

    <details class="card">
      <summary>高级设置</summary>
      <div class="grid" style="margin-top:12px">
        <label class="kv"><span class="k">Base URL</span><input type="text" id="cfgBaseURL"></label>
        <label class="kv"><span class="k">Session 模式</span>
          <select id="cfgSessionMode"><option value="auto">auto（跟随 CPA 会话）</option><option value="fixed">fixed（固定值）</option></select>
        </label>
        <label class="kv"><span class="k">Session 值</span><input type="text" id="cfgSession"></label>
        <label class="kv"><span class="k">插件代理 URL（留空继承 CPA）</span><input type="text" id="cfgProxyURL" placeholder="http://127.0.0.1:7890 或 direct"></label>
        <label class="kv"><span class="k">默认代理模式</span>
          <select id="cfgDefaultProxy"><option value="inherit">inherit</option><option value="proxy">proxy</option><option value="direct">direct</option></select>
        </label>
        <label class="kv"><span class="k">请求超时（秒）</span><input type="number" id="cfgTimeout" min="1"></label>
        <label class="kv"><span class="k">Key 冷却（秒）</span><input type="number" id="cfgCooldown" min="0"></label>
        <label class="kv"><span class="k">失败重试次数</span><input type="number" id="cfgRetries" min="0"></label>
        <label class="kv"><span class="k">状态目录（留空=自动）</span><input type="text" id="cfgStateDir"></label>
        <label class="switch" style="align-self:end"><input type="checkbox" id="cfgDebug"> <span class="hint">写调试日志</span></label>
      </div>
    </details>
  </div>
</div>

<script>
(function(){
  'use strict';
  var PLUGIN_ID = "__PLUGIN_ID__";
  var ROLE = "__ROLE__";
  var FAMILIES = ['chat','responses','messages','gemini'];
  var FAMILY_LABEL = {chat:'chat/completions', responses:'responses', messages:'messages', gemini:'gemini'};
  var PROXY_MODES = ['inherit','proxy','direct'];
  var CRED_STORE = 'cli-proxy-auth';       // management centre credential store
  var CRED_LEGACY = 'managementKey';       // older management centre builds
  var READONLY_NOTICE = '本页只读：管理中心没有保存可用于写入的凭证。在 CPA 登录页勾选「记住密码」后再打开本页，保存按钮即可用（插件本身不保存任何密码）。';
  var AUTH_FAILED_NOTICE = '管理凭证已失效（管理中心重新登录过？）。刷新本页会自动重新读取，仍失败时请重新登录管理中心。';

  // ---------------------------------------------------------------- theme
  // The management centre owns the active theme on its own root element and
  // toggles it live, so mirror it and keep following changes.
  function applyTheme(theme){
    var root = document.documentElement;
    if(theme){ root.setAttribute('data-theme', theme); } else { root.removeAttribute('data-theme'); }
  }
  function syncTheme(){
    try{
      if(window.parent === window) return;
      applyTheme(window.parent.document.documentElement.getAttribute('data-theme'));
    }catch(error){ /* cross-origin: prefers-color-scheme takes over */ }
  }
  function watchParent(callback){
    try{
      if(window.parent === window) return;
      new MutationObserver(callback).observe(window.parent.document.documentElement,
        {attributes:true, attributeFilter:['data-theme','class','style']});
    }catch(error){ /* cross-origin: nothing to watch */ }
  }
  syncTheme();

  // ------------------------------------------------------- host geometry
  // The plugin page lives in an iframe placed under the management centre's
  // fixed header, so the visible area starts below it. The header height is
  // read from the parent document (same origin) and applied as a top inset;
  // when the parent is not reachable the inset stays 0 and the page simply
  // starts at the top of the iframe.
  function hostHeaderHeight(){
    try{
      if(window.parent === window) return 0;
      var parentDocument = window.parent.document;
      var header = parentDocument.querySelector('.main-header');
      if(header){
        var height = header.getBoundingClientRect().height;
        if(height > 0) return Math.round(height);
      }
      var raw = window.parent.getComputedStyle(parentDocument.documentElement).getPropertyValue('--header-height');
      var parsed = parseFloat(raw);
      if(parsed > 0) return Math.round(parsed);
    }catch(error){ /* cross-origin */ }
    return 0;
  }
  var lastHeader = -1;
  function syncHostMetrics(){
    var height = hostHeaderHeight();
    if(height === lastHeader) return;
    lastHeader = height;
    document.documentElement.style.setProperty('--host-header', height + 'px');
  }
  syncHostMetrics();
  watchParent(syncTheme);
  window.addEventListener('resize', syncHostMetrics);
  window.addEventListener('focus', syncHostMetrics);
  if(window.parent !== window){
    try{ window.parent.addEventListener('resize', syncHostMetrics); }catch(error){}
    try{ window.parent.addEventListener('scroll', syncHostMetrics, {passive:true}); }catch(error){}
  }

  // ----------------------------------------------------------------- bases
  function routeBases(){
    var marker = '/v0/resource/plugins/';
    var path = window.location.pathname;
    var index = path.indexOf(marker);
    var prefix = index >= 0 ? path.slice(0, index) : '';
    return { resource: prefix + marker + encodeURIComponent(PLUGIN_ID), management: prefix };
  }
  var BASES = routeBases();

  // ------------------------------------------------------ host credential
  // The management centre stores the credential it authenticated with in its
  // own browser storage. The plugin reads it from there so the operator never
  // types a password into this page; the plugin itself keeps no credentials.
  function deobfuscate(raw){
    var prefix = 'enc::v1::';
    if(!raw) return null;
    if(raw.indexOf(prefix) !== 0) return raw;
    try{
      var material = new TextEncoder().encode('cli-proxy-api-webui::secure-storage|' + window.location.host + '|' + navigator.userAgent);
      var binary = atob(raw.slice(prefix.length));
      var bytes = new Uint8Array(binary.length);
      for(var i=0;i<binary.length;i++){ bytes[i] = binary.charCodeAt(i) ^ material[i % material.length]; }
      return new TextDecoder().decode(bytes);
    }catch(error){ return null; }
  }
  function decodeJSON(raw){
    var decoded = deobfuscate(raw);
    if(!decoded) return null;
    try{ return JSON.parse(decoded); }catch(error){ return decoded; }
  }
  function hostCredential(){
    try{
      var stored = decodeJSON(localStorage.getItem(CRED_STORE));
      if(stored && typeof stored === 'object'){
        var state = stored.state && typeof stored.state === 'object' ? stored.state : stored;
        var key = state.managementKey;
        if(typeof key === 'string' && key.trim()) return key.trim();
      }
      var legacy = decodeJSON(localStorage.getItem(CRED_LEGACY));
      if(typeof legacy === 'string' && legacy.trim()) return legacy.trim();
    }catch(error){ /* storage unavailable */ }
    return null;
  }

  function api(path, options){
    options = options || {};
    var credential = hostCredential();
    if(!credential) return Promise.reject(new Error('no_credential'));
    var init = { method: options.method || 'GET', headers: { 'Authorization': 'Bearer ' + credential } };
    if(options.body !== undefined){
      init.headers['Content-Type'] = 'application/json';
      init.body = JSON.stringify(options.body);
    }
    var url = BASES.management + '/v8/management' + path;
    return fetch(url, init).then(function(response){
      if(response.status === 404 && !options.noFallback){
        return fetch(BASES.management + '/v0/management' + path, init);
      }
      return response;
    }).then(function(response){
      if(response.status === 401 || response.status === 403){
        setReadOnly(AUTH_FAILED_NOTICE);
        throw new Error('管理凭证被拒绝（HTTP ' + response.status + '）');
      }
      return response.text().then(function(body){
        if(!response.ok){ throw new Error('HTTP ' + response.status + ': ' + body.slice(0, 200)); }
        return body ? JSON.parse(body) : {};
      });
    });
  }

  function getConfig(){
    return api('/config/plugins/configs/' + encodeURIComponent(PLUGIN_ID)).catch(function(error){
      if(String(error.message).indexOf('404') >= 0) return {};
      throw error;
    });
  }
  function putConfig(values){
    return api('/config/plugins/configs/' + encodeURIComponent(PLUGIN_ID), { method:'PUT', body: values });
  }
  function probeModel(id){
    return api('/opencode-go/probe?model=' + encodeURIComponent(id));
  }

  // ---------------------------------------------------------------- state
  var config = {};
  var state = null;
  var upstream = null;
  var dirty = false;
  var readOnly = false;
  var configLoaded = false;
  var expanded = {};
  var probes = {};

  function el(id){ return document.getElementById(id); }
  function text(node, value){ if(node) node.textContent = value == null ? '' : String(value); }
  function esc(value){
    return String(value == null ? '' : value).replace(/[&<>"']/g, function(c){
      return ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c];
    });
  }
  function clone(value){ return JSON.parse(JSON.stringify(value)); }
  function models(){ if(!config.models || typeof config.models !== 'object') config.models = {}; return config.models; }
  function modelEntry(id){ var map = models(); if(!map[id]) map[id] = {}; return map[id]; }
  function keys(){ if(!Array.isArray(config.keys)) config.keys = []; return config.keys; }
  function fmtTime(value){
    if(!value) return '';
    var date = new Date(value);
    return isNaN(date.getTime()) ? '' : date.toLocaleString();
  }
  function shorten(value, keep){
    var trimmed = String(value == null ? '' : value);
    keep = keep || 22;
    if(trimmed.length <= keep * 2 + 3) return trimmed;
    return trimmed.slice(0, keep) + '…' + trimmed.slice(-keep);
  }

  // ------------------------------------------------------- toolbar state
  function setReadOnly(message){
    readOnly = true;
    renderCredNotice(message);
    renderToolbar();
  }
  function renderCredNotice(message){
    var node = el('credNotice');
    if(!node) return;
    if(!message){ node.style.display = 'none'; node.textContent = ''; return; }
    node.textContent = message;
    node.style.display = 'block';
  }
  function renderToolbar(){
    var chip = el('saveState');
    if(readOnly){
      text(chip, '只读');
      chip.className = 'pill warn';
    } else if(dirty){
      text(chip, '有未保存的修改');
      chip.className = 'pill warn';
    } else {
      text(chip, '已与配置同步');
      chip.className = 'pill ok';
    }
    el('save').disabled = readOnly || !dirty;
    el('save').title = readOnly ? '未取得管理中心凭证，无法保存' : '';
  }
  function markDirty(){
    dirty = true;
    renderToolbar();
  }
  function markClean(){
    dirty = false;
    renderToolbar();
  }

  function renderStatus(){
    var rows = [
      ['Provider', state.provider],
      ['插件 ID', state.plugin_id],
      ['版本', state.version],
      ['Base URL', state.base_url],
      ['Session', state.session_mode + (state.session_mode === 'fixed' ? ' / ' + state.session : '')],
      ['默认代理', state.default_proxy],
      ['插件代理', state.proxy_url || '(未设置)'],
      ['CPA 全局代理', state.host_proxy_url || '(未暴露)'],
      ['状态文件', state.state_path]
    ];
    el('statusGrid').innerHTML = rows.map(function(row){
      return '<div class="kv"><span class="k">' + esc(row[0]) + '</span><span class="v">' + esc(shorten(row[1] || '', 26)) + '</span></div>';
    }).join('');
    var served = (state.models || []).filter(function(m){ return m.served; }).length;
    var total = (state.models || []).length;
    text(el('statusSummary'), ' · ' + state.role + ' 族 · 提供 ' + served + ' / ' + total + ' 个模型 id');
    var errorBox = el('configError');
    if(state.config_error){ text(errorBox, state.config_error); errorBox.style.display = 'block'; }
    else { errorBox.style.display = 'none'; }
  }

  function renderKeys(){
    var rows = (state.keys || []).map(function(key){
      var health;
      if(key.cooldown_till && new Date(key.cooldown_till) > new Date()){
        health = '<span class="pill err">冷却至 ' + esc(fmtTime(key.cooldown_till)) + '</span>';
      } else if(key.failures > 0 && key.successes === 0){
        health = '<span class="pill warn">未成功</span>';
      } else {
        health = '<span class="pill ok">正常</span>';
      }
      if(key.last_error){ health += ' <span class="hint">' + esc(String(key.last_error).slice(0, 60)) + '</span>'; }
      return '<tr>' +
        '<td data-label="名称">' + esc(key.name) + '</td>' +
        '<td data-label="Key"><code>' + esc(key.masked || '(未配置)') + '</code></td>' +
        '<td data-label="权重">' + esc(key.weight) + '</td>' +
        '<td data-label="状态">' + (key.enabled ? '<span class="pill ok">启用</span>' : '<span class="pill">停用</span>') + '</td>' +
        '<td data-label="健康">' + health + '<div class="hint">成功 ' + key.successes + ' / 失败 ' + key.failures + '</div></td>' +
        '<td data-label=""><button class="danger small" type="button" data-key-name="' + esc(key.name) + '">移除</button></td>' +
      '</tr>';
    });
    el('keyRows').innerHTML = rows.length ? rows.join('') : '<tr><td colspan="6" class="hint">尚未配置 Key，上游调用会返回 401。</td></tr>';
    Array.prototype.forEach.call(el('keyRows').querySelectorAll('button[data-key-name]'), function(button){
      button.addEventListener('click', function(){
        var name = button.getAttribute('data-key-name');
        config.keys = keys().filter(function(item){ return item.name !== name; });
        markDirty(); renderKeys();
      });
    });
  }

  function modelStateOf(row){
    var entry = models()[row.id] || {};
    return {
      entry: entry,
      enabled: entry.enabled === undefined ? row.enabled : !!entry.enabled,
      family: entry.family || row.family,
      proxy: entry.proxy || row.proxy
    };
  }

  function matchesFilter(row, current){
    var filter = (el('modelFilter').value || '').trim().toLowerCase();
    var family = el('familyFilter').value;
    var stateFilter = el('stateFilter').value;
    if(family && current.family !== family) return false;
    if(filter){
      var haystack = (row.id + ' ' + (row.display_name || '') + ' ' + (row.aliases || []).join(' ') + ' ' + (row.upstream || '')).toLowerCase();
      if(haystack.indexOf(filter) < 0) return false;
    }
    switch(stateFilter){
      case 'served': return current.enabled && current.family === ROLE;
      case 'disabled': return !current.enabled;
      case 'discovered': return !!row.discovered;
      case 'missing': return row.upstream_available === false;
      case 'alias': return !!row.is_alias;
      case 'inferred': return !row.is_alias && row.family_source === 'inferred';
      default: return true;
    }
  }

  function filteredRows(){
    var all = (state && state.models) || [];
    return all.filter(function(row){ return matchesFilter(row, modelStateOf(row)); });
  }

  // Bulk assignment over the current filter result, so an operator can curate a
  // long model list without clicking every switch.
  function setEnabledForFiltered(enabled){
    var rows = filteredRows().filter(function(row){ return !row.is_alias; });
    if(!rows.length){ text(el('modelHint'), '当前筛选没有可操作的模型'); return; }
    rows.forEach(function(row){
      var entry = modelEntry(row.id);
      entry.enabled = enabled;
      if(!enabled){ return; }
      // A discovered id needs an explicit family before it can be served.
      if(row.discovered && !entry.family){ entry.family = el('familyFilter').value || row.family || 'chat'; }
    });
    markDirty();
    renderModels();
  }

  function badgeList(row, current){
    var badges = [];
    // The family pill is green only when this provider build actually serves the
    // model; a model owned by another protocol family is labelled as such.
    var servedHere = current.enabled && current.family === ROLE;
    var familyLabel = FAMILY_LABEL[current.family] || current.family;
    if(current.family !== ROLE) familyLabel += ' · 其他实例';
    badges.push('<span class="pill' + (servedHere ? ' ok' : '') + '">' + esc(familyLabel) + '</span>');
    if(!row.is_alias && row.family_source === 'inferred') badges.push('<span class="pill warn" title="按模型 ID 前缀推测，可用「探测协议」确认">协议推测</span>');
    if(row.discovered) badges.push('<span class="pill accent">上游新增</span>');
    if(row.is_alias) badges.push('<span class="pill accent">别名 → ' + esc(row.base_id || '') + '</span>');
    if(row.upstream_available === false) badges.push('<span class="pill err">上游已下线</span>');
    if(row.alias_shadowed) badges.push('<span class="pill err">别名被占用</span>');
    if(current.entry.aliases && current.entry.aliases.length) badges.push('<span class="pill accent">别名 ' + current.entry.aliases.length + '</span>');
    if(current.entry.proxy && current.entry.proxy !== 'inherit') badges.push('<span class="pill accent">' + esc(current.entry.proxy === 'proxy' ? '走代理' : '直连') + '</span>');
    if(row.note) badges.push('<span class="pill warn">注意</span>');
    return badges;
  }

  function renderProbe(row){
    var result = probes[row.id];
    if(!result) return '';
    if(result === 'loading') return '<div class="probe-result hint">正在逐个协议族探测…</div>';
    if(result.error) return '<div class="probe-result err-text">探测失败：' + esc(result.error) + '</div>';
    var items = (result.results || []).map(function(item){
      var pill = item.ok ? '<span class="pill ok">可用' + (item.status ? ' ' + item.status : '') + '</span>' : '<span class="pill">不可用</span>';
      var detail = item.error ? '<span class="hint">' + esc(String(item.error).slice(0, 90)) + '</span>' : '';
      var use = item.ok ? '<button class="small" type="button" data-probe-use="' + esc(row.id) + '" data-probe-family="' + esc(item.family) + '">使用 ' + esc(item.family) + '</button>' : '';
      return '<span class="probe-result">' + pill + '<code>' + esc(FAMILY_LABEL[item.family] || item.family) + '</code>' + detail + use + '</span>';
    });
    var summary = result.best ? '探测结果：' + esc(result.best) + ' 可用' : '探测结果：没有协议族接受这个模型 id';
    return '<div class="model-note hint">' + summary + '</div><div class="probe-result">' + items.join('') + '</div>';
  }

  function renderModelRow(row){
    var current = modelStateOf(row);
    var isOpen = !!expanded[row.id];
    var aliasValue = (current.entry.aliases || row.aliases || []).join(', ');
    var editable = !row.is_alias;
    var head = '<div class="model-top">' +
        '<span class="model-name">' + esc(row.display_name || row.id) + '</span>' +
        '<span class="model-badges">' + badgeList(row, current).join(' ') + '</span>' +
        '<label class="switch" title="是否由本插件提供该模型">' +
          '<input type="checkbox" data-model-enabled="' + esc(row.id) + '"' + (current.enabled ? ' checked' : '') + '>' +
          '<span class="hint">' + (current.enabled ? '启用' : '关闭') + '</span>' +
        '</label>' +
        (editable
          ? '<button class="ghost small settings-toggle" type="button" data-model-settings="' + esc(row.id) + '" aria-expanded="' + (isOpen ? 'true' : 'false') + '">' + (isOpen ? '收起设置' : '设置') + '</button>'
          : '') +
      '</div>' +
      '<div class="model-id">' + esc(row.id) + (row.upstream && row.upstream !== row.id ? ' → ' + esc(row.upstream) : '') + '</div>';

    var panel = '';
    if(editable && isOpen){
      panel = '<div class="settings-panel">' +
        '<label class="field"><span>别名（逗号分隔）</span>' +
          '<input type="text" data-model-alias="' + esc(row.id) + '" value="' + esc(aliasValue) + '" placeholder="例如 gpt6, luna" autocomplete="off">' +
        '</label>' +
        '<label class="field"><span>上游协议' + (row.family_source === 'inferred' ? '（推测）' : '') + '</span>' +
          '<select data-model-family="' + esc(row.id) + '">' +
          FAMILIES.map(function(f){
            return '<option value="' + f + '"' + (current.family === f ? ' selected' : '') + '>' + esc(FAMILY_LABEL[f]) + '</option>';
          }).join('') +
          '</select></label>' +
        '<label class="field"><span>代理</span>' +
          '<select data-model-proxy="' + esc(row.id) + '">' +
          PROXY_MODES.map(function(mode){
            return '<option value="' + mode + '"' + (current.proxy === mode ? ' selected' : '') + '>' + mode + '</option>';
          }).join('') +
          '</select></label>' +
        '<div class="field actions">' +
          '<button class="small" type="button" data-model-probe="' + esc(row.id) + '"' + (readOnly ? ' disabled' : '') + '>探测协议</button>' +
          '<button class="ghost small" type="button" data-model-reset="' + esc(row.id) + '">恢复默认</button>' +
        '</div>' +
      '</div>' + renderProbe(row);
    } else if(!editable){
      panel = '<div class="hint" style="margin-top:6px">别名在基础模型行上维护</div>';
    }

    return '<div class="model-card' + (current.enabled ? '' : ' off') + '" data-model-card="' + esc(row.id) + '">' +
      head + panel +
      (row.note ? '<div class="model-note hint warn-text">' + esc(row.note) + '</div>' : '') +
    '</div>';
  }

  function renderModels(){
    var all = (state && state.models) || [];
    var rows = filteredRows();

    if(!rows.length){
      el('modelList').innerHTML = '<div class="empty-state">没有匹配的模型。换个筛选条件，或点「从上游刷新」重新拉取模型列表。</div>';
      text(el('modelHint'), '显示 0 / 共 ' + all.length + ' 个 id');
      return;
    }

    el('modelList').innerHTML = rows.map(renderModelRow).join('');

    var served = all.filter(function(row){ var c = modelStateOf(row); return c.enabled && c.family === ROLE; }).length;
    var hint = '显示 ' + rows.length + ' / 共 ' + all.length + ' 个 id · 本实例提供 ' + served + ' 个';
    if(upstream && upstream.count){ hint += ' · 上游 ' + upstream.count + ' 个'; }
    if(upstream && upstream.error){ hint += ' · 上游拉取失败：' + upstream.error; }
    text(el('modelHint'), hint);

    Array.prototype.forEach.call(el('modelList').querySelectorAll('input[data-model-enabled]'), function(input){
      input.addEventListener('change', function(){
        var id = input.getAttribute('data-model-enabled');
        var row = all.filter(function(item){ return item.id === id; })[0] || {};
        var entry = modelEntry(id);
        entry.enabled = input.checked;
        if(row.discovered && input.checked && !entry.family){
          // A discovered id needs an explicit family before it can be served.
          entry.family = el('familyFilter').value || row.family || 'chat';
        }
        markDirty(); renderModels();
      });
    });
    Array.prototype.forEach.call(el('modelList').querySelectorAll('button[data-model-settings]'), function(button){
      button.addEventListener('click', function(){
        var id = button.getAttribute('data-model-settings');
        if(expanded[id]){ delete expanded[id]; } else { expanded[id] = true; }
        renderModels();
      });
    });
    Array.prototype.forEach.call(el('modelList').querySelectorAll('input[data-model-alias]'), function(input){
      input.addEventListener('change', function(){
        var id = input.getAttribute('data-model-alias');
        var aliases = input.value.split(',').map(function(part){ return part.trim(); }).filter(Boolean);
        if(aliases.length){ modelEntry(id).aliases = aliases; } else { delete modelEntry(id).aliases; }
        markDirty();
      });
    });
    Array.prototype.forEach.call(el('modelList').querySelectorAll('select[data-model-family]'), function(select){
      select.addEventListener('change', function(){
        modelEntry(select.getAttribute('data-model-family')).family = select.value;
        markDirty(); renderModels();
      });
    });
    Array.prototype.forEach.call(el('modelList').querySelectorAll('select[data-model-proxy]'), function(select){
      select.addEventListener('change', function(){
        modelEntry(select.getAttribute('data-model-proxy')).proxy = select.value;
        markDirty(); renderModels();
      });
    });
    Array.prototype.forEach.call(el('modelList').querySelectorAll('button[data-model-reset]'), function(button){
      button.addEventListener('click', function(){
        delete models()[button.getAttribute('data-model-reset')];
        markDirty(); renderModels();
      });
    });
    Array.prototype.forEach.call(el('modelList').querySelectorAll('button[data-model-probe]'), function(button){
      button.addEventListener('click', function(){
        var id = button.getAttribute('data-model-probe');
        probes[id] = 'loading';
        renderModels();
        probeModel(id).then(function(payload){
          probes[id] = payload || {};
        }).catch(function(error){
          probes[id] = { error: error.message === 'no_credential' ? '未取得管理中心凭证' : error.message };
        }).then(function(){ renderModels(); });
      });
    });
    Array.prototype.forEach.call(el('modelList').querySelectorAll('button[data-probe-use]'), function(button){
      button.addEventListener('click', function(){
        var id = button.getAttribute('data-probe-use');
        var family = button.getAttribute('data-probe-family');
        modelEntry(id).family = family;
        delete probes[id];
        markDirty(); renderModels();
      });
    });
  }

  function renderSiblings(){
    el('siblingGrid').innerHTML = (state.siblings || []).map(function(sibling){
      return '<div class="sibling">' +
        '<div class="kv"><span class="k">' + esc(sibling.role) + '</span><span class="v"><code>' + esc(sibling.plugin_id) + '</code></span></div>' +
        '<div class="hint" style="margin-top:6px">provider: ' + esc(sibling.provider) + '</div>' +
        '<div class="hint">由 plugins.configs.' + esc(sibling.plugin_id) + '.enabled 控制</div>' +
      '</div>';
    }).join('');
  }

  function renderAdvanced(){
    var fields = {
      cfgBaseURL: config['base-url'] || (state && state.base_url) || '',
      cfgSessionMode: config['session-mode'] || (state && state.session_mode) || 'auto',
      cfgSession: config['session'] || (state && state.session) || '',
      cfgProxyURL: config['proxy-url'] || '',
      cfgDefaultProxy: config['default-proxy'] || (state && state.default_proxy) || 'inherit',
      cfgTimeout: config['request-timeout-seconds'] || 900,
      cfgCooldown: config['key-cooldown-seconds'] === undefined ? 60 : config['key-cooldown-seconds'],
      cfgRetries: config['max-retries'] === undefined ? 2 : config['max-retries'],
      cfgStateDir: config['state-dir'] || ''
    };
    // The resolved values are written back into the working copy so that a save
    // can never replace a value the page has not managed to read yet (the model
    // list and the configuration arrive in two independent requests).
    config['base-url'] = fields.cfgBaseURL;
    config['session-mode'] = fields.cfgSessionMode;
    config['session'] = fields.cfgSession;
    config['default-proxy'] = fields.cfgDefaultProxy;
    Object.keys(fields).forEach(function(id){ var node = el(id); if(node) node.value = fields[id]; });
    el('cfgDebug').checked = !!config.debug;
  }

  function renderQuota(){
    el('quotaBox').innerHTML = '<span class="hint">加载中…</span>';
    fetch(BASES.resource + '/api/quota').then(function(response){ return response.json(); }).then(function(payload){
      var entries = payload.keys || [];
      if(!entries.length){ el('quotaBox').innerHTML = '<span class="hint">尚未配置 Key。</span>'; return; }
      el('quotaBox').innerHTML = '<div class="quota-grid">' + entries.map(function(entry){
        var head = '<div class="kv"><span class="k">' + esc(entry.key_name || entry.masked) + '</span><span class="v"><code>' + esc(entry.masked || '') + '</code></span></div>';
        if(entry.error){
          return '<div class="quota-key">' + head + '<div class="err-text" style="margin-top:6px">' + esc(entry.error) + '</div></div>';
        }
        return '<div class="quota-key">' + head + (entry.windows || []).map(function(window){
          var used = Math.max(0, Math.min(100, window.percent || 0));
          var level = used >= 90 ? 'err' : (used >= 70 ? 'warn' : '');
          return '<div><div class="bar ' + level + '"><span style="width:' + used + '%"></span></div>' +
            '<div class="hint">' + esc(window.label) + ' 已用 ' + used.toFixed(0) + '%' +
            (window.resets_at ? ' · 重置 ' + esc(fmtTime(window.resets_at)) : '') + '</div></div>';
        }).join('') + '</div>';
      }).join('') + '</div>';
      text(el('quotaHint'), '数据来自上游 /usage 接口，按 Key 统计');
    }).catch(function(error){
      el('quotaBox').innerHTML = '<span class="err-text">读取额度失败：' + esc(error.message) + '</span>';
    });
  }

  function collectConfig(){
    var next = clone(config);
    function value(id){ var node = el(id); return node ? node.value.trim() : ''; }
    // A field the operator never touched keeps the value the page read (or the
    // plugin's live value), never an empty string that would erase it.
    function resolved(id, fallback){
      var current = value(id);
      if(current !== '') return current;
      return fallback === undefined || fallback === null ? '' : String(fallback);
    }
    next['base-url'] = resolved('cfgBaseURL', config['base-url'] || (state && state.base_url));
    next['session-mode'] = resolved('cfgSessionMode', config['session-mode'] || 'auto');
    next['session'] = resolved('cfgSession', config['session'] || (state && state.session));
    next['proxy-url'] = resolved('cfgProxyURL', config['proxy-url']);
    next['default-proxy'] = resolved('cfgDefaultProxy', config['default-proxy'] || 'inherit');
    next['request-timeout-seconds'] = parseInt(value('cfgTimeout'), 10) || config['request-timeout-seconds'] || 900;
    next['key-cooldown-seconds'] = parseInt(value('cfgCooldown'), 10);
    if(isNaN(next['key-cooldown-seconds'])){
      next['key-cooldown-seconds'] = config['key-cooldown-seconds'] === undefined ? 60 : config['key-cooldown-seconds'];
    }
    next['max-retries'] = parseInt(value('cfgRetries'), 10);
    if(isNaN(next['max-retries'])) next['max-retries'] = config['max-retries'] === undefined ? 2 : config['max-retries'];
    next['state-dir'] = resolved('cfgStateDir', config['state-dir']);
    next.debug = el('cfgDebug').checked;
    next.enabled = true;
    Object.keys(next.models || {}).forEach(function(id){
      var entry = next.models[id];
      if(!entry || !Object.keys(entry).length){ delete next.models[id]; }
    });
    return next;
  }

  function loadConfig(){
    return getConfig().then(function(existing){
      config = existing && typeof existing === 'object' ? existing : {};
      // A save replaces the whole plugin configuration block, so it is only
      // allowed once the block has actually been read back from the host.
      configLoaded = Object.keys(config).length > 0;
      if(!config['base-url'] && state) config['base-url'] = state.base_url;
      renderKeys(); renderAdvanced(); markClean();
      renderModels();
    });
  }

  function loadModels(force){
    var url = BASES.resource + '/api/models' + (force ? '?refresh=1' : '');
    return fetch(url).then(function(response){ return response.json(); }).then(function(payload){
      state = payload;
      upstream = payload.upstream || null;
      renderStatus(); renderModels(); renderSiblings();
      // The configuration and the model list arrive independently; once the
      // plugin's own state is known, fill in anything the configuration did not
      // provide so a later save cannot write an empty value.
      renderAdvanced();
    }).catch(function(error){
      text(el('modelHint'), '模型列表加载失败：' + error.message);
    });
  }

  function load(){
    // Everything on this page that is not part of the write path comes from the
    // plugin's own read-only resource routes and needs no credential.
    readOnly = false;
    renderCredNotice('');
    loadModels(false);
    renderQuota();
    if(!hostCredential()){
      setReadOnly(READONLY_NOTICE);
      return;
    }
    loadConfig().catch(function(error){
      if(error.message === 'no_credential'){ setReadOnly(READONLY_NOTICE); return; }
      text(el('modelHint'), '读取配置失败：' + error.message);
    });
  }

  el('refreshQuota').addEventListener('click', renderQuota);
  el('enableFiltered').addEventListener('click', function(){ setEnabledForFiltered(true); });
  el('disableFiltered').addEventListener('click', function(){ setEnabledForFiltered(false); });
  el('refreshModels').addEventListener('click', function(){
    text(el('modelHint'), '正在从上游拉取…');
    loadModels(true);
  });
  el('collapseModels').addEventListener('click', function(){ expanded = {}; renderModels(); });
  el('reload').addEventListener('click', load);
  ['modelFilter','familyFilter','stateFilter'].forEach(function(id){
    el(id).addEventListener('input', renderModels);
    el(id).addEventListener('change', renderModels);
  });
  document.addEventListener('keydown', function(event){
    if(event.key !== '/' || event.metaKey || event.ctrlKey || event.altKey) return;
    var target = event.target || {};
    var tag = String(target.tagName || '').toLowerCase();
    if(tag === 'input' || tag === 'select' || tag === 'textarea') return;
    event.preventDefault();
    var filter = el('modelFilter');
    filter.focus();
    filter.select();
  });

  el('addKey').addEventListener('click', function(){
    var name = (el('keyName').value || '').trim();
    var secret = (el('keySecret').value || '').trim();
    var weight = parseInt(el('keyWeight').value, 10);
    if(!name){ text(el('modelHint'), 'Key 名称不能为空'); return; }
    if(!secret){ text(el('modelHint'), 'Key 内容不能为空'); return; }
    if(isNaN(weight) || weight < 1) weight = 1;
    keys().push({ name: name, key: secret, weight: weight, enabled: true });
    el('keyName').value = ''; el('keySecret').value = ''; el('keyWeight').value = '1';
    markDirty(); renderKeys();
  });

  el('save').addEventListener('click', function(){
    if(!configLoaded){
      text(el('modelHint'), '未能读取当前插件配置，已取消保存以免覆盖它（请重新载入本页后再试）');
      return;
    }
    var payload = collectConfig();
    putConfig(payload).then(function(){
      markClean();
      setTimeout(function(){ load(); }, 800);
    }).catch(function(error){
      if(error.message === 'no_credential'){ setReadOnly(READONLY_NOTICE); return; }
      text(el('modelHint'), '保存失败：' + error.message);
    });
  });

  // Follow the credential the management centre stores: once it is saved (for
  // example after ticking 记住密码 on the login page in another tab) this page
  // becomes writable without a reload.
  window.addEventListener('storage', function(event){
    if(event.key !== CRED_STORE && event.key !== CRED_LEGACY) return;
    if(readOnly && hostCredential()){ load(); }
  });

  renderToolbar();
  load();
})();
</script>
</body>
</html>
`
