import React from 'react';
import { Card, Tag, Typography, Space, Row, Col } from 'antd';
import { FolderOutlined, FileOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import type { BackupRepo, Symlink } from '../../types';

interface RepoOverviewCardProps {
  repo: BackupRepo;
  symlinks: Symlink[];
}

const RepoOverviewCard: React.FC<RepoOverviewCardProps> = ({ repo, symlinks }) => {
  const fileCount = symlinks.filter((s) => s.type === 'file').length;
  const dirCount = symlinks.filter((s) => s.type === 'directory').length;

  const statusColor =
    repo.status === 'active' ? 'green'
      : repo.status === 'error' ? 'red'
        : 'blue';

  const statItemStyle: React.CSSProperties = { textAlign: 'center' };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        flex: 1,
        padding: 24,
      }}
    >
      <Card style={{ maxWidth: 480, width: '100%' }}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div style={{ textAlign: 'center' }}>
            <FolderOutlined
              style={{ fontSize: 36, color: '#1677ff', marginBottom: 12 }}
            />
            <Typography.Title level={4} style={{ marginBottom: 4 }}>
              {repo.name}
            </Typography.Title>
            <Tag color={statusColor}>{repo.status}</Tag>
          </div>

          <div style={{ textAlign: 'center' }}>
            <Typography.Text
              type="secondary"
              ellipsis={{ tooltip: repo.path }}
              style={{ fontSize: 12 }}
            >
              {repo.path}
            </Typography.Text>
          </div>

          <Row justify="center" gutter={48}>
            <Col style={statItemStyle}>
              <div style={{ fontSize: 24, fontWeight: 600, color: '#1677ff' }}>
                <FileOutlined style={{ marginRight: 6, fontSize: 18 }} />
                {fileCount}
              </div>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                Files
              </Typography.Text>
            </Col>
            <Col style={statItemStyle}>
              <div style={{ fontSize: 24, fontWeight: 600, color: '#1677ff' }}>
                <FolderOutlined style={{ marginRight: 6, fontSize: 18 }} />
                {dirCount}
              </div>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                Directories
              </Typography.Text>
            </Col>
          </Row>

          {repo.last_backup_at && (
            <div style={{ textAlign: 'center' }}>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                Last backup: {dayjs(repo.last_backup_at).fromNow()}
              </Typography.Text>
            </div>
          )}

          <Typography.Text
            type="secondary"
            style={{ textAlign: 'center', display: 'block', fontSize: 12 }}
          >
            Select a file from the tree to preview its contents
          </Typography.Text>
        </Space>
      </Card>
    </div>
  );
};

export default RepoOverviewCard;
