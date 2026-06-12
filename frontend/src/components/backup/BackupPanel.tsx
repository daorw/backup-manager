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
} from 'antd';
import {
  PlayCircleOutlined,
  ReloadOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  HistoryOutlined,
  FileTextOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import type { ColumnsType } from 'antd/es/table';
import { useAppStore } from '../../store/appStore';
import type { CommitEntry } from '../../types';

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

  const currentRepo = useAppStore((s) => s.currentRepo);
  const triggerBackup = useAppStore((s) => s.triggerBackup);
  const fetchBackupHistory = useAppStore((s) => s.fetchBackupHistory);

  const [backingUp, setBackingUp] = useState(false);
  const [page, setPage] = useState(1);
  const pageSize = 10;

  useEffect(() => {
    if (repoId) {
      fetchBackupHistory(repoId, pageSize, 0);
    }
  }, [repoId, fetchBackupHistory]);

  const handleBackup = async () => {
    setBackingUp(true);
    try {
      await triggerBackup(repoId);
      message.success('Backup completed');
      fetchBackupHistory(repoId, pageSize, 0);
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setBackingUp(false);
    }
  };

  const handlePageChange = (newPage: number) => {
    setPage(newPage);
    fetchBackupHistory(repoId, pageSize, (newPage - 1) * pageSize);
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

  const isBackingUp = backupProgress?.status === 'running';

  return (
    <div>
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

      <Space style={{ marginBottom: 16 }}>
        <Button
          type="primary"
          size="large"
          icon={<PlayCircleOutlined />}
          onClick={handleBackup}
          loading={backingUp || isBackingUp}
          disabled={isBackingUp}
        >
          {isBackingUp ? 'Backing up...' : 'Trigger Backup'}
        </Button>
        <Button
          icon={<ReloadOutlined />}
          onClick={() => fetchBackupHistory(repoId, pageSize, 0)}
        >
          Refresh
        </Button>
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
    </div>
  );
};

export default BackupPanel;
