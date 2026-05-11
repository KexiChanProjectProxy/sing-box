# :material-decagram: 功能

#### 界面选项

* 永不休眠（Always On）
* 包含所有网络（代理局域网和蜂窝网络流量）
* （Apple tvOS）从 iPhone/iPad 导入配置

#### 服务

SFI/SFM/SFT 支持通过 NetworkExtension 配合 Application Extension 或 System Extension 运行 sing-box。

#### TUN

SFI/SFM/SFT 通过 NetworkExtension 提供非特权 TUN 实现。

| TUN 入站配置项                | 可用             | 说明              |
|-------------------------------|------------------|-------------------|
| `interface_name`              | :material-close: | 由 Darwin 管理     |
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

| 路由/DNS 规则配置项 | 可用            | 说明                  |
|-----------------------|-----------------|-----------------------|
| `process_name`        | :material-close: | 无权限                |
| `process_path`        | :material-close: | 无权限                |
| `process_path_regex`  | :material-close: | 无权限                |
| `package_name`        | :material-close: | /                     |
| `user`                | :material-close: | 无权限                |
| `user_id`             | :material-close: | 无权限                |
| `wifi_ssid`           | :material-alert: | 仅在 iOS 上支持        |
| `wifi_bssid`          | :material-alert: | 仅在 iOS 上支持        |

### 杂项

* 崩溃日志位于「设置」→「查看服务日志」
