---
icon: material/pencil-ruler
---

# 通用

描述并解释由 sing-box 图形客户端统一实现的功能。

### 配置

配置（Profile）用于描述一个 sing-box 配置文件及其状态。

#### 本地

* 本地配置表示一个具有最小状态的本地 sing-box 配置
* 图形客户端必须提供编辑器来修改配置内容

#### iCloud（iOS 和 macOS）

* iCloud 配置表示以 iCloud 为更新来源的远程 sing-box 配置
* 配置文件存储在 iCloud 下的 sing-box 文件夹中
* 图形客户端必须提供编辑器来修改配置内容

#### 远程

* 远程配置表示以 URL 为更新来源的远程 sing-box 配置
* 图形客户端应提供配置内容查看器
* 图形客户端必须实现自动更新配置功能（默认间隔为 60 分钟）以及 HTTP Basic 认证

同时，图形客户端必须支持通过特定的 URL Scheme 导入远程配置。
URL 定义如下：

```
sing-box://import-remote-profile?url=urlEncodedURL#urlEncodedName
```

### 仪表盘

当 sing-box 服务运行时，图形客户端应提供仪表盘界面来管理服务。

#### 状态

仪表盘应显示内存、连接和流量等状态信息。

#### 模式

当配置使用了至少两个 `clash_mode` 值时，仪表盘应提供模式切换器。

#### 分组

当配置包含分组出站代理（特别是 Selector 或 URLTest）时，
仪表盘应提供分组选择器，用于状态显示或切换。

### 杂项

#### 核心

图形客户端应提供核心区域：

* 显示当前 sing-box 版本
* 提供清理工作目录的按钮
* 提供内存限制器开关
