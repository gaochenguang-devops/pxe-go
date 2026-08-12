// ---------- 操作日志 ----------
let logRefreshTimer = null;

function loadLogs(){
  const opType = document.getElementById('logOpFilter')?.value || '';
  const search = (document.getElementById('logSearch')?.value || '').trim();
  let url = '/api/operlog?limit=200';
  if(opType) url += '&opType='+encodeURIComponent(opType);
  if(search) url += '&search='+encodeURIComponent(search);
  api(url).then(r=>{
    if(r.code!==0)return;
    document.getElementById('logTbody').innerHTML = (r.data||[]).map(x=>{
      const opClass = x.op_type==='login' ? 'badge-green' : (x.op_type.startsWith('del')||x.op_type.includes('delete') ? 'badge-red' : 'badge-blue');
      return `<tr>
        <td class="log-id">${x.id}</td>
        <td>${esc(x.operator)}</td>
        <td><span class="badge ${opClass}">${esc(x.op_type)}</span></td>
        <td class="log-detail" title="${esc(x.detail)}">${esc(x.detail)}</td>
        <td class="log-ip">${esc(x.client_ip)}</td>
        <td class="log-time">${fmtTime(x.op_time)}</td>
      </tr>`;
    }).join('')||'<tr><td colspan="6"><div class="empty">暂无日志</div></td></tr>';
  });
}

function filterLogs(){ loadLogs(); }

function startLogAutoRefresh(){
  stopLogAutoRefresh();
  logRefreshTimer = setInterval(loadLogs, 15000);
}

function stopLogAutoRefresh(){
  if(logRefreshTimer){clearInterval(logRefreshTimer);logRefreshTimer=null;}
}