// ---------- 配置 ----------
let cfgRefreshTimer = null;
let cfgCurrentTab = 'dhcp';

function loadConfig(){
  api('/api/config').then(r=>{
    if(r.code!==0)return;const c=r.data;
    setSel('dhcpEnabled',c.dhcp_enabled);
    val('dhcpListenIp',c.dhcp_listen_ip);val('dhcpInterface',c.dhcp_interface);val('dhcpPxeIp',c.dhcp_pxe_ip);
    val('dhcpPoolStart',c.dhcp_ip_pool_start);val('dhcpPoolEnd',c.dhcp_ip_pool_end);
    val('dhcpMask',c.dhcp_subnet_mask);val('dhcpGateway',c.dhcp_gateway);val('dhcpDns',c.dhcp_dns_servers);
    val('dhcpLease',c.dhcp_lease_time);val('dhcpBios',c.dhcp_boot_file_bios);val('dhcpX86',c.dhcp_boot_file_x86);
    val('dhcpArm',c.dhcp_boot_file_arm);val('dhcpIpxe',c.dhcp_ipxe_script);
    setSel('tftpEnabled',c.tftp_enabled);val('tftpListenIp',c.tftp_listen_ip);val('tftpRoot',c.tftp_root_dir);
    val('tftpTimeout',c.tftp_transfer_timeout);val('tftpMaxConn',c.tftp_max_connections);setSel('tftpAccessLog',c.tftp_access_log);
    val('httpListenAddr',c.http_listen_addr);val('httpWebRoot',c.http_web_root);
  });
}

function saveDhcp(){api('/api/config/dhcp','PUT',{
  enabled:document.getElementById('dhcpEnabled').value==='true',
  listen_ip:gv('dhcpListenIp'),interface:gv('dhcpInterface'),pxe_ip:gv('dhcpPxeIp'),
  ip_pool_start:gv('dhcpPoolStart'),ip_pool_end:gv('dhcpPoolEnd'),subnet_mask:gv('dhcpMask'),
  gateway:gv('dhcpGateway'),dns_servers:gv('dhcpDns'),lease_time:parseInt(gv('dhcpLease'))||0,
  boot_file_bios:gv('dhcpBios'),boot_file_x86:gv('dhcpX86'),boot_file_arm:gv('dhcpArm'),ipxe_script:gv('dhcpIpxe')
}).then(r=>{if(r.code===0){showToast(r.msg);loadConfig();loadCfgLog('dhcp');}else showToast(r.msg,'error');});}
function saveTftp(){api('/api/config/tftp','PUT',{
  enabled:document.getElementById('tftpEnabled').value==='true',
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
  document.querySelectorAll('.cfg-tab').forEach(t=>t.classList.toggle('active', t.dataset.tab===tab));
  document.querySelectorAll('.cfg-panel').forEach(p=>p.classList.remove('active'));
  document.getElementById('cfg-panel-'+tab).classList.add('active');
  loadCfgLog(tab);
  loadFileLog(tab);
}

// ---------- 日志加载 + 自动刷新 ----------
function loadCfgLog(module){
  const el = document.getElementById('cfgLog'+module.charAt(0).toUpperCase()+module.slice(1));
  if(!el) return;
  api('/api/operlog?module='+module+'&limit=30').then(r=>{
    if(r.code!==0 || !r.data || r.data.length===0){
      el.innerHTML = '<div class="empty">暂无日志</div>';
      return;
    }
    el.innerHTML = r.data.map(l=>{
      const time = l.op_time ? l.op_time.replace('T',' ').substring(0,19) : '';
      return `<div class="cfg-log-item">
        <span class="cfg-log-time">${time}</span>
        <span class="cfg-log-action">${esc(l.op_type)}</span>
        <span class="cfg-log-msg">${esc(l.detail||'')}</span>
      </div>`;
    }).join('');
  });
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
  api('/api/logfile?lines=50&filter='+encodeURIComponent(filter)).then(r=>{
    if(r.code!==0 || !r.data || r.data.length===0){
      el.innerHTML = '<div class="empty">暂无日志</div>';
      return;
    }
    el.innerHTML = r.data.map(line=>{
      const clean = esc(line);
      // 根据日志级别着色
      let cls = '';
      if(line.includes('[WARN]')) cls = 'cfg-log-warn';
      else if(line.includes('[ERROR]')) cls = 'cfg-log-err';
      return `<div class="cfg-log-item ${cls}">
        <span class="cfg-log-msg" style="font-family:ui-monospace,Menlo,monospace;white-space:pre-wrap;word-break:break-all">${clean}</span>
      </div>`;
    }).join('');
    // 自动滚动到底部
    el.scrollTop = el.scrollHeight;
  });
}

function stopCfgAutoRefresh(){
  if(cfgRefreshTimer){clearInterval(cfgRefreshTimer);cfgRefreshTimer=null;}
}
