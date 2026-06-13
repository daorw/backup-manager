import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, Tag, Typography, Space, Button } from 'antd';
import {
  FolderOutlined,
  RightCircleOutlined,
  ClockCircleOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import 'dayjs/locale/zh-cn';
import type { BackupRepo } from '../../types';

dayjs.extend(relativeTime);
dayjs.locale('zh-cn');

interface RepoCardProps {
  repo: BackupRepo;
}

const statusConfig: Record<
  string,
  { color: string; icon: React.ReactNode; text: string }
> = {
  active: {
    color: 'green',
    icon: <CheckCircleOutlined />,
    text: 'Active',
  },
  error: {
    color: 'red',
    icon: <ExclamationCircleOutlined />,
    text: 'Error',
  },
  backing_up: {
    color: 'blue',
    icon: <SyncOutlined spin />,
    text: 'Backing up',
  },
};

const RepoCard: React.FC<RepoCardProps> = ({ repo }) => {
  const navigate = useNavigate();
  const status = statusConfig[repo.status] || statusConfig.active;

  return (
    <Card
      hoverable
      style={{ height: '100%' }}
      actions={[
        <Button
          type="link"
          icon={<RightCircleOutlined />}
          onClick={() => navigate(`/repos/${repo.id}`)}
        >
          Open
        </Button>,
      ]}
    >
      <Card.Meta
        avatar={<FolderOutlined style={{ fontSize: 32, color: '#1890ff' }} />}
        title={
          <Space>
            <Typography.Text strong style={{ fontSize: 16 }}>
              {repo.name}
            </Typography.Text>
            <Tag color={status.color} icon={status.icon}>
              {status.text}
            </Tag>
          </Space>
        }
        description={
          <div>
            <Typography.Paragraph
              ellipsis={{ rows: 1, tooltip: repo.path }}
              style={{ marginBottom: 8 }}
            >
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {repo.path}
              </Typography.Text>
            </Typography.Paragraph>
            <Space direction="vertical" size={2}>
              {repo.last_backup_at ? (
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  <ClockCircleOutlined /> Last backup:{' '}
                  {dayjs(repo.last_backup_at).fromNow()}
                </Typography.Text>
              ) : (
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  <ClockCircleOutlined /> No backups yet
                </Typography.Text>
              )}
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                Created {dayjs(repo.created_at).fromNow()}
              </Typography.Text>
            </Space>
          </div>
        }
      />
    </Card>
  );
};

export default RepoCard;
