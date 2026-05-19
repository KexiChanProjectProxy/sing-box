---
icon: material/book-lock-open
---

# TunnelVision

TunnelVision 是一种利用 DHCP option 121 设置更高优先级路由的攻击手段，
使流量不经过 VPN。

参考：https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2024-3661

## 状态

### Android

Android 不处理 DHCP option 121，不受影响。

### Apple 平台

将 [sing-box 图形客户端](/clients/apple/#download) 更新至 `1.9.0-rc.16` 或更高版本，
然后在「设置」—「数据包隧道」中启用 `includeAllNetworks`，即可免受影响。

注意：启用 `includeAllNetworks` 后，默认的 TUN 堆栈将更改为 `gvisor`，
`system` 和 `mixed` 堆栈将不可用。

### Linux

将 sing-box 更新至 `1.9.0-rc.16` 或更高版本，由 `auto-route` 生成的规则不受影响。

### Windows

暂无解决方案。

## 缓解措施

* 不要连接不受信任的网络
* 通过另一台设备中转不受信任的网络
* 忽略它