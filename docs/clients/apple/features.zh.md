# :material-decagram: 功能

#### UI 选项

* 始终开启
* 包含所有网络（为 LAN 和蜂窝服务代理流量）
* (Apple tvOS) 从 iPhone/iPad 导入配置

#### 服务

SFI/SFM/SFT 允许您通过 NetworkExtension 与 Application Extension 或 System Extension 运行 sing-box。

#### TUN

SFI/SFM/SFT 通过 NetworkExtension 提供非特权 TUN 实现。

| TUN 入站选项                   | 可用          | 备注              |
|-------------------------------|---------------|-------------------|
| `interface_name`              | :material-close: | 由 Darwin 管理    |
| `inet4_address`               | :material-check: | /                 |
| `inet6_address`               | :material-check: | /                 |
| `mtu`                         | :material-check: | /                 |
| `gso`                         | :material-close: | 未实现            |
| `auto_route`                  | :material-check: | /                 |
| `strict_route`                | :material-close: | 未实现            |
| `inet4_route_address`         | :material-check: | /                 |
| `inet6_route_address`         | :material-check: | /                 |
| `inet4_route_exclude_address` | :material-check: | /                 |
| `inet6_route_exclude_address` | :material-check: | /                 |
| `endpoint_independent_nat`    | :material-check: | /                 |
| `stack`                       | :material-check: | /                 |
| `include_interface`           | :material-close: | 未实现            |
| `exclude_interface`           | :material-close: | 未实现            |
| `include_uid`                 | :material-close: | 未实现            |
| `exclude_uid`                 | :material-close: | 未实现            |
| `include_android_user`        | :material-close: | 未实现            |
| `include_package`             | :material-close: | 未实现            |
| `exclude_package`             | :material-close: | 未实现            |
| `platform`                    | :material-check: | /                 |

| 路由/DNS 规则选项 | 可用          | 备注                  |
|--------------------|--------------|-----------------------|
| `process_name`     | :material-close: | 无权限               |
| `process_path`     | :material-close: | 无权限               |
| `process_path_regex` | :material-close: | 无权限              |
| `package_name`     | :material-close: | /                     |
| `package_name_regex` | :material-close: | /                    |
| `user`             | :material-close: | 无权限               |
| `user_id`          | :material-close: | 无权限               |
| `wifi_ssid`        | :material-alert: | 仅 iOS 支持          |
| `wifi_bssid`       | :material-alert: | 仅 iOS 支持          |

### 杂项

* 崩溃日志位于「设置」->「查看服务日志」
