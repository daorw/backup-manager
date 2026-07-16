# Backup Manager — 项目规范文档

## 项目概述

文件/目录聚合备份可视化管理工具。基于 Git 的反向追踪模式（白名单机制），用户主动指定哪些文件/目录需要备份，通过软链接聚合管理源文件，提供可视化界面。

**核心价值**：让用户以"指定要备份什么"而非"排除什么"的直观方式管理备份。

## 架构设计

### 分层架构

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

### 核心数据流

```
用户添加软链接 → .links/ 创建 symlink → data/ 同步复制源文件
                    ↓
执行备份 → 增量检测(mtime+size) → 同步到 data/ → git add → git commit → git push(可选)
```

## 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| 后端语言 | Go 1.22+ | 单二进制、跨平台编译 |
| HTTP 框架 | Gin (github.com/gin-gonic/gin) | 轻量高性能 |
| 数据库 | SQLite (modernc.org/sqlite, 纯Go, 无需CGO) | 单文件存储 |
| 定时调度 | robfig/cron/v3 | 秒级精度 cron 表达式 |
| Git 操作 | os/exec 调用系统 git | 复用用户本地 git 配置 |
| 加密 | crypto/aes + crypto/gcm | AES-256-GCM 加密存储 |
| 前端框架 | React 18 + TypeScript | — |
| 前端构建 | Vite 5 | — |
| UI 组件 | Ant Design 5 | — |
| 状态管理 | Zustand | 轻量级状态管理 |
| HTTP 客户端 | axios | — |
| 前后端一体 | Go embed.FS + Gin 静态文件服务 | 单二进制部署 |
| Markdown 渲染 | react-markdown + remark-gfm | 基础 Markdown 渲染，本地引用按浏览器默认行为处理 |

## 项目目录结构

```
backup-manager/
├── main.go                          # 入口：初始化各模块 → 启动 HTTP → 优雅关闭
├── go.mod / go.sum
├── AGENTS.md                        # 本项目规范文档
├── REQUIREMENT.md                   # 需求文档
├── DESIGN.md                        # 技术设计方案
├── .gitignore
│
├── internal/
│   ├── api/                         # API 层
│   │   ├── router.go                # 路由注册 + SPA 静态文件挂载
│   │   ├── middleware.go            # CORS + 错误恢复中间件
│   │   └── handler/                 # HTTP 处理器（8个 + errors.go）
│   │       ├── repo.go              # Repo CRUD + Config Update + Git Init
│   │       ├── symlink.go           # Symlink CRUD + Batch Import + Nested + BrowseDirEntries
│   │       ├── browse.go            # 本地文件浏览（安全限定 AllowedRoots）
│   │       ├── preview.go           # 文件预览 + 保存（源文件回写 + data/ 同步）
│   │       ├── backup.go            # 备份触发 + 历史查询 + Push
│   │       ├── auth.go              # Git 认证管理
│   │       ├── rollback.go          # 源文件回滚 + 单文件恢复 + 提交文件预览
│   │       ├── system.go            # 健康检查
│   │       └── errors.go            # 错误码映射（respondError）
│   │
│   ├── service/                     # 业务逻辑层（8个）
│   │   ├── repo_service.go          # 仓库生命周期（创建/删除/配置/Git Init）
│   │   ├── symlink_service.go       # 软链接 CRUD + 镜像同步 data/ + 嵌套软链接
│   │   ├── backup_service.go        # 备份执行引擎（增量检测→同步→git）
│   │   ├── auth_service.go          # Git 认证管理（加密存储/注入）
│   │   ├── browser_service.go       # 安全文件浏览（AllowedRoots 机制）
│   │   ├── preview_service.go       # 文件预览（ResolveSource） + 保存（源文件+data/）
│   │   ├── rollback_service.go      # 源文件回滚 + 单文件恢复 + 提交文件预览
│   │   └── repo_mutex.go            # 仓库级互斥锁（并发控制）
│   │
│   ├── store/                       # 数据持久化层
│   │   ├── db.go                    # SQLite 初始化 + 迁移
│   │   ├── store.go                 # Store 聚合
│   │   ├── repo_store.go            # repos 表操作
│   │   ├── repo_config_store.go     # repo_configs 表操作
│   │   ├── repo_auth_store.go       # repo_auths 表操作
│   │   └── symlink_store.go         # symlinks 表操作
│   │
│   ├── model/                       # 数据模型
│   │   ├── repo.go                  # Repo, RepoConfig, RepoStatus
│   │   ├── symlink.go               # Symlink, SymlinkType
│   │   └── auth.go                  # GitAuth, GitAuthType
│   │
│   ├── git/                         # Git 引擎
│   │   └── git.go                   # Init/Add/Commit/Push/Log/Status/Config/LsTree/Show/WriteFileContentTo/GetChangedFilesInCommit
│   │
│   ├── resolver/                    # Git 路径解析器
│   │   └── symlink_resolver.go      # data/ 路径 → 源文件路径映射
│   │
│   ├── scheduler/                   # 定时调度器
│   │   └── scheduler.go             # 基于 cron 的注册/注销/生命周期管理
│   │
│   ├── servermgr/                   # HTTP 服务器生命周期管理
│   ├── shortcut/                    # 桌面快捷方式创建
│   ├── tray/                        # 系统托盘（菜单栏）管理
│   │
│   └── util/                        # 工具包
│       ├── path.go                  # SafeResolve/SafeResolveFile（四层路径校验）
│       ├── crypto.go                # KeyManager（AES-256-GCM）
│       └── file.go                  # CopyFile/CopyDir/DetectMIME
│
└── frontend/                        # React SPA
    ├── package.json
    ├── vite.config.ts               # 开发代理 /api → localhost:9800
    ├── index.html
    ├── tsconfig.json
    └── src/
        ├── main.tsx                 # React 入口
        ├── App.tsx                  # 路由配置 + ConfigProvider
        ├── App.css                  # 全局样式
        ├── api/client.ts            # axios 实例 + 所有 API 函数
        ├── types/index.ts           # TypeScript 类型定义
        ├── store/appStore.ts        # Zustand 状态管理
        ├── routes/                  # 页面组件
        │   ├── Dashboard.tsx        # 仓库列表
        │   └── RepoDetail.tsx       # 仓库详情（Tabs）
        └── components/              # 功能组件
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
            │   ├── BackupPanel.tsx
            │   ├── RollbackConfirmModal.tsx
            │   └── RollbackResultModal.tsx
            └── config/
                └── ConfigPanel.tsx
```

## 数据库 Schema

4 张核心表，外键级联删除：

```sql
repos         — 仓库: id, name, path, created_at, updated_at, last_backup_at, status
repo_configs  — 配置: repo_id(FK), remote_url, branch, auto_backup, auto_backup_interval, git_user_name, git_user_email
repo_auths    — 认证: repo_id(FK), auth_type, ssh_private_key(BLOB), ssh_private_key_path, username, password_encrypted(BLOB)
symlinks      — 软链接: id, repo_id(FK), relative_path(UNIQUE), target_path, type, file_size, modified_at, created_at
```

- WAL 模式启用
- 外键约束启用

## API 端点

所有端点前缀 `/api/v1`，响应格式统一为 `{"data": ...}` 或 `{"error": "..."}`：

### 仓库管理
| 方法 | 路径 | 功能 |
|------|------|------|
| POST | /repos | 创建仓库 |
| GET | /repos | 仓库列表 |
| GET | /repos/:id | 仓库详情 |
| DELETE | /repos/:id | 删除仓库 |
| PUT | /repos/:id/config | 更新配置（部分更新） |
| POST | /repos/:id/git-init | 初始化 Git 仓库 |

### 软链接管理
| 方法 | 路径 | 功能 |
|------|------|------|
| POST | /repos/:id/symlinks | 添加软链接 |
| GET | /repos/:id/symlinks | 软链接列表（含 `is_new` 变更标记） |
| GET | /repos/:id/symlinks/:linkId | 软链接详情 |
| DELETE | /repos/:id/symlinks/:linkId | 删除软链接 |
| PUT | /repos/:id/symlinks/:linkId | 修改目标路径 |
| POST | /repos/:id/symlinks/batch | 批量导入 |
| GET | /repos/:id/symlinks/:linkId/entries?sub_path= | 浏览目录 symlink 内容 |
| POST | /repos/:id/symlinks/:linkId/nested | 在目录 symlink 内添加嵌套软链接 |

### 文件操作
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /browse?path=... | 浏览本地文件系统 |
| GET | /browse/allowed-roots | 列出可浏览根目录 |
| GET | /repos/:id/preview?path=... | 预览源文件内容 |
| PUT | /repos/:id/save | 编辑保存源文件（同步到 data/） |

### 备份
| 方法 | 路径 | 功能 |
|------|------|------|
| POST | /repos/:id/backup | 触发备份（可指定 commit_message） |
| GET | /repos/:id/backup/history?limit=&offset= | 备份历史 |
| POST | /repos/:id/push | 推送到远程仓库（可选 force 参数） |

### 回滚
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /repos/:id/commits/:hash/changed-files | 列出提交中的变更文件 |
| GET | /repos/:id/commits/:hash/files?path= | 预览提交中的文件内容 |
| POST | /repos/:id/commits/:hash/restore | 从提交恢复单个文件到源 |
| POST | /repos/:id/rollback | 批量回滚源文件到历史版本 |

### 认证
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /repos/:id/auth | 获取认证配置 |
| PUT | /repos/:id/auth | 设置认证 |
| DELETE | /repos/:id/auth | 清除认证 |

### 系统
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /health | 健康检查 |

## 关键设计决策

### 1. 镜像一致性
添加/删除/修改软链接时，同步操作 `data/` 目录，确保 `.links/` 和 `data/` 目录结构始终镜像一致。备份执行时只做增量检测（mtime+size 比较）和 git 操作。

### 2. 路径安全（SafeResolve）
四层路径校验防止路径穿越：
1. `filepath.Clean()` — 消除 `../` 遍历
2. `filepath.Abs()` — 转为绝对路径
3. `filepath.EvalSymlinks()` — 防止 symlink 逃逸
4. `strings.HasPrefix()` — 验证在 allowedRoot 范围内

- `EvalSymlinks` 失败时仅 `fs.ErrNotExist` 可降级，其他错误直接拒绝
- 预览文件限制 ≤ 10MB，最大 5 并发
- 浏览文件限定在 AllowedRoots（$HOME + repo 根目录）

### 3. Git 认证加密
- SSH 私钥和 HTTPS 密码使用 AES-256-GCM 加密存储在 SQLite
- 密钥文件 `~/.config/backup-manager/master.key` 权限 0600，首次启动自动生成
- SSH 通过 `GIT_SSH_COMMAND` 环境变量注入
- HTTPS 通过 `GIT_ASKPASS` 脚本注入

### 4. 并发控制
- 每个仓库独立互斥锁（map[string]*sync.Mutex）
- 预览接口限流（channel semaphore，最大 5）
- 定时备份跳过 backing_up 状态的仓库

### 5. 错误处理
- 备份失败时 repo 状态设为 error（而非 active）
- Git push 失败不阻断本地 commit，记录日志
- 软链接创建失败时回滚 data/ 复制

### 6. 自动备份调度
- 基于 robfig/cron/v3，支持秒级 cron 表达式
- 应用启动时从数据库加载启用了 auto_backup 的 repo
- 配置更新时自动注册/注销调度任务

### 7. 源文件回滚
- 支持批量回滚（按 symlink_ids 过滤）和单文件恢复
- 回滚复制时保留文件权限（git mode → os.FileMode）
- 使用 SymlinkResolver 将 data/ 路径映射回源文件路径
- 回滚需要 repo 级互斥锁，禁止与备份并发

### 8. 文件编辑与保存
- 预览和编辑操作目标为软链接指向的**源文件**（target_path）
- 通过 PreviewService.ResolveSource 解析相对路径到源文件绝对路径
- 保存时同时写入源文件和 data/ 目录（镜像一致性）
- 保留原始文件权限（os.Stat → origMode → os.Chmod）

### 9. 嵌套软链接
- 支持在目录 symlink 内部创建子级软链接（nested symlink）
- 软链接链解析支持循环检测和深度限制（max 50）
- 目录浏览时自动标记嵌套软链接类型和变更状态（is_new）

### 10. 系统托盘
- macOS 菜单栏 / 系统托盘图标
- 提供"打开 UI"、"启动/停止服务器"、"退出"操作
- HTTP 服务器通过 servermgr 管理独立启停生命周期

## 文档规范

- **功能变更必须同步更新文档**：所有功能新增、修改、删除，必须同步更新对应的需求文档（`REQUIREMENT.md`）、技术方案（`DESIGN.md`）和快速入门（`docs/quick-start.md`）
- **文档更新必须双语同步**：所有文档更新，必须同时更新英文版（原始路径）和中文版（`docs/zh/` 下对应文件），保持中英文内容一致
- 默认文档为英文，中文文档通过顶部链接引用

## 代码规范

### Go 后端

- **包命名**: 全小写单数形式（`store`, `model`, `service`）
- **文件命名**: 蛇形命名（`repo_service.go`, `auth_handler.go`）
- **测试文件**: 与源文件同目录，命名 `_test.go` 后缀
- **错误处理**: 函数返回 `error`，使用 `fmt.Errorf("context: %w", err)` 包装
- **HTTP 处理**: Handler 只做请求解析和响应返回，业务逻辑委托给 Service
- **模型定义**: 使用 `*time.Time` 表示可空时间字段
- **JSON 标签**: 使用蛇形命名（`json:"last_backup_at,omitempty"`）
- **日志**: 使用标准库 `log` 包

### 前端 TypeScript

- **文件命名**: PascalCase 组件（`BackupPanel.tsx`），camelCase 工具（`client.ts`）
- **类型定义**: 在 `types/index.ts` 中集中管理
- **API 调用**: 在 `api/client.ts` 中集中管理，通过 axios 拦截器解包 `{data: ...}`
- **状态管理**: 使用 Zustand `useAppStore` 单一 store
- **组件模式**: 函数组件 + React Hooks
- **路由**: react-router-dom v6

## 常用命令

```bash
# 开发 - 启动后端
go run .

# 开发 - 启动前端
cd frontend && npm run dev

# 生产构建
cd frontend && npm run build && cd .. && go build -o backup-manager .

# 运行测试
go test ./... -count=1

# 前端类型检查
cd frontend && npx tsc --noEmit
```

## 环境与配置

- 应用数据目录：`~/.config/backup-manager/`
- 配置文件：`~/.config/backup-manager/config.json`（JSON 字段：`port`, `open_browser`, `theme`）
- 加密密钥：`~/.config/backup-manager/master.key`
- 数据库：`~/.config/backup-manager/backup-manager.db`
- 默认端口：9800
- 启动后自动打开浏览器

## 仓库目录结构

```
<repo-root>/
├── .links/              # 软链接目录（结构与 data/ 复刻）
├── data/                # 实际备份数据（结构与 .links/ 复刻）
└── .git/                # Git 版本库
```
