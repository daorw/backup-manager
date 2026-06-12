import React, { useEffect, useState } from 'react';
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
} from 'antd';
import {
  PlayCircleOutlined,
  ReloadOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  HistoryOutlined,
  RollbackOutlined,
  FileOutlined,
  FolderOutlined,
  SwapOutlined,
  GithubOutlined,
  SendOutlined,
  ExclamationCircleOutlined,
  CheckOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import type { ColumnsType } from 'antd/es/table';
import { useAppStore } from '../../store/appStore';
import type { CommitEntry, CommitFileChange } from '../../types';
import RollbackConfirmModal from './RollbackConfirmModal';
import RollbackResultModal from './RollbackResultModal';

dayjs.extend(relativeTime);

interface BackupPanelProps {
  repoId: string;
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

const BackupPanel: React.FC<BackupPanelProps> = ({ repoId }) => {
  const backupProgress = useAppStore((s) => s.backupProgress);
  const backupHistory = useAppStore((s) => s.backupHistory);
  const commitFiles = useAppStore((s) => s.commitFiles);
  const rollbackResult = useAppStore((s) => s.rollbackResult);
  const rollbackLoading = useAppStore((s) => s.rollbackLoading);

  const currentRepo = useAppStore((s) => s.currentRepo);
  const triggerBackup = useAppStore((s) => s.triggerBackup);
  const pushRepo = useAppStore((s) => s.pushRepo);
  const gitInitRepo = useAppStore((s) => s.gitInitRepo);
  const fetchBackupHistory = useAppStore((s) => s.fetchBackupHistory);
  const fetchCommitFiles = useAppStore((s) => s.fetchCommitFiles);
  const rollbackSourceFiles = useAppStore((s) => s.rollbackSourceFiles);
  const clearRollbackResult = useAppStore((s) => s.clearRollbackResult);

  const [backingUp, setBackingUp] = useState(false);
  const [pushing, setPushing] = useState(false);
  const [initializing, setInitializing] = useState(false);
  const [forcePushConfirmOpen, setForcePushConfirmOpen] = useState(false);
  const [page, setPage] = useState(1);
  const pageSize = 10;

  // Rollback modal state
  const [expandedCommitHash, setExpandedCommitHash] = useState<string | null>(null);
  const [commitFilesLoading, setCommitFilesLoading] = useState(false);
  const [confirmModalOpen, setConfirmModalOpen] = useState(false);
  const [resultModalOpen, setResultModalOpen] = useState(false);
  const [rollbackTarget, setRollbackTarget] = useState<{
    commitHash: string;
    commitMessage: string;
    commitDate: string;
    symlinkCount: number;
    isFull: boolean;
    symlinkIDs?: string[];
  } | null>(null);

  useEffect(() => {
    if (repoId) {
      fetchBackupHistory(repoId, pageSize, 0);
    }
  }, [repoId, fetchBackupHistory]);

  useEffect(() => {
    if (rollbackResult) {
      setResultModalOpen(true);
    }
  }, [rollbackResult]);

  const handleBackup = async () => {
    setBackingUp(true);
    try {
      const result = await triggerBackup(repoId);
      if (result) {
        const detail = result.commit_hash
          ? `Committed ${result.files_changed} changed, ${result.files_removed} removed`
          : 'No changes to commit';
        message.success(`Backup completed — ${detail}`);
      } else {
        message.success('Backup completed');
      }
      fetchBackupHistory(repoId, pageSize, 0);
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
      message.success('Pushed to remote successfully');
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
      message.success('Force pushed to remote successfully');
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
      message.success('Git repository initialized');
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
      setCommitFilesLoading(true);
      try {
        await fetchCommitFiles(repoId, record.hash);
      } catch (err) {
        // Error handled by store
      } finally {
        setCommitFilesLoading(false);
      }
    } else {
      setExpandedCommitHash(null);
    }
  };

  const handleSymlinkRollback = (
    commitHash: string,
    symlinkID: string,
    commitMessage: string,
    commitDate: string,
  ) => {
    setRollbackTarget({
      commitHash,
      commitMessage,
      commitDate,
      symlinkCount: 1,
      isFull: false,
      symlinkIDs: [symlinkID],
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
      symlinkCount: count,
      isFull: true,
    });
    setConfirmModalOpen(true);
  };

  const handleConfirmRollback = async () => {
    if (!rollbackTarget) return;

    try {
      await rollbackSourceFiles(repoId, {
        commit_hash: rollbackTarget.commitHash,
        symlink_ids: rollbackTarget.symlinkIDs,
      });
      message.success('Rollback completed');
      fetchBackupHistory(repoId, pageSize, (page - 1) * pageSize);
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

  const getUniqueSymlinks = (files: CommitFileChange[]) => {
    const grouped = new Map<string, { id: string; type: string; files: CommitFileChange[] }>();
    for (const file of files) {
      const key = file.symlink_id || file.relative_path;
      if (!grouped.has(key)) {
        const symType = file.symlink_type || 'file';
        grouped.set(key, { id: key, type: symType, files: [] });
      }
      grouped.get(key)!.files.push(file);
    }
    return Array.from(grouped.values());
  };

  const columns: ColumnsType<CommitEntry> = [
    {
      title: 'Commit',
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
      title: 'Author',
      dataIndex: 'author',
      key: 'author',
      width: 150,
      ellipsis: true,
    },
    {
      title: 'Date',
      dataIndex: 'date',
      key: 'date',
      width: 180,
      render: (val: string) =>
        val ? dayjs(val).format('YYYY-MM-DD HH:mm:ss') : '-',
    },
    {
      title: 'Message',
      dataIndex: 'message',
      key: 'message',
      ellipsis: true,
    },
  ];

  const expandedRowRender = (record: CommitEntry) => {
    if (commitFilesLoading && expandedCommitHash === record.hash) {
      return (
        <div style={{ textAlign: 'center', padding: 24 }}>
          <Spin size="small" />
          <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
            Loading changed files...
          </Typography.Text>
        </div>
      );
    }

    if (commitFiles.length === 0 && expandedCommitHash === record.hash) {
      return <Empty description="No changed files in this commit" />;
    }

    const symlinkGroups = getUniqueSymlinks(commitFiles);

    return (
      <div style={{ padding: '8px 0' }}>
        <Typography.Text strong style={{ marginBottom: 8, display: 'block' }}>
          Files changed ({commitFiles.length})
        </Typography.Text>

        {symlinkGroups.map((group) => (
          <div
            key={group.id}
            style={{
              padding: '6px 12px',
              marginBottom: 4,
              background: '#fafafa',
              borderRadius: 4,
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
              <Typography.Text>
                {group.files[0]?.relative_path || group.id}
              </Typography.Text>
              <Tag color={group.type === 'directory' ? 'orange' : 'blue'}>
                {group.type}
              </Tag>
              {group.files.length > 1 && (
                <Tag>{group.files.length} files</Tag>
              )}
            </Space>
            <Tooltip title="Restore this symlink's source files to the version in this commit">
              <Button
                type="link"
                size="small"
                icon={<RollbackOutlined />}
                onClick={() =>
                  handleSymlinkRollback(
                    record.hash,
                    group.id,
                    record.message,
                    record.date,
                  )
                }
              >
                Rollback
              </Button>
            </Tooltip>
          </div>
        ))}

        {symlinkGroups.length > 1 && (
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
                  symlinkGroups.length,
                )
              }
            >
              Rollback All ({symlinkGroups.length})
            </Button>
          </div>
        )}
      </div>
    );
  };

  const isBackingUp = backupProgress?.status === 'running';
  const isGitInit = currentRepo?.git_initialized ?? true;
  const hasRemote = !!(currentRepo?.remote_url || currentRepo?.has_remote);

  return (
    <div>
      {!isGitInit && (
        <Alert
          message="Git repository not initialized"
          description="The .git directory is missing. Click 'Git Init' below to initialize the repository before running backups."
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
              title="Last Backup"
              value={
                currentRepo?.last_backup_at
                  ? dayjs(currentRepo.last_backup_at).fromNow()
                  : 'Never'
              }
              valueStyle={{ fontSize: 16 }}
              prefix={<ClockCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic
              title="Total Backups"
              value={backupHistory.length}
              prefix={<HistoryOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic
              title="Status"
              value={currentRepo?.status || 'unknown'}
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
      </Row>

      <Space wrap style={{ marginBottom: 16 }}>
        <Button
          icon={isGitInit ? <CheckOutlined /> : <ExclamationCircleOutlined />}
          onClick={handleGitInit}
          loading={initializing}
          type={isGitInit ? 'default' : 'primary'}
          disabled={isGitInit && !initializing}
        >
          {initializing ? 'Initializing...' : isGitInit ? 'Git Init ✓' : 'Git Init'}
        </Button>

        <Button
          type="primary"
          size="large"
          icon={<PlayCircleOutlined />}
          onClick={handleBackup}
          loading={backingUp || isBackingUp}
          disabled={!isGitInit || isBackingUp}
        >
          {isBackingUp ? 'Backing up...' : 'Trigger Backup'}
        </Button>

        {hasRemote && (
          <Space>
            <Button
              icon={<SendOutlined />}
              onClick={handlePush}
              loading={pushing}
              disabled={!isGitInit}
            >
              {pushing ? 'Pushing...' : 'Push to Remote'}
            </Button>
            <Tooltip title="Force push overwrites remote history. Use when remote is out of sync with local.">
              <Button
                danger
                icon={<WarningOutlined />}
                onClick={() => setForcePushConfirmOpen(true)}
                loading={pushing}
                disabled={!isGitInit}
              >
                Force Push
              </Button>
            </Tooltip>
          </Space>
        )}
      </Space>

      {isBackingUp && (
        <Card size="small" style={{ marginBottom: 16 }}>
          <Space direction="vertical" style={{ width: '100%' }}>
            <Typography.Text>{backupProgress?.message}</Typography.Text>
            <Progress
              percent={backupProgress?.progress || 0}
              status="active"
              strokeColor={{ from: '#108ee9', to: '#87d068' }}
            />
          </Space>
        </Card>
      )}

      <Typography.Title level={5}>Backup History</Typography.Title>

      {backupHistory.length === 0 ? (
        <Empty description="No backup history yet" />
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

      <RollbackConfirmModal
        open={confirmModalOpen}
        commitHash={rollbackTarget?.commitHash || ''}
        commitMessage={rollbackTarget?.commitMessage || ''}
        commitDate={rollbackTarget?.commitDate || ''}
        symlinkCount={rollbackTarget?.symlinkCount || 0}
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
        title="Force Push Confirmation"
        open={forcePushConfirmOpen}
        onCancel={() => setForcePushConfirmOpen(false)}
        onOk={handleForcePush}
        okText="Force Push"
        okButtonProps={{ danger: true }}
        confirmLoading={pushing}
      >
        <Typography.Text>
          This will{' '}
          <Typography.Text strong type="danger">
            force push
          </Typography.Text>{' '}
          and overwrite the remote branch history. Any commits on the remote
          that are not in your local branch will be{' '}
          <Typography.Text strong type="danger">
            permanently lost
          </Typography.Text>
          .
        </Typography.Text>
        <Typography.Paragraph style={{ marginTop: 12 }}>
          Are you sure you want to continue?
        </Typography.Paragraph>
      </Modal>
    </div>
  );
};

export default BackupPanel;
