# 技术方案 — Backup Manager（修订版）

> 修复 5 个 P0 评审问题：仓库配置编辑 API、路径穿越安全、定时备份机制、Git 认证配置、增量同步与数据一致性冲突。
> 第二轮评审"有条件通过"，修复 6 个 P1 问题后进入开发阶段。

## 1. 技术栈选择

| 层级 | 技术组件 | 选择理由 |
|------|----------|----------|
| 语言 | Go 1.22+ | 跨平台编译、单二进制、标准库丰富 |
| HTTP 框架 | Gin | 轻量、高性能、中间件生态完善 |
| 数据库 | SQLite (modernc.org/sqlite) | 无需额外数据库服务、单文件存储、适合桌面级应用 |
| 前端 | React 18 + TypeScript + Vite + Ant Design 5 | 生态成熟，组件库丰富 |
| 定时调度 | `robfig/cron/v3` | Go 生态标准 cron 库 |
| Git 操作 | `os/exec` 调用系统 git | 复用用户本地 git 配置 |
| 加密 | `crypto/aes` + `crypto/gcm` | 对称加密存储敏感信息 |

### 前后端一体方案

```
生产模式: 单二进制, Go embed 内嵌前端构建产物, Go Server 同时提供静态文件和 API
开发模式: Vite Dev Server (5173) 代理 /api/* 到 Go 后端 (9800)
```

## 2. 架构设计

```
┌────────────────────────────────────────────────────────────┐
│                     Backup Manager (单进程)                  │
│  ┌──────────────┐  ┌─────────────────────────────────────┐ │
│  │  Frontend     │  │  Backend (Go)                       │ │
│  │  (embed.FS)   │  │  ┌────────┐ ┌─────────┐ ┌────────┐ │ │
│  │               │──│──│ Router │─│ Service │─│ Store  │ │ │
│  │  React SPA    │  │  │ (Gin)  │ │  Layer  │ │(SQLite)│ │ │
│  └──────────────┘  │  └────────┘ └─────────┘ └────────┘ │ │
│                    │       │            │                 │ │
│                    │  ┌────┴────┐ ┌────┴──────┐          │ │
│                    │  │  Path   │ │ Git Engine│          │ │
│                    │  │ Security│ │ (os/exec) │          │ │
│                    │  └─────────┘ └───────────┘          │ │
│                    │       │            │                 │ │
│                    │  ┌────┴────────────┴──────┐          │ │
│                    │  │ Scheduler (robfig/cron) │          │ │
│                    │  └─────────────────────────┘          │ │
└────────────────────────────────────────────────────────────┘
         │                          │
         ▼                          ▼
   ┌──────────────┐        ┌──────────────────────────┐
   │  本机路径     │        │  仓库根目录               │
   │  （视图）     │◀───────│  ├─ .backup-manager/     │
   │  软链接 ──────┼───────▶│  │   └─ manifest.json    │
   └──────────────┘        │  ├─ data/   （真实文件）  │
                           │  └─ .git/                │
                           └──────────────────────────┘
```

### 架构核心原则

| 原则 | 说明 |
|------|------|
| **前后端一体** | 所有代码编译为单一二进制，前端通过 embed.FS 内嵌 |
| **响应式 API** | RESTful JSON API；仓库固有状态存 SQLite，条目/链接/设备定义存于仓库内（§9） |
| **路径安全第一** | 所有用户输入的路径必须通过 SafeResolve 安全校验函数 |
| **内容单一归属** | 内容只存在于 `data/`；本机路径只是指向它的软链接视图 —— 没有镜像目录，没有同步步骤（§9） |
| **链接等价** | 链接只绑定完整条目，绝不绑定子路径；所有链接完全等价，没有 in/out 之分（§9.3.3） |
| **认证隔离** | Git 认证信息加密存储，仅 git 操作时注入环境变量 |

## 3. 详细设计

### 3.1 路由注册

```
POST   /api/v1/repos                          → RepoHandler.Create
GET    /api/v1/repos                          → RepoHandler.List
GET    /api/v1/repos/:id                      → RepoHandler.Get
DELETE /api/v1/repos/:id                      → RepoHandler.Delete
PUT    /api/v1/repos/:id/config               → RepoHandler.UpdateConfig  // ★ P0-1: 配置编辑
POST   /api/v1/repos/:id/git-init             → RepoHandler.GitInit

POST   /api/v1/repos/:id/entries/adopt          → EntryHandler.Adopt          // 创建条目及其第一条链接（mv 本机 → data/）
GET    /api/v1/repos/:id/entries?device=&state= → EntryHandler.List
GET    /api/v1/repos/:id/entries/:entryId       → EntryHandler.Get
PATCH  /api/v1/repos/:id/entries/:entryId       → EntryHandler.Update
DELETE /api/v1/repos/:id/entries/:entryId       → EntryHandler.Delete

GET    /api/v1/repos/:id/entries/:entryId/links           → LinkHandler.List
POST   /api/v1/repos/:id/entries/:entryId/links           → LinkHandler.Create // 添加链接
POST   /api/v1/repos/:id/links/bulk                       → LinkHandler.Bulk
PATCH  /api/v1/repos/:id/entries/:entryId/links/:linkId   → LinkHandler.Update
POST   /api/v1/repos/:id/entries/:entryId/links/:linkId/repair  → LinkHandler.Repair
POST   /api/v1/repos/:id/entries/:entryId/links/:linkId/readopt → LinkHandler.Readopt
POST   /api/v1/repos/:id/entries/:entryId/links/:linkId/remove  → LinkHandler.Remove

GET    /api/v1/devices/current                  → DeviceHandler.Current
GET    /api/v1/repos/:id/devices                → DeviceHandler.List
POST   /api/v1/repos/:id/devices                → DeviceHandler.Register
PATCH  /api/v1/repos/:id/devices/:fp            → DeviceHandler.Rename
DELETE /api/v1/repos/:id/devices/:fp            → DeviceHandler.Delete
GET    /api/v1/repos/:id/devices/:fp/links       → DeviceHandler.Links
POST   /api/v1/repos/:id/devices/:fp/apply       → DeviceHandler.Apply
POST   /api/v1/repos/:id/devices/:fp/detach      → DeviceHandler.Detach

GET    /api/v1/repos/:id/consistency             → ConsistencyHandler.Audit
POST   /api/v1/repos/:id/consistency/repair      → ConsistencyHandler.Repair

GET    /api/v1/browse         ?path=...         → BrowseHandler.Browse       // ★ P0-2: 安全修复
GET    /api/v1/browse/allowed-roots              → BrowseHandler.AllowedRoots

GET    /api/v1/repos/:id/tree    ?path=...        → ContentHandler.Tree
GET    /api/v1/repos/:id/preview ?path=...        → ContentHandler.Preview
PUT    /api/v1/repos/:id/save                     → ContentHandler.Save
GET    /api/v1/repos/:id/changes                  → ContentHandler.Changes

POST   /api/v1/repos/:id/backup                   → BackupHandler.Trigger
GET    /api/v1/repos/:id/backup/history?limit=&offset= → BackupHandler.History
POST   /api/v1/repos/:id/push                     → BackupHandler.Push

GET    /api/v1/repos/:id/auth                     → AuthHandler.Get
PUT    /api/v1/repos/:id/auth                     → AuthHandler.Set
DELETE /api/v1/repos/:id/auth                     → AuthHandler.Clear

GET    /api/v1/repos/:id/commits/:hash/changed-files  → RollbackHandler.ListFiles
GET    /api/v1/repos/:id/commits/:hash/files?path=    → RollbackHandler.GetCommitFile
POST   /api/v1/repos/:id/commits/:hash/restore        → RollbackHandler.RestoreFile
POST   /api/v1/repos/:id/rollback                     → RollbackHandler.Rollback

GET    /api/v1/health                                → SystemHandler.Health
```

### 3.2 仓库配置编辑（★ P0-1 修复）

```go
// PUT /api/v1/repos/:id/config
// 接受部分更新，只传需要修改的字段
type UpdateConfigRequest struct {
    RemoteURL          *string `json:"remote_url,omitempty"`
    Branch             *string `json:"branch,omitempty"`
    AutoBackup         *bool   `json:"auto_backup,omitempty"`
    AutoBackupInterval *string `json:"auto_backup_interval,omitempty"`
    GitUserName        *string `json:"git_user_name,omitempty"`
    GitUserEmail       *string `json:"git_user_email,omitempty"`
}
```

### 3.3 路径安全校验（★ P0-2 修复）

**核心安全函数 SafeResolve：**

```go
func SafeResolve(allowedRoot, userPath string) (string, error) {
    // Step 1: filepath.Clean() 消除 ../ 遍历
    cleaned := filepath.Clean(userPath)
    // Step 2: 相对路径拼接到 allowedRoot
    if !filepath.IsAbs(cleaned) {
        cleaned = filepath.Join(allowedRoot, cleaned)
    }
    // Step 3: 转为绝对路径
    absPath, _ := filepath.Abs(cleaned)
    // Step 4: 解析软链接（防止 symlink 逃逸）
    realPath, err := filepath.EvalSymlinks(absPath)
    if err != nil { realPath = absPath }
    // Step 5: 校验是否在 allowedRoot 范围内
    absRoot, _ := filepath.Abs(allowedRoot)
    if !strings.HasPrefix(realPath, absRoot + string(filepath.Separator)) && realPath != absRoot {
        return "", fmt.Errorf("path outside allowed root")
    }
    return realPath, nil
}
```

**安全措施：**

| 措施 | 说明 |
|------|------|
| Clean → Abs → EvalSymlinks → Prefix | 四层路径安全校验 |
| 文件大小限制 | Preview 限制 ≤ 10MB |
| 二进制检测 | 读取前 512 字节检测 MIME 类型 |
| 编码检测 | 非 UTF-8 编码文件返回提示 |
| 并发限制 | Preview 最大 5 并发 |

### 3.4 条目与链接模型（取代 P0-5 镜像一致性设计）

最初的 P0-5 修复靠「复制」维持 `.links/`、`data/` 与源文件三者一致。条目/链接模型（§9）取消了这份重复：**条目**持有内容，而每个**链接**都是同一种东西 —— 指向 `data/<repo_path>` 的软链接。

```
ADOPT（创建条目及其第一条链接）:
  1. 校验 local_path → SafeResolve
  2. 计算 repo_path
  3. 移动 local_path → data/<repo_path>          （os.Rename，跨文件系统降级）
  4. os.Symlink(data/<repo_path> → local_path)
  5. 把 {条目, 链接} 写入 .backup-manager/manifest.json 并提交

添加链接（分发；文件系统效果完全相同）:
  1. 校验条目存在且 data/<repo_path> 存在
  2. os.Symlink(data/<repo_path> → local_path)   （不复制内容）
  3. 向该条目追加链接，提交
```

不再有镜像目录与复制步骤，因此备份操作退化为：`git add -A → git commit → git push`。
完整设计见 §9，其中包含防止链接绑定到目录条目子路径的各项一致性不变量。

### 3.5 定时备份调度器（★ P0-3 修复）

使用 `robfig/cron/v3` 实现：

```go
type Scheduler struct {
    cron     *cron.Cron
    entries  map[string]cron.EntryID  // repoID → cron entryID
    backupFn BackupJobFunc
}

func (s *Scheduler) Start()
func (s *Scheduler) Stop()  // 优雅关闭，等待任务完成
func (s *Scheduler) Register(repoID, cronExpr string) error
func (s *Scheduler) Unregister(repoID string)
func (s *Scheduler) IsRegistered(repoID string) bool
```

- 应用启动时从数据库加载所有开启了 auto_backup 的 repo 注册到调度器
- 配置更新时自动触发 Register/Unregister
- 备份任务执行时通过 repo 级互斥锁防止并发

### 3.6 Git 认证配置（★ P0-4 修复）

**数据模型：**

```go
type GitAuthType string
const (
    GitAuthNone     GitAuthType = "none"
    GitAuthSSHKey   GitAuthType = "ssh_key"
    GitAuthPassword GitAuthType = "password"
)

type GitAuth struct {
    RepoID           string
    AuthType         GitAuthType
    SSHPrivateKey    string  // AES-GCM 加密存储
    SSHPrivateKeyPath string
    Username         string
    PasswordEncrypted []byte // AES-GCM 加密存储
}
```

**认证注入实现：**
- SSH: 通过 `GIT_SSH_COMMAND=ssh -i <key_path>` 环境变量
- HTTPS: 通过 `GIT_ASKPASS` 脚本注入密码/Token
- 密钥和密码使用 AES-256-GCM 加密存储在 SQLite

## 4. 数据存储

共有**三份彼此独立的存储**，各自有独立的文件与存在理由，不可混为一谈：

| 数据 | 文件 | 格式 | 为什么放这里 |
|------|------|------|------|
| `repos`、`repo_configs`、`repo_auths` | `~/.config/backup-manager/backup-manager.db` | SQLite（单个二进制文件） | 本机私有：含加密凭据、本机仓库路径、定时任务。**绝不能**提交进仓库 |
| 条目、链接、设备 | `<repo-root>/.backup-manager/manifest.json` | JSON，由 Git 跟踪 | 必须随 `git clone` / `git push` 跨机器传输。SQLite 文件是按机器独立的，传不过去 |
| 应用设置 | `~/.config/backup-manager/config.json` | JSON | 应用级设置，与具体仓库无关 |

### 4.1 SQLite 数据库（本机，按机器独立）

**文件**：`~/.config/backup-manager/backup-manager.db` —— 单个 SQLite 二进制文件。

```sql
CREATE TABLE repos (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    path          TEXT NOT NULL UNIQUE,
    created_at    DATETIME DEFAULT (datetime('now')),
    updated_at    DATETIME DEFAULT (datetime('now')),
    last_backup_at DATETIME,
    status        TEXT DEFAULT 'active'
);

CREATE TABLE repo_configs (
    repo_id             TEXT PRIMARY KEY,
    remote_url          TEXT,
    branch              TEXT DEFAULT 'main',
    auto_backup         INTEGER DEFAULT 0,
    auto_backup_interval TEXT,
    git_user_name       TEXT,
    git_user_email      TEXT,
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE
);

CREATE TABLE repo_auths (
    repo_id             TEXT PRIMARY KEY,
    auth_type           TEXT NOT NULL DEFAULT 'none',
    ssh_private_key     BLOB,
    ssh_private_key_path TEXT,
    username            TEXT,
    password_encrypted  BLOB,
    updated_at          DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE
);
```

这就是完整表结构 —— 三张表。**没有**任何一张表存条目、链接或设备：旧的 `symlinks` 表已删除（§9.17），也没有新建表替代。这些定义**有意**放在 SQLite 之外，见 §4.2。

### 4.2 仓库清单（位于仓库内，由 Git 跟踪）

**文件**：`<repo-root>/.backup-manager/manifest.json` —— 一个位于**仓库内**的 JSON 文件，**不在** SQLite 数据库中。

```json
{
  "version": 1,
  "updated_at": "2026-09-18T10:00:00Z",
  "devices": [ { "fingerprint": "9f2c…", "name": "MacBook Pro", "hostname": "mbp.local",
                 "os": "darwin", "last_seen_at": "2026-09-18T10:00:00Z" } ],
  "entries": [ { "id": "e1a2…", "repo_path": "opencode/opencode.json", "kind": "file",
                 "created_at": "2026-09-01T08:12:00Z",
                 "links": [ { "id": "l1a2…", "device": "9f2c…",
                              "local_path": "/Users/x/.config/opencode/opencode.json",
                              "enabled": true, "created_at": "2026-09-01T08:12:00Z" } ] } ]
}
```

条目/链接/设备**为什么不放进 SQLite** —— 见 §9.3.6：

- SQLite 数据库位于 `~/.config/backup-manager/`，因此是**按机器独立**的。存在那里的定义无法到达第二台机器。
- 仓库才是被克隆、被推送的对象。把定义放进仓库，才使备份自带描述，并让另一台机器仅凭 `git clone` 就能发现所有设备的链接。
- 同时免费获得版本化、diff、合并与传输能力 —— 无需另设同步机制。

### 4.3 应用配置

**文件**：`~/.config/backup-manager/config.json`

```json
{
  "port": 9800,
  "open_browser": true,
  "theme": "light"
}
```

## 5. 项目目录结构

```
backup-manager/
├── main.go
├── go.mod / go.sum
├── Makefile
├── REQUIREMENT.md
├── DESIGN.md
├── internal/
│   ├── api/
│   │   ├── router.go
│   │   ├── middleware.go
│   │   └── handler/
│   │       ├── repo.go
│   │       ├── entry.go         # 条目：list / adopt / delete
│   │       ├── link.go          # 链接：add / bulk / repair / readopt / remove
│   │       ├── device.go        # 设备：current / register / rename / delete / apply
│   │       ├── consistency.go   # 一致性巡检 + 修复
│   │       ├── browse.go
│   │       ├── content.go       # tree / preview / save / changes
│   │       ├── backup.go
│   │       ├── auth.go
│   │       ├── rollback.go
│   │       ├── system.go
│   │       └── errors.go
│   ├── model/
│   │   ├── repo.go
│   │   ├── link.go              # Entry、Link、Device、Manifest、LinkState
│   │   └── auth.go
│   ├── entry/                   # 条目与链接子系统（§9）
│   │   ├── manifest.go          # 加载 / 保存 / 原子写 / R-1..R-3 校验
│   │   ├── service.go           # Service 装配、仓库互斥锁、清单提交、公共辅助
│   │   ├── entry_service.go     # adopt、list、remove（unlink/move_back/purge）
│   │   ├── link_service.go      # 添加链接、批量链接、repair、readopt、remove
│   │   ├── device_service.go    # register、rename、delete、apply
│   │   ├── entry_state.go       # 逐链接状态诊断与视图构建
│   │   └── consistency.go       # 一致性巡检 + 修复（§9.8）
│   ├── service/
│   │   ├── repo_service.go
│   │   ├── backup_service.go
│   │   ├── auth_service.go
│   │   ├── browser_service.go
│   │   ├── content_service.go
│   │   └── rollback_service.go
│   ├── store/
│   │   ├── db.go
│   │   ├── store.go
│   │   ├── repo_store.go
│   │   ├── repo_config_store.go
│   │   └── repo_auth_store.go
│   ├── git/
│   │   └── git.go
│   ├── scheduler/
│   │   └── scheduler.go
│   ├── servermgr/
│   ├── shortcut/
│   ├── tray/
│   └── util/
│       ├── path.go          # SafeResolve 安全函数
│       ├── crypto.go        # AES-GCM 加密
│       ├── device.go        # MachineFingerprint()
│       ├── repo_mutex.go    # 仓库级互斥锁（备份/回滚/链接操作共享）
│       └── file.go          # 文件操作工具
├── frontend/
│   ├── package.json
│   ├── vite.config.ts
│   ├── index.html
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── routes/
│       ├── components/
│       ├── api/client.ts
│       ├── store/
│       ├── types/
│       └── utils/
└── scripts/build.sh
```

## 6. 评审问题修复对照表

| 评审问题 | 修复方案 |
|----------|----------|
| **P0-1** 缺少仓库配置编辑 API | 新增 `PUT /api/v1/repos/:id/config` + SQLite 持久化配置 |
| **P0-2** 路径穿越高危风险 | SafeResolve 四层防护（Clean→Abs→EvalSymlinks→Prefix）+ 文件大小/并发限制 |
| **P0-3** 缺少定时备份 | `robfig/cron/v3` 调度器 + 生命周期管理 + 配置控制 |
| **P0-4** 缺少 Git 认证配置 | SSH/HTTPS 认证 + AES-GCM 加密存储 + 环境变量注入 |
| **P0-5** 增量同步与一致性冲突 | 添加软链接时同步复制到 data/，备份退化为 git add+commit+push |
| **P1-1** Browse API 安全边界模糊 | 移除 `root` 参数，引入 AllowedRoots 机制（仅 $HOME 和 repo 根目录可浏览） |
| **P1-2** AES-GCM 密钥管理策略缺失 | 首次启动生成 AES-256 密钥存储在 `~/.config/backup-manager/master.key`（0600 权限），内存中 `[]byte` 管理+使用后清零 |
| **P1-3** 源文件外部删除时 data/ 同步策略未明确 | 增量备份检测阶段补充：源文件不存在时自动删除 data/ 和软链接 |
| **P1-4** 增量备份检测算法未定义 | 明确为 mtime + fileSize 双字段比对 |
| **P1-5** SafeResolve EvalSymlinks 降级安全盲区 | 区分错误类型：fs.ErrNotExist 可降级，其他错误拒绝 |
| **P1-6** 缺少错误处理与用户通知方案 | 补充完整错误处理章节：错误分类、SSE 通知、崩溃恢复、回滚机制 |

## 7. 预览与编辑

挂载模型（§9）取消了「源文件 / data/ 副本」的二元结构：内容存放在 `<repo>/data/`，本机路径只是指向它的软链接。因此预览与编辑始终只操作同一个文件 `data/<repo_path>`，不存在双写与同步步骤。

| 操作 | 目标 | 说明 |
|------|------|------|
| 预览 | `<repo>/data/<repo_path>` | `repo_path` 即 Browse 树中的节点路径 |
| 保存 | `<repo>/data/<repo_path>` | 就地写入；本机挂载是同一 inode 的软链接，会立即反映变更 |
| 备份 | `data/` | `git add -A` 直接收录改动 —— 不涉及任何增量同步 |

### 7.1 API 契约

```
GET  /api/v1/repos/:id/tree?path=          → 列出 data/<path> 下的条目，含挂载徽标
GET  /api/v1/repos/:id/preview?path=       → {content, mime_type, size, text, truncated}
PUT  /api/v1/repos/:id/save                → {path, content} → {file_size, modified_at}
GET  /api/v1/repos/:id/changes             → {dirty, changes:[{status, path}]}  (git status --porcelain data/)
```

`path` 始终相对于 `data/`（不再是本机绝对路径），因此原先 `ResolveSource` 的最长前缀匹配逻辑被完全删除。

### 7.2 约束条件

| 场景 | 处理 |
|------|------|
| `path` 逃逸出 `data/` | `util.SafeJoin` 拒绝 → 400 |
| 内容 > 10MB | 413 |
| `path` 是目录 | 400 |
| `path` 不存在 | 404 |
| 并发 | 保存走仓库级互斥锁；预览限流 5 并发 |
| 权限保持 | 先读原始 mode，写入后 `Chmod` 还原 |

### 7.3 安全考量

| 风险 | 防护 |
|------|------|
| 路径遍历 | 每个请求都过 `util.SafeJoin(dataDir, path)` |
| 写入二进制内容 | `truncated` 或 `text == false` 时前端禁用编辑；后端做大小校验 |
| 超大内容 OOM | Content 长度 ≤ 10MB |

## 8. 系统托盘设计

### 8.1 概述

应用以内置系统托盘（macOS 菜单栏）图标运行，提供常驻操作：

- **打开 UI**：在默认浏览器中打开 Web 界面
- **启动/停止服务器**：独立于托盘进程开启/关闭 HTTP 服务器
- **退出**：完全退出应用

### 8.2 实现

- `internal/tray/` — 系统托盘管理器（macOS 菜单栏 / 系统托盘图标）
- `internal/servermgr/` — HTTP 服务器生命周期管理器（无需退出进程即可启停）
- `internal/shortcut/` — 首次运行时创建桌面快捷方式

### 8.3 生命周期

```
main()
  ├── 初始化所有服务（DB、git、scheduler、handlers）
  ├── 创建 servermgr（HTTP 服务器管理器）
  ├── 创建 tray manager（含 OpenUI/StartServer/StopServer/Quit 回调）
  ├── srvMgr.Start() → HTTP 服务器开始监听
  ├── trayMgr.SetServerRunning(true)
  ├── openURL(serverURL) → 打开浏览器（若 open_browser: true）
  ├── trayMgr.Run() → 阻塞直到用户点击"Quit"
  ├── srvMgr.Stop() → 优雅关闭 HTTP 服务器
  └── 最终清理（关闭 DB、销毁 key manager）
```

## 9. 条目与链接模型 —— 全新设计

> 落地 Issue #4（软链接管理与多设备支持）。
> **取代 §3.4 与整个 `/symlinks` API。** 不兼容旧的 `symlinks` 表、`.links/` 目录与「复制式」备份流程 —— 见 §9.17。

### 9.1 旧模型为什么复杂

旧设计为同一个文件维护了**三份表示**：源文件、`data/` 副本、`.links/` 软链接。维持三者一致带来大量机制：

| 机制 | 存在的原因 |
|------|------|
| 增量检测（mtime + size） | 源文件与 `data/` 会漂移 |
| `syncOneFile` / `syncDirectoryFiles` / `walkSourceDir` | 需要把源文件重新拷入 `data/` |
| `.links/` 镜像一致性 | 第三份表示也要同步 |
| `SyncDeletedSource` | 要猜「路径消失」是「源被删」还是「链接被删」（一个数据丢失缺陷） |
| `is_new` 比较 | 源 ≠ `data/` |
| 预览双写（源文件 **和** `data/`） | 存在两份可写副本 |
| `ResolveSource` 最长前缀匹配 | 需要把仓库路径反查回本机路径 |
| 嵌套软链接特例 | 源目录树内还要放软链接 |

根因就是这份重复。因此本次设计直接取消它：

> **内容归属于仓库。本机路径只是它的视图。**

### 9.2 设计原则

| # | 原则 | 推论 |
|------|------|------|
| **P-1** | 内容只有一个归属：`data/<repo_path>` | 无漂移、无同步、无比对 |
| **P-2** | 本机路径是指向 `data/` 的**软链接** | 通过它写入即写入仓库内容 |
| **P-3** | **所有链接完全等价** | 只有一套链接机制；没有 in/out 之分需要定义、校验或切换 |
| **P-4** | 链接绑定**完整条目**，绝不绑定子路径 | 条目与其链接保持完全一致 |
| **P-5** | 定义存放在仓库内并由 Git 跟踪 | `<repo>/.backup-manager/manifest.json` 是唯一事实来源 |
| **P-6** | Git 是安全网 | 每次破坏性操作前先提交 |
| **P-7** | 幂等收敛，而非增量记账 | `apply` 比对「期望状态 vs 实际状态」并收敛 |

### 9.3 核心模型

两个实体：**条目（Entry）** 是被备份的文件/目录，**链接（Link）** 是它的本机视图。

#### 9.3.1 条目（Entry）

```
条目:  repo_path = opencode/opencode.json   kind = file
       内容    = <repo>/data/opencode/opencode.json      ← 唯一的一份副本
```

**条目**是备份的最小单位：`data/` 下的一个 `repo_path`，加上所有指向它的链接。条目**就是**白名单里的一条 —— 只要它存在于清单中，就是「被备份对象」。它可以有 **0 条链接**：内容在仓库里，只是当前没有本机视图。

| 字段 | 说明 |
|------|------|
| `id` | 稳定短 id，供 API 使用 |
| `repo_path` | 相对 `data/` 的路径 —— 内容的全局身份 |
| `kind` | `file` \| `dir` —— 缓存的内容类型 |
| `created_at` | |
| `links` | 绑定到该条目的链接列表（有序） |

#### 9.3.2 链接（Link）

```
   link.local_path                          entry.repo_path
   /Users/x/.config/opencode/o.json   ⟷   opencode/opencode.json
                  │
                  └── 软链接 ──▶  <repo>/data/opencode/opencode.json
```

**链接**把一个本机路径绑定到一个条目。它的物理形态**永远相同**：`local_path` 是指向 `<repo>/data/<entry.repo_path>` 的软链接。同一条目的所有链接**完全等价** —— 没有类型、没有主次、没有「跟踪链接」。

| 字段 | 说明 |
|------|------|
| `id` | 稳定短 id |
| `device` | 所属设备的指纹 |
| `local_path` | 软链接的本机绝对路径 |
| `enabled` | 禁用后仍保留，但 `apply` 跳过 |
| `created_at` | |

#### 9.3.3 所有链接完全等价

每条链接都是指向同一 `data/<repo_path>` 的软链接，因此下面这些是**对同一份内容的同一个操作**：

- 通过 `adopt` 随条目一起创建的那条链接编辑
- 通过后来为分发而添加的链接编辑
- 在 Browse 标签页里编辑

所以模型**不**区分「创建条目的那条链接」（Issue 所说的*入方向*）与「后来添加的链接」（*出方向*）。入/出只是描述**链接是怎么来的**，不是需要存储的状态：

| | 如何产生 | 存储差异 |
|------|------|------|
| 第一条链接 | 由 `adopt` 创建：先把内容移入 `data/`，再与条目一起写入 | 无 |
| 后续链接 | 由「添加链接」创建，指向已存在的内容 | 无 |

推论：

- **只有一套实现。** `createLink(entry, localPath)` 同时服务两者；`adopt` = 「添加链接」+ 前置的「内容移入」。
- **没有需要切换的东西。** 不存在需要跨机器移交的「跟踪链接」—— 每台机器只是各自持有自己的链接（§9.5）。
- **条目可以有 0 条链接。** 这正是从 clone 或导入得到的内容，在任何本机路径绑定到它之前的存在形式。

#### 9.3.4 一致性不变量

Issue #4 要求链接必须与被备份的文件/目录保持**完整一致性**：跟踪一个目录的链接，不应伴随一条指向该目录内部单个文件的链接。该规则被泛化为三条不变量：

| # | 不变量 | 由谁强制 |
|------|------|------|
| **R-1** | 条目的每个链接都绑定**该条目的完整 `repo_path`** —— 绝不指向其内部路径 | 结构性保证：链接只能挂在条目上，API 不接受子路径 |
| **R-2** | 条目之间**永不重叠**：任何 `repo_path` 都不是另一个的祖先或后代 | `adopt` 时校验 → 409 |
| **R-3** | 链接的 `local_path` 不得位于某个目录条目的 `local_path` **之内** | 创建链接时校验 → 409 |

R-2 是 Issue 规则的泛化：如果某个目录被跟踪在 `~/Documents` 上，那么它内部的任何东西都不能被单独跟踪或单独链接。要单独处理就改用互不重叠的 `repo_path`（例如把 `~/Projects/vendor` 记为 `projects/vendor`，而不是 `docs/vendor`）。

R-3 是那条字面规则：`docs` 已作为目录条目跟踪在 `~/Documents` 上时，在 `~/Desktop/notes/file.md` 建链接会被拒绝。

「同一条目的所有链接指向同一目标」现在是构造特性，而不是需要校验的不变量 —— 而且由于链接不带类型，也不存在基数规则需要强制。

#### 9.3.5 设备（Device）

**设备**是引用该仓库的一台机器，是元数据；链接通过指纹引用它。

| 字段 | 说明 |
|------|------|
| `fingerprint` | 稳定机器标识，`sha256("<GOOS>|<raw>")` 十六进制 —— 即设备主键 |
| `name` | 用户可见名称，默认取 hostname，可重命名 |
| `hostname`、`os` | 仅用于展示 |
| `last_seen_at` | 每次 `apply` 更新 |

指纹取值顺序：

| 系统 | 来源 | 兜底 |
|------|------|------|
| Linux | `/etc/machine-id`，其次 `/var/lib/dbus/machine-id` | — |
| macOS | `ioreg -rd1 -c IOPlatformExpertDevice` → `IOPlatformUUID` | — |
| Windows | `reg query HKLM\SOFTWARE\Microsoft\Cryptography /v MachineGuid` | — |
| 任意 | — | `sha256(hostname + "|" + username)` |

只存哈希。缓存于 `~/.config/backup-manager/device.json`。

#### 9.3.6 仓库结构与清单

```
<repo-root>/
├── .backup-manager/
│   └── manifest.json      # 设备 + 条目 + 链接（Git 跟踪，唯一事实来源）
├── data/                  # 真实内容，Git 跟踪 —— 唯一的内容存放处
└── .git/
```

清单以**条目**为单位组织，这正是让 R-1、R-2 在结构上显而易见的原因 —— 一个条目的所有链接都紧邻该条目，子路径链接根本无法被表达出来。

```json
{
  "version": 1,
  "updated_at": "2026-09-18T10:00:00Z",
  "devices": [
    { "fingerprint": "9f2c1a4b7d8e0f31", "name": "MacBook Pro",
      "hostname": "mbp.local", "os": "darwin",
      "last_seen_at": "2026-09-18T10:00:00Z" }
  ],
  "entries": [
    {
      "id": "e1a2b3c4d5e6f708",
      "repo_path": "opencode/opencode.json",
      "kind": "file",
      "created_at": "2026-09-01T08:12:00Z",
      "links": [
        { "id": "l1a2b3c4d5e6f708",
          "device": "9f2c1a4b7d8e0f31",
          "local_path": "/Users/x/.config/opencode/opencode.json",
          "enabled": true, "created_at": "2026-09-01T08:12:00Z" },
        { "id": "l2b3c4d5e6f70819",
          "device": "9f2c1a4b7d8e0f31",
          "local_path": "/Users/x/Desktop/opencode.json",
          "enabled": true, "created_at": "2026-09-10T12:00:00Z" }
      ]
    },
    {
      "id": "e2b3c4d5e6f70819",
      "repo_path": "docs/notes",
      "kind": "dir",
      "created_at": "2026-09-02T09:00:00Z",
      "links": [
        { "id": "l3c4d5e6f7081920",
          "device": "9f2c1a4b7d8e0f31",
          "local_path": "/Users/x/Documents/notes",
          "enabled": true, "created_at": "2026-09-02T09:00:00Z" }
      ]
    }
  ]
}
```

| 规则 | 说明 |
|------|------|
| 事实来源 | 清单是权威。SQLite 只保存 `repos`、`repo_configs`、`repo_auths` 这类天生属于单机的数据 |
| 写入 | 任何变更都重写文件并立即提交（提交信息形如 `link: add opencode/opencode.json`）。写入是原子的：`manifest.json.tmp` → `fsync` → `os.Rename` |
| 读取 | 打开仓库时加载，按 `repoID` + 文件 mtime 缓存于内存 |
| 传输 | 它位于仓库内，随 `git clone` / `git push` 一起走。新机器一次即可获知所有设备的链接 |
| 手工编辑 | 支持。缺少 `id` 的条目在加载时补全；未知字段原样保留；无法解析的文件会阻止所有写入，而不是被静默重写 |
| 冲突 | Git 就是冲突解决器。合并冲突表现为该文件上的普通 Git 冲突；在冲突解决前应用拒绝写入 |

### 9.4 创建流程

#### 9.4.1 Adopt —— 创建条目及其第一条链接

```
1. 校验 local_path：存在、不是软链接、父目录可写、不在 repo.Path 内
2. repo_path = 用户输入，默认 filepath.Base(local_path)
3. R-2 校验：repo_path 不得是任何既有条目的祖先或后代 → 409
4. data/<repo_path> 不得已存在 → 409
5. 若 local_path 是目录，扫描其中的软链接。
     发现任何软链接 → 拒绝（它会把仓库之外的内容拖进来，破坏「内容只存在于 data/」），
     并列出问题路径。可用 `follow_symlinks: true` 改为解引用。
6. 移动 local_path → <repo>/data/<repo_path>
     - 优先 os.Rename；跨文件系统（EXDEV）降级为 CopyFile + 校验大小 + Remove
7. 创建链接：local_path → <repo>/data/<repo_path>
8. 把 {条目, 链接} 追加到清单，提交
```

失败回滚：

| 失败点 | 补偿动作 |
|------|------|
| 步骤 6 失败 | 未移动任何内容 —— 直接返回错误 |
| 步骤 7 失败 | 把 `data/<repo_path>` 移回 `local_path` |
| 步骤 8 失败 | 删除软链接，把 `data/<repo_path>` 移回 `local_path` |

#### 9.4.2 添加链接 —— 已有条目的又一个视图

```
1. 解析条目；它必须存在。**不要求**已有链接 —— 这条流程正是
   新初始化的设备为「尚无链接」的条目建立首个链接的方式
2. 校验 local_path：不存在，或为空目录
3. R-3 校验：local_path 不得位于任何目录条目的 local_path 之内 → 409
4. local_path 不得位于 repo.Path 内
5. 确保 local_path 的父目录存在
6. 创建软链接：local_path → <repo>/data/<entry.repo_path>
7. 向该条目追加链接，提交
```

同一个 `repo_path` 可以带多条链接，同设备或跨设备皆可 —— 这正是 Issue 所说的「把同一份备份分发到不同位置」。

#### 9.4.3 批量链接 —— 把仓库落到一台机器上

```
POST /api/v1/repos/:id/links/bulk  { local_root, entry_ids? }
```

选择一个或多个条目（或全部）与一个本机根目录；服务端为每个条目在 `<local_root>/<repo_path>` 创建一条链接。`local_root` 同样要过 R-3 校验。这就是「我刚把这台新机器 clone 下来，一键把我的文件放回去」的操作。

#### 9.4.4 禁止的形态

| 形态 | 为什么禁止 | 错误 |
|------|------|------|
| 对已跟踪目录内部的单个文件建链接 | 破坏 Issue 要求的完整一致性（R-1、R-3） | 409 |
| 嵌套条目：某条目的 `repo_path` 位于另一条目之内 | 同一个子树出现两个归属（R-2）；这也取代了旧的 `AddNestedSymlink` 功能 | 409 |
| 链接的 `local_path` 位于目录条目的 `local_path` 之内 | 归属含糊（R-3） | 409 |

> **没有**任何链接的条目不属于禁止形态 —— 它正是从 clone 或导入得到的内容在任何本机路径绑定到它之前的存在形式。

> 注意这里是有意为之的删减：旧的 `AddNestedSymlink` 允许目录软链接内部再放指向无关位置的链接。在 R-2 下，这种需求改为把该子内容作为**自己的、互不重叠的条目**来 adopt（例如用 `projects/vendor` 而不是 `docs/vendor`）。

### 9.5 链接等价

由于链接不带类型，不存在「跟踪链接」，也就没有在换机时需要移交的东西。过去挂在跟踪链接上的事情，现在要么是派生的，要么是显式指定的：

| 问题 | 答案 |
|------|------|
| UI 为某个条目打开哪个本机路径？ | **当前设备上**的第一条链接（`is_current`）；若本机没有，则显示该条目「本机无视图」 |
| `move_back` 把内容放回哪里？ | 显式指定的链接（`link_id`）；未指定时取本机第一条，其次取列表第一条 —— 都是确定的 |
| 哪条链接是条目的「主位置」？ | 不存在这个概念。每台机器各自持有自己的链接；条目由 `repo_path` 标识 |

#### 9.5.1 换机场景

```
设备 A 退役：
  1. 在设备 B 上，在期望的位置添加一条链接
     POST /entries/:id/links { local_path: ~/dev/opencode/opencode.json }
  2. 删除设备 A（或 detach 它）
     → A 的链接从清单移除，A 上的软链接被删除
  条目、data/ 中的内容、以及其他所有链接都不受影响。
```

不需要「提升」任何东西：在设备 B 上这条新链接本来就是唯一的本机视图，UI 在它出现后立即使用它。

### 9.6 设备生命周期

#### 9.6.1 注册

```
1. 计算当前机器指纹
2. 打开仓库 → 解析清单
3. 若已存在同指纹设备 → 更新 last_seen_at
   否则 → 追加新设备（name = hostname），提交
4. 将其设为当前设备（会话内存态，不写入清单）
```

注册是隐式的：打开仓库、执行 `apply`、创建链接时都会发生。UI 不会把「先创建设备」作为前置步骤。

#### 9.6.2 Apply —— 让本机收敛

```
POST /api/v1/repos/:id/devices/:fingerprint/apply   { dry_run: true }

1. 收集 device 为当前指纹的全部链接
2. 逐条诊断实际文件系统状态（§9.7）
3. 生成计划：
     create  —— missing      → 创建软链接
     repair  —— wrong_target → 重建软链接
     skip    —— ok / disabled / replaced
     conflict—— occupied     → 报告，绝不覆盖
     orphan  —— dangling     → 仅报告（仓库内容已缺失）
4. 返回计划（dry run）→ UI 在确认弹窗中展示
5. 用户确认后，在仓库级互斥锁下执行，并逐条返回结果
```

`apply` 是幂等的，且从不触碰内容：它只创建、修复或报告软链接。

#### 9.6.3 Detach / 移交

```
POST /api/v1/repos/:id/devices/:fingerprint/detach  { mode: "unlink" | "keep" }
```

| 模式 | 行为 |
|------|------|
| `unlink` | 删除该设备的所有本机软链接。`data/` 完全不动，内容完整保留 |
| `keep` | 不动文件系统，只是不再管理（用于把机器交出去） |

Detach **不会**删除设备条目，并且会让链接定义继续保持启用 —— 这正是「重新挂载只需一次 Apply」的前提。代价是 `unlink` 卸载后，这些链接会在巡检里以 `link_missing` 出现，直到该机器重新挂载。

删除设备（`DELETE /devices/:fingerprint`）会移除它的链接定义。条目本身保留 —— 一条链接都不剩的条目完全合法，内容仍在仓库中。删除设备从不触碰该设备的文件系统。当前设备不允许被删除。

### 9.7 链接状态与修复

状态按需从文件系统计算，从不持久化。

| 状态 | 判定条件 | 可提供的动作 |
|------|------|------|
| `ok` | `local_path` 是软链接，且解析目标为 `data/<entry.repo_path>` | — |
| `missing` | `Lstat(local_path)` 返回 `ErrNotExist` | 创建 |
| `wrong_target` | 是软链接但指向别处 | 修复 |
| `replaced` | `local_path` 存在且是**真实**文件/目录 | 「重新纳入」（把内容移入 `data/` 后重建链接）或移除链接 |
| `dangling` | 软链接存在但仓库中 `data/<repo_path>` 缺失 | 从 Git 历史回滚，或移除 |
| `occupied` | 路径被无关对象占用，无法安全替换 | 人工处理 |
| `disabled` | `enabled == false` | — |
| `not_current` | 属于其他设备 | 只读展示 |

> `replaced` 值得强调：采用「临时文件 + rename」原子写配置的应用，会把软链接替换成真实文件。UI 将其呈现为 `replaced`，并提供一键**重新纳入**：把新内容移入 `data/` 后恢复软链接。建议：当某个应用自己管理该文件时，优先跟踪**目录**而不是单个文件。

> `dangling` 天生安全：内容都在 Git 里，`git checkout <commit> -- data/<repo_path>` 即可恢复。UI 直接给出回滚入口。

注意状态是**逐链接**的；没有链接的条目根本没有逐链接状态 —— 这不是问题，它只是本机没有视图而已。

### 9.8 一致性巡检

```
GET  /api/v1/repos/:id/consistency            # 条目级 + 链接级的巡检结论
POST /api/v1/repos/:id/consistency/repair     # 收敛所有可收敛项
```

| 检查项 | 结论码 | 级别 |
|------|------|------|
| 条目缺少 `id` / `repo_path`、id 重复、`repo_path` 非法 | `invalid_entry` | error |
| 链接缺少 `local_path` / `device`、id 重复 | `invalid_link` | error |
| 链接引用了未登记的设备 | `unknown_device` | error |
| 两个条目重叠（手工编辑清单造成） | `overlapping_entries` | error |
| 链接的 `local_path` 位于某目录条目的 `local_path` 之内 | `nested_link` | error |
| 本机软链接缺失、指向别处、或被真实文件替换 | `link_missing` / `link_wrong_target` / `link_replaced`（§9.7 的状态） | warning |
| 本机路径被无关对象占用 | `link_occupied` | error |
| 活跃条目的内容在 `data/` 中缺失 | `content_missing` | error |
| `data/` **内部**存在软链接 | `symlink_in_data` | error —— 破坏「内容只存在于 `data/`」 |
| 扫描已注册链接的父目录时，发现指向 `data/` 的未托管软链接 | `unmanaged_link` | warning |

`unmanaged_link` 正是 Issue 所禁止形态的直接探测手段：一个绕过应用创建的、指向子路径的链接。它无法扫描整个文件系统，因此范围限定在已注册链接的父目录，并作为 warning 而非 error 报告。

#### 9.8.1 修复能做什么、不能做什么

| 结论 | 修复动作 |
|------|------|
| `nested_link` | 禁用违规链接 —— 已禁用的链接不再活跃，因此不再违反 R-3 |
| `link_missing` / `link_wrong_target` | 重建本机软链接 |
| `link_replaced` | 只报告 —— 恢复内容需要用户显式做出「重新纳入」的决定 |
| `link_occupied`、`content_missing`、`symlink_in_data`、`overlapping_entries`、`invalid_entry`、`invalid_link`、`unknown_device`、`unmanaged_link` | 只报告 —— 没有安全的自动处理手段 |

#### 9.8.2 校验发生在哪一步

| 阶段 | 行为 | 原因 |
|------|------|------|
| `Load` | 解析并补全缺失 id；**不做**校验 | 手工编辑过的清单必须保持可读。若加载失败，仓库会完全不可用，用户连问题是什么都看不到 |
| `Save` | 校验；拒绝任何会引入 error 级违规的写入 | 应用绝不把不合规状态固化下来 |
| `saveConverging` | 跳过校验 | 仅供修复与移除使用，这两类操作只会减少违规数量。没有它，被手工编辑成不合规的清单就再也无法通过应用修正 |

注意 R-3 只对**启用中**的链接判定。这正是「禁用违规链接」能成为合法收敛手段、而不是制造新违规的原因。

### 9.9 移除

**链接级** —— 永远安全：

```
POST /api/v1/repos/:id/entries/:entryId/links/:linkId/remove
```

删除本机软链接与该链接记录。`data/` 不动，条目的其他链接继续可用；移除最后一条链接也只是留下一个没有本机视图的合法条目。下面的条目级操作用于「这份内容本身不应再被跟踪」的情形。

**条目级**：

```
DELETE /api/v1/repos/:id/entries/:entryId?mode=...
```

| 模式 | 行为 | 守卫 |
|------|------|------|
| `unlink` | 只删除本机的软链接；条目与其内容保留 | 无 —— 安全 |
| `move_back` | 把 `data/<repo_path>` 移到指定链接的 `local_path`，然后移除整个条目 | 提示其他链接将失效；需要确认 |
| `purge` | 删除 `data/<repo_path>` **以及**整个条目 | 需要输入 `repo_path` 二次确认；提示其他设备的链接将失效，并在其下次 `apply` 时被报告 |

每次移除前都会先提交，因此 `git revert` 永远可用 —— **Git 就是回收站**，无需再设计一套独立的回收机制。

### 9.10 服务与存储划分

```
internal/
├── entry/                        # 新包 —— 整个子系统
│   ├── manifest.go               # 加载 / 保存 / 原子写 / R-1..R-3 校验
│   ├── service.go                # Service 装配、仓库互斥锁、清单提交、公共辅助
│   ├── entry_service.go          # adopt、list、readopt、remove（unlink/move_back/purge）
│   ├── link_service.go           # 添加链接、批量链接、repair、remove
│   ├── device_service.go         # register、rename、delete、apply、detach
│   ├── entry_state.go            # 逐链接状态诊断与视图构建（§9.7）
│   ├── consistency.go            # 一致性巡检 + 修复（§9.8）
│   └── entry_service_test.go
├── model/
│   ├── repo.go                   # 不变
│   ├── auth.go                   # 不变
│   └── link.go                   # 新增 —— Entry、Link、Device、Manifest、LinkState
└── util/
    ├── device.go                 # 新增 —— MachineFingerprint()
    └── repo_mutex.go             # 从 service 迁入 —— 备份/回滚/链接操作共享
```

重构后的 SQLite 表 —— 全部位于 `~/.config/backup-manager/backup-manager.db`；条目/链接/设备定义则位于仓库清单中（§4.2）：

```sql
repos         — id, name, path, created_at, updated_at, last_backup_at, status   （不变）
repo_configs  — repo_id(FK), remote_url, branch, auto_backup, ...                （不变）
repo_auths    — repo_id(FK), auth_type, ssh_private_key, ...                      （不变）
-- symlinks 表：已删除
```

### 9.11 API

**条目**

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/v1/repos/:id/entries` | 条目列表；每条链接都带按需计算的状态，设备/状态分组由前端完成（服务端过滤推迟，§9.19） |
| GET | `/api/v1/repos/:id/entries/:entryId` | 条目详情（含其链接及状态） |
| POST | `/api/v1/repos/:id/entries/adopt` | 创建条目及其第一条链接：`{local_path, repo_path?, follow_symlinks?}` |
| DELETE | `/api/v1/repos/:id/entries/:entryId?mode=&link_id=` | `unlink`（可带 `link_id` 只删一条链接）/ `move_back` / `purge` |
| PATCH | `/api/v1/repos/:id/entries/:entryId` | `{repo_path?}` —— 重命名；重新校验 R-2（推迟，§9.19） |

**链接**

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/api/v1/repos/:id/entries/:entryId/links` | 添加一条链接：`{local_path, device?}` |
| POST | `/api/v1/repos/:id/links/bulk` | 批量链接：`{local_root, entry_ids?}` |
| POST | `/api/v1/repos/:id/entries/:entryId/links/:linkId/repair` | 重建软链接 |
| POST | `/api/v1/repos/:id/entries/:entryId/links/:linkId/readopt` | `replaced` → 把新内容移入 `data/` 并重建链接 |
| POST | `/api/v1/repos/:id/entries/:entryId/links/:linkId/remove` | 移除单个链接 |
| GET | `/api/v1/repos/:id/entries/:entryId/links` | 条目的链接及状态 —— 条目详情已包含（推迟，§9.19） |
| PATCH | `/api/v1/repos/:id/entries/:entryId/links/:linkId` | `{local_path?, enabled?}`（推迟，§9.19） |

**设备**

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/v1/devices/current` | 当前机器的指纹 / 主机名 / 建议名称 |
| GET | `/api/v1/repos/:id/devices` | 设备列表（`fingerprint`、名称、`last_seen_at`、链接数、`is_current`） |
| POST | `/api/v1/repos/:id/devices` | 注册 / 认领设备（`name`、可选 `fingerprint`） |
| PATCH | `/api/v1/repos/:id/devices/:fingerprint` | 重命名设备 |
| DELETE | `/api/v1/repos/:id/devices/:fingerprint` | 删除设备及其链接定义 |
| POST | `/api/v1/repos/:id/devices/:fingerprint/apply` | 收敛本机（支持 `dry_run`） |
| POST | `/api/v1/repos/:id/devices/:fingerprint/detach` | 卸载本机（`mode`：`unlink` / `keep`） |
| GET | `/api/v1/repos/:id/devices/:fingerprint/links` | 该设备的链接及状态 —— 可由条目列表推导（推迟，§9.19） |

**一致性**

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/v1/repos/:id/consistency` | 巡检结论（§9.8） |
| POST | `/api/v1/repos/:id/consistency/repair` | 收敛所有可收敛项 |

**内容**（结构不变，语义简化 —— 见 §7）

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/v1/repos/:id/tree?path=` | `data/` 下的条目，附条目徽标与链接数 |
| GET | `/api/v1/repos/:id/preview?path=` | 文件预览 |
| PUT | `/api/v1/repos/:id/save` | 保存到 `data/` |
| GET | `/api/v1/repos/:id/changes` | `git status --porcelain data/` |

载荷：

```go
type EntryView struct {
    ID        string      `json:"id"`
    RepoPath  string      `json:"repo_path"`
    Kind      string      `json:"kind"` // file | dir
    CreatedAt time.Time   `json:"created_at"`
    Links     []*LinkView `json:"links"`
}

type LinkView struct {
    ID         string    `json:"id"`
    EntryID    string    `json:"entry_id"`
    Device     string    `json:"device"` // 设备指纹
    DeviceName string    `json:"device_name,omitempty"`
    LocalPath  string    `json:"local_path"`
    Enabled    bool      `json:"enabled"`
    IsCurrent  bool      `json:"is_current"` // 派生：device == 当前指纹
    State      string    `json:"state"`
    StateNote  string    `json:"state_note,omitempty"`
    CreatedAt  time.Time `json:"created_at"`
}

type ApplyRequest struct {
    DryRun bool `json:"dry_run"`
}

type ApplyAction struct {
    EntryID   string `json:"entry_id"`
    LinkID    string `json:"link_id"`
    RepoPath  string `json:"repo_path"`
    LocalPath string `json:"local_path"`
    Action    string `json:"action"` // create / repair / skip / conflict / orphan
    Reason    string `json:"reason,omitempty"`
}

type ApplyResult struct {
    Device      string        `json:"device"`
    Created     []ApplyAction `json:"created"`
    Repaired    []ApplyAction `json:"repaired"`
    Skipped     []ApplyAction `json:"skipped"`
    Conflicts   []ApplyAction `json:"conflicts"`
    Orphans     []ApplyAction `json:"orphans"`
    DryRun      bool          `json:"dry_run"`
    CompletedAt time.Time     `json:"completed_at"`
}
```

### 9.12 前端设计

仓库详情页标签：**Browse** · **Entries** · **Backup** · **Config**。

```
components/entry/
├── EntriesPanel.tsx          # Tab 根组件：工具栏（设备、新建条目、应用、巡检、卸载）、
│                             #   条目列表与可展开的链接行，以及添加链接 / 应用计划 /
│                             #   巡检 / 卸载 / 移除条目等弹窗
└── AdoptModal.tsx            # 创建条目：本机选择 + repo_path 编辑
                              #   + 「内容将被移入仓库」警告
```

列表以条目为中心，因为不变量本身就是条目级的：

```
[设备: MacBook Pro（当前）]  [+ 新建条目]  [应用]  [巡检]  [卸载]
────────────────────────────────────────────────────────────────────
▾ opencode/opencode.json                             文件   ok
    ● ~/.config/opencode/opencode.json        MacBook Pro     [重新纳入?] [移除]
    ● ~/Desktop/opencode.json                 MacBook Pro     [移除]
    ○ ~/work/opencode/opencode.json           MacBook-Pro-2   其他设备
▸ docs/notes                                          目录   missing — [修复] [移除]
▸ projects/vendor                                     目录   0 条链接 — [添加链接]
```

链接统一展示：没有 in/out 徽标，也没有「设为跟踪」操作，因为所有链接完全等价（§9.3.3）。逐链接操作只有三个：**修复**（`missing` / `wrong_target`）、**重新纳入**（`replaced`）与**移除**。`添加链接` 始终可用，包括对没有任何链接的条目。

`components/files/FilesPanel.tsx`（Browse）渲染 `data/` 目录树，并在每个节点显示徽标：是条目 / 不是条目 / 存在链接漂移。`symlink/` 下组件全部删除。

`frontend/src/types/index.ts` 新增类型：

```typescript
export type EntryKind = 'file' | 'dir';
export type LinkState =
  | 'ok' | 'missing' | 'wrong_target' | 'replaced'
  | 'dangling' | 'occupied' | 'disabled' | 'not_current';

export interface Device {
  fingerprint: string;
  name: string;
  hostname?: string;
  os?: string;
  is_current: boolean;
  last_seen_at?: string | null;
  link_count: number;
}

export interface Link {
  id: string;
  entry_id: string;
  device: string;
  device_name?: string;
  local_path: string;
  enabled: boolean;
  is_current: boolean;
  created_at: string;
  state: LinkState;
  state_note?: string;
}

export interface Entry {
  id: string;
  repo_path: string;
  kind: EntryKind;
  created_at: string;
  links: Link[];
}
```

`api/client.ts` 删除全部软链接函数，新增条目、链接、设备与一致性巡检函数。

### 9.13 本次设计删除了什么

| 删除项 | 替代 |
|------|------|
| `internal/service/symlink_service.go`（约 720 行） | `internal/entry/entry_service.go` + `link_service.go` |
| `internal/service/backup_service.go` 的同步部分（`syncChangedFiles`、`syncOneFile`、`syncDirectoryFiles`、`walkSourceDir`、`cleanEmptyDataDirs`） | 无需替代 —— 内容本来就在 `data/` |
| `SymlinkService.SyncDeletedSource` | 无需替代 —— 同类数据丢失缺陷不可能再发生，因为「是否存在」由 `data/` 判定，而非本机路径 |
| `internal/resolver/symlink_resolver.go` | 无需替代 —— 回滚直接写 `data/`，所有链接自动反映 |
| `internal/store/symlink_store.go`、`model/symlink.go` | `internal/entry/manifest.go`、`model/link.go` |
| `.links/` 及全部镜像一致性代码 | 无需替代 —— 只有一份表示 |
| `SymlinkHandler`（8 个端点，含 `nested`） | `EntryHandler` + `LinkHandler` + `DeviceHandler` |
| `is_new` 计算 | `GET /repos/:id/changes`（`git status`） |
| `PreviewService.ResolveSource` 前缀匹配 | `path` 始终是仓库相对路径 |
| 预览双写（源文件 **和** `data/`） | 只写 `data/` 一次 |
| `repo_service.go` 生成 `.gitignore` | 无需替代 —— 没有需要忽略的内容 |

`BackupService.Trigger` 退化为：

```
1. 仓库级互斥锁
2. status = backing_up
3. 落盘清单（冲刷待写的定义变更），若为脏则提交
4. git add -A
5. git commit -m <message>
6. 可选 git push（失败不阻断本地提交）
7. last_backup_at = now；status = active（失败则为 error）
```

### 9.14 安全考量

| 风险 | 防护 |
|------|------|
| `adopt` 破坏原始文件 | 明确确认：「文件将被移入仓库，此位置将被替换为软链接」；任一失败路径全量回滚；跨文件系统路径下先校验大小再删除源文件 |
| 链接覆盖无关数据 | 添加链接时拒绝已存在的非空路径；`apply` 从不覆盖 —— `occupied` 条目只报告，不强制 |
| `purge` 删除内容 | 需输入 `repo_path` 确认；上一个提交可恢复；操作前先报告其余所有链接 |
| 路径遍历 | 所有用户路径过 `util.SafeResolve` / `util.SafeJoin`；本机路径限定在 AllowedRoots（`$HOME` + 仓库根目录） |
| 自引用 | 拒绝位于 `repo.Path` 内的 `local_path` |
| 危险目标 | 拒绝把 `/`、`$HOME`、仓库根目录本身作为 `local_path` |
| 软链接成环 | 创建任何链接前用 `util.ResolveNestedSymlink` 解析候选链；检测到环即拒绝 |
| adopt 一棵含软链接的目录树 | 除非设置 `follow_symlinks`，否则拒绝（§9.4.1），从而 `data/` 内永不出现指向外部的软链接 |
| 手工编辑的清单 | 每次写入前按 R-1..R-3 校验；每条路径重新过 `SafeResolve`；文件无法解析时阻止写入，不合规时由巡检报告 |
| 并发变更 | 所有会改文件系统的操作都持有 `RepoMutexManager` 的仓库级互斥锁 |
| 清单写入撕裂 | `manifest.json.tmp` → `fsync` → `os.Rename` |

### 9.15 边界与异常处理

| 场景 | 处理 |
|------|------|
| `adopt` 的源已是软链接 | 400 —— 拒绝（避免链式链接） |
| `adopt` 的源位于 `repo.Path` 内 | 400 —— 拒绝 |
| `adopt` 的源是仓库本身或其祖先 | 400 —— 拒绝 |
| `adopt` 的源目录树内含软链接 | 409 —— 拒绝并列出；可用 `follow_symlinks: true` 解引用 |
| `repo_path` 与既有条目重叠 | 409（R-2） |
| `data/<repo_path>` 已存在 | 409 |
| `os.Rename` 跨文件系统 | 降级为 `CopyFile` + 校验大小 + `Remove` |
| `mv` 成功但建链接失败 | 回滚：把内容移回 |
| 链接目标已存在真实文件 | 409 —— 提示用户先移走 |
| 链接目标是非空目录 | 409 |
| 链接的 `local_path` 位于目录条目的 `local_path` 之内 | 409（R-3） |
| 移除条目的最后一条链接 | 允许 —— 条目与内容保留，只是本机没有视图 |
| 删除设备 | 其链接定义随之移除；一条链接都不剩的条目合法 |
| 仓库中 `data/<repo_path>` 缺失 | `dangling`；`apply` 只报告，绝不创建链接 |
| 给没有任何链接的条目添加链接 | 允许 —— 这正是新设备为「仓库中已存在的内容」建立绑定的方式 |
| 清单存在结构性问题（id 非法、设备未登记、条目重叠） | 由巡检报告；修复前拒绝写入 |
| 无法获取设备指纹 | 降级为 `sha256(hostname+username)` 并告警；允许用户手动命名设备 |
| 清单无法解析（Git 冲突） | 409 并返回原始错误；在冲突解决前阻止所有写入 |
| 两台机器同时改清单 | Git 冲突，由 Git 解决；应用绝不自动合并 |
| 同一设备上两个链接使用同一 `local_path` | 409 |

### 9.16 测试策略

**单元测试**（`internal/entry/`）：

- 清单：加载 / 保存 / 原子写 / 文件缺失 / 文件格式错误 / 未知字段保留 / id 自动补全
- 清单校验：条目重叠（R-2）、链接嵌套在目录条目之内（R-3）被拒绝；无链接条目被接受
- `MachineFingerprint`：各平台分支 + 兜底逻辑的确定性
- Adopt：正常路径、R-2 冲突、`data/` 冲突、跨文件系统降级、各失败点回滚、源目录树含软链接
- 添加链接：正常路径、目标被占用、非空目录、自引用、R-3 违规、同一条目的多条链接
- **链接等价**：同一条目的两条链接读写的是同一份内容
- 移除最后一条链接后条目与内容都保留
- 无链接条目的清单经保存/加载往返后保持不变
- 状态诊断：全部 8 种状态
- 模拟原子写（把软链接替换为真实文件）后的「重新纳入」
- Detach：`unlink` 删除本机软链接且随后 `apply` 可恢复；`keep` 不动文件系统
- 移除：链接级移除、条目级 `unlink` / `move_back` / `purge`，以及各项守卫
- 一致性巡检：每个结论码各一组 fixture

**集成测试**：

- Adopt → 通过第一条链接编辑 → 变更立即体现在 `data/` → `git commit` 收录
- Adopt → 删除某条链接的软链接 → `data/` 保留 → 状态为 `missing` → `apply` 重建
- 一个条目在本机有 3 条链接 → 编辑任意一条在 `data/` 中只产生一处变更
- 双设备模拟：在 A 上生成清单 → clone → 注册设备 B → 批量链接 → B 的链接符合 B 的布局且 `data/` 不变
- 删除设备 A → 其链接随之消失；只依赖 A 的条目依然合法且巡检保持干净
- HTTP 层的 R-2 / R-3 拒绝路径（409 且信息可读）
- `purge` 后 `git revert` → 内容恢复
- 并发 `apply` 与备份 → 无数据损坏

### 9.17 升级行为（不做迁移）

由于明确不要求向后兼容：

| 项目 | 行为 |
|------|------|
| `symlinks` 表 | 新版本首次启动时删除 |
| `.links/` 目录 | 首次打开旧仓库时删除（一次 `os.RemoveAll`） |
| `data/` 内容 | **原样保留** —— 不删除任何承载用户数据的内容 |
| `manifest.json` | 以空清单创建（`{"version":1,"devices":[],"entries":[]}`） |
| 重建条目与链接 | 对每个要跟踪的文件/目录执行 `adopt`，再用批量链接做分发 |
| 恢复旧的源路径 | 无法自动恢复 —— 旧的 `target_path` 随表一并丢弃 |
| SQLite 中已注册的仓库 | 保留；`repos` / `repo_configs` / `repo_auths` 不变 |
| 仓库的 `.gitignore` | 已存在的保持原样；新仓库不再生成 |

### 9.18 里程碑

| 里程碑 | 范围 |
|------|------|
| M1 | `model/link.go`、`entry/manifest.go`、原子写、R-1..R-3 校验、id 分配 |
| M2 | `util/device.go`（指纹）+ 设备注册/列表/重命名/删除 |
| M3 | 条目创建：adopt（mv、R-2 校验、软链接扫描、跨文件系统降级、回滚） |
| M4 | 链接创建：添加链接、R-3 校验、批量链接、路径安全 |
| M5 | 状态诊断 + `apply`（dry run 与执行）+ repair + readopt |
| M6 | 移除：链接级、条目级 `unlink` / `move_back` / `purge`、守卫；设备 `detach` |
| M7 | 一致性巡检 + 修复 |
| M8 | 拆除旧子系统：删除 `.links/`、`symlinks` 表、同步机制、resolver、软链接 API；简化 `BackupService.Trigger` |
| M9 | 内容 API 简化（§7）+ `changes` 端点 |
| M10 | 前端：Entries 标签页、弹窗、徽标、Browse 集成 |
| M11 | 文档同步（所有文档与 README，中英双份） |

### 9.19 待办项（尚未实现）

上述主线已实现并通过验证。以下为有意推迟的部分 —— 它们都是增量功能，且不改变模型：

| 项目 | 推迟原因 |
|------|------|
| **条目重命名**（`PATCH /entries/:entryId`） | 需要重指该条目的每一个既有软链接，不只是元数据变更 |
| **逐链接编辑**（`PATCH /entries/:entryId/links/:linkId`，`{local_path?, enabled?}`） | `enabled` 已被 `apply` 与 R-3 校验遵循，缺的只是端点 |
| **`GET /entries/:entryId/links`、`GET /devices/:fingerprint/links`** | 便利性投影 —— 条目列表已经返回每条链接及其状态 |
| **服务端条目过滤**（`?device=&state=`） | 条目列表规模很小，前端直接分组过滤 |
| 批量链接 UI、设备重命名 UI | 前端便利项；相关 API（`POST /links/bulk`、`PATCH /devices/:fingerprint`）已经具备 |
| 批量「重新纳入」 | `readopt` 已按链接可用（§9.7）；批量版本未实现 |

