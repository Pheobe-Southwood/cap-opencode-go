package dashboard

// The document is assembled from four raw string parts so that each concern can
// be edited on its own: pageHead (document head), pageMarkup (the body), and
// pageStyles / pageScript (the single stylesheet and script).
const pageHead = `<!doctype html>
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
<style>`

const pageHeadEnd = `</style>
</head>
<body>`

const pageFoot = `</body>
</html>
`

// pageMarkup is the page body. The shell is a flex column: a top action bar, a
// tab strip, and one scrolling region that holds the panes. Accounts comes
// first because the quota of every key is what the operator opens the page for.
const pageMarkup = `
<div class="app">
  <header class="toolbar">
    <div class="toolbar-title">
      <h1>OpenCode Go</h1>
      <span class="pill" id="saveState">加载中…</span>
    </div>
    <div class="toolbar-actions">
      <button id="reload" class="small" type="button" title="重新读取运行状态、模型与额度">重载</button>
      <button id="save" class="save-btn" type="button" disabled>
        <span id="saveLabel">保存</span><span class="count" id="saveCount" hidden>0</span>
      </button>
    </div>
  </header>

  <nav class="tabs" id="tabBar" role="tablist" aria-label="页面分区">
    <button class="tab" type="button" role="tab" id="tab-accounts" data-tab="accounts" aria-controls="pane-accounts" aria-selected="true">账号管理<span class="dot" hidden></span></button>
    <button class="tab" type="button" role="tab" id="tab-models" data-tab="models" aria-controls="pane-models" aria-selected="false">模型<span class="dot" hidden></span></button>
    <button class="tab" type="button" role="tab" id="tab-status" data-tab="status" aria-controls="pane-status" aria-selected="false">状态<span class="dot" hidden></span></button>
    <button class="tab" type="button" role="tab" id="tab-settings" data-tab="settings" aria-controls="pane-settings" aria-selected="false">设置<span class="dot" hidden></span></button>
  </nav>

  <main class="panes" id="panes">

    <!-- ------------------------------------------------ 账号管理 -->
    <section class="pane active" id="pane-accounts" role="tabpanel" aria-labelledby="tab-accounts">
      <div class="stat-strip" id="accountStats"></div>

      <div class="card">
        <div class="card-head">
          <h2>Key 余量</h2>
          <div class="toolbar-group">
            <span class="hint" id="quotaHint"></span>
            <button id="refreshQuota" class="small" type="button">刷新余量</button>
          </div>
        </div>
        <div id="keyList" class="key-list"></div>

        <details class="adder" id="keyAdder">
          <summary>添加 Key</summary>
          <div class="key-form">
            <label class="field"><span>名称</span>
              <input type="text" id="keyName" placeholder="key-2" autocomplete="off" spellcheck="false">
            </label>
            <label class="field"><span>API Key</span>
              <input type="password" id="keySecret" placeholder="oc_sk_…" autocomplete="off" spellcheck="false">
            </label>
            <label class="field narrow"><span>权重</span>
              <input type="number" id="keyWeight" min="1" step="1" value="1" inputmode="numeric">
            </label>
            <div class="field actions">
              <button id="addKey" class="small" type="button">添加</button>
            </div>
          </div>
          <div class="hint" id="keyFormHint">Key 只写入 CPA 的 config.yaml，本插件不落盘。</div>
        </details>
      </div>

      <div class="card">
        <div class="card-head"><h2>轮换与故障转移</h2></div>
        <div class="grid" id="rotationGrid"></div>
        <div class="hint" style="margin-top:10px">请求按权重在启用中的 Key 之间轮换；某个 Key 返回 401/429/5xx 时会被冷却，冷却期内其它 Key 接管。关闭的 Key 不参与轮换，也不会上报额度。</div>
      </div>
    </section>

    <!-- ---------------------------------------------------- 模型 -->
    <section class="pane" id="pane-models" role="tabpanel" aria-labelledby="tab-models" hidden>
      <div class="stat-strip" id="modelStats"></div>

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
          <label class="search field"><input type="search" id="modelFilter" placeholder="搜索模型 ID / 名称 / 别名…" autocomplete="off" spellcheck="false"></label>
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
            <option value="alias">已被别名替代</option>
            <option value="inferred">协议为推测</option>
            <option value="edited">本次已修改</option>
          </select>
          <div class="bulk">
            <button id="enableFiltered" class="small" type="button">启用筛选结果</button>
            <button id="disableFiltered" class="small" type="button">停用筛选结果</button>
          </div>
        </div>
        <div id="modelList" class="model-list"></div>
      </div>
    </section>

    <!-- ---------------------------------------------------- 状态 -->
    <section class="pane" id="pane-status" role="tabpanel" aria-labelledby="tab-status" hidden>
      <div id="statusError" class="notice err" hidden></div>

      <div class="card">
        <div class="card-head">
          <h2>运行状态</h2>
          <div class="toolbar-group"><span class="hint" id="statusSummary"></span></div>
        </div>
        <div class="grid" id="statusGrid"></div>
      </div>

      <div class="card">
        <div class="card-head">
          <h2>上游模型列表</h2>
          <div class="toolbar-group"><button id="refreshUpstream" class="small" type="button">重新拉取</button></div>
        </div>
        <div class="grid" id="upstreamGrid"></div>
      </div>

      <div class="card">
        <div class="card-head"><h2>插件实例</h2></div>
        <div class="sibling-grid" id="siblingGrid"></div>
        <div class="hint" style="margin-top:8px">四个协议族各由一个独立插件实例承载；只有主实例（cap-opencode-go）有侧边栏页面与配置块。别名与协议归属只在主实例配置里维护，开关在「插件管理」里。</div>
      </div>

      <div class="card">
        <div class="card-head"><h2>关于与帮助</h2></div>
        <div id="aboutBox" class="about"></div>
      </div>
    </section>

    <!-- ---------------------------------------------------- 设置 -->
    <section class="pane" id="pane-settings" role="tabpanel" aria-labelledby="tab-settings" hidden>
      <div class="notice" id="settingsNotice">
        这些字段写回 CPA 的 config.yaml，保存后热生效。留空的字段保持原值，不会清空。
      </div>

      <div class="card">
        <div class="card-head"><h2>连接</h2></div>
        <div class="grid">
          <label class="kv"><span class="k">Base URL</span><input type="text" id="cfgBaseURL" spellcheck="false"></label>
        </div>
      </div>

      <div class="card">
        <div class="card-head"><h2>会话</h2></div>
        <div class="grid">
          <label class="kv"><span class="k">Session 模式</span>
            <select id="cfgSessionMode"><option value="auto">auto（跟随 CPA 会话）</option><option value="fixed">fixed（固定值）</option></select>
          </label>
          <label class="kv"><span class="k">Session 值</span><input type="text" id="cfgSession" spellcheck="false"></label>
        </div>
        <div class="hint" style="margin-top:8px">auto 会把 CPA 的会话标识放进 x-opencode-session；fixed 始终使用固定值。</div>
      </div>

      <div class="card">
        <div class="card-head"><h2>代理</h2></div>
        <div class="grid">
          <label class="kv"><span class="k">插件代理 URL（留空继承 CPA）</span><input type="text" id="cfgProxyURL" placeholder="http://127.0.0.1:7890 或 direct" spellcheck="false"></label>
          <label class="kv"><span class="k">默认代理模式</span>
            <select id="cfgDefaultProxy"><option value="inherit">inherit（继承）</option><option value="proxy">proxy（强制走代理）</option><option value="direct">direct（强制直连）</option></select>
          </label>
        </div>
      </div>

      <div class="card">
        <div class="card-head"><h2>超时、重试与冷却</h2></div>
        <div class="grid">
          <label class="kv"><span class="k">请求超时（秒）</span><input type="number" id="cfgTimeout" min="1" step="1" inputmode="numeric"></label>
          <label class="kv"><span class="k">Key 冷却（秒）</span><input type="number" id="cfgCooldown" min="0" step="1" inputmode="numeric"></label>
          <label class="kv"><span class="k">失败重试次数</span><input type="number" id="cfgRetries" min="0" step="1" inputmode="numeric"></label>
        </div>
      </div>

      <div class="card">
        <div class="card-head"><h2>状态与调试</h2></div>
        <div class="grid">
          <label class="kv"><span class="k">状态目录（留空=自动）</span><input type="text" id="cfgStateDir" spellcheck="false"></label>
          <label class="switch" style="align-self:end"><input type="checkbox" id="cfgDebug"> <span class="hint">写调试日志</span></label>
        </div>
        <div class="hint" style="margin-top:8px" id="stateDirHint"></div>
      </div>

      <div class="card">
        <div class="card-head">
          <h2>待保存的修改</h2>
          <div class="toolbar-group"><button id="discard" class="small" type="button">放弃全部修改</button></div>
        </div>
        <div id="pendingBox"></div>
      </div>
    </section>
  </main>

  <div class="modal-backdrop" id="credModal" hidden>
    <div class="modal" role="dialog" aria-modal="true" aria-labelledby="credTitle">
      <h3 id="credTitle">需要管理中心密码</h3>
      <p class="hint" id="credHint"></p>
      <label class="field"><span>WebUI 密码（管理密钥）</span>
        <input type="password" id="credInput" autocomplete="current-password" placeholder="请输入管理密钥" spellcheck="false">
      </label>
      <label class="switch remember"><input type="checkbox" id="credRemember" checked> <span class="hint">在本标签页内记住（关闭标签页即失效）</span></label>
      <div class="err-text" id="credError" hidden></div>
      <div class="modal-actions">
        <button id="credCancel" type="button">取消</button>
        <button id="credSubmit" class="save-btn cta" type="button">保存</button>
      </div>
    </div>
  </div>

  <div class="toast" id="toast" hidden></div>
</div>
`
