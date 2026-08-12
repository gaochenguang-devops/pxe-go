// ---------- 主机管理 ----------
let hostPage = 1;
let hostPageSize = 20;
let hostTotal = 0;
let hostTotalPages = 1;
let hostSearchTerm = '';

function loadHosts(){
  let url = `/api/host?page=${hostPage}&pageSize=${hostPageSize}`;
  if(hostSearchTerm) url += `&search=${encodeURIComponent(hostSearchTerm)}`;
  api(url).then(r=>{
    if(r.code!==0)return;
    hostTotal = r.total || 0;
    hostTotalPages = r.totalPages || 1;
    document.getElementById('hostTotal').textContent = hostTotal;
    document.getElementById('hostPageInfo').textContent = `${hostPage} / ${hostTotalPages}`;
    document.getElementById('hostPrevBtn').disabled = hostPage <= 1;
    document.getElementById('hostNextBtn').disabled = hostPage >= hostTotalPages;

    document.getElementById('hostTbody').innerHTML = r.data.map(h=>{
      const checked = hostSelected.has(h.id) ? 'checked' : '';
      return `<tr>
        <td><input type="checkbox" class="hostChk" value="${h.id}" ${checked} onchange="onHostCheck()"></td>
        <td>${h.id}</td><td>${esc(h.hostname)}</td>
        <td>${esc(h.ipmi_addr)}</td><td>${esc(h.ipmi_user)}</td>
        <td><span class="badge badge-blue">${esc(h.install_status)}</span></td>
        <td class="host-power-cell"><button class="btn btn-sm" onclick="ipmiPower(${h.id},'status')" title="查询电源状态">状态</button><button class="btn btn-sm btn-ok" onclick="ipmiPower(${h.id},'on')" title="开机">开机</button><button class="btn btn-sm btn-warn" onclick="ipmiPower(${h.id},'cycle')" title="重启">重启</button><button class="btn btn-sm btn-danger" onclick="ipmiPower(${h.id},'off')" title="关机">关机</button><button class="btn btn-sm btn-ghost" onclick="ipmiBoot(${h.id},'pxe')" title="PXE 启动">PXE</button></td>
        <td><button class="btn btn-sm" onclick="editHost(${h.id})">编辑</button> <button class="btn btn-sm btn-danger" onclick="delHost(${h.id})">删除</button></td>
      </tr>`;
    }).join('')||'<tr><td colspan="8"><div class="empty">暂无主机，点击右上角新增</div></td></tr>';
    refreshHostSelUI();
  });
}

function hostSearch(){
  hostSearchTerm = document.getElementById('hostSearch').value.trim();
  hostPage = 1;
  loadHosts();
}

function hostClearSearch(){
  document.getElementById('hostSearch').value = '';
  hostSearchTerm = '';
  hostPage = 1;
  loadHosts();
}

function hostPrevPage(){
  if(hostPage>1){hostPage--;loadHosts();}
}
function hostNextPage(){
  if(hostPage<hostTotalPages){hostPage++;loadHosts();}
}
function hostChangePageSize(){
  hostPageSize = parseInt(document.getElementById('hostPageSize').value) || 20;
  hostPage = 1;
  loadHosts();
}

function openHostModal(){
  document.getElementById('hostModalTitle').textContent='新增主机';
  document.getElementById('hostId').value='';clearHost();
  document.getElementById('hostModalBg').style.display='flex';
}

function editHost(id){
  // 编辑时从当前页数据中查找；若当前页没有则查全量（主机数量通常不会太大）
  api('/api/host').then(r=>{
    const h=r.data.find(x=>x.id===id);if(!h)return;
    document.getElementById('hostModalTitle').textContent='编辑主机 '+h.hostname;
    document.getElementById('hostId').value=h.id;
    val('hostName',h.hostname);val('hostIpmi',h.ipmi_addr);
    val('hostIpmiUser',h.ipmi_user);val('hostIpmiPass','');
    document.getElementById('hostModalBg').style.display='flex';
  });
}
function clearHost(){
  ['hostName','hostIpmi','hostIpmiUser','hostIpmiPass'].forEach(i=>document.getElementById(i).value='');
}
function saveHost(){
  const id=document.getElementById('hostId').value;
  const body={hostname:gv('hostName'),ipmi_addr:gv('hostIpmi'),
    ipmi_user:gv('hostIpmiUser'),ipmi_pass:gv('hostIpmiPass')};
  const p=id?('/api/host/'+id):('/api/host');const m=id?'PUT':'POST';
  api(p,m,body).then(r=>{
    if(r.code===0){showToast(r.msg);closeModal('hostModalBg');loadHosts();}
    else showToast(r.msg,'error');
  });
}
function delHost(id){if(!confirm('确认删除主机？'))return;api('/api/host/'+id,'DELETE').then(r=>{if(r.code===0){showToast('已删除');loadHosts();}});}
function ipmiPower(id,action){showToast('正在执行 IPMI '+action+'...');api('/api/host/'+id+'/ipmi/power','POST',{action}).then(r=>{if(r.code===0)showToast('操作已执行');else showToast(r.msg,'error');loadHosts();});}
function ipmiBoot(id,device){api('/api/host/'+id+'/ipmi/boot','POST',{device}).then(r=>{if(r.code===0)showToast('已设置 '+device+' 启动');else showToast(r.msg,'error');});}

// ---------- 主机批量管理（跨页保持选中）----------
let hostSelected = new Set();
function onHostCheck(){
  document.querySelectorAll('.hostChk').forEach(cb=>{
    const id = parseInt(cb.value);
    if(cb.checked) hostSelected.add(id); else hostSelected.delete(id);
  });
  refreshHostSelUI();
}
function toggleAllHost(el){
  document.querySelectorAll('.hostChk').forEach(cb=>{cb.checked=el.checked;});
  onHostCheck();
}
function refreshHostSelUI(){
  const n=hostSelected.size;
  const el=document.getElementById('hostSelCount');
  if(el)el.textContent='已选 '+n+' 台';
  const all=document.getElementById('hostSelAll');
  const total=document.querySelectorAll('.hostChk').length;
  if(all)all.checked = total>0 && n===total;
}
function selectedHostIDs(){
  return Array.from(hostSelected);
}
function batchHost(action){
  const ids=selectedHostIDs();
  if(ids.length===0){showToast('请先勾选主机','error');return;}
  if(action==='delete'){
    if(!confirm('确认删除选中的 '+ids.length+' 台主机（ID: '+ids.join(',')+'）？'))return;
    api('/api/host/batch/delete','POST',{ids}).then(r=>{
      if(r.code===0){showToast(r.msg);hostSelected.clear();loadHosts();}else showToast(r.msg,'error');
    });
  }else{
    const label={on:'开机',off:'关机',cycle:'重启'}[action]||action;
    if(!confirm('确认对选中的 '+ids.length+' 台主机执行批量'+label+'？'))return;
    api('/api/host/batch/ipmi','POST',{ids,action}).then(r=>{
      if(r.code===0){showToast(r.msg);}else showToast(r.msg,'error');
    });
  }
}
function batchExportHost(){
  const ids=selectedHostIDs();
  if(ids.length===0){showToast('请先勾选主机','error');return;}
  fetch('/api/host/batch/export',{method:'POST',headers:{'Content-Type':'application/json',...token?{Authorization:token}:{}},body:JSON.stringify({ids})}).then(r=>{
    if(r.status===401){showLoginPage('登录已过期，请重新登录');return;}
    if(!r.ok){showToast('导出失败','error');return;}
    const disp=r.headers.get('Content-Disposition')||'';
    const m=disp.match(/filename\*?=(?:UTF-8'')?["']?([^"';]+)/i);
    const fn=m&&m[1]?decodeURIComponent(m[1]):'hosts.xlsx';
    return r.blob().then(b=>{
      const u=URL.createObjectURL(b);const a=document.createElement('a');a.href=u;a.download=fn;
      document.body.appendChild(a);a.click();setTimeout(()=>{URL.revokeObjectURL(u);a.remove();},100);
    });
  }).catch(()=>showToast('导出失败，请检查网络','error'));
}

// ---------- 主机管理 Excel 导入 ----------
function importHostsExcel(){
  const f=document.getElementById('hostImportFile');
  if(!f.files||!f.files[0]){showToast('请先选择 Excel 文件','error');return;}
  const fd=new FormData();fd.append('file',f.files[0]);
  const xhr=new XMLHttpRequest();
  xhr.open('POST','/api/host/excel/import');xhr.setRequestHeader('Authorization',token);
  xhr.onload=function(){
    let r={};try{r=JSON.parse(xhr.responseText);}catch(e){}
    if(xhr.status===200&&r.code===0){
      showToast(r.msg);
      hostPage = 1;  // 导入后回到第一页
      f.value = '';
      loadHosts();
    } else showToast(r.msg||'导入失败','error');
  };
  xhr.onerror=function(){showToast('导入失败，请检查网络','error');};
  xhr.send(fd);
}
