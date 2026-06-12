# Backup Manager

文件/目录聚合备份可视化管理工具。基于 Git 的反向追踪模式（白名单机制），让用户以"指定要备份什么"而非"排除什么"的直观方式管理备份。

## 核心概念

```
指定哪些文件需要备份 → 自动创建软链接聚合 → 增量同步到备份仓库 → Git 版本管理
```

### 工作原理

1. **创建备份仓库** — 在本地路径下初始化仓库（含 `.links/`、`data/` 目录和 `.git/`）
2. **添加软链接** — 选择要追踪的源文件/目录，自动在 `.links/` 中创建软链接，同时复制源文件到 `data/`
3. **执行备份** — 增量检测（mtime+size）→ 同步变更到 `data/` → `git add` → `git commit` →（可选）`git push`

### 仓库目录结构

```
<repo-root>/
├── .links/        # 软链接目录（指向源文件）
├── data/          # 实际备份数据（与 .links/ 目录结构一致）
└── .git/          # Git 版本库
```

## 功能特性

- **仓库管理** — 创建/删除/查看备份仓库，可视化配置（远程仓库、分支、Git 用户）
- **软链接管理** — 树形展示、添加/删除/批量导入、修改目标路径、已删除源文件同步清理
- **文件预览** — 纯文本/代码语法高亮、Markdown 渲染（含本地图片）、二进制文件标识
- **备份执行** — 手动触发或定时自动备份（秒级 cron），增量同步，Git push（可选）
- **备份历史** — 查看 Git 提交历史，支持分页
- **源文件回滚** — 选择历史提交版本，将源文件恢复到指定版本
- **Git 集成** — 远程仓库配置、SSH/HTTPS 认证管理（AES-256-GCM 加密存储）
- **本地文件浏览** — 安全限定在用户主目录和仓库根目录，防止路径穿越
- **定时调度** — 基于 cron 的自动备份，应用启动时自动加载，配置变更时动态注册/注销
- **前后端一体** — 单二进制文件，一键启动

## 快速开始

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
| 状态管理 | Zustand |
| 定时调度 | robfig/cron/v3 |
| 加密 | AES-256-GCM |
| Markdown | react-markdown + remark-gfm |
| 打包 | Go embed (前端内嵌到二进制) |

### REST API

所有端点前缀 `/api/v1`，响应统一格式 `{"data": ...}`。

| 分类 | 端点 | 功能 |
|------|------|------|
| 仓库 | `POST/GET/DELETE /repos` | 仓库 CRUD |
| 仓库 | `PUT /repos/:id/config` | 更新配置（部分更新） |
| 软链接 | `POST/GET /repos/:id/symlinks` | 软链接创建/列表 |
| 软链接 | `GET/DELETE/PUT /repos/:id/symlinks/:linkId` | 软链接详情/删除/修改目标 |
| 软链接 | `POST /repos/:id/symlinks/batch` | 批量导入 |
| 浏览 | `GET /browse?path=...` | 浏览本地文件系统 |
| 预览 | `GET /repos/:id/preview?path=...` | 预览文件内容 |
| 备份 | `POST /repos/:id/backup` | 触发备份 |
| 备份 | `GET /repos/:id/backup/history` | 备份历史（分页） |
| 回滚 | `GET /repos/:id/commits/:hash/changed-files` | 提交中变更的文件列表 |
| 回滚 | `POST /repos/:id/rollback` | 回滚源文件到历史版本 |
| 认证 | `GET/PUT/DELETE /repos/:id/auth` | Git 认证管理 |
| 系统 | `GET /health` | 健康检查 |

## 使用流程

```
1. 打开应用 → 仓库列表页
2. 点击"创建仓库" → 输入名称、选择路径
3. 进入仓库详情 → 添加软链接（选择要备份的源文件）
4. 在预览标签页查看文件内容
5. 切换到备份标签页 → 点击"触发备份"
6. 配置远程仓库和认证信息（可选）
7. 设置定时备份（可选）
8. 在备份历史中选择提交 → 回滚源文件到历史版本（可选）
```

## 环境配置

| 路径 | 说明 |
|------|------|
| `~/.config/backup-manager/config.json` | 应用配置（端口、主题、自动打开浏览器等） |
| `~/.config/backup-manager/master.key` | AES-256 加密密钥（首次启动自动生成） |
| `~/.config/backup-manager/backup-manager.db` | SQLite 数据库 |

## 安全设计

- **路径安全**: 四层校验（Clean→Abs→EvalSymlinks→Prefix）防止路径穿越
- **认证加密**: SSH 私钥和 HTTPS 密码使用 AES-256-GCM 加密存储
- **并发控制**: 仓库级互斥锁防止并发备份，预览接口限流（最大 5 并发）
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
│   │       ├── repo.go         # 仓库 CRUD
│   │       ├── symlink.go      # 软链接 CRUD + 批量导入
│   │       ├── browse.go       # 本地文件浏览
│   │       ├── preview.go      # 文件预览
│   │       ├── backup.go       # 备份触发 + 历史查询
│   │       ├── auth.go         # Git 认证管理
│   │       ├── rollback.go     # 源文件回滚
│   │       ├── system.go       # 健康检查
│   │       └── errors.go       # 错误码映射
│   ├── service/                # 业务逻辑层
│   │   ├── repo_service.go     # 仓库生命周期
│   │   ├── symlink_service.go  # 软链接 CRUD + 镜像同步
│   │   ├── backup_service.go   # 备份执行引擎
│   │   ├── auth_service.go     # Git 认证管理
│   │   ├── browser_service.go  # 安全文件浏览
│   │   ├── rollback_service.go # 源文件回滚逻辑
│   │   └── repo_mutex.go       # 仓库级互斥锁
│   ├── store/                  # 数据持久化层
│   │   ├── db.go               # SQLite 初始化 + 迁移
│   │   ├── store.go            # Store 聚合
│   │   ├── repo_store.go       # repos 表操作
│   │   ├── repo_config_store.go# repo_configs 表操作
│   │   ├── repo_auth_store.go  # repo_auths 表操作
│   │   └── symlink_store.go    # symlinks 表操作
│   ├── model/                  # 数据模型
│   │   ├── repo.go             # Repo, RepoConfig, RepoStatus
│   │   ├── symlink.go          # Symlink, SymlinkType
│   │   └── auth.go             # GitAuth, GitAuthType
│   ├── git/                    # Git 引擎
│   │   └── git.go              # Init/Add/Commit/Push/Log/Status/Config/LsTree
│   ├── resolver/               # Git 路径解析
│   │   └── symlink_resolver.go # data/ 路径 ↔ 源文件路径映射
│   ├── scheduler/              # 定时调度器
│   │   └── scheduler.go        # 基于 cron 的注册/注销
│   └── util/                   # 工具包
│       ├── path.go             # SafeResolve 四层路径校验
│       ├── crypto.go           # KeyManager (AES-256-GCM)
│       └── file.go             # CopyFile/CopyDir/DetectMIME
└── frontend/                   # React SPA
    ├── package.json
    ├── vite.config.ts           # 开发代理 /api → localhost:9800
    └── src/
        ├── main.tsx             # React 入口
        ├── App.tsx              # 路由配置
        ├── App.css              # 全局样式
        ├── api/client.ts        # axios 实例 + 所有 API 函数
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
            ├── symlink/
            │   ├── SymlinkPanel.tsx
            │   └── SymlinkAddModal.tsx
            ├── preview/
            │   ├── PreviewPanel.tsx
            │   ├── TextPreview.tsx
            │   ├── MarkdownPreview.tsx
            │   └── BinaryInfo.tsx
            ├── backup/
            │   └── BackupPanel.tsx
            │   ├── RollbackConfirmModal.tsx
            │   └── RollbackResultModal.tsx
            └── config/
                └── ConfigPanel.tsx
```

## License

MIT
