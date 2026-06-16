import React, { useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Tabs, Typography, Button, Space, Spin, Tag } from 'antd';
import {
  ArrowLeftOutlined,
  LinkOutlined,
  EyeOutlined,
  CloudUploadOutlined,
  SettingOutlined,
  FolderOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import { useAppStore } from '../store/appStore';
import SymlinkPanel from '../components/symlink/SymlinkPanel';
import PreviewPanel from '../components/preview/PreviewPanel';
import BackupPanel from '../components/backup/BackupPanel';
import ConfigPanel from '../components/config/ConfigPanel';

dayjs.extend(relativeTime);

const RepoDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const currentRepo = useAppStore((s) => s.currentRepo);
  const loading = useAppStore((s) => s.loading);
  const error = useAppStore((s) => s.error);
  const fetchRepo = useAppStore((s) => s.fetchRepo);

  useEffect(() => {
    if (id) {
      fetchRepo(id);
    }
  }, [id, fetchRepo]);

  if (loading && !currentRepo) {
    return (
      <div style={{ textAlign: 'center', padding: 80 }}>
        <Spin size="large" />
      </div>
    );
  }

  if (error && !currentRepo) {
    return (
      <div style={{ textAlign: 'center', padding: 80 }}>
        <Typography.Text type="danger">{error}</Typography.Text>
        <br />
        <Button onClick={() => navigate('/')} style={{ marginTop: 16 }}>
          Back to Dashboard
        </Button>
      </div>
    );
  }

  if (!currentRepo) {
    return (
      <div style={{ textAlign: 'center', padding: 80 }}>
        <Typography.Text type="secondary">Repository not found</Typography.Text>
        <br />
        <Button onClick={() => navigate('/')} style={{ marginTop: 16 }}>
          Back to Dashboard
        </Button>
      </div>
    );
  }

  const statusColor =
    currentRepo.status === 'active'
      ? 'green'
      : currentRepo.status === 'error'
      ? 'red'
      : 'blue';

  const tabItems = [
    {
      key: 'symlinks',
      label: (
        <Space>
          <LinkOutlined />
          <span>Symlinks</span>
        </Space>
      ),
      children: <SymlinkPanel repoId={currentRepo.id} repoPath={currentRepo.path} />,
    },
    {
      key: 'preview',
      label: (
        <Space>
          <EyeOutlined />
          <span>Preview</span>
        </Space>
      ),
      children: <PreviewPanel repoId={currentRepo.id} />,
    },
    {
      key: 'backup',
      label: (
        <Space>
          <CloudUploadOutlined />
          <span>Backup</span>
        </Space>
      ),
      children: <BackupPanel repoId={currentRepo.id} />,
    },
    {
      key: 'config',
      label: (
        <Space>
          <SettingOutlined />
          <span>Config</span>
        </Space>
      ),
      children: <ConfigPanel repoId={currentRepo.id} />,
    },
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
      <Space style={{ marginBottom: 24 }} align="start">
        <Button
          icon={<ArrowLeftOutlined />}
          onClick={() => navigate('/')}
        />
        <div>
          <Space>
            <FolderOutlined style={{ fontSize: 24 }} />
            <Typography.Title level={3} style={{ margin: 0 }}>
              {currentRepo.name}
            </Typography.Title>
            <Tag color={statusColor}>{currentRepo.status}</Tag>
          </Space>
          <div style={{ marginTop: 4 }}>
            <Typography.Text
              type="secondary"
              style={{ fontSize: 12 }}
              ellipsis={{ tooltip: currentRepo.path }}
            >
              {currentRepo.path}
            </Typography.Text>
            {currentRepo.last_backup_at && (
              <Typography.Text
                type="secondary"
                style={{ fontSize: 12, marginLeft: 16 }}
              >
                Last backup: {dayjs(currentRepo.last_backup_at).fromNow()}
              </Typography.Text>
            )}
          </div>
        </div>
      </Space>

      <Tabs 
        defaultActiveKey="symlinks" 
        items={tabItems}
        className="repo-detail-tabs"
        style={{ flex: 1, display: 'flex', flexDirection: 'column' }}
      />
    </div>
  );
};

export default RepoDetail;
