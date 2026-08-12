// ---------- 装机记录 ----------
let installPage = 1;
let installTotal = 0;
let installTotalPages = 1;
let installPageSize = 20;
let installAllData = [];

function loadInstallRecords(){
  installPageSize = parseInt(document.getElementById('installPageSize')?.value) || 20;
  const offset = (installPage - 1) * installPageSize;
  api(`/api/install-records?limit=${installPageSize}&offset=${offset}`).then(r=>{
    if(r.code!==0)return;
    installTotal = r.total || 0;
    installTotalPages = Math.ceil(installTotal / installPageSize) || 1;
    installAllData = r.data || [];
    updateInstallStats(r);
    renderInstallList();
  });
}

function renderInstallList(){
  const search = (document.getElementById('installSearch')?.value || '').toLowerCase().trim();
  const status = document.getElementById('installStatusFilter')?.value || '';

  const filtered = installAllData.filter(x=>{
    if(status && x.status !== status) return false;
    if(search){
      const hay = [
        x.hostname||'', x.ip||'', x.ipmi_addr||'', x.mac||'',
        x.interfaces||'', x.lldp||''
      ].join(' ').toLowerCase();
      if(!hay.includes(search)) return false;
    }
    return true;
  });

  document.getElementById('installTbody').innerHTML = filtered.map(x=>{
    const isOk = x.status === 'success';
    const statusClass = isOk ? 'install-status-ok' : 'install-status-err';
    return `<div class="install-row">
      <div class="install-row-main">
        <div class="install-col-time">
          <div class="install-time mono">${fmtTime(x.report_time)}</div>
          <div class="install-status ${statusClass}">${esc((x.status||'').toUpperCase()||'UNKNOWN')}</div>
        </div>
        <div class="install-col-host">
          <div class="install-host-name">${esc(x.hostname||'unknown')}</div>
          <div class="install-host-sub">IPMI: ${esc(x.ipmi_addr||'unknown')}</div>
          <div class="install-host-sub">来源: ${esc(x.client_ip||'?')}</div>
        </div>
        <div class="install-col-meta">
          <div class="install-meta-row"><span class="install-meta-label">架构</span><span class="badge badge-blue">${esc(x.arch||'-')}</span></div>
          <div class="install-meta-row"><span class="install-meta-label">来源 IP</span><span class="mono">${esc(x.ip||'-')}</span></div>
        </div>
        <div class="install-col-lldp">
          ${renderLLDPMainTable(x.lldp)}
        </div>
      </div>
    </div>`;
  }).join('')||'<div class="empty">暂无装机完成记录</div>';

  document.getElementById('installCount').textContent = `${installTotal} 条记录`;
  document.getElementById('installPageInfo').textContent = `${installPage} / ${installTotalPages}`;
  document.getElementById('installPrevBtn').disabled = installPage <= 1;
  document.getElementById('installNextBtn').disabled = installPage >= installTotalPages;
}

function updateInstallStats(r){
  document.getElementById('statTotal').textContent = r.total || 0;
  document.getElementById('statSuccess').textContent = r.success || 0;
  document.getElementById('statFailed').textContent = r.failed || 0;
  document.getElementById('statNoLldp').textContent = r.noLldp || 0;
}

function filterInstall(){ renderInstallList(); }

function installPrevPage(){
  if(installPage>1){installPage--;loadInstallRecords();}
}
function installNextPage(){
  if(installPage<installTotalPages){installPage++;loadInstallRecords();}
}

function installChangePageSize(){
  installPage = 1;
  loadInstallRecords();
}

// 主行直接显示 物理网卡/MAC/交换机/端口 信息表
function renderLLDPMainTable(lldp){
  if(!lldp || lldp==='-' || lldp==='unknown' || lldp.includes('%s')) {
    return `<div class="lldp-main-empty">⚠ LLDP 未识别</div>`;
  }
  const items = lldp.split(';').filter(Boolean);
  if(items.length===0) return `<div class="lldp-main-empty">-</div>`;
  const summary = items.length > 1 ? ` <span class="lldp-tag">${items.length} 个接口</span>` : '';
  return `<div class="lldp-main-wrap">
    <div class="lldp-main-header">
      <span class="lldp-main-title">物理网卡与 LLDP</span>${summary}
    </div>
    <table class="lldp-table lldp-table-main">
      <thead><tr><th>网卡</th><th>MAC</th><th>交换机</th><th>端口</th></tr></thead>
      <tbody>${items.map(item=>renderLLDPRow(item)).join('')}</tbody>
    </table>
  </div>`;
}

function renderLLDPRow(item){
  // LLDP 格式：interface=mac|switch|port
  const eqIdx = item.indexOf('=');
  if(eqIdx < 0) return `<tr><td colspan="4">${esc(item)}</td></tr>`;
  const interface = item.substring(0, eqIdx);
  const rest = item.substring(eqIdx + 1);
  const parts = rest.split('|');
  return `<tr>
    <td><strong>${esc(interface)}</strong></td>
    <td class="mono">${esc(parts[0]||'-')}</td>
    <td>${esc(parts[1]||'-')}</td>
    <td>${esc(parts[2]||'-')}</td>
  </tr>`;
}