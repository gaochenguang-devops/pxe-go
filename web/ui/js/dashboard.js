// ---------- 仪表盘 ----------
function init(){
  loadDashboard();
  loadConfig();
  loadHosts();
  loadImages();
  loadKs();
  setInterval(()=>{document.getElementById('clock').textContent=new Date().toLocaleString();},1000);
}
function loadDashboard(){
  // 服务状态（从后端实时检测）
  api('/api/status').then(r=>{
    if(r.code!==0)return;
    const data=r.data||[];
    window.__svcStatus=data;
    const st=document.getElementById('serviceStatus');
    st.innerHTML=(r.data||[]).map(x=>{
      const isRunning=x.status==='running';
      return `<div class="svc-item" onclick="switchPage('config')" style="cursor:pointer">
        <span class="svc-dot" style="background:${isRunning?'var(--ok)':'var(--err)'}"></span>
        <div><div class="svc-name">${esc(x.name)}</div><div class="svc-port">${esc(x.port)}</div></div>
        <span style="margin-left:auto;font-size:12px;color:${isRunning?'var(--ok)':'var(--err)'}">${isRunning?'运行中':'已停止'}</span>
      </div>`;
    }).join('');
  }).catch(()=>{});

  // 快速统计（主机/镜像/KS/iPXE/子网/装机记录）
  const stats=[
    ['主机数量','host',0,'/api/host'],
    ['系统镜像','image',0,'/api/image'],
    ['KS 模板','ks',0,'/api/ks/template'],
    ['iPXE 脚本','ipxe',0,'/api/ipxe/script'],
    ['DHCP 子网','subnet',0,'/api/config/dhcp/subnets']
  ];
  Promise.all([
    ...stats.map(s=>api(s[3]).then(r=>({key:s[1],n:(r&&r.code===0&&r.data)?r.data.length:0,err:false})).catch(()=>({key:s[1],n:'?',err:true}))),
    api('/api/install-records?limit=1').then(r=>({key:'install',n:r.total||0,sub:('成功 '+r.success+' / 失败 '+r.failed)||'',err:false})).catch(()=>({key:'install',n:'?',sub:'',err:true}))
  ]).then(rs=>{
    const map={};rs.forEach(x=>map[x.key]=x);
    const colors=['var(--accent2)','var(--accent)','var(--ok)','var(--warn)','var(--accent)'];
    const pageMap={host:'hosts',image:'images',ks:'scripts',ipxe:'scripts',subnet:'config',install:'install-records'};
    document.getElementById('quickStats').innerHTML=[
      ...stats.map((s,i)=>`
        <div class="stat-card" onclick="switchPage('${pageMap[s[1]]}')" style="cursor:pointer">
          <div class="stat-label">${s[0]}</div>
          <div class="stat-value" style="color:${colors[i]}">${map[s[1]].n}</div>
          <div class="stat-sub">总数统计</div>
        </div>`),
      `<div class="stat-card" onclick="switchPage('install-records')" style="cursor:pointer">
        <div class="stat-label">装机记录</div>
        <div class="stat-value" style="color:var(--accent2)">${map['install'].n}</div>
        <div class="stat-sub">${map['install'].sub||'完成装机数'}</div>
      </div>`
    ].join('');
  });

  // 配置概览（服务状态取自 /api/status 实时检测结果）
  Promise.all([api('/api/config'), api('/api/config/dhcp/subnets')]).then(([r, sr])=>{
    if(r.code!==0)return;const c=r.data;
    const subList=(sr&&sr.code===0&&sr.data)||[];
    const subnetCnt=subList.filter(s=>s.enabled).length;
    const getStatus=(name)=>{
      const cached=window.__svcStatus;
      if(cached){
        const e=cached.find(x=>x.name===name);
        if(e) return e.status==='running';
      }
      return c.dhcp_enabled&&c.tftp_enabled;
    };
    const dhcpOn=getStatus('DHCP 服务')?'已启用':'已停止';
    const tftpOn=getStatus('TFTP 服务')?'已启用':'已停止';
    const cfg=[['DHCP',dhcpOn],['TFTP',tftpOn],
      ['HTTP 监听',c.http_listen_addr],['PXE 服务 IP',c.dhcp_pxe_ip],
      ['DHCP 子网',subnetCnt+' 个'],['租期',c.dhcp_lease_time+' 秒']];
    document.getElementById('configOverview').innerHTML=cfg.map(x=>{
      const isStop=x[1]==='已停止';
      return `<div class="field"><span>${x[0]}</span><div style="margin-top:4px;font-weight:600;color:${isStop?'var(--err)':'var(--txt)'}">${x[1]}</div></div>`;
    }).join('');
  }).catch(()=>{});

  // 最近操作日志
  api('/api/operlog?limit=8').then(r=>{
    if(r.code!==0)return;
    document.getElementById('dashLogTbody').innerHTML=(r.data||[]).map(x=>`<tr>
      <td>${esc(x.operator)}</td><td><span class="badge badge-blue">${esc(x.op_type)}</span></td>
      <td>${esc(x.detail)}</td><td>${esc(x.client_ip)}</td><td>${fmtTime(x.op_time)}</td>
    </tr>`).join('')||'<tr><td colspan="5"><div class="empty">暂无日志</div></td></tr>';
  });
}