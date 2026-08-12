// ---------- KS 模板 ----------
function loadKs(){
  api('/api/ks/template').then(r=>{
    if(r.code!==0)return;
    // 默认模板排最前
    const list=(r.data||[]).sort((a,b)=>(b.is_default||0)-(a.is_default||0));
    document.getElementById('ksTbody').innerHTML=list.map(x=>{
      const def=x.is_default===1;
      return `<tr>
        <td>${esc(x.name)}${def?' <span class="badge badge-blue">内置</span>':''}</td>
        <td>${esc(x.os_type)}</td>
        <td>${x.active===1?'<span class="badge badge-green">生效中</span>':'<span class="badge badge-gray">未生效</span>'}</td>
        <td>${fmtTime(x.create_time)}</td>
        <td>
          ${def?'<span class="badge badge-gray" style="font-size:12px">受保护</span>':`
            <button class="btn btn-sm" onclick="editKs(${x.id})">编辑</button>
            ${x.active===1?'':'<button class="btn btn-sm btn-ok" onclick="setActiveKs('+x.id+')">设置生效</button>'}
            <button class="btn btn-sm btn-danger" onclick="delKs(${x.id})">删除</button>
          `}
        </td>
      </tr>`;
    }).join('')||'<tr><td colspan="5"><div class="empty">暂无模板</div></td></tr>';
  });
  // 填充系统镜像下拉框（供 %pre 软件源 URL 选择镜像名）
  api('/api/image').then(r=>{
    if(r.code!==0)return;
    const sel=document.getElementById('ksImage');
    if(!sel)return;
    sel.innerHTML=r.data.map(x=>`<option value="${esc(x.name)}">${esc(x.name)}</option>`).join('');
  });
}
// 设置生效：同一时间仅一个 KS 模板生效（后端自动写盘 ks.cfg）
function setActiveKs(id){
  if(!confirm('确认将该模板设置为生效？生效后立即用于安装。'))return;
  api('/api/ks/template/'+id+'/active','POST').then(r=>{
    if(r.code===0){showToast(r.msg);loadKs();}
    else showToast(r.msg||'设置失败','error');
  });
}
// 按所选系统镜像生成 KS 模板内容，并打开新建模板弹窗
function generateKSByImage(){
  const sel=document.getElementById('ksImage');
  const imgName=sel ? sel.value : '';
  if(!imgName){showToast('请先选择系统镜像','error');return;}
  if(!confirm('将以默认 KS 模板为基础，替换软件源为所选镜像 "'+imgName+'"，确认生成？'))return;
  const url='/api/ks/template/render?image='+encodeURIComponent(imgName);
  api(url).then(r=>{
    if(r.code!==0){showToast(r.msg||'生成失败','error');return;}
    document.getElementById('ksModalTitle').textContent='新建模板（基于镜像生成）';
    document.getElementById('ksId').value='';
    document.getElementById('ksName').value='ks-'+imgName;
    document.getElementById('ksOsType').value='EulerOS';
    val('ksRootPassword','');
    document.getElementById('ksContent').value=r.data||'';
    document.getElementById('ksModalBg').style.display='flex';
    showToast('已按所选镜像生成 KS 模板，可编辑后保存');
  });
}
function openKsModal(){
  document.getElementById('ksModalTitle').textContent='新建 KS 模板';
  document.getElementById('ksId').value='';
  val('ksName','');val('ksOsType','');val('ksRootPassword','');
  document.getElementById('ksContent').value='';
  document.getElementById('ksModalBg').style.display='flex';
}
function editKs(id){api('/api/ks/template').then(r=>{const x=r.data.find(v=>v.id===id);if(!x)return;document.getElementById('ksModalTitle').textContent='编辑模板 '+x.name;document.getElementById('ksId').value=x.id;val('ksName',x.name);val('ksOsType',x.os_type);val('ksRootPassword',x.root_password||'');val('ksContent',x.content);document.getElementById('ksModalBg').style.display='flex';});}
function saveKs(){
  const id=document.getElementById('ksId').value;
  const body={name:gv('ksName'),os_type:gv('ksOsType'),root_password:gv('ksRootPassword'),content:gv('ksContent')};
  const p=id?('/api/ks/template/'+id):('/api/ks/template');const m=id?'PUT':'POST';
  api(p,m,body).then(r=>{if(r.code===0){showToast(r.msg);closeModal('ksModalBg');loadKs();}else showToast(r.msg,'error');});
}
function previewKs(){document.getElementById('previewContent').textContent=gv('ksContent')||'(空)';document.getElementById('previewBg').style.display='flex';}
function delKs(id){if(!confirm('确认删除模板？'))return;api('/api/ks/template/'+id,'DELETE').then(r=>{if(r.code===0){showToast('已删除');loadKs();}});}

