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
import { useTranslation } from 'react-i18next';
import type { BackupRepo } from '../../types';

dayjs.extend(relativeTime);

interface RepoCardProps {
  repo: BackupRepo;
}

const statusConfig: Record<
  string,
  { color: string; icon: React.ReactNode; translationKey: string }
> = {
  active: {
    color: 'green',
    icon: <CheckCircleOutlined />,
    translationKey: 'repo.status.active',
  },
  error: {
    color: 'red',
    icon: <ExclamationCircleOutlined />,
    translationKey: 'repo.status.error',
  },
  backing_up: {
    color: 'blue',
    icon: <SyncOutlined spin />,
    translationKey: 'repo.status.backing_up',
  },
};

const RepoCard: React.FC<RepoCardProps> = ({ repo }) => {
  const navigate = useNavigate();
  const { t } = useTranslation();
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
          {t('repo.card.open')}
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
              {t(status.translationKey)}
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
                  <ClockCircleOutlined />{' '}
                  {t('repo.card.lastBackup', { time: dayjs(repo.last_backup_at).fromNow() })}
                </Typography.Text>
              ) : (
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  <ClockCircleOutlined /> {t('repo.card.noBackups')}
                </Typography.Text>
              )}
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {t('repo.card.created', { time: dayjs(repo.created_at).fromNow() })}
              </Typography.Text>
            </Space>
          </div>
        }
      />
    </Card>
  );
};

export default RepoCard;
