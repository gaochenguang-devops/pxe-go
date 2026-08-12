// ---------- 主机资源管理（node-info.txt / Excel 导入导出） ----------
function importResExcel(){
  const f=document.getElementById('resImportFile');
  if(!f.files||!f.files[0]){showToast('请先选择 Excel 文件','error');return;}
  const fd=new FormData();fd.append('file',f.files[0]);
  const xhr=new XMLHttpRequest();
  xhr.open('POST','/api/host-resource/import');xhr.setRequestHeader('Authorization',token);
  xhr.onload=function(){
    let r={};try{r=JSON.parse(xhr.responseText);}catch(e){}
    if(xhr.status===200&&r.code===0){
      showToast(r.msg);
      resPage = 1;
      f.value = '';
      loadHostResList();
      loadNodeInfo();
    } else showToast(r.msg||'导入失败','error');
  };
  xhr.onerror=function(){showToast('导入失败，请检查网络','error');};
  xhr.send(fd);
}
function generateNodeInfo(){
  showToast('正在生成 node-info.txt...');
  fetch('/api/host-resource/node-info/export',{headers:token?{Authorization:token}:{}}).then(r=>{
    if(r.status===200){showToast('已生成 node-info.txt');loadNodeInfo();}
    else showToast('生成失败','error');
  }).catch(()=>showToast('生成失败，请检查网络','error'));
}
function loadNodeInfo(){
  api('/api/host-resource/node-info').then(r=>{
    if(r&&r.code===0)document.getElementById('nodeInfoPreview').textContent=r.data||'(空)';
  });
}

// ---------- iPXE 脚本（列表 + 编辑 + 设置生效） ----------
function loadIPxeScript(){
  api('/api/ipxe/script').then(r=>{
    if(r.code!==0){showToast(r.msg||'加载失败','error');return;}
    // 默认模板排最前
    const list=(r.data||[]).sort((a,b)=>(b.is_default||0)-(a.is_default||0));
    document.getElementById('ipxeTbody').innerHTML=list.map(x=>{
      const def=x.is_default===1;
      return `<tr>
        <td>${esc(x.name)}${def?' <span class="badge badge-blue">内置</span>':''}</td>
        <td>${x.active===1?'<span class="badge badge-green">生效中</span>':'<span class="badge badge-gray">未生效</span>'}</td>
        <td>${fmtTime(x.create_time)}</td>
        <td>
          ${def?'<span class="badge badge-gray" style="font-size:12px">受保护</span>':`
            <button class="btn btn-sm" onclick="editIPxeScript(${x.id})">编辑</button>
            ${x.active===1?'':'<button class="btn btn-sm btn-ok" onclick="setActiveIPxeScript('+x.id+')">设置生效</button>'}
            <button class="btn btn-sm btn-danger" onclick="delIPxeScript(${x.id})">删除</button>
          `}
        </td>
      </tr>`;
    }).join('')||'<tr><td colspan="4"><div class="empty">暂无脚本，点击右上角新建</div></td></tr>';
  });
  // 填充系统镜像下拉框
  api('/api/image').then(r=>{
    if(r.code!==0)return;
    const sel=document.getElementById('ipxeImage');
    sel.innerHTML=r.data.map(x=>`<option value="${x.id}">${esc(x.name)}</option>`).join('');
  });
}
function openIPxeModal(){
  document.getElementById('ipxeModalTitle').textContent='新建 iPXE 脚本';
  document.getElementById('ipxeId').value='';
  val('ipxeName','');
  val('ipxeContent','');
  document.getElementById('ipxeModalBg').style.display='flex';
}
function editIPxeScript(id){
  api('/api/ipxe/script').then(r=>{
    const x=(r.data||[]).find(v=>v.id===id);if(!x)return;
    document.getElementById('ipxeModalTitle').textContent='编辑脚本 '+x.name;
    document.getElementById('ipxeId').value=x.id;
    val('ipxeName',x.name);
    val('ipxeContent',x.content);
    document.getElementById('ipxeModalBg').style.display='flex';
  });
}
function saveIPxeScript(){
  const id=document.getElementById('ipxeId').value;
  const name=document.getElementById('ipxeName').value.trim();
  const content=document.getElementById('ipxeContent').value;
  if(!name){showToast('请输入脚本名称','error');return;}
  if(!content.trim()){showToast('内容为空','error');return;}
  const body={name,content};
  const p=id?('/api/ipxe/script/'+id):('/api/ipxe/script');
  const m=id?'PUT':'POST';
  api(p,m,body).then(r=>{
    if(r.code===0){showToast(r.msg);closeModal('ipxeModalBg');loadIPxeScript();}
    else showToast(r.msg||'保存失败','error');
  });
}
// 设置生效：同一时间仅一个生效（后端自动写盘 autoexec.ipxe）
function setActiveIPxeScript(id){
  if(!confirm('确认将该脚本设置为生效？生效后立即用于客户端 PXE 引导。'))return;
  api('/api/ipxe/script/'+id+'/active','POST').then(r=>{
    if(r.code===0){showToast(r.msg);loadIPxeScript();}
    else showToast(r.msg||'设置失败','error');
  });
}
function delIPxeScript(id){
  if(!confirm('确认删除该脚本？'))return;
  api('/api/ipxe/script/'+id,'DELETE').then(r=>{
    if(r.code===0){showToast('已删除');loadIPxeScript();}
  });
}
// 按所选系统镜像渲染并自动打开"新建脚本"弹窗（填入生成的脚本内容）
function renderIPxeByImage(){
  const sel=document.getElementById('ipxeImage');
  const imageID=sel.value;
  if(!imageID||imageID==='0'){showToast('请先选择系统镜像','error');return;}
  const imgName=sel.options[sel.selectedIndex] ? sel.options[sel.selectedIndex].text.split(' (')[0] : '';
  api('/api/ipxe/script/render?image_id='+imageID).then(r=>{
    if(r.code!==0){showToast(r.msg||'渲染失败','error');return;}
    document.getElementById('ipxeModalTitle').textContent='新建脚本（基于镜像生成）';
    document.getElementById('ipxeId').value='';
    document.getElementById('ipxeName').value='autoexec-'+(imgName||imageID);
    document.getElementById('ipxeContent').value=r.data||'';
    document.getElementById('ipxeModalBg').style.display='flex';
    showToast('已按所选镜像生成脚本，可编辑后保存');
  });
}
function ipxePreview(){
  const d=document.getElementById('previewContent');
  d.textContent=document.getElementById('ipxeContent').value||'(空)';
  document.getElementById('previewBg').style.display='flex';
}

// ---------- 部署脚本（deploy.sh）列表管理 ----------
function loadDeployScript(){
  api('/api/deploy/script').then(r=>{
    if(r.code!==0)return;
    // 默认模板排最前
    const list=(r.data||[]).sort((a,b)=>(b.is_default||0)-(a.is_default||0));
    document.getElementById('deployTbody').innerHTML=list.map(x=>{
      const def=x.is_default===1;
      return `<tr>
        <td>${esc(x.name)}${def?' <span class="badge badge-blue">内置</span>':''}</td>
        <td>${x.active===1?'<span class="badge badge-green">生效中</span>':'<span class="badge badge-gray">未生效</span>'}</td>
        <td>${fmtTime(x.create_time)}</td>
        <td>
          ${def?'<span class="badge badge-gray" style="font-size:12px">受保护</span>':`
            <button class="btn btn-sm" onclick="editDeploy(${x.id})">编辑</button>
            ${x.active===1?'':'<button class="btn btn-sm btn-ok" onclick="setActiveDeploy('+x.id+')">设置生效</button>'}
            <button class="btn btn-sm btn-danger" onclick="delDeploy(${x.id})">删除</button>
          `}
        </td>
      </tr>`;
    }).join('')||'<tr><td colspan="4"><div class="empty">暂无脚本，点击右上角新建</div></td></tr>';
  });
}
function openDeployModal(){
  document.getElementById('deployModalTitle').textContent='新建部署脚本';
  document.getElementById('deployId').value='';
  document.getElementById('deployName').value='';
  document.getElementById('deployPreviewWrap').style.display='none';
  // 自动加载默认模板内容作为基础
  api('/api/deploy/script').then(r=>{
    if(r.code===0){
      const def=(r.data||[]).find(x=>x.is_default===1);
      document.getElementById('deployContent').value=(def&&def.content)||'';
    } else { document.getElementById('deployContent').value=''; }
    document.getElementById('deployModalBg').style.display='flex';
  });
}

function previewDeploy(){
  const content = document.getElementById('deployContent').value;
  const wrap = document.getElementById('deployPreviewWrap');
  const pre = document.getElementById('deployPreview');
  // 从配置中获取真实 PXE IP 用于预览
  api('/api/config').then(r=>{
    const pxeIP = (r.code===0 && r.data && r.data.dhcp_pxe_ip) || '@@PXE_SERVER@@';
    const preview = esc(content.replace(/@@PXE_SERVER@@/g, pxeIP));
    pre.textContent = preview;
    wrap.style.display = 'block';
  }).catch(()=>{
    pre.textContent = esc(content);
    wrap.style.display = 'block';
  });
}
function editDeploy(id){
  api('/api/deploy/script/'+id+'/content').then(r=>{
    if(r.code!==0){showToast(r.msg,'error');return;}
    document.getElementById('deployModalTitle').textContent='编辑部署脚本';
    document.getElementById('deployId').value=id;
    const d = r.data || {};
    document.getElementById('deployName').value = d.name || '';
    document.getElementById('deployContent').value = d.content || '';
    document.getElementById('deployPreviewWrap').style.display='none';
    document.getElementById('deployModalBg').style.display='flex';
  });
}
function saveDeploy(){
  const id=document.getElementById('deployId').value;
  const name=document.getElementById('deployName').value.trim();
  const content=document.getElementById('deployContent').value;
  if(!name){showToast('脚本名称不能为空','error');return;}
  if(!content.trim()){showToast('脚本内容不能为空','error');return;}
  const p=id?('/api/deploy/script/'+id):('/api/deploy/script');
  const m=id?'PUT':'POST';
  api(p,m,{name,content}).then(r=>{
    if(r.code===0){showToast(r.msg);closeModal('deployModalBg');loadDeployScript();}
    else showToast(r.msg,'error');
  });
}
function delDeploy(id){
  if(!confirm('确认删除此部署脚本？'))return;
  api('/api/deploy/script/'+id,'DELETE').then(r=>{
    if(r.code===0){showToast('已删除');loadDeployScript();}
    else showToast(r.msg,'error');
  });
}
function setActiveDeploy(id){
  if(!confirm('确认将此脚本设为生效？生效后会自动持久化到 deploy.sh。'))return;
  api('/api/deploy/script/'+id+'/active','POST').then(r=>{
    if(r.code===0){showToast(r.msg);loadDeployScript();}
    else showToast(r.msg,'error');
  });
}


// ---------- 主机资源批量管理 ----------
let resPage = 1;
let resPageSize = 20;
let resTotal = 0;
let resTotalPages = 1;
let resSelected = new Set();
let resCurrentDetail = null;
let resSearchTerm = '';

function loadHostResList(){
  let url = `/api/host-resource/list?page=${resPage}&pageSize=${resPageSize}`;
  if(resSearchTerm) url += `&search=${encodeURIComponent(resSearchTerm)}`;
  api(url).then(r=>{
    if(r.code!==0)return;
    resTotal = r.total || 0;
    resTotalPages = r.totalPages || 1;
    document.getElementById('resTotal').textContent = resTotal;
    document.getElementById('resPageInfo').textContent = `${resPage} / ${resTotalPages}`;
    document.getElementById('resPrevBtn').disabled = resPage <= 1;
    document.getElementById('resNextBtn').disabled = resPage >= resTotalPages;

    document.getElementById('resTbody').innerHTML = (r.data||[]).map(h=>{
      const checked=resSelected.has(h.id)?'checked':'';
      const active = resCurrentDetail && resCurrentDetail.id===h.id;
      return `<tr class="res-row${active?' active':''}" data-id="${h.id}" style="cursor:pointer" onclick="showResDetail(${h.id})">
        <td onclick="event.stopPropagation()"><input type="checkbox" class="resChk" value="${h.id}" ${checked} onchange="onResCheck()"></td>
        <td>${h.id}</td><td>${esc(h.hostname)}</td>
        <td>${esc(h.ipmi_addr)}</td><td>${esc(h.bond0_ip)||'-'}</td>
        <td>${esc(h.bond2_ip)||'-'}</td><td>${esc(h.bond1_ip)||'-'}</td>
        <td><button class="btn btn-sm btn-ghost" onclick="event.stopPropagation();showResDetail(${h.id})">查看</button></td>
      </tr>`;
    }).join('')||'<tr><td colspan="8"><div class="empty">暂无资源记录，点击表格行查看完整网络配置</div></td></tr>';
    refreshResSelUI();
  });
}

// 显示资源详情弹窗
function showResDetail(id){
  document.querySelectorAll('.res-row').forEach(tr=>tr.classList.toggle('active', parseInt(tr.dataset.id)===id));
  api('/api/host-resource/list').then(r=>{
    if(r.code!==0)return;
    const item = (r.data||[]).find(x=>x.id===id);
    if(!item)return;
    resCurrentDetail = item;
    document.getElementById('resDetailName').textContent = item.hostname;
    document.getElementById('resDetailIPMI').textContent = 'IPMI: ' + (item.ipmi_addr || '-');
    document.getElementById('resDetailContent').innerHTML = renderResDetail(item);
    document.getElementById('resDetailModalBg').style.display = 'flex';
  });
}

function closeResDetail(){
  resCurrentDetail = null;
  closeModal('resDetailModalBg');
  document.querySelectorAll('.res-row').forEach(tr=>tr.classList.remove('active'));
}

function renderResDetail(h){
  const groups = [
    {title: 'bond0 管理网络', color: 'var(--accent2)', rows: [
      ['IPv4', h.bond0_ip],
      ['子网掩码', h.bond0_mask],
      ['网关', h.bond0_gateway],
      ['IPv6', h.bond0_ipv6],
      ['IPv6 掩码', h.bond0_ipv6mask],
      ['IPv6 网关', h.bond0_ipv6gw],
    ]},
    {title: 'bond2 业务网络', color: 'var(--ok)', rows: [
      ['IPv4', h.bond2_ip],
      ['子网掩码', h.bond2_mask],
      ['网关', h.bond2_gateway],
      ['IPv6', h.bond2_ipv6],
      ['IPv6 掩码', h.bond2_ipv6mask],
      ['IPv6 网关', h.bond2_ipv6gw],
    ]},
    {title: 'bond1 存储网络', color: 'var(--warn)', rows: [
      ['IPv4', h.bond1_ip],
      ['子网掩码', h.bond1_mask],
      ['网关', h.bond1_gateway],
    ]},
  ];
  return groups.map(g=>{
    const rows = g.rows.filter(r=>r[1] && r[1].trim()!=='');
    if(rows.length===0) return '';
    return `
      <div style="margin-bottom:12px">
        <div style="font-size:13px;font-weight:600;color:${g.color};margin-bottom:8px;padding-bottom:4px;border-bottom:1px solid var(--border)">${g.title}</div>
        <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:8px">
          ${rows.map(r=>`<div class="res-detail-item">
            <span class="res-detail-label">${r[0]}</span>
            <span class="res-detail-value"><code>${esc(r[1])}</code></span>
          </div>`).join('')}
        </div>
      </div>`;
  }).join('');
}

function resPrevPage(){
  if(resPage>1){resPage--;loadHostResList();}
}
function resNextPage(){
  if(resPage<resTotalPages){resPage++;loadHostResList();}
}
function resChangePageSize(){
  resPageSize = parseInt(document.getElementById('resPageSize').value) || 20;
  resPage = 1;
  loadHostResList();
}

function resSearch(){
  resSearchTerm = document.getElementById('resSearch').value.trim();
  resPage = 1;
  loadHostResList();
}

function resClearSearch(){
  document.getElementById('resSearch').value = '';
  resSearchTerm = '';
  resPage = 1;
  loadHostResList();
}

function onResCheck(){
  document.querySelectorAll('.resChk').forEach(cb=>{
    const id = parseInt(cb.value);
    if(cb.checked) resSelected.add(id); else resSelected.delete(id);
  });
  refreshResSelUI();
}
function toggleAllRes(el){
  document.querySelectorAll('.resChk').forEach(cb=>{cb.checked=el.checked;});
  onResCheck();
}
function refreshResSelUI(){
  const n=resSelected.size;
  const el=document.getElementById('resSelCount');
  if(el)el.textContent='已选 '+n+' 台';
  const all=document.getElementById('resSelAll');
  const total=document.querySelectorAll('.resChk').length;
  if(all)all.checked=total>0&&n===total;
}
function batchResDelete(){
  const ids=Array.from(resSelected);
  if(ids.length===0){showToast('请先勾选主机资源','error');return;}
  if(!confirm('确认删除选中的 '+ids.length+' 台主机资源？删除后 node-info.txt 将移除这些资源。'))return;
  api('/api/host-resource/batch/delete','POST',{ids}).then(r=>{
    if(r.code===0){showToast(r.msg);resSelected.clear();loadHostResList();loadNodeInfo();}
    else showToast(r.msg,'error');
  });
}
function batchResExport(){
  const ids=Array.from(resSelected);
  if(ids.length===0){showToast('请先勾选主机资源','error');return;}
  fetch('/api/host-resource/batch/export',{method:'POST',headers:{'Content-Type':'application/json',...token?{Authorization:token}:{}},body:JSON.stringify({ids})}).then(r=>{
    if(r.status===401){showLoginPage('登录已过期，请重新登录');return;}
    if(!r.ok){showToast('导出失败','error');return;}
    const m=(r.headers.get('Content-Disposition')||'').match(/filename\*?=(?:UTF-8'')?["']?([^"';]+)/i);
    const fn=m&&m[1]?decodeURIComponent(m[1]):'host-resource.xlsx';
    return r.blob().then(b=>{
      const u=URL.createObjectURL(b);const a=document.createElement('a');a.href=u;a.download=fn;
      document.body.appendChild(a);a.click();setTimeout(()=>{URL.revokeObjectURL(u);a.remove();},100);
      loadNodeInfo();
    });
  }).catch(()=>showToast('导出失败，请检查网络','error'));
}

