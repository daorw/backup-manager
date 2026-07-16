# AGENTS.md Template

Project conventions document. Copy to `<project>/AGENTS.md` and customize.

```markdown
# <Project Name> — 项目规范文档

## 项目概述

<One-paragraph description>

**核心价值**：<Why this project matters>

## 架构设计

### 分层架构

┌─────────────────────────────────────────────┐
│  Frontend (React SPA, embedded via Go embed) │
├─────────────────────────────────────────────┤
│  API Layer (Gin handlers)                   │
├─────────────────────────────────────────────┤
│  Service Layer (business logic)             │
├─────────────────────────────────────────────┤
│  Store Layer (SQLite) + External Engines    │
└─────────────────────────────────────────────┘

### 核心数据流

<Main data flow in a few ASCII lines>

## 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| 后端语言 | Go 1.22+ | 单二进制、跨平台编译 |
| HTTP 框架 | Gin | 轻量高性能 |
| 数据库 | SQLite (modernc.org/sqlite, 纯Go, 无需CGO) | 单文件存储 |
| 定时调度 | robfig/cron/v3 | 秒级精度 cron 表达式 |
| 系统托盘 | getlantern/systray | macOS/Windows |
| 加密 | crypto/aes + crypto/gcm | AES-256-GCM 加密存储 |
| 前端框架 | React 18 + TypeScript | — |
| 前端构建 | Vite 5 | — |
| UI 组件 | Ant Design 5 | — |
| 状态管理 | Zustand | 轻量级状态管理 |
| HTTP 客户端 | axios | — |
| 前后端一体 | Go embed.FS + Gin 静态文件服务 | 单二进制部署 |

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
- **API 调用**: 在 `api/client.ts` 中集中管理
- **状态管理**: 使用 Zustand 单一 store
- **组件模式**: 函数组件 + React Hooks
- **路由**: react-router-dom v6

## 环境与配置

- 应用数据目录：`~/.config/<app>/`
- 配置文件：`~/.config/<app>/config.json`
- 加密密钥：`~/.config/<app>/master.key`
- 数据库：`~/.config/<app>/<app>.db`
- 默认端口：<BACKEND_PORT>

## 常用命令

```bash
go run .                              # 开发 - 启动后端
cd frontend && npm run dev            # 开发 - 启动前端
./scripts/dev-start.sh                # 开发 - 一键启动
./scripts/dev-stop.sh                 # 开发 - 一键关闭
cd frontend && npm run build && cd .. # 生产构建
go build -o <app> .                   # 生产构建
go test ./... -count=1                # 运行测试
cd frontend && npx tsc --noEmit       # 前端类型检查
```
```

## Usage

1. Copy this template to `<project>/AGENTS.md`
2. Fill in all `<...>` placeholders
3. Add project-specific sections (API endpoints, database schema, design decisions)
4. Keep ENGLISH as primary; add Chinese mirror in `docs/zh/` if needed
