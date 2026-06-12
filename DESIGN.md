# 技术方案 — Backup Manager（修订版）

> 修复 5 个 P0 评审问题：仓库配置编辑 API、路径穿越安全、定时备份机制、Git 认证配置、增量同步与数据一致性冲突。
> 第二轮评审"有条件通过"，修复 6 个 P1 问题后进入开发阶段。

## 1. 技术栈选择

| 层级 | 技术组件 | 选择理由 |
|------|----------|----------|
| 语言 | Go 1.22+ | 跨平台编译、单二进制、标准库丰富 |
| HTTP 框架 | Gin | 轻量、高性能、中间件生态完善 |
| 数据库 | SQLite (mattn/go-sqlite3) | 无需额外数据库服务、单文件存储、适合桌面级应用 |
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
   ┌──────────┐           ┌──────────────────┐
    │  Source  │           │  Repository Root │
    │  Files   │◀─symlink──│  ├─ .links/       │
    │  (任意   │           │  ├─ data/         │
    │   路径)  │────copy──▶│  └─ .git/         │
                          └──────────────────┘
```

### 架构核心原则

| 原则 | 说明 |
|------|------|
| **前后端一体** | 所有代码编译为单一二进制，前端通过 embed.FS 内嵌 |
| **响应式 API** | RESTful JSON API，无状态设计（状态由 SQLite 持久化） |
| **路径安全第一** | 所有用户输入的路径必须通过 SafeResolve 安全校验函数 |
| **镜像一致性** | `.links/` 和 `data/` 始终镜像一致（添加/删除软链接时同步操作） |
| **认证隔离** | Git 认证信息加密存储，仅 git 操作时注入环境变量 |

## 3. 详细设计

### 3.1 路由注册

```
POST   /api/v1/repos                          → RepoHandler.Create
GET    /api/v1/repos                          → RepoHandler.List
GET    /api/v1/repos/:id                      → RepoHandler.Get
DELETE /api/v1/repos/:id                      → RepoHandler.Delete
PUT    /api/v1/repos/:id/config               → RepoHandler.UpdateConfig  // ★ P0-1: 配置编辑

POST   /api/v1/repos/:id/symlinks             → SymlinkHandler.Create
GET    /api/v1/repos/:id/symlinks             → SymlinkHandler.List
GET    /api/v1/repos/:id/symlinks/:linkId     → SymlinkHandler.Get
DELETE /api/v1/repos/:id/symlinks/:linkId     → SymlinkHandler.Delete
PUT    /api/v1/repos/:id/symlinks/:linkId     → SymlinkHandler.UpdateTarget
POST   /api/v1/repos/:id/symlinks/batch       → SymlinkHandler.BatchImport

GET    /api/v1/browse         ?path=...&root=...  → BrowseHandler.Browse   // ★ P0-2: 安全修复
GET    /api/v1/repos/:id/preview ?path=...        → PreviewHandler.Preview // ★ P0-2: 安全修复

POST   /api/v1/repos/:id/backup               → BackupHandler.Trigger
GET    /api/v1/repos/:id/backup/history?limit=&offset= → BackupHandler.History

GET    /api/v1/repos/:id/auth                 → AuthHandler.Get
PUT    /api/v1/repos/:id/auth                 → AuthHandler.Set
DELETE /api/v1/repos/:id/auth                 → AuthHandler.Clear

GET    /api/v1/health                         → SystemHandler.Health
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

### 3.4 软链接镜像一致性（★ P0-5 修复）

添加软链接时**同步复制源文件到 data/**，确保 `.links/` 和 `data/` 始终一致：

```
CREATE Symlink:
  1. 校验 sourcePath → SafeResolve
  2. 计算相对路径
  3. os.Symlink(sourcePath → .links/<relPath>)
  4. copyFile/copyDir(sourcePath → data/<relPath>)  ← 同步复制
  5. INSERT symlinks 表

DELETE Symlink:
  1. 删除 .links/<relPath>
  2. 删除 data/<relPath>
  3. 清理空目录
  4. DELETE FROM symlinks

UPDATE Symlink Target:
  1. 删除旧 .links/ + data/ 文件
  2. 创建新软链接 → 复制新源文件
  3. 更新数据库
```

备份操作退化为：`增量检测源文件变更 → 同步到 data/ → git add -A → git commit → git push`

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

### SQLite 表结构

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

CREATE TABLE symlinks (
    id              TEXT PRIMARY KEY,
    repo_id         TEXT NOT NULL,
    relative_path   TEXT NOT NULL,
    target_path     TEXT NOT NULL,
    type            TEXT NOT NULL,
    file_size       INTEGER,
    modified_at     DATETIME,
    created_at      DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE,
    UNIQUE(repo_id, relative_path)
);
```

### 应用配置

路径: `~/.config/backup-manager/config.json`
```json
{
  "port": 9800,
  "openBrowser": true,
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
│   │   ├── middleware/
│   │   └── handler/
│   │       ├── repo.go
│   │       ├── symlink.go
│   │       ├── browse.go
│   │       ├── preview.go
│   │       ├── backup.go
│   │       ├── auth.go
│   │       └── system.go
│   ├── model/
│   │   ├── repo.go
│   │   ├── symlink.go
│   │   └── auth.go
│   ├── service/
│   │   ├── repo_service.go
│   │   ├── symlink_service.go
│   │   ├── backup_service.go
│   │   └── auth_service.go
│   ├── store/
│   │   ├── db.go
│   │   ├── repo_store.go
│   │   ├── repo_config_store.go
│   │   ├── repo_auth_store.go
│   │   └── symlink_store.go
│   ├── git/
│   │   └── git.go
│   ├── scheduler/
│   │   └── scheduler.go
│   └── util/
│       ├── path.go          # SafeResolve 安全函数
│       ├── crypto.go        # AES-GCM 加密
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
