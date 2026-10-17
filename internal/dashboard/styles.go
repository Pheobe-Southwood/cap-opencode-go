package dashboard

// pageStyles is the single stylesheet of the page. It mirrors the management
// centre design tokens so the page looks native in both themes.
const pageStyles = `
  /* Design tokens mirror the management centre so the page looks native.
     Light values live on :root, dark values on [data-theme='dark']; the theme
     attribute is copied from the parent document at runtime and kept in sync. */
  :root{
    --bg-primary:#f0eee8; --bg-secondary:#faf9f5; --bg-tertiary:#e9e6df; --bg-hover:#e9e6df;
    --text-primary:#2d2a26; --text-secondary:#6d6760; --text-tertiary:#a29c95;
    --border-color:#e3e1db; --border-primary:#d5d2cb;
    --primary-color:#8b8680; --primary-contrast:#fff;
    --cta-bg:#2d2a26; --cta-fg:#faf9f5; --cta-ring:#2d2a2629; --cta-shadow:#2d2a2645;
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
    --cta-bg:#f6f4f1; --cta-fg:#151412; --cta-ring:#f6f4f12e; --cta-shadow:#00000080;
    --success-color:#10b981; --warning-color:#ff8a7a; --error-color:#ff8a7a; --amber-color:#ffd862;
    --shadow:0 1px 2px 0 #00000040; --shadow-lg:0 10px 18px -3px #00000059;
  }
  @media (prefers-color-scheme: dark){
    html:not([data-theme]){
      --bg-primary:#1d1b18; --bg-secondary:#151412; --bg-tertiary:#262320; --bg-hover:#2e2a26;
      --text-primary:#f6f4f1; --text-secondary:#c9c3bb; --text-tertiary:#9c958d;
      --border-color:#3a3530; --border-primary:#4a453f;
      --primary-contrast:#151412;
      --cta-bg:#f6f4f1; --cta-fg:#151412; --cta-ring:#f6f4f12e; --cta-shadow:#00000080;
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
  /* The page fills the iframe: action bar and tab strip on top, one scrolling
     region below and nothing anchored to the bottom edge. */
  .app{position:relative;display:flex;flex-direction:column;height:100%;padding-top:var(--host-header,0px)}

  /* ------------------------------------------------------------ toolbar */
  .toolbar{flex:0 0 auto;display:flex;align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap;
    padding:9px clamp(12px,2.4vw,32px);border-bottom:1px solid var(--border-color);background:var(--bg-secondary)}
  .toolbar-title{display:flex;align-items:center;gap:8px;min-width:0}
  .toolbar h1{font-size:16px;margin:0;font-weight:700;white-space:nowrap}
  /* The mark the four plugin rows in 插件管理 also show (internal/brand). */
  .brand-mark{display:inline-flex;flex:0 0 auto;width:26px;height:26px;border-radius:7px;overflow:hidden;
    box-shadow:0 1px 2px var(--shadow)}
  .brand-mark svg{display:block;width:100%;height:100%}
  .toolbar-actions{display:flex;gap:8px;align-items:center}
  .save-btn{display:inline-flex;align-items:center;gap:6px;font:inherit;font-weight:700;cursor:pointer;
    border-radius:var(--radius-md);border:1px solid var(--border-primary);background:var(--bg-secondary);color:var(--text-secondary);
    padding:8px 14px;transition:background var(--dur),color var(--dur),box-shadow var(--dur),border-color var(--dur)}
  .save-btn:disabled{cursor:default;opacity:.5}
  .save-btn .count{display:inline-block;min-width:17px;padding:0 5px;border-radius:999px;font-size:11px;line-height:16px;
    background:color-mix(in srgb,var(--cta-fg) 24%,transparent)}
  .save-btn .count[hidden]{display:none}
  /* Lit up while the working copy differs from what is stored. */
  .save-btn.is-dirty{background:var(--cta-bg);border-color:var(--cta-bg);color:var(--cta-fg);
    box-shadow:0 0 0 3px var(--cta-ring),0 6px 14px -8px var(--cta-shadow);animation:cta-glow 1.9s ease-in-out infinite}
  .save-btn.is-dirty:hover,.save-btn.cta:hover{filter:brightness(1.06)}
  .save-btn.cta{background:var(--cta-bg);border-color:var(--cta-bg);color:var(--cta-fg)}
  @keyframes cta-glow{
    0%,100%{box-shadow:0 0 0 3px var(--cta-ring),0 6px 14px -8px var(--cta-shadow)}
    50%{box-shadow:0 0 0 5px var(--cta-ring),0 8px 18px -8px var(--cta-shadow)}
  }
  @media (prefers-reduced-motion: reduce){ .save-btn.is-dirty{animation:none} }

  /* --------------------------------------------------------------- tabs */
  .tabs{flex:0 0 auto;display:flex;gap:2px;padding:0 clamp(12px,2.4vw,32px);
    border-bottom:1px solid var(--border-color);background:var(--bg-secondary);
    overflow-x:auto;scrollbar-width:none;-webkit-overflow-scrolling:touch}
  .tabs::-webkit-scrollbar{display:none}
  .tab{flex:0 0 auto;display:inline-flex;align-items:center;gap:6px;font:inherit;font-weight:600;cursor:pointer;
    color:var(--text-secondary);background:transparent;border:0;border-bottom:2px solid transparent;border-radius:0;
    padding:10px 12px;min-height:42px;white-space:nowrap}
  .tab:hover{background:transparent;color:var(--text-primary)}
  .tab[aria-selected='true']{color:var(--text-primary);border-bottom-color:var(--text-primary)}
  .tab .dot{display:inline-block;width:6px;height:6px;border-radius:50%;background:var(--amber-color);flex:0 0 auto}
  .tab .dot[hidden]{display:none}

  /* -------------------------------------------------------------- panes */
  .panes{flex:1 1 auto;min-height:0;overflow-y:auto;-webkit-overflow-scrolling:touch;
    padding:12px clamp(12px,2.4vw,32px) calc(20px + env(safe-area-inset-bottom))}
  .pane{display:none}
  .pane.active{display:block}

  h2{font-size:14px;margin:0;font-weight:700}
  h3{font-size:14px;margin:0}
  .sub{color:var(--text-secondary);font-size:12px}
  .hint{color:var(--text-tertiary);font-size:12px}
  .card{background:var(--bg-primary);border:1px solid var(--border-color);border-radius:var(--radius-lg);
    padding:14px;margin-top:12px;box-shadow:var(--shadow)}
  .card:first-child{margin-top:0}
  .card-head{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-bottom:12px;flex-wrap:wrap}
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
  .switch.remember{margin-top:2px}
  .bar{height:8px;border-radius:999px;background:var(--bg-tertiary);overflow:hidden}
  .bar > span{display:block;height:100%;background:var(--success-color);transition:width var(--dur)}
  .bar.warn > span{background:var(--amber-color)}
  .bar.err > span{background:var(--error-color)}
  .toolbar-group{display:flex;gap:8px;align-items:center;flex-wrap:wrap}
  .notice{border:1px solid color-mix(in srgb,var(--amber-color) 45%,transparent);background:color-mix(in srgb,var(--amber-color) 10%,transparent);
    border-radius:var(--radius-md);padding:10px 12px;margin-top:12px;color:var(--text-secondary);font-size:12px}
  .notice:first-child{margin-top:0}
  .notice.err{border-color:color-mix(in srgb,var(--error-color) 45%,transparent);background:color-mix(in srgb,var(--error-color) 10%,transparent)}
  .err-text{color:var(--error-color)}
  .warn-text{color:var(--amber-color)}
  .ok-text{color:var(--success-color)}

  table{width:100%;border-collapse:collapse}
  th,td{text-align:left;padding:8px;border-bottom:1px solid var(--border-color);vertical-align:middle}
  th{color:var(--text-tertiary);font-size:11px;text-transform:uppercase;letter-spacing:.06em;font-weight:700}
  tbody tr:last-child td{border-bottom:0}
  code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px}

  /* ------------------------------------------------------- stat strips */
  .stat-strip{display:grid;grid-template-columns:repeat(auto-fit,minmax(118px,1fr));gap:8px;margin-bottom:12px}
  .stat{border:1px solid var(--border-color);border-radius:var(--radius-md);background:var(--bg-primary);
    padding:8px 10px;display:flex;flex-direction:column;gap:2px;text-align:left;min-width:0}
  .stat .k{color:var(--text-tertiary);font-size:11px;text-transform:uppercase;letter-spacing:.06em}
  .stat .v{font-weight:700;font-size:14px;word-break:break-all}
  button.stat{cursor:pointer;font-weight:400}
  button.stat:hover{background:var(--bg-hover)}
  button.stat[aria-pressed='true']{border-color:var(--border-primary);background:var(--bg-tertiary)}

  /* ------------------------------------------------------ accounts tab */
  .key-list{display:grid;gap:10px}
  .key-card{border:1px solid var(--border-color);border-radius:var(--radius-md);background:var(--bg-secondary);
    padding:10px 12px;display:flex;flex-direction:column;gap:9px}
  .key-card.is-off{opacity:.72}
  .key-card.is-new{border-style:dashed}
  .key-head{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
  .key-name{font-weight:700;word-break:break-all}
  .key-mask{color:var(--text-tertiary)}
  .key-head .spacer{flex:1 1 auto}
  .key-weight{display:inline-flex;align-items:center;gap:6px}
  .key-weight input{width:66px;padding:5px 7px}
  .key-quota{display:grid;grid-template-columns:repeat(auto-fit,minmax(198px,1fr));gap:10px}
  .quota-row{display:flex;flex-direction:column;gap:3px;min-width:0}
  .quota-top{display:flex;align-items:baseline;justify-content:space-between;gap:8px}
  .quota-top .label{color:var(--text-secondary);font-size:12px}
  .quota-top .value{font-weight:700;white-space:nowrap}
  .key-meta{display:flex;gap:10px;flex-wrap:wrap}
  .adder{margin-top:12px;border-top:1px dashed var(--border-color);padding-top:10px}
  .adder > summary{cursor:pointer;font-weight:700;list-style:none;display:flex;gap:8px;align-items:center}
  .adder > summary::-webkit-details-marker{display:none}
  .adder > summary::before{content:'＋';color:var(--text-tertiary);font-size:12px}
  .adder[open] > summary::before{content:'−'}
  .key-form{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1.6fr) 96px auto;gap:8px;align-items:end;margin-top:10px}
  .field{display:flex;flex-direction:column;gap:4px;min-width:0}
  .field > span{font-size:11px;color:var(--text-tertiary);text-transform:uppercase;letter-spacing:.05em}
  .field.actions{justify-content:flex-end}

  /* --------------------------------------------------------- model tab */
  .filters{position:sticky;top:0;z-index:3;background:var(--bg-primary);padding:2px 0 10px;
    display:grid;grid-template-columns:minmax(0,2fr) repeat(2,minmax(0,1fr)) auto;gap:8px;align-items:center;
    border-bottom:1px solid var(--border-color);margin-bottom:10px}
  .filters .search{position:relative}
  .filters .bulk{display:flex;gap:8px;flex-wrap:wrap;justify-content:flex-end}
  .model-list{display:grid;gap:8px}
  .model-card{border:1px solid var(--border-color);border-radius:var(--radius-md);background:var(--bg-secondary);padding:9px 11px}
  .model-card.off{opacity:.62}
  .model-card.edited{border-color:color-mix(in srgb,var(--amber-color) 55%,transparent)}
  .model-top{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
  .model-name{font-weight:700;word-break:break-all}
  .model-id{color:var(--text-tertiary);font-size:12px;word-break:break-all;margin-top:2px}
  .model-top .switch{margin-left:auto}
  .model-badges{display:flex;gap:6px;flex-wrap:wrap}
  .settings-panel{margin-top:10px;padding-top:10px;border-top:1px dashed var(--border-color);
    display:grid;grid-template-columns:minmax(0,2fr) repeat(2,minmax(0,1fr)) auto;gap:10px;align-items:end}
  .settings-panel .actions{display:flex;flex-direction:row;gap:8px;flex-wrap:wrap;align-items:center;justify-content:flex-end}
  .model-note{margin-top:8px}
  .probe-result{margin-top:8px;display:flex;gap:8px;flex-wrap:wrap;align-items:center}
  .empty-state{border:1px dashed var(--border-color);border-radius:var(--radius-md);padding:18px;color:var(--text-secondary)}
  .sibling-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:10px}
  .sibling{border:1px solid var(--border-color);border-radius:var(--radius-md);padding:10px 12px;background:var(--bg-secondary)}
  /* ------------------------------------------------------- protocol faces */
  .face{border:1px solid var(--border-color);border-radius:var(--radius-md);padding:10px 12px;
    background:var(--bg-secondary);display:flex;flex-direction:column;gap:6px;min-width:0}
  /* The face that owns the page and the configuration. */
  .face-main{border-color:var(--border-primary);background:var(--bg-tertiary)}
  .face-head{display:flex;align-items:center;justify-content:space-between;gap:8px;flex-wrap:wrap}
  .face-name{font-weight:700;font-size:13px}
  .face-tag{flex:0 0 auto;border:1px solid var(--border-color);border-radius:999px;padding:1px 7px;
    font-size:10px;letter-spacing:.04em;color:var(--text-tertiary);background:var(--bg-primary)}
  .face-models{display:flex;flex-wrap:wrap;gap:4px}
  .chip{display:inline-block;max-width:100%;padding:1px 7px;border-radius:999px;border:1px solid var(--border-color);
    background:var(--bg-primary);color:var(--text-secondary);font-size:11px;word-break:break-all}
  .chip-more{color:var(--text-tertiary)}
  .face-models .chip{max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .face-note{margin-top:2px}

  /* --------------------------------------------------------- stats tab */
  /* The page avoids a wall of numbers on a phone: the summary stays visible,
     the per-model table is a <details> that starts folded below 640px, and the
     methodology is always folded. */
  .stats-plan-line{display:flex;align-items:center;justify-content:space-between;gap:8px;flex-wrap:wrap;margin:0 0 2px}
  .stats-plan-line .hint{min-width:0}
  .fold-block{border-top:1px dashed var(--border-color);margin-top:12px;padding-top:10px}
  .fold-block > summary{cursor:pointer;list-style:none;display:flex;align-items:center;justify-content:space-between;gap:10px;
    font-weight:700;min-height:32px;flex-wrap:wrap}
  .fold-block > summary::-webkit-details-marker{display:none}
  .fold-block > summary::before{content:'▸';color:var(--text-tertiary);font-size:12px;margin-right:2px}
  .fold-block[open] > summary::before{content:'▾'}
  .fold-block > summary > span:first-child{margin-right:auto}
  .fold-card > summary{cursor:pointer;font-weight:700;list-style:none;display:flex;align-items:center;gap:8px;min-height:24px}
  .fold-card > summary::-webkit-details-marker{display:none}
  .fold-card > summary::before{content:'▸';color:var(--text-tertiary);font-size:12px}
  .fold-card[open] > summary::before{content:'▾'}
  .fold-card[open] > summary{margin-bottom:10px}
  .stats-table-wrap{overflow-x:auto;-webkit-overflow-scrolling:touch}
  .stats-table td,.stats-table th{vertical-align:top}
  .stats-table th.num,.stats-table td.num{text-align:right;white-space:nowrap;font-variant-numeric:tabular-nums}
  .stats-table td.stats-model-cell{min-width:168px}
  .stats-model-name{font-weight:700;word-break:break-all}
  .stats-model-id{color:var(--text-tertiary);font-size:12px;word-break:break-all;margin-top:2px}
  .stats-model-badges{display:flex;gap:4px;flex-wrap:wrap;margin-top:4px}
  .stats-cell-main{font-weight:700;font-variant-numeric:tabular-nums;white-space:nowrap}
  .stats-sub{display:block;color:var(--text-tertiary);font-size:11px;white-space:nowrap}
  .stats-percent{display:flex;flex-direction:column;gap:3px;min-width:106px}
  .stats-percent .v{font-weight:700;font-variant-numeric:tabular-nums}
  .stats-percent .bar{height:5px}
  .stats-footnote{margin-top:10px;line-height:1.65}
  .stats-notes{margin:8px 0 0;padding-left:18px;display:grid;gap:6px;color:var(--text-secondary);font-size:12px}
  .stats-notes strong{color:var(--text-primary)}
  .stats-notes code{font-size:11px}

  /* -------------------------------------------------------- status tab */
  .about{display:grid;gap:10px}
  .about-row{display:flex;gap:8px;flex-wrap:wrap;align-items:baseline}
  .about-row .k{color:var(--text-tertiary);font-size:11px;text-transform:uppercase;letter-spacing:.06em;min-width:74px}
  .about a{color:inherit;text-decoration:underline}
  .pending{display:grid;gap:6px;margin:0;padding:0;list-style:none}
  .pending li{display:flex;gap:8px;align-items:baseline;border-bottom:1px dashed var(--border-color);padding-bottom:6px;font-size:12px;flex-wrap:wrap}
  .pending li:last-child{border-bottom:0;padding-bottom:0}
  .pending .group{flex:0 0 auto;color:var(--text-tertiary);font-size:11px;text-transform:uppercase;letter-spacing:.05em}

  /* -------------------------------------------------------- credential */
  .modal-backdrop{position:absolute;inset:0;z-index:20;display:flex;align-items:center;justify-content:center;
    padding:16px;background:color-mix(in srgb,#000 55%,transparent)}
  /* display:flex above would beat the hidden attribute, which is what keeps the
     dialog out of the way (and out of the tab order) until it is needed. */
  .modal-backdrop[hidden]{display:none}
  .modal{width:100%;max-width:420px;background:var(--bg-primary);border:1px solid var(--border-color);
    border-radius:var(--radius-lg);box-shadow:var(--shadow-lg);padding:16px;display:flex;flex-direction:column;gap:10px}
  .modal p{margin:0}
  .modal-actions{display:flex;gap:8px;justify-content:flex-end;margin-top:2px}
  .toast{position:absolute;left:50%;bottom:calc(16px + env(safe-area-inset-bottom));transform:translateX(-50%);z-index:30;
    max-width:calc(100% - 32px);overflow:hidden;text-overflow:ellipsis;white-space:nowrap;
    background:var(--bg-primary);border:1px solid var(--border-primary);border-radius:999px;
    padding:8px 16px;box-shadow:var(--shadow-lg);font-weight:600}
  .toast.ok{color:var(--success-color)}
  .toast.error{color:var(--error-color)}

  @media (max-width:900px){
    .filters{grid-template-columns:minmax(0,1fr) minmax(0,1fr)}
    .filters .search{grid-column:1/-1}
    .filters .bulk{grid-column:1/-1;justify-content:flex-start}
    .settings-panel{grid-template-columns:minmax(0,1fr) minmax(0,1fr)}
    .key-form{grid-template-columns:minmax(0,1fr) minmax(0,1fr)}
    .key-form .field.actions{grid-column:1/-1}
  }
  @media (max-width:640px){
    .toolbar{padding:8px 12px;gap:8px}
    .toolbar h1{font-size:15px}
    .toolbar-actions{flex:1 1 100%}
    .toolbar-actions button{flex:1 1 0}
    .tabs{padding:0 8px}
    .tab{padding:10px 10px}
    .panes{padding:10px 12px calc(18px + env(safe-area-inset-bottom))}
    .card{padding:12px;border-radius:10px}
    .grid{grid-template-columns:minmax(0,1fr)}
    .model-top .switch{margin-left:0;order:9}
    .model-card .settings-toggle{margin-left:auto}
    .settings-panel{grid-template-columns:minmax(0,1fr)}
    .stat-strip{grid-template-columns:repeat(auto-fit,minmax(104px,1fr))}
    .key-quota{grid-template-columns:minmax(0,1fr)}
    .key-head .spacer{display:none}
    .key-head .key-actions{flex:1 1 100%;display:flex;justify-content:space-between;align-items:center;gap:8px}
    .key-form{grid-template-columns:minmax(0,1fr)}
    .key-form .field.actions{grid-column:auto}
    table.responsive thead{display:none}
    table.responsive,table.responsive tbody,table.responsive tr,table.responsive td{display:block;width:100%}
    table.responsive tr{border:1px solid var(--border-color);border-radius:var(--radius-md);
      padding:10px 12px;margin-bottom:10px;background:var(--bg-secondary)}
    table.responsive td{border:0;padding:3px 0;display:flex;gap:10px;align-items:baseline}
    table.responsive td::before{content:attr(data-label);flex:0 0 74px;color:var(--text-tertiary);
      font-size:11px;text-transform:uppercase;letter-spacing:.05em}
    /* The stats table becomes one card per model. The model name is the card
       title, so it loses its label; the remaining fields keep theirs. */
    .stats-table-wrap{overflow:visible}
    table.responsive.stats-table td{display:flex;gap:10px;align-items:baseline}
    table.responsive.stats-table td.num{text-align:left}
    table.responsive.stats-table td::before{flex:0 0 84px}
    table.responsive.stats-table td.stats-model-cell{display:block;padding-bottom:6px}
    table.responsive.stats-table td.stats-model-cell::before{display:none}
    .stats-table td.stats-model-cell{min-width:0}
    .stats-percent{min-width:0;width:100%}
    .stats-sub{white-space:normal}
    .stats-plan-line{align-items:flex-start}
    .stats-notes{padding-left:16px}
    /* The credential dialog becomes a bottom sheet on phones. */
    .modal-backdrop{padding:0;align-items:flex-end}
    .modal{max-width:none;border-radius:14px 14px 0 0;border-bottom:0;padding:16px 14px calc(16px + env(safe-area-inset-bottom))}
    .modal-actions{position:sticky;bottom:0}
    .modal-actions button{flex:1 1 0}
  }
`
