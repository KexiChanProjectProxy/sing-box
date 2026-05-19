---
icon: material/pencil-ruler
---

# 通用说明

描述和解释 sing-box 图形客户端统一实现的功能。

### 配置

配置描述了一个 sing-box 配置文件及其状态。

#### 本地

* 本地配置表示具有最小状态的本地 sing-box 配置
* 图形客户端必须提供用于修改配置内容的编辑器

#### iCloud（iOS 和 macOS）

* iCloud 配置表示以 iCloud 为更新源的远程 sing-box 配置
* 配置文件存储在 iCloud 下的 sing-box 文件夹中
* 图形客户端必须提供用于修改配置内容的编辑器

#### 远程

* 远程配置表示以 URL 为更新源的远程 sing-box 配置。
* 图形客户端应提供配置内容查看器
* 图形客户端必须实现自动配置更新（默认间隔为 60 分钟）和 HTTP Basic
  认证。

同时，图形客户端必须提供通过特定 URL Scheme 导入远程配置的支持。
URL 定义如下：

```
sing-box://import-remote-profile?url=urlEncodedURL#urlEncodedName
```

### 控制面板

sing-box 服务运行时，图形客户端应提供用于管理服务的控制面板界面。

#### 状态

控制面板应显示内存、连接和流量等状态信息。

#### 模式

当配置使用至少两个 `clash_mode` 值时，控制面板应提供模式选择器用于切换。

#### 分组

当配置包含分组出口（特别是 Selector 或 URLTest）时，
控制面板应提供分组选择器用于状态显示或切换。

### 杂项

#### 核心

图形客户端应提供核心区域：

* 显示当前 sing-box 版本
* 提供用于清理工作目录的按钮
* 提供内存限制器开关
