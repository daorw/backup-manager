# Backup Manager — 项目规范文档

## 项目概述

文件/目录聚合备份可视化管理工具。基于 Git 的反向追踪模式（白名单机制），用户主动指定哪些文件/目录需要备份；内容移入仓库 `data/`，本机路径以指向它的软链接作为视图，并提供可视化界面与多设备分发能力。

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
创建条目(adopt) → 内容 mv 进 data/<repo_path> → 原位置替换为软链接（该条目的第一条链接）
                    ↓
添加链接 → 同一条目再分发到其它本机路径（纯软链接，不复制内容）
                    ↓
执行备份 → 落盘 manifest.json → git add -A → git commit → git push(可选)
```

**所有链接完全等价**：每条链接都是指向 `data/<repo_path>` 的软链接，没有 in/out 类型、没有跟踪链接。`adopt` 创建条目时一并创建它的第一条链接，后续链接只是分发 —— 但这只是「链接怎么来的」，不是存储的状态。
内容只存在于 `data/`，本机路径只是视图，因此**不存在增量同步步骤**。`apply` 负责让本机链接与清单收敛。

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
| 国际化 | i18next + react-i18next | 中英文资源；Ant Design 与 dayjs 语言环境同步 |
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
│   │   └── handler/                 # HTTP 处理器
│   │       ├── repo.go              # Repo CRUD + Config Update + Git Init
│   │       ├── entry.go             # 条目：list / adopt / delete
│   │       ├── link.go              # 链接：add / bulk / repair / readopt / remove
│   │       ├── device.go            # 设备：current / register / rename / delete / apply
│   │       ├── consistency.go       # 一致性巡检 + 修复
│   │       ├── browse.go            # 本地文件浏览（任意路径 + 隐藏文件开关）
│   │       ├── content.go           # tree / preview / save / changes
│   │       ├── backup.go            # 备份触发 + 历史查询 + Push
│   │       ├── auth.go              # Git 认证管理
│   │       ├── rollback.go          # 内容回滚 + 单文件恢复 + 提交文件预览
│   │       ├── system.go            # 健康检查 + 应用设置
│   │       └── errors.go            # 错误码映射（respondError）
│   │
│   ├── appconfig/                   # config.json 加载、校验与原子持久化
│   │   └── manager.go
│   │
│   ├── entry/                       # 条目与链接子系统
│   │   ├── manifest.go              # 清单加载/保存/原子写 + R-1..R-3 校验
│   │   ├── service.go               # Service 装配、仓库互斥锁、清单提交、公共辅助
│   │   ├── entry_service.go         # adopt、list、remove（unlink/move_back/purge）
│   │   ├── link_service.go          # 添加链接、批量链接、repair、readopt、remove
│   │   ├── device_service.go        # register、rename、delete、apply
│   │   ├── entry_state.go           # 逐链接状态诊断与视图构建
│   │   └── consistency.go           # 一致性巡检 + 修复
│   │
│   ├── service/                     # 业务逻辑层
│   │   ├── repo_service.go          # 仓库生命周期（创建/删除/配置/Git Init）
│   │   ├── backup_service.go        # 备份执行（git add/commit/push）
│   │   ├── auth_service.go          # Git 认证管理（加密存储/注入）
│   │   ├── browser_service.go       # 文件浏览（任意路径 + 隐藏文件开关）
│   │   ├── content_service.go       # 内容树 / 预览 / 保存（直接读写 data/）
│   │   └── rollback_service.go      # 内容回滚 + 单文件恢复 + 提交文件预览
│   │
│   ├── store/                       # 数据持久化层
│   │   ├── db.go                    # SQLite 初始化 + 迁移
│   │   ├── store.go                 # Store 聚合
│   │   ├── repo_store.go            # repos 表操作
│   │   ├── repo_config_store.go     # repo_configs 表操作
│   │   └── repo_auth_store.go       # repo_auths 表操作
│   │
│   ├── model/                       # 数据模型
│   │   ├── repo.go                  # Repo, RepoConfig, RepoStatus
│   │   ├── link.go                  # Entry, Link, Device, Manifest, LinkState
│   │   └── auth.go                  # GitAuth, GitAuthType
│   │
│   ├── git/                         # Git 引擎
│   │   └── git.go                   # Init/Add/Commit/Push/Log/Status/Config/LsTree/Show/WriteFileContentTo/GetChangedFilesInCommit
│   │
│   ├── scheduler/                   # 定时调度器
│   │   └── scheduler.go             # 基于 cron 的注册/注销/生命周期管理
│   │
│   ├── servermgr/                   # HTTP 服务器生命周期管理
│   ├── shortcut/                    # 桌面快捷方式创建
│   ├── tray/                        # 系统托盘（菜单栏）管理
│   │
│   └── util/                        # 工具包
│       ├── path.go                  # SafeResolve/SafeJoin（四层路径校验）
│       ├── crypto.go                # KeyManager（AES-256-GCM）
│       ├── device.go                # MachineFingerprint（跨平台机器指纹）
│       ├── repo_mutex.go            # 仓库级互斥锁（备份/回滚/链接操作共享）
│       └── file.go                  # CopyFile/CopyDir/DetectMIME
│
└── frontend/                        # React SPA
    ├── package.json
    ├── vite.config.ts               # 开发代理 /api → localhost:9800
    ├── index.html
    ├── tsconfig.json
    └── src/
        ├── main.tsx                 # React 入口
        ├── App.tsx                  # 路由配置 + ConfigProvider 语言环境同步
        ├── App.css                  # 全局样式
        ├── api/client.ts            # axios 实例 + 所有 API 函数
        ├── i18n/                    # i18next 中英文资源 + dayjs 语言环境同步
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
            ├── entry/
            │   ├── EntriesPanel.tsx  # 条目与链接：列表、添加链接、修复、重新纳入、移除、apply
            │   └── AdoptModal.tsx    # 创建条目（内容移入仓库，原位置建第一条链接）
            ├── files/
            │   └── FilesPanel.tsx    # 浏览 data/ 树 + 预览编辑
            ├── preview/
            │   ├── TextPreview.tsx
            │   ├── MarkdownPreview.tsx
            │   └── BinaryInfo.tsx
            ├── backup/
            │   ├── BackupPanel.tsx
            │   ├── RollbackConfirmModal.tsx
            │   └── RollbackResultModal.tsx
            ├── common/
            │   ├── DirectoryPickerModal.tsx
            │   └── RepositoryDirectoryPickerModal.tsx  # data/ 内父目录选择 + 待创建目录
            └── config/
                └── ConfigPanel.tsx
```

## 数据存储

**三份彼此独立的存储** —— 注意不要把 SQLite 表与仓库清单混为一谈：

| 数据 | 文件 | 格式 | 为什么放这里 |
|------|------|------|------|
| `repos`、`repo_configs`、`repo_auths` | `~/.config/backup-manager/backup-manager.db` | SQLite（单个二进制文件） | 本机私有：加密凭据、本机路径、定时任务。绝不提交进仓库 |
| 条目、链接、设备 | `<repo-root>/.backup-manager/manifest.json` | JSON，由 Git 跟踪 | 必须随 `git clone` / `git push` 跨机器传输；SQLite 是按机器独立的 |
| 应用设置 | `~/.config/backup-manager/config.json` | JSON | 应用级设置，含默认 UI 语言 `language`（默认 `en`） |

### SQLite 数据库（本机）

文件：`~/.config/backup-manager/backup-manager.db`。共 **3 张表**，外键级联删除，WAL 模式与外键约束启用：

```sql
repos         — 仓库: id, name, path, created_at, updated_at, last_backup_at, status
repo_configs  — 配置: repo_id(FK), remote_url, branch, auto_backup, auto_backup_interval, git_user_name, git_user_email
repo_auths    — 认证: repo_id(FK), auth_type, ssh_private_key(BLOB), ssh_private_key_path, username, password_encrypted(BLOB)
```

**注意**：`symlinks` 表已删除，且**没有**任何新表替代它 —— 条目/链接/设备**不**存放在 SQLite 中。

### 仓库清单（位于仓库内，Git 跟踪）

文件：`<repo-root>/.backup-manager/manifest.json`（JSON，不在 .db 文件里）。

```json
{
  "version": 1, "updated_at": "...",
  "devices": [ {"fingerprint","name","hostname","os","last_seen_at"} ],
  "entries": [ {"id","repo_path","kind","created_at",
                "links": [ {"id","device","local_path","enabled","created_at"} ]} ]
}
```

**条目、链接与设备为什么放仓库而不放 SQLite**：数据库是按机器独立的，清单随 `git clone` / `git push` 传输，新机器才能获知所有设备的链接；同时也免费获得版本化与冲突解决。

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

### 条目与链接
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /repos/:id/entries | 条目列表（含链接与状态；分组/过滤由前端完成） |
| GET | /repos/:id/entries/:entryId | 条目详情（含其链接与状态） |
| POST | /repos/:id/entries/adopt | 创建条目及其第一条链接（内容 mv 进 data/） |
| DELETE | /repos/:id/entries/:entryId?mode=&link_id= | unlink / move_back / purge |
| POST | /repos/:id/entries/:entryId/links | 添加链接 |
| POST | /repos/:id/links/bulk | 在某个本机根目录下批量创建链接 |
| POST | .../links/:linkId/repair | 重建软链接 |
| POST | .../links/:linkId/readopt | `replaced` → 把新内容移入 data/ 并重建链接 |
| POST | .../links/:linkId/remove | 移除单个链接 |
| PATCH | /repos/:id/entries/:entryId | 重命名 `repo_path`（待办，未实现） |
| PATCH | .../links/:linkId | 修改 local_path / enabled（待办，未实现） |

### 设备
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /devices/current | 当前机器的指纹 / 主机名 |
| GET/POST/PATCH/DELETE | /repos/:id/devices[/:fp] | 设备注册 / 重命名 / 删除 |
| POST | /repos/:id/devices/:fp/apply | 让本机收敛（支持 dry_run） |
| POST | /repos/:id/devices/:fp/detach | 卸载本机（mode：unlink / keep） |
| GET | /repos/:id/devices/:fp/links | 该设备的链接及状态（待办，未实现） |

### 一致性
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /repos/:id/consistency | 巡检结论（R-1..R-3、未托管链接） |
| POST | /repos/:id/consistency/repair | 修复所有可收敛项 |

### 文件操作
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /browse?path=...&include_hidden=true | 浏览任意本地目录（`~` 展开，可选隐藏文件） |
| GET | /browse/home | 浏览默认起始目录（服务端家目录） |
| GET | /repos/:id/tree?path=...&include_hidden=true | 列出 data/ 下的内容（可选隐藏项） |
| GET | /repos/:id/preview?path=... | 预览文件内容 |
| PUT | /repos/:id/save | 保存到 data/ |
| GET | /repos/:id/changes | data/ 下的未提交变更（git status） |

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
| GET | /settings | 获取应用级默认语言；响应 `{"data":{"language":"en"}}` |
| PUT | /settings | 持久化 `{"language":"en"}` 或 `{"language":"zh-CN"}`；响应返回相同的 `data.language` 结构 |

## 关键设计决策

### 1. 内容单一归属与条目级一致性
- 内容只存在于 `data/<repo_path>`；本机路径是指向它的软链接视图，不存在镜像目录、副本或同步步骤
- **所有链接完全等价**：物理形态完全相同（都是指向 `data/<repo_path>` 的软链接），语义也完全相同，因此不落库区分类型
- **条目即白名单成员**：条目存在于清单中就是「被备份」的定义；**0 条链接的条目合法**（内容在仓库里，本机暂无视图）
- 强制不变量 R-1..R-3：
  - R-1 链接只绑定完整条目，绝不绑定子路径
  - R-2 条目之间永不重叠（`repo_path` 无祖先/后代关系）
  - R-3 链接的 `local_path` 不得位于某个目录条目的 `local_path` 之内
- **禁止**：跟踪一个目录、却对该目录内的单个文件建链接；条目嵌套（取代了旧的嵌套软链接功能）
- 破坏性操作前先 git 提交，`git revert` 即回收站

### 2. 路径安全（SafeResolve）
四层路径校验防止路径穿越：
1. `filepath.Clean()` — 消除 `../` 遍历
2. `filepath.Abs()` — 转为绝对路径
3. `filepath.EvalSymlinks()` — 防止 symlink 逃逸
4. `strings.HasPrefix()` — 验证在 allowedRoot 范围内

- `EvalSymlinks` 失败时仅 `fs.ErrNotExist` 可降级，其他错误直接拒绝
- 预览文件限制 ≤ 10MB，最大 5 并发
- 浏览文件**无根目录白名单**：服务端可见的任意路径都可浏览，仅做 `~` 展开 + Clean/Abs/软链接归一；隐藏文件由 `include_hidden` 参数控制
- 链接的 `local_path`：拒绝位于 `repo.Path` 内部的自引用；拒绝把 `/`、`$HOME`、仓库根目录本身作为目标
- **新建备份文件/目录时内容必须落在 `<repo>/data/`**：条目 `repo_path` 起点固定为 `data/`，拒绝绝对路径与任何 `..`，并在解析软链接后校验仍在 `data/` 内（`resolveRepoPathIn`）；仓库内容的浏览/预览/保存同样以 `data/` 为根
- Browse 标签页始终请求 `include_hidden=true`，确保点号开头的已备份文件/目录不会从 `data/` 内容树中消失
- 新建条目的仓库父目录选择器只浏览 `data/`，选择后追加源对象原名并回填完整 `repo_path`；“显示隐藏文件”开关控制点号目录展示；选择器内的新目录仅为前端待创建路径，确认 adopt 时才随内容移动创建
- 添加链接的 `local_path` 可手动输入完整路径，也可通过本机目录选择器选择父目录并自动追加条目名称
- 建链接前用 `util.ResolveNestedSymlink` 解析候选链，检测到环即拒绝

### 3. Git 认证加密
- SSH 私钥和 HTTPS 密码使用 AES-256-GCM 加密存储在 SQLite
- 密钥文件 `~/.config/backup-manager/master.key` 权限 0600，首次启动自动生成
- SSH 通过 `GIT_SSH_COMMAND` 环境变量注入
- HTTPS 通过 `GIT_ASKPASS` 脚本注入

### 4. 并发控制
- 每个仓库独立互斥锁（map[string]*sync.Mutex）
- 备份、回滚、以及所有会改文件系统的链接操作（adopt / add / repair / readopt / remove / detach / apply）都持同一把锁
- 预览接口限流（channel semaphore，最大 5）
- 定时备份跳过 backing_up 状态的仓库

### 5. 错误处理
- 备份失败时 repo 状态设为 error（而非 active）
- Git push 失败不阻断本地 commit，记录日志
- adopt 失败按阶段回滚：mv 失败无副作用；建链接失败把内容移回原位置；写清单失败再撤销链接与移动；本次为目标创建的空父目录一并清理
- 跨文件系统（EXDEV）降级为 CopyFile + 校验大小 + Remove，校验通过前不删除源文件
- 清单无法解析（如 Git 冲突）时阻止所有写入，返回 409 与原始错误

### 6. 自动备份调度
- 基于 robfig/cron/v3，支持秒级 cron 表达式
- 应用启动时从数据库加载启用了 auto_backup 的 repo
- 配置更新时自动注册/注销调度任务

### 7. 内容回滚
- 支持批量回滚（按 entry 过滤）和单文件恢复
- 直接写回 `data/<repo_path>`；由于所有链接都指向 `data/`，本机路径自动反映回滚结果，无需映射回源文件路径
- 回滚复制时保留文件权限（git mode → os.FileMode）
- 回滚需要 repo 级互斥锁，禁止与备份并发

### 8. 文件编辑与保存
- 预览和编辑的目标就是 `data/<repo_path>` —— 只写一次，不存在双写
- `path` 参数始终是仓库相对路径，一律经 `util.SafeJoin` 校验
- 保留原始文件权限（os.Stat → origMode → os.Chmod）
- 「是否有新变更」用 `GET /repos/:id/changes`（`git status --porcelain data/`）表达，取代旧的 `is_new` 比对

### 9. 链接状态诊断与收敛
- 逐链接状态：`ok` / `missing` / `wrong_target` / `replaced` / `dangling` / `occupied` / `disabled` / `not_current`
- `apply` 幂等收敛：先出 dry-run 计划（create / repair / skip / conflict / orphan）再执行，从不覆盖已占用路径；没有 create/repair 动作时前端只关闭计划，后端 no-op 也不更新时间戳、不重写清单
- `replaced`（应用原子写把软链接换成真实文件）→「重新纳入」把新内容移入 `data/` 后重建链接
- 一致性巡检覆盖 R-1..R-3，并探测 `data/` 内的软链接与未托管链接

### 10. 多设备分发
- 设备以稳定机器指纹标识（Linux `/etc/machine-id`、macOS `IOPlatformUUID`、Windows `MachineGuid`，兜底 `sha256(hostname+user)`）
- 设备/条目/链接存放于仓库内 `manifest.json`，随 Git 传输
- 换机流程：注册设备 → `apply` 重建链接 → `bulk` 批量分发
- 移交条目：在新机器上添加链接即可，无需提升任何东西（所有链接等价）
- 删除设备只移除其链接定义；一条链接都不剩的条目依然合法
- `manifest.json` 原子写：`.tmp` → `fsync` → `os.Rename`；无法解析时阻止写入而非静默重写

### 11. 系统托盘
- macOS 菜单栏 / 系统托盘图标
- 提供"打开 UI"、"启动/停止服务器"、"退出"操作
- HTTP 服务器通过 servermgr 管理独立启停生命周期

### 12. 双语 UI 与默认语言
- 支持语言仅为英文 `en` 与简体中文 `zh-CN`，默认 `en`
- 侧边栏语言切换器即时更新 i18next/react-i18next 文案，并同步 Ant Design `ConfigProvider`、dayjs 与页面 `lang`
- UI 渲染前通过 `GET /api/v1/settings` 加载；切换后通过 `PUT /api/v1/settings` 将 `language` 原子持久化到 `~/.config/backup-manager/config.json`
- 持久化失败时恢复原语言；缺失或无效的配置值归一化为 `en`

## 文档规范

- **功能变更必须同步更新文档**：所有功能新增、修改、删除，必须同步更新对应的需求文档（`REQUIREMENT.md`）、技术方案（`DESIGN.md`）、快速入门（`docs/quick-start.md`）、`README.md` 和本文件（`AGENTS.md`）
- **文档更新必须双语同步**：所有文档更新，必须同时更新英文版（原始路径）和中文版（`docs/zh/` 下对应文件），保持中英文内容一致
- 默认文档为英文，中文文档通过顶部链接引用
- 中英文文档的章节编号与表格结构必须保持一致

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
- **国际化**: 所有用户可见文案经 i18next/react-i18next；切换语言时同步 Ant Design、dayjs 与文档 `lang`
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
- 配置文件：`~/.config/backup-manager/config.json`（JSON 字段：`port`, `open_browser`, `theme`, `language`；`language` 支持 `en` / `zh-CN`，默认 `en`）
- 加密密钥：`~/.config/backup-manager/master.key`
- SQLite 数据库：`~/.config/backup-manager/backup-manager.db`（存 repos / repo_configs / repo_auths）
- 仓库清单：`<repo-root>/.backup-manager/manifest.json`（存条目 / 链接 / 设备，随 Git 传输）
- 默认端口：9800
- 启动后自动打开浏览器

## 仓库目录结构

```
<repo-root>/
├── .backup-manager/
│   └── manifest.json    # 设备 + 条目 + 链接（Git 跟踪，唯一事实来源）
├── data/                # 真实内容 —— 唯一的内容存放处
└── .git/                # Git 版本库
```

本机侧（每个链接都是指向仓库的软链接）：

```
~/Documents/notes.txt   ->  <repo>/data/documents/notes.txt   （创建该条目时产生）
~/Desktop/notes.txt     ->  <repo>/data/documents/notes.txt   （分发时添加）
```
