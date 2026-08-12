#!/bin/bash
Ifaces=$(ls /sys/class/net/ | egrep '^eth|^en|^em|^p')

sn=$(cat /sys/devices/virtual/dmi/id/product_serial)
hostname=$(hostname)

info=""
for iface in $Ifaces;do
  switch_info=''
  #ethtool --set-priv-flags $iface disable-fw-lldp on > /dev/null 2>&1
  lldptool set-lldp -i $iface adminStatus=rxtx > /dev/null 2>&1
  lldptool -T -i $iface -V  sysName enableTx=yes > /dev/null 2>&1
  lldptool -T -i $iface -V  portDesc enableTx=yes > /dev/null 2>&1
  lldptool -T -i $iface -V  sysDesc enableTx=yes > /dev/null 2>&1
  lldptool -T -i $iface -V sysCap enableTx=yes > /dev/null 2>&1
  lldptool -T -i $iface -V mngAddr enableTx=yes > /dev/null 2>&1
  port_info=$(lldptool -t -n -i $iface -V portID | egrep -v 'Port ID|Agent' | awk -F: '{print $2}')
  switch_name=$(lldptool -t -n -i $iface -V sysName | egrep -v 'System Name|Agent')
  mac=$(ethtool -P $iface | awk '{print $3}')

  echo "$hostname $iface $mac $switch_name $port_info"
done

#echo "$hostname $sn $info"

