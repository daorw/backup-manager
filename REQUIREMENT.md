# 需求分析文档 — Backup Manager

## 1. 产品概述

| 项目 | 内容 |
|------|------|
| 产品名称 | Backup Manager（备份管理器） |
| 产品定位 | 文件/目录聚合备份可视化管理软件 |
| 一句话描述 | 基于 Git 的反向追踪模式（白名单机制），通过软链接聚合管理源文件，并提供可视化界面的备份管理工具。 |
| 核心价值 | 让用户以"指定要备份什么"（而非"排除什么"）的直观方式管理备份，同时提供可视化操作界面，降低备份管理门槛。 |

### 1.1 核心设计理念

**反向追踪模型**：与 `.gitignore` 的"排除模式"相反，用户主动指定哪些文件/目录需要被追踪备份，未被指定的文件自动忽略。类似于一个白名单系统。

**软链接聚合**：用户创建的备份仓库（repo）内部使用软链接来"指向"源文件，软链接集中管理在 `.links/` 目录下。实际备份时，系统将软链接指向的源文件内容同步到 `data/` 目录，然后通过 Git 进行版本管理。

**前后端一体**：系统采用前后端一体架构，单进程一键启动，无需分别部署前端和后端服务。

---

## 2. 用户角色

| 角色 | 描述 | 核心诉求 |
|------|------|----------|
| **普通用户** | 个人用户，希望备份自己的文件/目录 | 简单易用，可视化操作，快速预览文件内容 |
| **高级用户** | 有一定技术背景，了解 Git 概念 | 精细控制备份策略，查看 Git 提交历史，手动管理软链接，回滚源文件 |

---

## 3. 功能需求

### 3.1 备份仓库管理

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| FR-1 | 创建备份仓库 | 用户通过 UI 创建一个新的备份仓库，选择本地路径作为 repo 根目录 | P0 |
| FR-2 | 查看仓库列表 | 展示所有已创建的备份仓库，包含基本信息 | P0 |
| FR-3 | 删除备份仓库 | 删除一个已创建的备份仓库（仅移除数据库记录，保留文件系统数据） | P1 |
| FR-4 | 编辑仓库配置 | 可视化编辑仓库配置：远程仓库地址、分支、Git 用户名/邮箱、定时备份开关和间隔 | P0 |

### 3.2 软链接管理（.links/ 目录）

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| FR-5 | 添加软链接 | 用户通过 UI 选择源文件/目录，自动在 `.links/` 下创建对应的软链接，同时复制源文件到 `data/` | P0 |
| FR-6 | 查看软链接列表 | 以树形结构展示 `.links/` 目录下的所有软链接 | P0 |
| FR-7 | 删除软链接 | 删除指定的软链接，同步删除 `data/` 中的对应文件 | P0 |
| FR-8 | 修改软链接目标 | 修改软链接指向的源文件/目录路径 | P1 |
| FR-9 | 批量导入软链接 | 支持批量选择多个源文件/目录添加软链接，部分失败时自动回滚 | P1 |
| FR-10 | 已删除源文件同步清理 | 检测源文件已被删除的软链接，同步清理 `data/` 中的对应文件并生成 Git 提交 | P2 |

### 3.3 文件预览

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| FR-11 | 纯文本文件预览 | 在 UI 中快速阅览纯文本文件内容（如 .txt, .log, .json, .yaml, .py 等），含语法高亮 | P0 |
| FR-12 | Markdown 渲染预览 | 渲染显示 Markdown 文件（.md），支持本地图片显示 | P0 |
| FR-13 | 二进制文件标识 | 对于非文本文件显示文件类型信息和大小，不尝试预览内容 | P2 |

### 3.4 备份执行

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| FR-14 | 执行备份 | 手动触发一次备份操作：增量检测（mtime+size）→ 同步变更到 `data/` → `git add` → `git commit` →（可选）`git push`，支持进度展示 | P0 |
| FR-15 | 定时/自动备份 | 按配置的 cron 表达式定时自动执行备份，应用启动时自动加载已启用的仓库 | P1 |
| FR-16 | 备份历史查看 | 查看仓库的 Git 提交历史，支持分页 | P1 |
| FR-17 | 源文件回滚 | 选择历史提交版本，将源文件恢复到指定提交中的版本。支持选择部分文件回滚或全量回滚 | P1 |

### 3.5 配置管理

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| FR-18 | Git 远程仓库配置 | 可视化配置 Git 远程仓库地址和目标分支 | P0 |
| FR-19 | Git 认证配置 | 配置 Git 操作所需的认证信息（SSH 私钥 或 HTTPS 用户名密码），加密存储在 SQLite | P1 |
| FR-20 | 应用全局设置 | 应用级别的基本设置（端口号、主题、是否自动打开浏览器） | P1 |

### 3.6 系统管理

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| FR-21 | 应用启动/停止 | 一键启动和停止整个应用，启动后自动打开浏览器 | P0 |
| FR-22 | 本地文件浏览 | 安全浏览本地文件系统，用于选择软链接源文件和预览文件，限定在用户主目录和仓库根目录 | P0 |
| FR-23 | 健康检查 | 提供 `/health` 端点，返回应用运行状态、启动时间和版本信息 | P2 |

---

## 4. 非功能需求

| ID | 需求 | 描述 |
|----|------|------|
| NFR-1 | **前后端一体架构** | 前端 UI 和后端服务整合为一个应用，单进程运行 |
| NFR-2 | **跨平台支持** | 至少支持 macOS 和 Linux |
| NFR-3 | **响应式 UI** | 界面适配不同屏幕尺寸 |
| NFR-4 | **安全性** | 删除软链接和删除仓库前二次确认；不对源文件做任何修改（只读，回滚操作除外） |
| NFR-5 | **数据一致性** | `.links/` 和 `data/` 目录结构镜像一致 |
| NFR-6 | **备份原子性** | 备份失败应有清晰的提示和错误状态 |
| NFR-7 | **易用性** | 核心功能在 3 次点击内可完成 |
| NFR-8 | **启动方式** | 启动后自动打开浏览器 |
| NFR-9 | **路径安全** | 四层路径校验（Clean→Abs→EvalSymlinks→Prefix）防止路径穿越 |
| NFR-10 | **并发安全** | 每个仓库独立互斥锁防止并发备份；预览接口限流（最大 5 并发） |
| NFR-11 | **敏感信息加密** | SSH 私钥和 HTTPS 密码使用 AES-256-GCM 加密后存储在 SQLite，密钥文件权限 0600 |

---

## 5. 核心概念 / 数据模型

### 5.1 目录结构

```
<repo-root>/
├── .links/              # 软链接目录，结构与 data/ 完全复刻
│   ├── documents/
│   │   ├── report.docx -> /Users/xxx/Documents/report.docx
│   │   └── notes.txt -> /Users/xxx/Documents/notes.txt
│   └── config/
│       └── settings.json -> /Users/xxx/.config/settings.json
├── data/                # 实际备份数据目录，结构与 .links/ 完全复刻
│   ├── documents/
│   │   ├── report.docx
│   │   └── notes.txt
│   └── config/
│       └── settings.json
└── .git/                # Git 版本库
```

### 5.2 核心实体

```
BackupRepo
├── id: string
├── name: string
├── path: string          # repo 根目录绝对路径
├── createdAt: timestamp
├── updatedAt: timestamp
├── lastBackupAt: timestamp|null
├── status: 'active' | 'error' | 'backing_up'
├── config:
│   ├── remoteUrl: string
│   ├── branch: string (default: main)
│   ├── autoBackup: boolean
│   ├── autoBackupInterval: string (cron 表达式)
│   ├── gitUserName: string
│   └── gitUserEmail: string
└── symlinks: Symlink[]

Symlink
├── id: string
├── repoId: string
├── relativePath: string   # 在 .links/ 下的相对路径（含文件名）
├── targetPath: string     # 源文件/目录的绝对路径
├── type: 'file' | 'directory'
├── size: number           # 源文件大小
├── modifiedAt: timestamp  # 源文件最后修改时间
└── createdAt: timestamp

BackupResult
├── repoId: string
├── completedAt: timestamp
├── filesChanged: number    # 本次备份变更的文件数
├── filesRemoved: number    # 本次备份删除的文件数
├── commitHash: string|null
├── commitMessage: string|null
└── pushed: boolean         # 是否成功推送到远程

RollbackResult
├── repoId: string
├── commitHash: string      # 回滚到的目标提交
├── total: number           # 总文件数
├── success: number         # 成功数
├── skipped: number         # 跳过数（文件无变更）
├── failed: number          # 失败数
├── failures: RollbackFailure[]
└── completedAt: timestamp

CommitEntry (Git 提交记录)
├── hash: string
├── author: string
├── email: string
├── date: string
└── message: string
```

### 5.3 数据库 Schema

4 张核心表，外键级联删除：

```sql
repos         — 仓库: id, name, path, created_at, updated_at, last_backup_at, status
repo_configs  — 配置: repo_id(FK), remote_url, branch, auto_backup, auto_backup_interval, git_user_name, git_user_email
repo_auths    — 认证: repo_id(FK), auth_type, ssh_private_key(BLOB), ssh_private_key_path, username, password_encrypted(BLOB)
symlinks      — 软链接: id, repo_id(FK), relative_path(UNIQUE), target_path, type, file_size, modified_at, created_at
```

- WAL 模式启用
- 外键约束启用

---

## 6. 用户确认的关键决策

| 问题 | 决策 |
|------|------|
| Git 远程仓库 | `git push` 为可选项。未配置远程仓库时只做本地 commit 不 push |
| 源文件删除后策略 | `data/` 中对应文件同步删除，并生成 Git 提交 |
| 文件冲突 | 如果两个软链接指向同一个源文件，不做特别处理，正常执行 |
| 备份粒度 | 增量同步，只同步修改过的文件（对比 mtime+size） |
| 前端技术选型 | React 18 + TypeScript + Vite + Ant Design 5 |
| 启动方式 | 启动后自动打开浏览器 |
| Markdown 图片 | 支持 Markdown 中的本地图片显示 |
| 多仓库 | 支持多个仓库并行管理 |
| 后端框架 | Gin（Go 轻量高性能 HTTP 框架） |
| 数据库 | SQLite（纯 Go 实现 modernc.org/sqlite，无需 CGO） |
| 认证加密 | SSH 私钥和 HTTPS 密码使用 AES-256-GCM 加密存储 |
| 源文件回滚 | 回滚操作会覆写源文件（需要用户确认），回滚前展示变更文件列表 |
| 仓库删除 | 删除仓库仅移除数据库记录和调度任务，保留文件系统上的数据不丢失 |

---

**文档版本**：v1.1  
**状态**：已确认  
**编制日期**：2026-06-12
