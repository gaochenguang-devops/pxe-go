let imageList=[],ksList=[],resList=[];
let token = localStorage.getItem('pxe_token') || '';
let currentPage = 'dashboard';
const pages = ['dashboard','images','config','scripts','hosts','hostres','files','logs'];
const titles = {dashboard:'仪表盘',images:'系统镜像',config:'服务配置',scripts:'引导脚本',hosts:'主机管理',hostres:'主机资源',files:'文件管理',logs:'操作日志'};

// ---------- 基础工具 ----------
function showToast(msg, type='success'){
  const t=document.getElementById('toast');
  t.textContent=msg; t.className='toast '+type; t.style.display='block';
  setTimeout(()=>t.style.display='none',2600);
}
async function api(path, method='GET', body){
  const opt={method,headers:{}};
  if(token) opt.headers['Authorization']=token;
  if(body!==undefined){opt.headers['Content-Type']='application/json';opt.body=JSON.stringify(body);}
  const r=await fetch(path,opt);
  if(r.status===401){
    // 登录接口的 401 表示密码错误，由 login() 自己处理错误提示，不触发跳登录页
    if(path==='/api/login') throw new Error('login_failed');
    showLoginPage('登录已过期，请重新登录');
    throw new Error('401');
  }
  return r.json();
}
// 静默请求：用于预加载共享数据，401 时不触发登录页跳转（避免未登录时无限刷新）。
async function silentApi(path){
  const r=await fetch(path,{headers:token?{Authorization:token}:{}});
  if(r.status===401) return null;
  return r.json();
}
function showLoginPage(msg){
  token='';localStorage.removeItem('pxe_token');
  document.getElementById('app').style.display='none';
  document.getElementById('loginPage').style.display='flex';
  if(msg) showToast(msg,'error');
}
function esc(s){return (s||'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));}
function fmtSize(b){if(!b&&b!==0)return '-';if(b<1024)return b+'B';if(b<1048576)return (b/1024).toFixed(1)+'KB';return (b/1048576).toFixed(1)+'MB';}
function fmtTime(t){if(!t)return '-';return (t||'').replace('T',' ').slice(0,19);}

// ---------- 登录 ----------
// 登录页回车即可登录
function bindLoginEnter(){
  ['loginUser','loginPass'].forEach(id=>{
    const el=document.getElementById(id);
    if(el) el.addEventListener('keydown',e=>{ if(e.key==='Enter'){e.preventDefault();login();} });
  });
}

async function login(){
  const u=document.getElementById('loginUser').value, p=document.getElementById('loginPass').value;
  let resp, body;
  try {
    resp = await fetch('/api/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:u,password:p})});
    body = await resp.json();
  } catch(err) {
    showToast('登录请求失败，请检查网络','error');
    return;
  }
  if(resp.status===200 && body.code===0){
    token=body.data.token;localStorage.setItem('pxe_token',token);
    // 记住密码
    const rem=document.getElementById('rememberPwd');
    if(rem&&rem.checked){localStorage.setItem('pxe_user',u);localStorage.setItem('pxe_pass',p);}
    else{localStorage.removeItem('pxe_user');localStorage.removeItem('pxe_pass');}
    document.getElementById('loginPage').style.display='none';
    document.getElementById('app').style.display='block';
    showToast('登录成功');
    try { init(); } catch(e) { /* init 内部异常不影响登录结果 */ }
  } else {
    showToast(body.msg||'登录失败','error');
  }
}
function logout(){showLoginPage();}
function toggleUserMenu(){
  const d=document.getElementById('userDropdown');
  const b=document.getElementById('userBtn');
  const isOpen=d.style.display==='block';
  d.style.display=isOpen?'none':'block';
  if(b)b.classList.toggle('open',!isOpen);
}
function openPwdModal(){
  document.getElementById('userDropdown').style.display='none';
  document.getElementById('pwdOld').value='';
  document.getElementById('pwdNew').value='';
  document.getElementById('pwdConfirm').value='';
  document.getElementById('pwdModalBg').style.display='flex';
}
function changePassword(){
  const oldPwd=document.getElementById('pwdOld').value;
  const newPwd=document.getElementById('pwdNew').value;
  const confirm=document.getElementById('pwdConfirm').value;
  if(!oldPwd||!newPwd){showToast('密码不能为空','error');return;}
  if(newPwd!==confirm){showToast('两次输入的新密码不一致','error');return;}
  api('/api/password','PUT',{old_password:oldPwd,new_password:newPwd}).then(r=>{
    if(r.code===0){showToast(r.msg);closeModal('pwdModalBg');setTimeout(()=>showLoginPage(),1500);}
    else showToast(r.msg,'error');
  });
}
// 点击空白处关闭用户下拉菜单
document.addEventListener('click',function(e){if(!e.target.closest('.user-menu')){document.getElementById('userDropdown').style.display='none';const b=document.getElementById('userBtn');if(b)b.classList.remove('open');}});

// ---------- 导航 ----------
document.querySelectorAll('.sidebar nav a').forEach(a=>a.addEventListener('click',e=>{
  e.preventDefault();switchPage(a.dataset.page);
}));
function switchPage(p){
  currentPage=p;
  document.querySelectorAll('.sidebar nav a').forEach(a=>a.classList.toggle('active',a.dataset.page===p));
  document.querySelectorAll('.page').forEach(x=>x.classList.remove('active'));
  document.getElementById('page-'+p).classList.add('active');
  document.getElementById('pageTitle').textContent=titles[p];
  if(p==='config'){loadConfig();startCfgAutoRefresh();if(typeof switchCfgTab==='function')switchCfgTab(cfgCurrentTab);}
  else stopCfgAutoRefresh();
  if(p==='hosts')loadHosts();
  if(p==='hostres'){loadNodeInfo();loadHostResList();}
  if(p==='files')loadFiles();
  if(p==='scripts'){loadKs();loadIPxeScript();}
  if(p==='images')loadImages();
  if(p==='install-records')loadInstallRecords();
if(p==='logs'){if(typeof loadLogs==='function')loadLogs();if(typeof startLogAutoRefresh==='function')startLogAutoRefresh();} else if(typeof stopLogAutoRefresh==='function')stopLogAutoRefresh();
}

// 引导脚本页内标签切换
function switchScriptTab(tab){
  document.querySelectorAll('#page-scripts .tab-bar button').forEach(b=>b.classList.toggle('active',b.dataset.stab===tab));
  document.getElementById('stab-ks').style.display = tab==='ks' ? '' : 'none';
  document.getElementById('stab-ipxe').style.display = tab==='ipxe' ? '' : 'none';
  document.getElementById('stab-deploy').style.display = tab==='deploy' ? '' : 'none';
  if(tab==='ks')loadKs();
  if(tab==='ipxe')loadIPxeScript();
  if(tab==='deploy')loadDeployScript();
}

// ---------- 工具 ----------
// 带鉴权头的文件下载：接口依赖 Authorization 头，window.open 无法携带，需 fetch 后转 Blob 下载。
function downloadAuthed(url, filename){
  showToast('正在下载...');
  fetch(url,{headers:token?{Authorization:token}:{}}).then(r=>{
    if(r.status===401){showLoginPage('登录已过期，请重新登录');throw new Error('401');}
    if(!r.ok){showToast('下载失败','error');return;}
    const disposition=r.headers.get('Content-Disposition')||'';
    let fn=filename||'download';
    const m=disposition.match(/filename\*?=(?:UTF-8'')?["']?([^"';]+)/i);
    if(m&&m[1])fn=decodeURIComponent(m[1]);
    return r.blob().then(b=>{
      const urlObj=URL.createObjectURL(b);
      const a=document.createElement('a');
      a.href=urlObj;a.download=fn;document.body.appendChild(a);a.click();
      setTimeout(()=>{URL.revokeObjectURL(urlObj);a.remove();},100);
    });
  }).catch(()=>{});
}
function val(id,v){document.getElementById(id).value=v??'';}
function gv(id){return document.getElementById(id).value.trim();}
function setSel(id,v){const el=document.getElementById(id);el.value=v==='true'?'true':'false';}
function closeModal(id){document.getElementById(id).style.display='none';}

// 加载保存的登录凭据
(function(){
  const u=localStorage.getItem('pxe_user'),p=localStorage.getItem('pxe_pass');
  if(u){document.getElementById('loginUser').value=u;}
  if(p){document.getElementById('loginPass').value=p;document.getElementById('rememberPwd').checked=true;}
})();

// 绑定登录页回车登录
bindLoginEnter();

// 启动：仅在已有 token 时尝试恢复会话，预加载共享数据。
// 未登录或 token 失效时停留在登录页，避免 401 无限刷新。
if(token){
  silentApi('/api/config').then(r=>{
    if(r && r.code===0){
      // 会话有效，预加载共享数据（静默，401 不跳登录页）
      silentApi('/api/ks/template').then(x=>{if(x&&x.code===0)ksList=x.data;});
      silentApi('/api/resource').then(x=>{if(x&&x.code===0)resList=x.data;});
      document.getElementById('loginPage').style.display='none';
      document.getElementById('app').style.display='block';
      init();
    } else {
      // token 无效，清除并停留登录页
      showLoginPage('登录已过期，请重新登录');
    }
  });
}

