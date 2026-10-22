package dashboard

// The document is assembled from four raw string parts so that each concern can
// be edited on its own: pageHead (document head), pageMarkup (the body), and
// pageStyles / pageScript (the single stylesheet and script).
const pageHead = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<link rel="icon" type="image/svg+xml" href="__BRAND_FAVICON__">
<title>__BRAND_NAME__</title>
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
      <span class="brand-mark" aria-hidden="true">__BRAND_MARK__</span>
      <h1>__BRAND_NAME__</h1>
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
    <button class="tab" type="button" role="tab" id="tab-stats" data-tab="stats" aria-controls="pane-stats" aria-selected="false">统计<span class="dot" hidden></span></button>
    <button class="tab" type="button" role="tab" id="tab-requests" data-tab="requests" aria-controls="pane-requests" aria-selected="false">请求日志<span class="dot" hidden></span></button>
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
        <div class="hint key-name-hint" id="keyNameHint">已有 Key 的名称可直接点击修改，回车或失焦生效；保存后会同步重命名对应的 CPA 认证文件。新增 Key 请在下方表单里命名。</div>
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
            <label class="field narrow"><span>优先级</span>
              <input type="number" id="keyPriority" min="0" step="1" value="1" inputmode="numeric">
            </label>
            <div class="field actions">
              <button id="addKey" class="small" type="button">添加</button>
            </div>
          </div>
          <div class="hint" id="keyFormHint">Key 只写入 CPA 的 config.yaml，本插件不落盘。优先级数值越大越优先调用。</div>
        </details>
      </div>

      <div class="card">
        <div class="card-head"><h2>宿主路由与优先级</h2></div>
        <div class="grid" id="rotationGrid"></div>
        <div class="hint" style="margin-top:10px">Key 选择、轮换与故障转移完全由 CPA 宿主接管，跟随 CPA「网络配置」中的路由策略。插件提供 Key 优先级（数字越大越优先，相同优先级按宿主策略负载均衡）。关闭的 Key 不参与调度。</div>
      </div>
    </section>

    <!-- ---------------------------------------------------- 统计 -->
    <section class="pane" id="pane-stats" role="tabpanel" aria-labelledby="tab-stats" hidden>
      <div id="statsError" class="notice err" hidden></div>
      <div id="statsPriceNotice" class="notice remind" hidden>
        <div class="notice-title"><strong>数据该更新了</strong><span class="pill warn" id="statsPriceNoticeBadge"></span></div>
        <div id="statsPriceNoticeText"></div>
        <div class="notice-actions">
          <a id="statsPriceNoticeLink" href="https://opencode.ai/docs/go/" target="_blank" rel="noopener">去官方价目表核对 ↗</a>
          <span class="hint">核对后更新插件内嵌价目表和模型快照，并发布新版本；这只是提醒，不代表上游已经调价。</span>
        </div>
      </div>
      <div id="statsWarnings" class="notice" hidden></div>

      <div class="stat-strip" id="statsSummary"></div>

      <div class="stats-plan-line">
        <span class="hint" id="statsPlanHint"></span>
        <button id="statsPlanBtn" class="ghost small" type="button">调整套餐</button>
      </div>
      <div class="stats-freshness" id="statsPriceFreshLine">
        <span class="pill" id="statsPriceBadge">价目表 —</span>
        <span class="hint" id="statsPriceFreshness">正在读取…</span>
        <a id="statsPriceLink" href="https://opencode.ai/docs/go/" target="_blank" rel="noopener">官方价目表 ↗</a>
      </div>

      <div class="card">
        <div class="card-head">
          <h2>模型用量</h2>
          <div class="toolbar-group">
            <span class="hint" id="statsRangeHint"></span>
            <select id="statsKey" aria-label="筛选 Key">
              <option value="">全部 Key</option>
            </select>
            <select id="statsRange" aria-label="统计范围">
              <option value="month">本月</option>
              <option value="all">累计</option>
            </select>
            <button id="refreshStats" class="small" type="button">刷新</button>
          </div>
        </div>

        <!-- The table is folded by default on phones so the quota/费用 summary
             is the first screenful; on desktop it starts open. -->
        <details class="fold-block" id="statsModelsFold" open>
          <summary><span>模型明细</span><span class="hint" id="statsModelsHint">正在读取…</span></summary>
          <div id="usageStatsTable" class="stats-table-wrap"></div>
        </details>

        <div class="stats-footnote hint" id="statsFootnote"></div>
      </div>

      <details class="card fold-card" id="statsAboutFold">
        <summary>统计口径与价目说明</summary>
        <ul class="stats-notes">
          <li><strong>数据来源</strong>：token、失败与请求总延迟来自宿主的 usage 事件；TTFT 由插件读取上游流时自己测量（宿主对本插件的异步流桥接不提供 TTFT）。</li>
          <li><strong>刊例价</strong>：按官方价目表把 input / output / cache read / cache write 折算成美元。chat、responses、gemini 的 input 含缓存，messages 的 input 不含缓存；gemini 的输出费用含 reasoning。金额是估算，不是 OpenCode 账单。</li>
          <li><strong>月额度</strong>：模型本月刊例价 ÷ 该模型在当前套餐下的月限额。Go 与 Go Plus 的月限额不同，默认 Go，可在「设置」里切换；免费模型显示「不限」，已退出 Go 列表的旧模型月限额未知。</li>
          <li><strong>本月窗口</strong>：按上游 <code>/usage</code> 返回的 monthly 重置时间往前推一个月；拿不到重置时间时按 UTC 自然月，并在提示里说明。多 Key 取最晚的重置时间。</li>
          <li><strong>平均 TPS（整体）</strong>：成功请求的有效 output token ÷ 宿主请求总延迟。分子和分母来自同一条 usage 记录，包含 TTFT、排队与网络时间，因此是端到端吞吐，不是纯解码速度；gemini 的 reasoning token 计入分子，失败请求不计。</li>
          <li><strong>生成 TPS</strong>：插件实测的「首个内容帧 → 最后一个内容帧」output token 速率，不含 TTFT，更接近解码速度。宿主 usage 没有可与 timing 配对的 request id，所以它是次要估算：分子覆盖所有成功流式 output，分母只覆盖有 generation 的样本，样本数见列；不建议单独拿它做严格 SLA。</li>
          <li><strong>平均 TTFT</strong>：插件从发起上游请求到第一段文字/思考/工具参数内容帧的时间，包含连接、排队与重试，不含 host 下游发送；只统计有实际内容且正常结束的流式请求，不做 first-frame 回退。</li>
          <li><strong>保留期</strong>：逐小时桶保留 40 天，累计总量永久保留；v1.7 起可把旧 Token Usage Tracker 中 opencode-go 的请求回放到本统计，回放范围以导入时的截止时间为准。</li>
          <li><strong>价目版本</strong>：<span id="statsPriceRevision">—</span>。费用按记录时内嵌的价目表计算；上游调价不会重算历史金额。</li>
          <li><strong>价目核对提醒</strong>：内嵌价目表和模型列表快照核对于 <span id="statsPriceCheckedAt">—</span>，按 30 天周期在「统计」页提醒；另外，「模型」页拉取的上游模型列表会与核对时快照对照，出现新增或移除模型时也会提示「数据该更新了」，不必等到 30 天。它只是提示去<a href="https://opencode.ai/docs/go/" target="_blank" rel="noopener">官方价目表</a>重新核对，不代表上游一定已经调价。核对后需要更新 <code>internal/pricing</code> 并发布新版本，历史金额不会自动重算。</li>
          <li><strong>隐私</strong>：统计文件只有模型 id、计数、费用与时间戳，不包含 Key、提示词或响应内容。</li>
        </ul>
      </details>
    </section>


    <!-- -------------------------------------------------- 请求日志 -->
    <section class="pane" id="pane-requests" role="tabpanel" aria-labelledby="tab-requests" hidden>
      <div id="requestsError" class="notice err" hidden></div>
      <div id="requestsWarnings" class="notice" hidden></div>

      <div class="stat-strip" id="requestSummary"></div>

      <div class="card">
        <div class="card-head">
          <h2>逐条请求</h2>
          <div class="toolbar-group">
            <span class="hint" id="requestsHint"></span>
            <select id="requestsFamily" aria-label="协议面筛选">
              <option value="">全部协议</option>
              <option value="chat">chat/completions</option>
              <option value="responses">responses</option>
              <option value="messages">messages</option>
              <option value="gemini">gemini</option>
            </select>
            <select id="requestsKey" aria-label="Key 筛选"><option value="">全部 Key</option></select>
            <label class="search field"><input type="search" id="requestsModel" placeholder="精确搜索模型 / 别名…" autocomplete="off" spellcheck="false"></label>
            <label class="switch"><input type="checkbox" id="requestsFailed"> <span class="hint">仅失败</span></label>
            <select id="requestsLimit" aria-label="条数">
              <option value="100">最近 100 条</option>
              <option value="200" selected>最近 200 条</option>
              <option value="500">最近 500 条</option>
              <option value="1000">最近 1000 条</option>
            </select>
            <label class="switch"><input type="checkbox" id="requestsAuto" checked> <span class="hint">自动刷新</span></label>
            <button id="refreshRequests" class="small" type="button">刷新</button>
          </div>
        </div>
        <div class="stats-table-wrap" id="requestTable"></div>
        <div class="hint" id="requestsFootnote">时间、TTFT 与两个 TPS 由插件在转发时实测；token 从上游响应帧解析。「统计」页仍以宿主 usage 事件为准，两者口径略有差异。不记录提示词、响应正文、Key 或原始会话 ID。</div>
      </div>

      <details class="card fold-card" id="requestsAboutFold">
        <summary>口径与隐私</summary>
        <ul class="stats-notes">
          <li><strong>生成 TPS</strong>：上游报告的 output token ÷「首个内容帧 → 最后一个内容帧」时长，不含首字，接近纯解码速度。</li>
          <li><strong>端到端 TPS</strong>：同一批 output token ÷ 本次请求总耗时（含建连、排队、首字与全部解码）。</li>
          <li><strong>TTFT</strong>：插件发出上游请求到第一个文字/思考/工具参数内容帧的时间；非流式请求与失败请求没有 TTFT。</li>
          <li><strong>Token</strong>：主列是输入 / 输出 / 合计；缓存读写、推理 token 在次级行显示。Gemini 的有效输出含 reasoning。</li>
          <li><strong>失败请求</strong>：仍记录状态码与错误摘要（截断），不计算 TPS；流内错误帧也会标红。</li>
          <li><strong>日志 ID</strong>：上游返回的 <code>x-opencode-log-id</code>，反馈问题时可直接提供给 OpenCode。</li>
          <li><strong>保留期</strong>：每个协议面最多 2000 条、7 天，超出后自动裁剪；每个面写自己的 <code>requests-&lt;role&gt;.jsonl</code>。</li>
          <li><strong>隐私</strong>：文件里只有模型 id、计数、耗时、状态与错误摘要，会话只存 8 位哈希，不含任何正文或密钥。</li>
          <li><strong>关闭</strong>：在「设置 → 状态与调试」关闭「记录请求日志」后，新请求不再写入。</li>
        </ul>
      </details>
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
        <div class="card-head">
          <h2>协议面</h2>
          <div class="toolbar-group"><span class="hint">四个面 · 一套配置</span></div>
        </div>
        <div class="sibling-grid" id="siblingGrid"></div>
        <div class="hint" style="margin-top:8px" id="faceHint">
          OpenCode Go 由一个主插件加三个协议面插件组成：每个 .so 只声明一种上游协议，CLIProxyAPI 才能把任意客户端协议翻译成它 —— 所以「插件管理」里会出现四行，其中只有 <code>cpa-opencodego</code> 需要配置。四个 .so 必须同版本安装；禁用或缺失任一面，该协议族的模型就无法路由。
        </div>
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
        <div class="card-head"><h2>协议与兼容</h2></div>
        <div class="grid">
          <label class="kv"><span class="k">端点布局</span>
            <select id="cfgEndpointLayout">
              <option value="auto">auto（按 base-url 自动判断）</option>
              <option value="legacy">legacy（旧 /v1 单根）</option>
              <option value="family">family（新 /openai、/anthropic、/google）</option>
            </select>
          </label>
          <label class="kv"><span class="k">developer 角色</span>
            <select id="cfgDeveloperRole">
              <option value="retry-only">retry-only（默认，被 400 拒绝时改写并重试）</option>
              <option value="rewrite">rewrite（主动改写为 system）</option>
              <option value="keep">keep（原样透传，不重试）</option>
            </select>
          </label>
          <label class="kv"><span class="k">reasoning 回放</span>
            <select id="cfgReasoningReplay">
              <option value="retry-only">retry-only（默认，被 400 拒绝时补齐并重试，记忆会话）</option>
              <option value="auto">auto（仅思考请求提前补齐）</option>
              <option value="always">always（所有 chat 请求提前补齐）</option>
              <option value="off">off（原样透传，不重试）</option>
            </select>
          </label>
        </div>
        <div class="hint" id="endpointHint" style="margin-top:8px"></div>
        <div class="hint">思考模型要求历史 assistant 消息回传 reasoning_content，客户端丢弃旧思维链时会触发上游 400。retry-only 默认原样透传，仅在收到 400 报错时用占位文本补齐并重试一次，且自动记忆该会话避免后续重复重试。</div>
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
        <div class="card-head"><h2>超时设置</h2></div>
        <div class="grid">
          <label class="kv"><span class="k">请求超时（秒）</span><input type="number" id="cfgTimeout" min="1" step="1" inputmode="numeric"></label>
        </div>
      </div>

      <div class="card">
        <div class="card-head"><h2>状态与调试</h2></div>
        <div class="grid">
          <label class="kv"><span class="k">状态目录（留空=自动）</span><input type="text" id="cfgStateDir" spellcheck="false"></label>
          <label class="switch" style="align-self:end"><input type="checkbox" id="cfgDebug"> <span class="hint">写调试日志</span></label>
          <label class="switch" style="align-self:end"><input type="checkbox" id="cfgRequestLog"> <span class="hint">记录请求日志</span></label>
        </div>
        <div class="hint" style="margin-top:8px" id="stateDirHint"></div>
      </div>

      <div class="card">
        <div class="card-head"><h2>统计口径</h2></div>
        <div class="grid">
          <label class="kv"><span class="k">OpenCode Go 套餐</span>
            <select id="cfgPlan">
              <option value="go">Go（$10/月）</option>
              <option value="go-plus">Go Plus（$40/月）</option>
            </select>
          </label>
        </div>
        <div class="hint" style="margin-top:8px">只影响统计 Tab 的逐模型月额度百分比；不会改变上游计费或额度。</div>
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
