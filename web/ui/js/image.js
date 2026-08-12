// ---------- 系统镜像 ----------
function loadImages(){
  api('/api/image').then(r=>{
    if(r.code!==0)return;
    imageList=r.data;
    const grid=document.getElementById('imageGrid');
    if(!r.data||r.data.length===0){
      grid.innerHTML='<div class="empty">暂无镜像，点击右上角"上传 ISO 新增镜像"添加</div>';
      return;
    }
    grid.innerHTML=r.data.map(x=>{
      const isActive=x.active===1;
      const x86Ok=!!x.x86_repo_path;
      const armOk=!!x.arm_repo_path;
      const x86Repo=esc(x.x86_repo_path)||'-';
      const armRepo=esc(x.arm_repo_path)||'-';
      return `<div class="image-card${isActive?' active':''}">
        <div class="image-card-head">
          <div class="image-card-name-wrap">
            <span class="image-card-name">${esc(x.name)}</span>
            <span class="image-card-id">#${x.id}</span>
          </div>
          <span class="image-card-badge ${isActive?'on':'off'}">
            <span class="status-dot" style="background:${isActive?'var(--ok)':'var(--muted)'}"></span>
            ${isActive?'默认':'未生效'}
          </span>
        </div>
        <div class="image-card-body">
          <div class="image-arch${x86Ok?'':' miss'}">
            <div class="image-arch-label">x86_64</div>
            <div class="image-arch-status">
              <span class="status-dot" style="background:${x86Ok?'var(--ok)':'var(--err)'}"></span>
              <span class="${x86Ok?'st-ok':'st-err'}">${x86Ok?'已上传':'未上传'}</span>
            </div>
            <div class="image-arch-path"><code>${x86Repo}</code></div>
          </div>
          <div class="image-arch${armOk?'':' miss'}">
            <div class="image-arch-label">aarch64</div>
            <div class="image-arch-status">
              <span class="status-dot" style="background:${armOk?'var(--ok)':'var(--err)'}"></span>
              <span class="${armOk?'st-ok':'st-err'}">${armOk?'已上传':'未上传'}</span>
            </div>
            <div class="image-arch-path"><code>${armRepo}</code></div>
          </div>
        </div>
        <div class="image-card-actions">
          ${isActive
            ? '<span class="image-card-default-hint">正在作为默认镜像生效</span>'
            : '<button class="btn btn-sm btn-ok" onclick="setActiveImage('+x.id+')">设为默认</button>'}
          <button class="btn btn-sm btn-icon" onclick="openBootFileModal(${x.id},&quot;${esc(x.name)}&quot;)" title="更新 initrd.img / vmlinuz">⚙</button>
          <button class="btn btn-sm btn-icon" onclick="openImageModal(&quot;${esc(x.name)}&quot;)" title="补充/更新 ISO">↻</button>
          <button class="btn btn-sm btn-icon btn-danger-icon" onclick="delImage('+x.id+')" title="删除">✕</button>
        </div>
      </div>`;
    }).join('');
  });
}

function setActiveImage(id){
  if(!confirm('确认将该镜像设为默认安装镜像？'))return;
  api('/api/image/'+id+'/active','POST').then(r=>{
    if(r.code===0){showToast(r.msg);loadImages();}
    else showToast(r.msg||'设置失败','error');
  });
}

function openImageModal(preName){
  document.getElementById('imageModalTitle').textContent = preName ? ('补充/更新 ISO：'+preName) : '上传 ISO 新增系统镜像';
  val('imgName',preName||'');
  if(preName)document.getElementById('imgName').disabled=true;else document.getElementById('imgName').disabled=false;
  document.getElementById('imgIsoX86').value='';
  document.getElementById('imgIsoArm').value='';
  const wrap=document.getElementById('imgProgressWrap');
  const bar=document.getElementById('imgProgressBar');
  const uploadBtn=document.querySelector('#imageModalBg .modal-actions .btn:last-child');
  if(wrap){wrap.style.display='none';bar.style.width='0%';bar.style.background='linear-gradient(90deg,var(--accent),var(--accent2))';}
  if(uploadBtn)uploadBtn.disabled=false;
  document.getElementById('imageModalBg').style.display='flex';
}

function validateImgName(el){
  const clean=el.value.replace(/[^A-Za-z0-9_-]/g,'');
  if(clean!==el.value){el.value=clean;showToast('镜像名称仅限字母/数字/下划线/短横线','error');}
}

function uploadImage(){
  const name=document.getElementById('imgName').value.trim();
  const x86=document.getElementById('imgIsoX86').files[0];
  const arm=document.getElementById('imgIsoArm').files[0];
  if(!name){showToast('请输入镜像名称','error');return;}
  if(!/^[A-Za-z0-9_-]+$/.test(name)){showToast('镜像名称含非法字符，仅限字母/数字/下划线/短横线','error');return;}
  if(!x86 && !arm){showToast('请至少选择一个架构的 ISO 文件','error');return;}

  const fd=new FormData();
  fd.append('name',name);
  if(x86)fd.append('x86_iso',x86);
  if(arm)fd.append('arm_iso',arm);

  const wrap=document.getElementById('imgProgressWrap');
  const bar=document.getElementById('imgProgressBar');
  const text=document.getElementById('imgProgressText');
  const phase=document.getElementById('imgProgressPhase');
  const uploadBtn=document.querySelector('#imageModalBg .modal-actions .btn:last-child');
  wrap.style.display='block';
  bar.style.width='0%';
  text.textContent='准备上传 0%';
  phase.textContent='上传文件';
  uploadBtn.disabled=true;

  const xhr=new XMLHttpRequest();
  xhr.open('POST','/api/image/upload');
  xhr.setRequestHeader('Authorization',token);

  xhr.upload.onprogress=function(e){
    if(e.lengthComputable){
      const pct=Math.round(e.loaded/e.total*100);
      bar.style.width=pct+'%';
      text.textContent='上传中 '+pct+'%';
    }
  };
  xhr.upload.onload=function(){
    phase.textContent='解压 ISO 中...';
    text.textContent='正在解压，请稍候';
  };
  xhr.onload=function(){
    let r={};
    try{r=JSON.parse(xhr.responseText);}catch(err){}
    if(xhr.status===200 && r.code===0){
      bar.style.width='100%';
      text.textContent='完成 100%';
      phase.textContent='完成';
      showToast(r.msg+' '+r.repo_path);
      setTimeout(()=>{closeModal('imageModalBg');loadImages();},800);
    }else{
      bar.style.width='100%';
      bar.style.background='var(--err)';
      text.textContent='失败';
      phase.textContent='';
      showToast(r.msg||'上传失败','error');
    }
    uploadBtn.disabled=false;
  };
  xhr.onerror=function(){
    bar.style.background='var(--err)';
    text.textContent='网络错误';
    showToast('上传失败，请检查网络','error');
    uploadBtn.disabled=false;
  };

  xhr.send(fd);
}

function delImage(id){
  if(!confirm('确认删除镜像？'))return;
  api('/api/image/'+id,'DELETE').then(r=>{if(r.code===0){showToast('已删除');loadImages();}});
}

// ========== Boot 文件更新（initrd.img / vmlinuz） ==========
function openBootFileModal(id, name){
  document.getElementById('bootFileModalTitle').textContent = '更新引导文件 - '+name;
  document.getElementById('bootFileImageId').value = id;
  document.getElementById('bootFileArch').value = 'x86_64';
  document.getElementById('bootFileType').value = 'initrd';
  document.getElementById('bootFileInput').value = '';
  const wrap = document.getElementById('bootFileProgressWrap');
  const bar = document.getElementById('bootFileProgressBar');
  const btn = document.getElementById('bootFileUploadBtn');
  if(wrap){wrap.style.display='none';bar.style.width='0%';}
  if(btn)btn.disabled=false;
  document.getElementById('bootFileModalBg').style.display='flex';
}

function uploadBootFile(){
  const id = document.getElementById('bootFileImageId').value;
  const arch = document.getElementById('bootFileArch').value;
  const fileType = document.getElementById('bootFileType').value;
  const file = document.getElementById('bootFileInput').files[0];
  if(!file){showToast('请选择要上传的文件','error');return;}

  const fd = new FormData();
  fd.append('arch', arch);
  fd.append('type', fileType);
  fd.append('file', file);

  const wrap = document.getElementById('bootFileProgressWrap');
  const bar = document.getElementById('bootFileProgressBar');
  const text = document.getElementById('bootFileProgressText');
  const btn = document.getElementById('bootFileUploadBtn');
  wrap.style.display='block';
  bar.style.width='0%';
  text.textContent='准备上传 0%';
  btn.disabled=true;

  const xhr = new XMLHttpRequest();
  xhr.open('POST', '/api/image/'+id+'/boot-file');
  xhr.setRequestHeader('Authorization', token);

  xhr.upload.onprogress = function(e){
    if(e.lengthComputable){
      const pct = Math.round(e.loaded/e.total*100);
      bar.style.width = pct+'%';
      text.textContent = '上传中 '+pct+'%';
    }
  };
  xhr.onload = function(){
    let r={};
    try{r=JSON.parse(xhr.responseText);}catch(err){}
    if(xhr.status===200 && r.code===0){
      bar.style.width='100%';
      text.textContent='完成 100%';
      showToast(r.msg);
      setTimeout(()=>closeModal('bootFileModalBg'),800);
    }else{
      bar.style.width='100%';
      bar.style.background='var(--err)';
      text.textContent='失败';
      showToast(r.msg||'上传失败','error');
    }
    btn.disabled=false;
  };
  xhr.onerror = function(){
    bar.style.background='var(--err)';
    text.textContent='网络错误';
    showToast('上传失败，请检查网络','error');
    btn.disabled=false;
  };

  xhr.send(fd);
}
