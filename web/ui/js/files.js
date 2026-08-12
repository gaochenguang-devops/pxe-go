// ---------- 文件管理 ----------
function loadFiles(){
  api('/api/file/list').then(r=>{
    if(r.code!==0)return;
    document.getElementById('fileTbody').innerHTML=(r.data||[]).map(f=>{
      const url = esc(f.url);
      return `<tr>
        <td class="file-name-cell" title="${esc(f.name)}">${esc(f.name)}</td>
        <td>${fmtSize(f.size)}</td>
        <td>${fmtTime(f.mod_time)}</td>
        <td class="file-url-cell" title="点击复制 ${url}" onclick="copyText('${url}');showToast('已复制链接')"><code>${url}</code></td>
        <td class="file-action-cell"><button class="btn btn-sm" onclick="openFileUrl('${url}')">访问</button><button class="btn btn-sm btn-danger" onclick="delFile('${esc(f.name)}')">删除</button></td>
      </tr>`;
    }).join('')||'<tr><td colspan="5"><div class="empty">暂无文件，点击上方上传</div></td></tr>';
  });
}

function uploadFile(){
  const input=document.getElementById('fileUploadInput');
  if(!input.files||input.files.length===0){showToast('请先选择要上传的文件','error');return;}

  const fd=new FormData();
  for(let i=0;i<input.files.length;i++){
    fd.append('files',input.files[i]);
  }

  const wrap=document.getElementById('fileProgressWrap');
  const bar=document.getElementById('fileProgressBar');
  const text=document.getElementById('fileProgressText');
  const countEl=document.getElementById('fileProgressCount');
  const total=input.files.length;
  wrap.style.display='block';
  bar.style.width='0%';
  text.textContent='上传中 0%';
  countEl.textContent=`共 ${total} 个文件`;

  const xhr=new XMLHttpRequest();
  xhr.open('POST','/api/file/upload');xhr.setRequestHeader('Authorization',token);

  xhr.upload.onprogress=function(e){
    if(e.lengthComputable){
      const pct=Math.round(e.loaded/e.total*100);
      bar.style.width=pct+'%';
      text.textContent='上传中 '+pct+'%';
    }
  };
  xhr.onload=function(){
    let r={};try{r=JSON.parse(xhr.responseText);}catch(e){}
    if(xhr.status===200&&r.code===0){
      bar.style.width='100%';
      text.textContent='完成 100%';
      const d=r.data||{};
      const errs=(d.results||[]).filter(x=>x.error);
      let msg=d.success+'/'+d.total+' 个文件上传成功';
      if(errs.length>0) msg+='，'+errs.length+' 个失败';
      showToast(msg);
      input.value='';
      loadFiles();
    }else showToast(r.msg||'上传失败','error');
    setTimeout(()=>wrap.style.display='none',1500);
  };
  xhr.onerror=function(){
    showToast('上传失败，请检查网络','error');
    wrap.style.display='none';
  };

  xhr.send(fd);
}

function copyText(t){
  if(navigator.clipboard && navigator.clipboard.writeText){
    navigator.clipboard.writeText(t).then(()=>showToast('已复制链接')).catch(()=>fallbackCopy(t));
  } else {
    fallbackCopy(t);
  }
}
function fallbackCopy(t){
  const el=document.createElement('textarea');
  el.value=t;el.style.position='fixed';el.style.left='-9999px';
  document.body.appendChild(el);el.select();
  try{document.execCommand('copy');showToast('已复制链接');}catch(e){showToast('复制失败','error');}
  document.body.removeChild(el);
}
function openFileUrl(url){
  window.open(url,'_blank');
}
function delFile(name){
  if(!confirm('确认删除文件 '+name+' ？'))return;
  api('/api/file/'+encodeURIComponent(name),'DELETE').then(r=>{
    if(r.code===0){showToast('已删除');loadFiles();}else showToast(r.msg,'error');
  });
}
