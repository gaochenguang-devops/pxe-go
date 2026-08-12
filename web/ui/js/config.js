// ---------- 配置 ----------
let cfgRefreshTimer = null;
let cfgCurrentTab = 'dhcp';

function loadConfig(){
  api('/api/config').then(r=>{
    if(r.code!==0)return;const c=r.data;
    setChk('dhcpEnabledSw',c.dhcp_enabled);
    setStatusText('dhcpStatusText',c.dhcp_enabled);
    val('dhcpListenIp',c.dhcp_listen_ip);val('dhcpInterface',c.dhcp_interface);val('dhcpPxeIp',c.dhcp_pxe_ip);
    val('dhcpLease',c.dhcp_lease_time);val('dhcpBios',c.dhcp_boot_file_bios);val('dhcpX86',c.dhcp_boot_file_x86);
    val('dhcpArm',c.dhcp_boot_file_arm);val('dhcpIpxe',c.dhcp_ipxe_script);
    setChk('tftpEnabledSw',c.tftp_enabled);
    setStatusText('tftpStatusText',c.tftp_enabled);
    val('tftpListenIp',c.tftp_listen_ip);val('tftpRoot',c.tftp_root_dir);
    val('tftpTimeout',c.tftp_transfer_timeout);val('tftpMaxConn',c.tftp_max_connections);setSel('tftpAccessLog',c.tftp_access_log);
    val('httpListenAddr',c.http_listen_addr);val('httpWebRoot',c.http_web_root);
  });
  loadSubnets();
}

function saveDhcp(){api('/api/config/dhcp','PUT',{
  enabled:document.getElementById('dhcpEnabledSw').checked,
  listen_ip:gv('dhcpListenIp'),interface:gv('dhcpInterface'),pxe_ip:gv('dhcpPxeIp'),
  lease_time:parseInt(gv('dhcpLease'))||0,
  boot_file_bios:gv('dhcpBios'),boot_file_x86:gv('dhcpX86'),boot_file_arm:gv('dhcpArm'),ipxe_script:gv('dhcpIpxe')
}).then(r=>{if(r.code===0){showToast(r.msg);loadConfig();loadCfgLog('dhcp');}else showToast(r.msg,'error');});}
function saveTftp(){api('/api/config/tftp','PUT',{
  enabled:document.getElementById('tftpEnabledSw').checked,
  listen_ip:gv('tftpListenIp'),root_dir:gv('tftpRoot'),
  transfer_timeout:parseInt(gv('tftpTimeout'))||0,max_connections:parseInt(gv('tftpMaxConn'))||0,
  access_log:document.getElementById('tftpAccessLog').value==='true'
}).then(r=>{if(r.code===0){showToast(r.msg);loadConfig();loadCfgLog('tftp');}else showToast(r.msg,'error');});}
function saveHttp(){
  api('/api/config/http','PUT',{
    listen_addr:gv('httpListenAddr'),web_root:gv('httpWebRoot')
  }).then(r=>{if(r.code===0){showToast(r.msg);loadConfig();loadCfgLog('http');}else showToast(r.msg,'error');});
}

// ---------- Tab 切换 ----------
function switchCfgTab(tab){
  cfgCurrentTab = tab;
  document.querySelectorAll('#page-config .tab-bar button').forEach(t=>t.classList.toggle('active', t.dataset.tab===tab));
  document.querySelectorAll('.cfg-panel').forEach(p=>p.classList.remove('active'));
  document.getElementById('cfg-panel-'+tab).classList.add('active');
  loadCfgLog(tab);
  loadFileLog(tab);
}

// ---------- 日志加载 + 自动刷新 ----------
function loadCfgLog(module){
  const el = document.getElementById('cfgLog'+module.charAt(0).toUpperCase()+module.slice(1));
  if(!el) return;
  const tb = el.querySelector('tbody');
  api('/api/operlog?module='+module+'&limit=30').then(r=>{
    if(r.code!==0 || !r.data || r.data.length===0){
      tb.innerHTML = '<tr><td class="empty">暂无操作日志</td></tr>';
      return;
    }
    tb.innerHTML = r.data.map(l=>{
      const time = l.op_time ? l.op_time.replace('T',' ').substring(0,19) : '';
      const type = esc(l.op_type||'op');
      return `<tr>
        <td class="lt-time">${time}</td>
        <td><span class="op-badge op-${opTypeClass(type)}">${type}</span></td>
        <td class="lt-msg">${esc(l.detail||'')}</td>
      </tr>`;
    }).join('');
  });
}
// 操作类型 → 徽章配色类
function opTypeClass(t){
  const map={cfg:'purple',auth:'blue',host:'green',image:'cyan',ks:'orange',ipxe:'orange',file:'blue',resource:'green',deploy:'purple',node:'green',other:'gray',default:'gray'};
  for(const k in map){ if(t.startsWith(k)) return map[k]; }
  return 'gray';
}

function startCfgAutoRefresh(){
  stopCfgAutoRefresh();
  cfgRefreshTimer = setInterval(()=>{
    if(cfgCurrentTab){
      loadCfgLog(cfgCurrentTab);
      loadFileLog(cfgCurrentTab);
    }
  }, 10000); // 每10秒自动刷新
}

// ---------- 文件日志（客户端请求日志等）----------
function loadFileLog(module){
  const el = document.getElementById('cfgFileLog'+module.charAt(0).toUpperCase()+module.slice(1));
  if(!el) return;
  const filter = module.toUpperCase(); // DHCP / TFTP / HTTP 关键词过滤
  api('/api/logfile?lines=60&filter='+encodeURIComponent(filter)).then(r=>{
    if(r.code!==0 || !r.data || r.data.length===0){
      el.innerHTML = '<div class="t-empty">暂无日志</div>';
      return;
    }
    // 倒序展示（最新在顶部），带行号
    const lines = [...r.data].reverse();
    el.innerHTML = lines.map((line,i)=>{
      const num = String(i+1).padStart(3,'0');
      const clean = esc(line);
      let cls = '';
      if(line.includes('[WARN]')) cls = 't-warn';
      else if(line.includes('[ERROR]')) cls = 't-err';
      return `<div class="t-line ${cls}"><span class="t-num">${num}</span><span class="t-text">${clean}</span></div>`;
    }).join('');
  });
}

function stopCfgAutoRefresh(){
  if(cfgRefreshTimer){clearInterval(cfgRefreshTimer);cfgRefreshTimer=null;}
}

// ---------- 子网管理 ----------
let subnetEditId = 0;

function loadSubnets(){
  api('/api/config/dhcp/subnets').then(r=>{
    const tbody = document.querySelector('#subnetTable tbody');
    if(!tbody) return;
    if(r.code!==0){ tbody.innerHTML = '<tr><td colspan="7" class="empty">加载失败：'+esc(r.msg||'')+'</td></tr>'; return; }
    const list = r.data || [];
    const tag=document.getElementById('dhcpSubnetTag');
    if(tag){const on=list.filter(s=>s.enabled).length;tag.textContent='子网 '+on+' 个';}
    if(list.length===0){ tbody.innerHTML = '<tr><td colspan="7" class="empty">暂无子网，点击右上角「新增子网」添加</td></tr>'; return; }
    tbody.innerHTML = list.map(s=>`<tr>
      <td>${esc(s.name||'-')}</td>
      <td class="mono">${esc(s.ip_pool_start)} ~ ${esc(s.ip_pool_end)}</td>
      <td class="mono">${esc(s.subnet_mask)}</td>
      <td class="mono">${esc(s.gateway||'-')}</td>
      <td class="mono">${esc(s.dns_servers||'-')}</td>
      <td>${s.enabled?'<span class="badge badge-green">启用</span>':'<span class="badge badge-gray">停用</span>'}</td>
      <td>
        <button class="btn btn-sm" onclick="openSubnetModal(${s.id})">编辑</button>
        <button class="btn btn-sm btn-danger" onclick="deleteSubnet(${s.id})">删除</button>
      </td>
    </tr>`).join('');
  });
}

function openSubnetModal(id){
  subnetEditId = id||0;
  document.getElementById('subnetModal').style.display='flex';
  const title = document.querySelector('#subnetModal h3');
  title.textContent = id ? '编辑子网' : '新增子网';
  if(id){
    api('/api/config/dhcp/subnets').then(r=>{
      const s = (r.data||[]).find(x=>x.id===id);
      if(s){
        val('subnetName',s.name);val('subnetStart',s.ip_pool_start);val('subnetEnd',s.ip_pool_end);
        val('subnetMask',s.subnet_mask);val('subnetGateway',s.gateway);val('subnetDns',s.dns_servers);
        setSel('subnetEnabled',s.enabled?'true':'false');
      }
    });
  } else {
    val('subnetName','');val('subnetStart','');val('subnetEnd','');val('subnetMask','');val('subnetGateway','');val('subnetDns','');
    setSel('subnetEnabled','true');
  }
}

function closeSubnetModal(){ document.getElementById('subnetModal').style.display='none'; }

function submitSubnet(){
  const body = {
    name:gv('subnetName'), ip_pool_start:gv('subnetStart'), ip_pool_end:gv('subnetEnd'),
    subnet_mask:gv('subnetMask'), gateway:gv('subnetGateway'), dns_servers:gv('subnetDns'),
    enabled:document.getElementById('subnetEnabled').value==='true'
  };
  if(!body.ip_pool_start || !body.ip_pool_end || !body.subnet_mask){ showToast('地址池起止与掩码必填','error'); return; }
  const url = subnetEditId ? '/api/config/dhcp/subnets/'+subnetEditId : '/api/config/dhcp/subnets';
  const method = subnetEditId ? 'PUT' : 'POST';
  api(url, method, body).then(r=>{
    if(r.code===0){ showToast(r.msg); closeSubnetModal(); loadSubnets(); }
    else showToast(r.msg,'error');
  });
}

function deleteSubnet(id){
  if(!confirm('确定删除该子网吗？')) return;
  api('/api/config/dhcp/subnets/'+id,'DELETE').then(r=>{
    if(r.code===0){ showToast(r.msg); loadSubnets(); }
    else showToast(r.msg,'error');
  });
}
