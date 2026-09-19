import React, { useEffect, useState, useCallback } from 'react';
import {
  Button,
  Space,
  Typography,
  Table,
  Tag,
  Progress,
  Card,
  Statistic,
  Row,
  Col,
  Empty,
  message,
  Spin,
  Tooltip,
  Alert,
  Modal,
  Input,
  Divider,
} from 'antd';
import {
  PlayCircleOutlined,
  MessageOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  HistoryOutlined,
  RollbackOutlined,
  FileOutlined,
  FolderOutlined,
  SwapOutlined,
  SendOutlined,
  ExclamationCircleOutlined,
  CheckOutlined,
  WarningOutlined,
  CaretRightOutlined,
  CaretDownOutlined,
  UndoOutlined,
  CodeOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import relativeTime from 'dayjs/plugin/relativeTime';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { useAppStore } from '../../store/appStore';
import { fetchChanges, fetchBackupHistory as fetchHistoryApi } from '../../api/client';
import type { CommitEntry, CommitFileChange, CommitFileContent } from '../../types';
import RollbackConfirmModal from './RollbackConfirmModal';
import RollbackResultModal from './RollbackResultModal';

dayjs.extend(relativeTime);

interface BackupPanelProps {
  repoId: string;
  /** 该面板是否为当前激活的 Tab —— 激活时刷新未提交变更数。 */
  active?: boolean;
}

const statusTagConfig: Record<string, { color: string; icon: React.ReactNode }> = {
  success: {
    color: 'green',
    icon: <CheckCircleOutlined />,
  },
  failure: {
    color: 'red',
    icon: <CloseCircleOutlined />,
  },
  running: {
    color: 'blue',
    icon: <ClockCircleOutlined />,
  },
};

const BackupPanel: React.FC<BackupPanelProps> = ({ repoId, active = false }) => {
  const { t, i18n } = useTranslation();
  const dayjsLocale = i18n.resolvedLanguage?.toLowerCase().startsWith('zh') ? 'zh-cn' : 'en';

  const backupProgress = useAppStore((s) => s.backupProgress);
  const backupHistory = useAppStore((s) => s.backupHistory);
  const commitFilesByHash = useAppStore((s) => s.commitFilesByHash);
  const rollbackResult = useAppStore((s) => s.rollbackResult);
  const rollbackLoading = useAppStore((s) => s.rollbackLoading);

  const currentRepo = useAppStore((s) => s.currentRepo);
  const triggerBackup = useAppStore((s) => s.triggerBackup);
  const pushRepo = useAppStore((s) => s.pushRepo);
  const gitInitRepo = useAppStore((s) => s.gitInitRepo);
  const fetchBackupHistory = useAppStore((s) => s.fetchBackupHistory);
  const fetchCommitFiles = useAppStore((s) => s.fetchCommitFiles);
  const rollbackFiles = useAppStore((s) => s.rollbackFiles);
  const clearRollbackResult = useAppStore((s) => s.clearRollbackResult);

  const commitFileContent = useAppStore((s) => s.commitFileContent);
  const commitFileContentLoading = useAppStore((s) => s.commitFileContentLoading);
  const restoreFileLoading = useAppStore((s) => s.restoreFileLoading);
  const fetchCommitFileContent = useAppStore((s) => s.fetchCommitFileContent);
  const restoreCommitFile = useAppStore((s) => s.restoreCommitFile);
  const clearCommitFileContent = useAppStore((s) => s.clearCommitFileContent);

  const [backingUp, setBackingUp] = useState(false);
  const [pushing, setPushing] = useState(false);
  const [initializing, setInitializing] = useState(false);
  const [forcePushConfirmOpen, setForcePushConfirmOpen] = useState(false);
  const [commitModalOpen, setCommitModalOpen] = useState(false);
  const [commitMessage, setCommitMessage] = useState('');
  const [page, setPage] = useState(1);
  const pageSize = 10;
  const [uncommitted, setUncommitted] = useState<string[]>([]);
  const [totalCommits, setTotalCommits] = useState<number | null>(null);

  // Rollback modal state
  const [expandedCommitHash, setExpandedCommitHash] = useState<string | null>(null);
  const [commitFilesLoading, setCommitFilesLoading] = useState(false);
  const [confirmModalOpen, setConfirmModalOpen] = useState(false);
  const [resultModalOpen, setResultModalOpen] = useState(false);
  const [rollbackTarget, setRollbackTarget] = useState<{
    commitHash: string;
    commitMessage: string;
    commitDate: string;
    fileCount: number;
    isFull: boolean;
    paths?: string[];
  } | null>(null);

  // File-level preview and restore state
  const [expandedFilePath, setExpandedFilePath] = useState<string | null>(null);
  const [fileContentCache, setFileContentCache] = useState<Record<string, CommitFileContent>>({});

  useEffect(() => {
    if (repoId) {
      fetchBackupHistory(repoId, pageSize, 0);
    }
  }, [repoId, fetchBackupHistory]);

  // FR-30：data/ 下未提交的变更，作为「有新变更」的信号
  const refreshChanges = useCallback(async () => {
    try {
      setUncommitted((await fetchChanges(repoId)).changes);
    } catch {
      setUncommitted([]);
    }
  }, [repoId]);

  /** 历史总量：表格按页加载，这里用一次大 limit 的查询取总数。 */
  const refreshTotal = useCallback(async () => {
    try {
      setTotalCommits((await fetchHistoryApi(repoId, 1000, 0)).length);
    } catch {
      setTotalCommits(null);
    }
  }, [repoId]);

  useEffect(() => {
    refreshTotal();
  }, [refreshTotal]);

  // 切到本 Tab 时刷新一次，避免在 Browse 里编辑后数字过期
  useEffect(() => {
    if (active) {
      refreshChanges();
    }
  }, [active, refreshChanges]);

  useEffect(() => {
    if (rollbackResult) {
      setResultModalOpen(true);
    }
  }, [rollbackResult]);

  const handleBackupClick = () => {
    setCommitMessage(
      t('backup.commit.defaultMessage', {
        date: dayjs().format('YYYY-MM-DD HH:mm:ss'),
      }),
    );
    setCommitModalOpen(true);
  };

  const handleBackupConfirm = async () => {
    setCommitModalOpen(false);
    setBackingUp(true);
    try {
      const result = await triggerBackup(repoId, commitMessage || undefined);
      if (result) {
        message.success(
          result.commit_hash
            ? t('backup.toast.backupCommitted', {
                changed: result.files_changed,
                removed: result.files_removed,
              })
            : t('backup.toast.noChanges'),
        );
      } else {
        message.success(t('backup.toast.completed'));
      }
      fetchBackupHistory(repoId, pageSize, 0);
      refreshChanges();
      refreshTotal();
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setBackingUp(false);
    }
  };

  const handlePush = async () => {
    setPushing(true);
    try {
      await pushRepo(repoId);
      message.success(t('backup.toast.pushSuccess'));
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setPushing(false);
    }
  };

  const handleForcePush = async () => {
    setPushing(true);
    setForcePushConfirmOpen(false);
    try {
      await pushRepo(repoId, true);
      message.success(t('backup.toast.forcePushSuccess'));
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setPushing(false);
    }
  };

  const handleGitInit = async () => {
    setInitializing(true);
    try {
      await gitInitRepo(repoId);
      message.success(t('backup.toast.gitInitSuccess'));
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setInitializing(false);
    }
  };

  const handlePageChange = (newPage: number) => {
    setPage(newPage);
    fetchBackupHistory(repoId, pageSize, (newPage - 1) * pageSize);
  };

  const handleExpandRow = async (expanded: boolean, record: CommitEntry) => {
    if (expanded) {
      setExpandedCommitHash(record.hash);
      // Clear file-level preview state when switching to a different commit
      setExpandedFilePath(null);
      setFileContentCache({});
      clearCommitFileContent();
      // Check if we already have cached data for this commit
      if (!commitFilesByHash[record.hash]) {
        setCommitFilesLoading(true);
        try {
          await fetchCommitFiles(repoId, record.hash);
        } catch (err) {
          // Error handled by store
        } finally {
          setCommitFilesLoading(false);
        }
      }
    } else {
      setExpandedCommitHash(null);
      setExpandedFilePath(null);
      setFileContentCache({});
      clearCommitFileContent();
    }
  };

  /** 单个文件回滚：新模型下回滚以文件（data/ 相对路径）为单位。 */
  const handleSingleFileRollback = (
    commitHash: string,
    path: string,
    commitMessage: string,
    commitDate: string,
  ) => {
    setRollbackTarget({
      commitHash,
      commitMessage,
      commitDate,
      fileCount: 1,
      isFull: false,
      paths: [path],
    });
    setConfirmModalOpen(true);
  };

  const handleFullRollback = (
    commitHash: string,
    commitMessage: string,
    commitDate: string,
    count: number,
  ) => {
    setRollbackTarget({
      commitHash,
      commitMessage,
      commitDate,
      fileCount: count,
      isFull: true,
    });
    setConfirmModalOpen(true);
  };

  const handleConfirmRollback = async () => {
    if (!rollbackTarget) return;

    try {
      await rollbackFiles(repoId, {
        commit_hash: rollbackTarget.commitHash,
        paths: rollbackTarget.paths,
      });
      message.success(t('rollback.toast.completed'));
      fetchBackupHistory(repoId, pageSize, (page - 1) * pageSize);
      refreshChanges();
      refreshTotal();
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setConfirmModalOpen(false);
    }
  };

  const handleCloseResult = () => {
    setResultModalOpen(false);
    clearRollbackResult();
  };

  const handleFileExpand = async (filePath: string, commitHash: string, changeType: string) => {
    // Don't allow preview for deleted files
    if (changeType === 'D') {
      return;
    }

    if (expandedFilePath === filePath) {
      setExpandedFilePath(null);
      return;
    }

    setExpandedFilePath(filePath);

    // Load content if not cached
    if (!fileContentCache[filePath]) {
      try {
        const content = await fetchCommitFileContent(repoId, commitHash, filePath);
        setFileContentCache((prev) => ({ ...prev, [filePath]: content }));
      } catch {
        // Error handled by store
      }
    }
  };

  const handleFileRestore = useCallback(async (filePath: string, commitHash: string) => {
    Modal.confirm({
      title: t('rollback.restoreFile.title'),
      icon: <UndoOutlined />,
      content: (
        <div>
          <Typography.Paragraph>
            {t('rollback.restoreFile.questionPrefix')}
            <Typography.Text code>{`data/${filePath}`}</Typography.Text>
            {t('rollback.restoreFile.questionSuffix')}
          </Typography.Paragraph>
          <Typography.Text type="warning">
            {t('rollback.restoreFile.warning')}
          </Typography.Text>
        </div>
      ),
      okText: t('rollback.action.restore'),
      okButtonProps: { danger: true },
      cancelText: t('backup.action.cancel'),
      onOk: async () => {
        try {
          await restoreCommitFile(repoId, commitHash, filePath);
          message.success(t('rollback.toast.fileRestored', { path: filePath }));
          // 内容写回 data/，本机链接自动反映；只需刷新条目状态
          useAppStore.getState().fetchEntries(repoId);
          fetchBackupHistory(repoId, pageSize, (page - 1) * pageSize);
          refreshChanges();
          refreshTotal();
        } catch (err) {
          if (err instanceof Error) {
            message.error(err.message);
          }
          throw err;
        }
      },
    });
  }, [
    repoId,
    restoreCommitFile,
    fetchBackupHistory,
    page,
    pageSize,
    refreshChanges,
    refreshTotal,
    t,
  ]);

  const renderFilePreview = (filePath: string) => {
    const content = fileContentCache[filePath];

    if (commitFileContentLoading && expandedFilePath === filePath && !content) {
      return (
        <div style={{ padding: '12px 24px', textAlign: 'center' }}>
          <Spin size="small" />
          <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
            {t('backup.preview.loading')}
          </Typography.Text>
        </div>
      );
    }

    if (!content) {
      return null;
    }

    if (content.text && content.content) {
      const truncated = content.truncated || false;
      const maxPreviewLines = 50;
      const lines = content.content.split('\n');
      const displayLines = lines.slice(0, maxPreviewLines);
      const isTruncatedLines = lines.length > maxPreviewLines;

      return (
        <div
          style={{
            padding: '8px 24px 8px 40px',
            maxHeight: 400,
            overflow: 'auto',
          }}
        >
          <pre
            style={{
              margin: 0,
              padding: 12,
              background: '#1e1e1e',
              color: '#d4d4d4',
              borderRadius: 4,
              fontSize: 12,
              lineHeight: 1.5,
              overflow: 'auto',
              maxHeight: 320,
            }}
          >
            {displayLines.join('\n')}
            {(truncated || isTruncatedLines) && (
              <Typography.Text type="warning" style={{ display: 'block', marginTop: 8, color: '#f0ad4e' }}>
                {t('backup.preview.truncated', { count: maxPreviewLines })}
              </Typography.Text>
            )}
          </pre>
        </div>
      );
    }

    if (!content.text) {
      return (
        <div style={{ padding: '8px 24px 8px 40px' }}>
          <Space>
            <Tag color="default">{t('backup.preview.binaryFile')}</Tag>
            <Typography.Text type="secondary">
              {content.mime_type} -{' '}
              {t('backup.preview.fileSize', {
                size: content.size.toLocaleString(i18n.resolvedLanguage),
              })}
            </Typography.Text>
          </Space>
        </div>
      );
    }

    return null;
  };

  /** 按变更文件分组：新模型下回滚以文件为单位，每行对应 data/ 下的一个路径。 */
  const groupChangedFiles = (files: CommitFileChange[]) =>
    files.map((file) => ({ id: file.relative_path, type: 'file', files: [file] }));

  const columns: ColumnsType<CommitEntry> = [
    {
      title: t('backup.table.commit'),
      dataIndex: 'hash',
      key: 'hash',
      width: 110,
      render: (hash: string) =>
        hash ? (
          <Typography.Text code style={{ fontSize: 11 }}>
            {hash.substring(0, 8)}
          </Typography.Text>
        ) : (
          '-'
        ),
    },
    {
      title: t('backup.table.author'),
      dataIndex: 'author',
      key: 'author',
      width: 150,
      ellipsis: true,
    },
    {
      title: t('backup.table.date'),
      dataIndex: 'date',
      key: 'date',
      width: 180,
      render: (val: string) =>
        val ? dayjs(val).format('YYYY-MM-DD HH:mm:ss') : '-',
    },
    {
      title: t('backup.table.message'),
      dataIndex: 'message',
      key: 'message',
      ellipsis: true,
    },
  ];

  const expandedRowRender = (record: CommitEntry) => {
    const recordFiles = commitFilesByHash[record.hash] || [];

    if (commitFilesLoading && expandedCommitHash === record.hash) {
      return (
        <div style={{ textAlign: 'center', padding: 24 }}>
          <Spin size="small" />
          <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
            {t('backup.history.loadingFiles')}
          </Typography.Text>
        </div>
      );
    }

    if (recordFiles.length === 0 && expandedCommitHash === record.hash) {
      return <Empty description={t('backup.history.noChangedFiles')} />;
    }

    const fileGroups = groupChangedFiles(recordFiles);

    return (
      <div style={{ padding: '8px 0' }}>
        <Typography.Text strong style={{ marginBottom: 8, display: 'block' }}>
          {t('backup.history.filesChanged', { count: recordFiles.length })}
        </Typography.Text>

        {fileGroups.map((group) => (
          <div
            key={group.id}
            style={{
              marginBottom: 6,
              border: '1px solid #f0f0f0',
              borderRadius: 6,
              overflow: 'hidden',
            }}
          >
            {/* Group header */}
            <div
              style={{
                padding: '6px 12px',
                background: '#fafafa',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
              }}
            >
              <Space>
                {group.type === 'directory' ? (
                  <FolderOutlined style={{ color: '#faad14' }} />
                ) : (
                  <FileOutlined style={{ color: '#1890ff' }} />
                )}
                <Typography.Text strong>
                  {group.files[0]?.relative_path || group.id}
                </Typography.Text>
                <Tag color={group.type === 'directory' ? 'orange' : 'blue'}>
                  {group.type === 'directory'
                    ? t('backup.group.directory')
                    : t('backup.group.file')}
                </Tag>
                {group.files.length > 1 && (
                  <Tag>
                    {t('backup.group.fileCount', {
                      count: group.files.length,
                    })}
                  </Tag>
                )}
              </Space>
              <Tooltip title={t('rollback.tooltip.rollbackFile')}>
                <Button
                  type="link"
                  size="small"
                  icon={<RollbackOutlined />}
                  onClick={() =>
                    handleSingleFileRollback(
                      record.hash,
                      group.id,
                      record.message,
                      record.date,
                    )
                  }
                >
                  {t('rollback.action.rollback')}
                </Button>
              </Tooltip>
            </div>

            {/* Individual files within the group */}
            {group.files.map((file) => {
              const isExpanded = expandedFilePath === file.relative_path;
              const isDeleted = file.change_type === 'D';
              return (
                <div key={file.relative_path}>
                  <div
                    style={{
                      padding: '4px 12px 4px 28px',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      borderTop: '1px solid #f5f5f5',
                      cursor: isDeleted ? 'default' : 'pointer',
                      transition: 'background 0.15s',
                      opacity: isDeleted ? 0.6 : 1,
                    }}
                    onClick={() => handleFileExpand(file.relative_path, record.hash, file.change_type)}
                    onMouseEnter={(e) => {
                      if (!isDeleted) {
                        (e.currentTarget as HTMLElement).style.background = '#f5f5f5';
                      }
                    }}
                    onMouseLeave={(e) => {
                      (e.currentTarget as HTMLElement).style.background = 'transparent';
                    }}
                  >
                    <Space>
                      {isExpanded ? (
                        <CaretDownOutlined style={{ fontSize: 10, color: '#999' }} />
                      ) : (
                        <CaretRightOutlined style={{ fontSize: 10, color: '#999' }} />
                      )}
                      <FileOutlined style={{ color: isDeleted ? '#ff4d4f' : '#1890ff', fontSize: 13 }} />
                      <Typography.Text style={{ fontSize: 13, textDecoration: isDeleted ? 'line-through' : 'none' }}>
                        {file.relative_path}
                      </Typography.Text>
                      <Tag
                        color={
                          file.change_type === 'A'
                            ? 'green'
                            : file.change_type === 'D'
                            ? 'red'
                            : 'blue'
                        }
                        style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}
                      >
                        {file.change_type === 'A'
                          ? t('backup.change.added')
                          : file.change_type === 'D'
                            ? t('backup.change.deleted')
                            : t('backup.change.modified')}
                      </Tag>
                    </Space>
                    <Button
                      type="link"
                      size="small"
                      icon={<UndoOutlined />}
                      loading={restoreFileLoading}
                      disabled={isDeleted}
                      onClick={(e) => {
                        e.stopPropagation();
                        handleFileRestore(file.relative_path, record.hash);
                      }}
                      style={{ fontSize: 12 }}
                    >
                      {t('rollback.action.restore')}
                    </Button>
                  </div>

                  {/* Preview content when expanded */}
                  {isExpanded && !isDeleted && renderFilePreview(file.relative_path)}
                </div>
              );
            })}
          </div>
        ))}

        {fileGroups.length > 1 && (
          <div style={{ marginTop: 12, textAlign: 'right' }}>
            <Button
              type="primary"
              size="small"
              ghost
              icon={<SwapOutlined />}
              onClick={() =>
                handleFullRollback(
                  record.hash,
                  record.message,
                  record.date,
                  fileGroups.length,
                )
              }
            >
              {t('rollback.action.rollbackAll', { count: fileGroups.length })}
            </Button>
          </div>
        )}
      </div>
    );
  };

  const isBackingUp = backupProgress?.status === 'running';
  const isGitInit = currentRepo?.git_initialized ?? false;
  const hasRemote = !!(currentRepo?.remote_url || currentRepo?.has_remote);
  const repoStatusLabel =
    currentRepo?.status === 'active'
      ? t('backup.status.active')
      : currentRepo?.status === 'error'
        ? t('backup.status.error')
        : currentRepo?.status === 'backing_up'
          ? t('backup.status.backingUp')
          : t('backup.status.unknown');

  return (
    <div>
      {!isGitInit && (
        <Alert
          message={t('backup.git.notInitialized')}
          description={t('backup.git.notInitializedDescription')}
          type="warning"
          showIcon
          icon={<ExclamationCircleOutlined />}
          style={{ marginBottom: 16 }}
        />
      )}

      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card size="small">
            <Statistic
              title={t('backup.stats.lastBackup')}
              value={
                currentRepo?.last_backup_at
                  ? dayjs(currentRepo.last_backup_at).locale(dayjsLocale).fromNow()
                  : t('backup.stats.never')
              }
              valueStyle={{ fontSize: 16 }}
              prefix={<ClockCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic
              title={t('backup.stats.totalBackups')}
              value={totalCommits === null ? '-' : totalCommits >= 1000 ? '1000+' : totalCommits}
              prefix={<HistoryOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic
              title={t('backup.stats.status')}
              value={repoStatusLabel}
              valueStyle={{
                color:
                  currentRepo?.status === 'active'
                    ? '#52c41a'
                    : currentRepo?.status === 'error'
                    ? '#ff4d4f'
                    : '#1890ff',
              }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Tooltip
              title={
                uncommitted.length > 0 ? (
                  <div style={{ maxWidth: 380 }}>
                    {uncommitted.slice(0, 10).map((line) => (
                      <div key={line} style={{ fontFamily: 'monospace', fontSize: 12 }}>
                        {line.trim()}
                      </div>
                    ))}
                    {uncommitted.length > 10 && (
                      <div>
                        {t('backup.uncommitted.more', {
                          count: uncommitted.length - 10,
                        })}
                      </div>
                    )}
                  </div>
                ) : (
                  t('backup.uncommitted.none')
                )
              }
            >
              <div>
                <Statistic
                  title={t('backup.stats.uncommittedChanges')}
                  value={uncommitted.length}
                  valueStyle={{
                    color: uncommitted.length > 0 ? '#faad14' : '#52c41a',
                  }}
                  prefix={<ExclamationCircleOutlined />}
                />
              </div>
            </Tooltip>
          </Card>
        </Col>
      </Row>

      <Space wrap style={{ marginBottom: 16 }}>
        <Button
          icon={isGitInit ? <CheckOutlined /> : <ExclamationCircleOutlined />}
          onClick={handleGitInit}
          loading={initializing}
          type={isGitInit ? 'default' : 'primary'}
          disabled={isGitInit && !initializing}
        >
          {initializing
            ? t('backup.action.initializing')
            : isGitInit
              ? t('backup.action.gitInitialized')
              : t('backup.action.gitInit')}
        </Button>

        <Button
          type="primary"
          size="large"
          icon={<PlayCircleOutlined />}
          onClick={handleBackupClick}
          loading={backingUp || isBackingUp}
          disabled={!isGitInit || isBackingUp}
        >
          {isBackingUp ? t('backup.action.backingUp') : t('backup.action.trigger')}
        </Button>

        {hasRemote && (
          <Space>
            <Button
              icon={<SendOutlined />}
              onClick={handlePush}
              loading={pushing}
              disabled={!isGitInit}
            >
              {pushing ? t('backup.action.pushing') : t('backup.action.pushRemote')}
            </Button>
            <Tooltip title={t('backup.tooltip.forcePush')}>
              <Button
                danger
                icon={<WarningOutlined />}
                onClick={() => setForcePushConfirmOpen(true)}
                loading={pushing}
                disabled={!isGitInit}
              >
                {t('backup.action.forcePush')}
              </Button>
            </Tooltip>
          </Space>
        )}
      </Space>

      {isBackingUp && (
        <Card size="small" style={{ marginBottom: 16 }}>
          <Space direction="vertical" style={{ width: '100%' }}>
            <Typography.Text>
              {backupProgress?.message === 'Starting backup...'
                ? t('backup.progress.starting')
                : backupProgress?.message}
            </Typography.Text>
            <Progress
              percent={backupProgress?.progress || 0}
              status="active"
              strokeColor={{ from: '#108ee9', to: '#87d068' }}
            />
          </Space>
        </Card>
      )}

      <Typography.Title level={5}>{t('backup.history.title')}</Typography.Title>

      {backupHistory.length === 0 ? (
        <Empty description={t('backup.history.empty')} />
      ) : (
        <Table
          columns={columns}
          dataSource={backupHistory}
          rowKey="hash"
          expandable={{
            expandedRowRender,
            onExpand: handleExpandRow,
            expandRowByClick: true,
          }}
          pagination={{
            current: page,
            pageSize,
            onChange: handlePageChange,
            showSizeChanger: false,
            hideOnSinglePage: true,
          }}
          size="small"
          scroll={{ x: 800 }}
        />
      )}

      <Modal
        title={t('backup.commitModal.title')}
        open={commitModalOpen}
        onCancel={() => setCommitModalOpen(false)}
        onOk={handleBackupConfirm}
        okText={t('backup.commitModal.start')}
        cancelText={t('backup.action.cancel')}
        confirmLoading={backingUp}
        okButtonProps={{ icon: <PlayCircleOutlined /> }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Typography.Text type="secondary">
            {t('backup.commitModal.description')}
          </Typography.Text>
          <Input.TextArea
            value={commitMessage}
            onChange={(e) => setCommitMessage(e.target.value)}
            rows={3}
            placeholder={t('backup.commitModal.placeholder')}
          />
        </Space>
      </Modal>

      <RollbackConfirmModal
        open={confirmModalOpen}
        commitHash={rollbackTarget?.commitHash || ''}
        commitMessage={rollbackTarget?.commitMessage || ''}
        commitDate={rollbackTarget?.commitDate || ''}
        fileCount={rollbackTarget?.fileCount || 0}
        isFullRollback={rollbackTarget?.isFull || false}
        onCancel={() => setConfirmModalOpen(false)}
        onConfirm={handleConfirmRollback}
        loading={rollbackLoading}
      />

      <RollbackResultModal
        open={resultModalOpen}
        result={rollbackResult}
        onClose={handleCloseResult}
      />

      <Modal
        title={t('backup.forceModal.title')}
        open={forcePushConfirmOpen}
        onCancel={() => setForcePushConfirmOpen(false)}
        onOk={handleForcePush}
        okText={t('backup.action.forcePush')}
        cancelText={t('backup.action.cancel')}
        okButtonProps={{ danger: true }}
        confirmLoading={pushing}
      >
        <Typography.Text>
          {t('backup.forceModal.warningPrefix')}
          <Typography.Text strong type="danger">
            {t('backup.forceModal.action')}
          </Typography.Text>
          {t('backup.forceModal.warningMiddle')}
          <Typography.Text strong type="danger">
            {t('backup.forceModal.loss')}
          </Typography.Text>
          {t('backup.forceModal.warningSuffix')}
        </Typography.Text>
        <Typography.Paragraph style={{ marginTop: 12 }}>
          {t('backup.forceModal.question')}
        </Typography.Paragraph>
      </Modal>
    </div>
  );
};

export default BackupPanel;
