# Backup Manager - Quick Start Guide

## Overview

Backup Manager 是一个文件/目录聚合备份可视化管理工具。基于 Git 的反向追踪模式（白名单机制），让用户以"指定要备份什么"而非"排除什么"的直观方式管理备份。

## Dashboard

主界面展示所有备份仓库的状态和基本信息。

![Dashboard](../assets/repository-dashboard.png)

- 查看所有备份仓库
- 查看仓库状态（active/inactive）
- 快速操作（Open, Delete）
- 创建新仓库

## Repository Management

### Creating a Repository

1. 点击 Dashboard 右上角 "+ Create Repository" 按钮
2. 输入仓库名称和路径
3. 配置基本设置

### Repository Detail View

打开仓库后，可以看到四个主要标签页：**Symlinks**、**Preview**、**Backup**、**Config**。

## Symlinks Tab

Symlinks 标签页展示所有通过软链接备份的文件和目录。

![Symlinks Tab](../assets/symlinks.png)

### 功能：
- 树形展示软链接层级
- 点击 "+ Add Symlink" 添加新软链接
- 点击 "Refresh" 刷新列表
- 查看源文件原始路径

### 添加软链接：

点击 Symlinks 标签页中的 "+ Add Symlink" 按钮，打开添加软链接对话框：

![添加软链接对话框](../assets/add-symlink.jpeg)

1. **Source Path**：输入或浏览选择要备份的文件或目录路径（如 `~/.config/opencode/opencode.json`）
2. **Link Name**：指定备份文件在 `.links/` 中的相对存储路径（如 `opencode/opencode.json`），用于定义仓库内的目录结构
3. **File Browser**：预览当前 Link Name 根目录下已备份的文件
4. 点击 **Add** 确认添加，或 **Cancel** 取消

## Preview Tab

Preview 标签页允许您直接查看和编辑备份的文件。

![Preview Tab](../assets/preview.png)

### 功能：
- 浏览文件结构
- 预览文件内容
- 点击 "Edit" 编辑文件
- 点击 "Save" 保存更改
- 查看文件元数据

### 文件操作：
1. 在左侧树形视图选择文件
2. 右侧预览文件内容
3. 点击 "Edit" 进入编辑模式
4. 点击 "Save" 保存更改

## Backup Tab

Backup 标签页显示备份历史和备份控制按钮。

![Backup Tab](../assets/backup.png)

### 功能：
- 查看上次备份时间
- 查看总备份次数
- 监控备份状态
- 执行备份操作

### 备份控制：
- **Git Init**: 初始化 Git 仓库
- **Trigger Backup**: 手动触发备份
- **Push to Remote**: 推送到远程仓库
- **Force Push**: 强制推送（谨慎使用）

### 备份历史：
- 查看 commit hash
- 查看提交作者
- 查看提交日期
- 查看提交信息

## Config Tab

Config 标签页管理仓库配置设置。

![Config Tab](../assets/git-remote-config.png)

### 配置选项：
- **Remote URL**: Git 远程仓库地址
- **Branch**: 备份目标分支
- **Git User Name**: 提交作者名称
- **Git User Email**: 提交作者邮箱
- **Automatic Backup**: 启用/禁用定时自动备份

## Git Authentication & Danger Zone

配置 Git 认证信息，以及危险操作区域。

![Git Authentication & Danger Zone](../assets/git-auth-config.png)

### 认证类型：
- **SSH Key**: 使用 SSH 私钥认证
- **HTTPS**: 使用用户名/密码认证

### SSH Key 配置：
1. 在 Authentication Type 下拉框选择 "SSH Key"
2. 输入 SSH 私钥路径（如 `~/.ssh/id_ed25519`）
3. 点击 "Save Authentication"

### 清除认证：
- 点击 "Clear" 删除已保存的认证信息

### Danger Zone ⚠️
- **Delete Repository**: 永久删除仓库所有数据（软链接、备份数据、Git 历史），此操作不可逆
- **Back to Dashboard**: 返回仓库列表

## Getting Started Workflow

1. **安装运行**: 下载并启动 Backup Manager
2. **创建仓库**: 设置第一个备份仓库
3. **添加软链接**: 指定要备份的文件/目录
4. **配置 Git**: 设置远程仓库和认证信息
5. **执行备份**: 运行第一次备份
6. **监控状态**: 查看备份状态和历史记录

## Best Practices

- 从少量重要文件开始
- 使用有意义的 commit message
- 为关键数据配置自动备份
- 定期验证备份完整性
- 妥善保管认证凭据

## Troubleshooting

### 常见问题：
- **备份失败**: 检查 Git 配置和认证信息
- **软链接不显示**: 验证文件权限和路径
- **远程推送失败**: 确保远程仓库存在且凭据正确

### 获取帮助：
- 查看应用日志获取详细错误信息
- 确认所有依赖已正确安装
- 确保文件权限正确