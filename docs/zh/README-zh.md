# Backup Manager

文件/目录聚合备份可视化管理工具。基于 Git 的反向追踪模式（白名单机制），让用户以"指定要备份什么"而非"排除什么"的直观方式管理备份。

## 核心概念

```
指定要备份什么 → 内容移入仓库 → 本机路径变成软链接视图 → Git 版本管理
```

### 工作原理

1. **创建备份仓库** — 在本地路径下初始化仓库（含 `data/`、`.backup-manager/` 目录和 `.git/`）
2. **创建条目** — 选择一个要追踪的本机文件/目录。其内容被**移动**到 `data/<repo_path>`，原位置被替换为软链接 —— 即该条目的 **`in`** 链接
3. **分发（可选）** — 添加 **`out`** 链接，让同一条目出现在更多本机路径上。`in` 是 `out` 的特例：两者都是指向 `data/<repo_path>` 的软链接
4. **执行备份** — 落盘清单 → `git add -A` → `git commit` →（可选）`git push`。不存在增量同步步骤，因为内容本来就在 `data/` 中

### 仓库目录结构

```
<repo-root>/
├── .backup-manager/
│   └── manifest.json   # 设备 + 条目 + 链接（Git 跟踪，唯一事实来源）
├── data/               # 真实内容 —— 唯一的内容存放处
└── .git/               # Git 版本库
```

### 多设备

设备、条目与链接存放在仓库清单中，而不是按机器独立的本地 SQLite 数据库中 —— 因此新机器仅凭 `git clone` 就能获知所有设备的链接。注册该机器、点击 **Apply** 重建它的软链接，在文件应当出现的位置添加一条链接即可 —— 不需要提升任何东西，因为所有链接完全等价。

### 链接等价与一致性规则

- 每个条目有 **0..N 条链接**，且全部**完全等价** —— 同一目标、同一语义。没有 `in`/`out` 类型、没有跟踪链接、没有需要切换的东西。
- **没有链接的条目合法**：内容在仓库里，本机暂时没有视图。
- 系统强制一致性：一旦某个目录被跟踪，就不允许对该目录内的单个文件建链接。条目之间永不重叠，链接永远绑定完整条目、绝不绑定子路径。

任何违反上述规则的创建请求都会被拒绝，已发生的漂移由一致性巡检报告。

## 功能特性

- **仓库管理** — 创建/删除/查看备份仓库，可视化配置（远程仓库、分支、Git 用户）
- **条目与链接管理** — 每个被备份的文件/目录是一个条目，含 **0..N 条完全等价的链接**；可查看、分发、添加链接、修复、重新纳入，以及移除（unlink / move_back / purge）
- **链接状态诊断** — 逐链接状态（`ok` / `missing` / `wrong_target` / `replaced` / `dangling` / `occupied`），支持一键修复与重新纳入
- **一致性巡检** — 校验各项不变量（链接只绑定完整条目、条目不重叠、`data/` 内无软链接、未引用未登记设备），并报告未托管链接
- **多设备** — 机器指纹识别、设备注册、dry-run `apply` 重建本机链接、detach；删除设备只移除其链接定义
- **文件预览与编辑** — 纯文本/代码语法高亮、Markdown 渲染、二进制文件标识；编辑直接写入 `data/`，所有链接立即反映
- **备份执行** — 手动触发或定时自动备份（秒级 cron），Git push（可选）
- **备份历史** — 查看 Git 提交历史，支持分页，并展示未提交变更数量
- **内容回滚** — 选择历史提交版本，把 `data/` 恢复到指定版本（全量、按条目、或单文件）
- **单文件恢复** — 预览和恢复历史提交中的单个文件
- **Git 集成** — 远程仓库配置、SSH/HTTPS 认证管理（AES-256-GCM 加密存储）
- **本地文件浏览** — 无根目录白名单，服务端可见的任意路径都可浏览；可显示隐藏文件，也可直接填写完整路径
- **定时调度** — 基于 cron 的自动备份，应用启动时自动加载，配置变更时动态注册/注销
- **双语界面** — 支持在英文（`en`）与简体中文（`zh-CN`）之间切换；默认英文，选择由后端在应用范围内持久化
- **系统托盘** — macOS 菜单栏 / 系统托盘图标，支持启动/停止服务器控制
- **前后端一体** — 单二进制文件，一键启动

## 快速开始

详细图文指南请参考 [Quick Start Guide](quick-start.md)。

### 前置条件

- Go 1.22+
- Node.js 18+（仅开发需要）
- Git 2.3+

### 一键启动（生产模式）

```bash
# 下载预编译二进制或自行构建
go build -o backup-manager .
./backup-manager
# 自动在 http://localhost:9800 打开浏览器
```

### 开发模式

```bash
# 一键启动（前后端同时拉起）
./scripts/dev-start.sh

# 一键关闭
./scripts/dev-stop.sh
```

也可分别启动：

```bash
# 终端 1：启动后端
go run .

# 终端 2：启动前端开发服务器（热更新）
cd frontend && npm install && npm run dev
# 前端访问 http://localhost:5173，自动代理 /api 到后端
```

### 生产构建

```bash
cd frontend && npm install && npm run build && cd ..
go build -o backup-manager .
# 输出单二进制文件 backup-manager
```

## 架构

```
┌─────────────────────────────────────────────┐
│  Frontend (React SPA, embedded via Go embed) │
├─────────────────────────────────────────────┤
│  API Layer (Gin handlers)                   │
├─────────────────────────────────────────────┤
│  Service Layer (business logic)             │
├─────────────────────────────────────────────┤
│  Store Layer (SQLite) + Git Engine + File IO│
└─────────────────────────────────────────────┘
```

### 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.22+ (Gin, SQLite via modernc.org/sqlite) |
| 前端 | React 18 + TypeScript + Vite + Ant Design 5 |
| 国际化 | i18next + react-i18next；同步 Ant Design/dayjs 语言环境 |
| 状态管理 | Zustand |
| 定时调度 | robfig/cron/v3 |
| 加密 | AES-256-GCM |
| Markdown | react-markdown + remark-gfm |
| 打包 | Go embed (前端内嵌到二进制) |

### REST API

所有端点前缀 `/api/v1`，响应统一格式 `{"data": ...}` 或 `{"error": "..."}`。

| 分类 | 端点 | 功能 |
|------|------|------|
| 设置 | `GET /settings` | 读取应用级默认语言；响应 `{"data":{"language":"en"}}` |
| 设置 | `PUT /settings` | 持久化 `{"language":"en"}` 或 `{"language":"zh-CN"}`；响应返回相同的 `data.language` 结构 |
| 仓库 | `POST/GET/DELETE /repos` | 仓库 CRUD |
| 仓库 | `GET /repos/:id` | 仓库详情（含配置和状态） |
| 仓库 | `PUT /repos/:id/config` | 更新配置（部分更新） |
| 仓库 | `POST /repos/:id/git-init` | 初始化 Git 仓库 |
| 条目 | `GET /repos/:id/entries?device=&state=` | 条目列表（含链接与状态） |
| 条目 | `GET /repos/:id/entries/:entryId` | 条目详情 |
| 条目 | `POST /repos/:id/entries/adopt` | 创建条目及其第一条链接（移入内容） |
| 条目 | `PATCH /repos/:id/entries/:entryId` | 重命名 `repo_path`（重新校验不重叠） |
| 条目 | `DELETE /repos/:id/entries/:entryId?mode=` | `unlink` / `move_back` / `purge` |
| 链接 | `GET/POST /repos/:id/entries/:entryId/links` | 列出 / 添加链接 |
| 链接 | `POST /repos/:id/links/bulk` | 在某个本机根目录下批量创建链接 |
| 链接 | `PATCH /repos/:id/entries/:entryId/links/:linkId` | 修改 `local_path` / `enabled` |
| 链接 | `POST .../links/:linkId/repair` | 重建软链接 |
| 链接 | `POST .../links/:linkId/readopt` | `replaced` → 把新内容移入 `data/` 并重建链接 |
| 链接 | `POST .../links/:linkId/remove` | 移除单个链接 |
| 设备 | `GET /devices/current` | 当前机器的指纹 / 主机名 |
| 设备 | `GET/POST/PATCH/DELETE /repos/:id/devices[/:fp]` | 设备注册、重命名、删除 |
| 设备 | `GET /repos/:id/devices/:fp/links` | 该设备的链接及状态 |
| 设备 | `POST /repos/:id/devices/:fp/apply` | 让本机收敛（支持 dry-run） |
| 设备 | `POST /repos/:id/devices/:fp/detach` | 卸载本机 |
| 一致性 | `GET /repos/:id/consistency` | 巡检结论（R-1..R-3、未托管链接） |
| 一致性 | `POST /repos/:id/consistency/repair` | 修复所有可收敛项 |
| 内容 | `GET /repos/:id/tree?path=&include_hidden=` | 列出 `data/` 下的内容，可选显示隐藏项 |
| 内容 | `GET /repos/:id/preview?path=...` | 预览文件内容 |
| 内容 | `PUT /repos/:id/save` | 保存到 `data/` |
| 内容 | `GET /repos/:id/changes` | `data/` 下的未提交变更（`git status`） |
| 浏览 | `GET /browse?path=...&include_hidden=true` | 浏览任意本地目录（展开 `~`，可选显示隐藏文件） |
| 浏览 | `GET /browse/home` | 浏览默认起始目录（服务端家目录） |
| 备份 | `POST /repos/:id/backup` | 触发备份（可指定 commit_message） |
| 备份 | `GET /repos/:id/backup/history?limit=&offset=` | 备份历史（分页） |
| 备份 | `POST /repos/:id/push` | 推送到远程仓库（可选 force 参数） |
| 回滚 | `GET /repos/:id/commits/:hash/changed-files` | 提交中变更的文件列表 |
| 回滚 | `GET /repos/:id/commits/:hash/files?path=` | 预览提交中的文件内容 |
| 回滚 | `POST /repos/:id/commits/:hash/restore` | 从提交恢复单个文件 |
| 回滚 | `POST /repos/:id/rollback` | 批量回滚 `data/` 到历史版本 |
| 认证 | `GET/PUT/DELETE /repos/:id/auth` | Git 认证管理 |
| 系统 | `GET /health` | 健康检查（状态+运行时间+版本） |

## 使用流程

```
1. 启动应用 → 系统托盘图标出现在菜单栏
2. 点击托盘图标 → "Open UI" 打开浏览器
3. 仪表盘显示仓库列表
4. 点击"创建仓库" → 输入名称、选择路径
5. 进入仓库详情 → Entries 标签页 → "+ New Entry" → 选择本机源路径，并手动填写目标路径或在 `data/` 下可视化选择/新建待创建父目录；选择点号开头的目录时开启“显示隐藏文件”（确认后才移动内容，原位置成为它的第一条链接）
6. （可选）添加更多链接：可输入完整本机路径，或可视化选择父目录并自动追加条目名称
7. 在 Browse 标签页浏览、预览和编辑内容
8. 切换到备份标签页 → 点击"触发备份"
9. 配置远程仓库和认证信息（可选）
10. 设置定时备份（可选）
11. 在备份历史中选择提交 → 回滚（可选）
12. 在另一台机器上：克隆仓库、打开它，点击 "Apply" 重建该机器的链接
```

## 环境配置

| 路径 | 说明 |
|------|------|
| `~/.config/backup-manager/config.json` | 应用配置 —— JSON 字段：`port`、`open_browser`、`theme`、`language`；`language` 是应用级 UI 默认语言（`en` 或 `zh-CN`，默认 `en`） |
| `~/.config/backup-manager/master.key` | AES-256 加密密钥（首次启动自动生成） |
| `~/.config/backup-manager/backup-manager.db` | SQLite 数据库（本机，按机器独立）—— 存 `repos`、`repo_configs`、`repo_auths` |
| `<repo-root>/.backup-manager/manifest.json` | **位于仓库内**，由 Git 跟踪 —— 存条目、链接与设备。它**不是** SQLite 数据库；放在仓库里才能随 `git clone` / `git push` 传输 |

## 安全设计

- **路径安全**: 四层校验（Clean→Abs→EvalSymlinks→Prefix）防止路径穿越；浏览无根目录白名单（服务端可见的任意路径，展开 `~`，隐藏文件可选显示）；链接 `local_path` 拒绝指向仓库内部的自引用；新建/写入的内容一律限定在 `<repo>/data/` 内（拒绝绝对路径与 `..`，解析软链接后再校验）
- **链接一致性**: 每次写入前按 R-1..R-3 校验清单（链接只绑定完整条目；条目不重叠；链接不在目录条目内部）；文件无法解析时阻止写入，不合规时由巡检报告，而不是静默重写
- **清单原子写**: `manifest.json.tmp` → `fsync` → `os.Rename`
- **默认非破坏性**: 建链接拒绝已占用路径；`apply` 从不覆盖；删除内容需输入路径二次确认，且删除前先提交，因此 `git revert` 可恢复
- **认证加密**: SSH 私钥和 HTTPS 密码使用 AES-256-GCM 加密存储
- **并发控制**: 仓库级互斥锁串行化备份、回滚以及所有会改文件系统的链接操作；预览接口限流（最大 5 并发）
- **错误隔离**: Git push 失败不阻断本地 commit

## 开发

### 测试

```bash
# 运行所有 Go 测试
go test ./... -count=1

# 前端类型检查
cd frontend && npx tsc --noEmit
```

### 项目结构

```
backup-manager/
├── main.go                     # 入口：初始化各模块 → 启动 HTTP → 优雅关闭
├── scripts/
│   ├── dev-start.sh            # 一键启动开发环境
│   └── dev-stop.sh             # 一键关闭开发环境
├── internal/                   # 后端代码
│   ├── api/                    # API 层（路由 + 处理器）
│   │   ├── router.go           # 路由注册 + SPA 挂载
│   │   ├── middleware.go       # CORS + 错误恢复
│   │   └── handler/            # HTTP 处理器
│   │       ├── repo.go         # 仓库 CRUD + Git Init
│   │       ├── entry.go        # 条目 list / adopt / delete
│   │       ├── link.go         # 链接 add / bulk / repair / remove
│   │       ├── device.go       # 设备 current / register / rename / delete / apply
│   │       ├── consistency.go  # 一致性巡检 + 修复
│   │       ├── browse.go       # 本地文件浏览（任意路径 + 隐藏文件）
│   │       ├── content.go      # tree / preview / save / changes
│   │       ├── backup.go       # 备份触发 + 历史查询 + Push
│   │       ├── auth.go         # Git 认证管理
│   │       ├── rollback.go     # 内容回滚 + 单文件恢复
│   │       ├── system.go       # 健康检查 + 应用设置
│   │       └── errors.go       # 错误码映射
│   ├── appconfig/              # config.json 加载与持久化
│   ├── entry/                  # 条目与链接子系统
│   │   ├── manifest.go         # 清单加载/保存/原子写 + R-1..R-3 校验
│   │   ├── service.go          # Service 装配、仓库互斥锁、清单提交、公共辅助
│   │   ├── entry_service.go    # adopt、list、remove（unlink/move_back/purge）
│   │   ├── link_service.go     # 添加链接、批量链接、repair、readopt、remove
│   │   ├── device_service.go   # register、rename、delete、apply
│   │   ├── entry_state.go      # 逐链接状态诊断与视图构建
│   │   └── consistency.go      # 一致性巡检 + 修复
│   ├── service/                # 业务逻辑层
│   │   ├── repo_service.go     # 仓库生命周期
│   │   ├── backup_service.go   # 备份执行（git add/commit/push）
│   │   ├── auth_service.go     # Git 认证管理
│   │   ├── browser_service.go  # 本地文件浏览（任意路径 + 隐藏文件）
│   │   ├── content_service.go  # 内容树 / 预览 / 保存
│   │   └── rollback_service.go # 回滚逻辑
│   ├── store/                  # 数据持久化层
│   │   ├── db.go               # SQLite 初始化 + 迁移
│   │   ├── store.go            # Store 聚合
│   │   ├── repo_store.go       # repos 表操作
│   │   ├── repo_config_store.go# repo_configs 表操作
│   │   └── repo_auth_store.go  # repo_auths 表操作
│   ├── model/                  # 数据模型
│   │   ├── repo.go             # Repo, RepoConfig, RepoStatus
│   │   ├── link.go             # Entry, Link, Device, Manifest, LinkState
│   │   └── auth.go             # GitAuth, GitAuthType
│   ├── git/                    # Git 引擎
│   │   └── git.go              # Init/Add/Commit/Push/Log/Status/Config/LsTree/Show/WriteFileContentTo
│   ├── scheduler/              # 定时调度器
│   │   └── scheduler.go        # 基于 cron 的注册/注销
│   ├── servermgr/              # HTTP 服务器生命周期管理
│   ├── shortcut/               # 桌面快捷方式创建
│   ├── tray/                   # 系统托盘（菜单栏）管理
│   └── util/                   # 工具包
│       ├── path.go             # SafeResolve 四层路径校验
│       ├── crypto.go           # KeyManager (AES-256-GCM)
│       ├── device.go           # MachineFingerprint()
│       ├── repo_mutex.go       # 仓库级互斥锁（备份/回滚/链接操作共享）
│       └── file.go             # CopyFile/CopyDir/DetectMIME
└── frontend/                   # React SPA
    ├── package.json
    ├── vite.config.ts           # 开发代理 /api → localhost:9800
    └── src/
        ├── main.tsx             # React 入口
        ├── App.tsx              # 路由配置
        ├── App.css              # 全局样式
        ├── api/client.ts        # axios 实例 + 所有 API 函数
        ├── i18n/                # 中英文资源 + 语言环境同步
        ├── types/index.ts       # TypeScript 类型定义
        ├── store/appStore.ts    # Zustand 状态管理
        ├── routes/              # 页面组件
        │   ├── Dashboard.tsx    # 仓库列表
        │   └── RepoDetail.tsx   # 仓库详情（4 个 Tab）
        └── components/          # 功能组件
            ├── layout/
            │   ├── AppLayout.tsx
            │   └── Sidebar.tsx
            ├── repo/
            │   ├── RepoCard.tsx
            │   └── CreateRepoModal.tsx
            ├── entry/
            │   ├── EntriesPanel.tsx        # 条目与链接：列表、指定跟踪、修复、移除、apply
            │   └── AdoptModal.tsx          # 创建条目（内容移入仓库）
            ├── files/
            │   └── FilesPanel.tsx          # 浏览 data/ 树 + 预览编辑
            ├── preview/
            │   ├── PreviewPanel.tsx
            │   ├── TextPreview.tsx
            │   ├── MarkdownPreview.tsx
            │   └── BinaryInfo.tsx
            ├── backup/
            │   ├── BackupPanel.tsx
            │   ├── RollbackConfirmModal.tsx
            │   └── RollbackResultModal.tsx
            ├── common/
            │   ├── DirectoryPickerModal.tsx
            │   └── RepositoryDirectoryPickerModal.tsx  # data/ 范围内的父目录选择器
            └── config/
                └── ConfigPanel.tsx
```

## License

MIT
