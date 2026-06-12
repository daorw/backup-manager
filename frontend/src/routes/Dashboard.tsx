import React, { useEffect, useState } from 'react';
import { Row, Col, Typography, Button, Space } from 'antd';
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useAppStore } from '../store/appStore';
import RepoCard from '../components/repo/RepoCard';
import CreateRepoModal from '../components/repo/CreateRepoModal';

const Dashboard: React.FC = () => {
  const repos = useAppStore((s) => s.repos);
  const fetchRepos = useAppStore((s) => s.fetchRepos);
  const deleteRepo = useAppStore((s) => s.deleteRepo);
  const [createModalOpen, setCreateModalOpen] = useState(false);

  useEffect(() => {
    fetchRepos();
  }, [fetchRepos]);

  const handleDelete = async (id: string) => {
    try {
      await deleteRepo(id);
    } catch {
      // Error is handled by the store
    }
  };

  return (
    <div>
      <Space
        style={{
          marginBottom: 24,
          justifyContent: 'space-between',
          width: '100%',
        }}
      >
        <div>
          <Typography.Title level={3} style={{ margin: 0 }}>
            Repositories
          </Typography.Title>
          <Typography.Text type="secondary">
            Manage your backup repositories
          </Typography.Text>
        </div>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => fetchRepos()}>
            Refresh
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateModalOpen(true)}
          >
            Create Repository
          </Button>
        </Space>
      </Space>

      {repos.length === 0 ? (
        <div
          style={{
            textAlign: 'center',
            padding: 80,
            border: '2px dashed #d9d9d9',
            borderRadius: 8,
          }}
        >
          <Typography.Title level={4} type="secondary">
            No repositories yet
          </Typography.Title>
          <Typography.Paragraph type="secondary">
            Create your first backup repository to get started.
          </Typography.Paragraph>
          <Button
            type="primary"
            size="large"
            icon={<PlusOutlined />}
            onClick={() => setCreateModalOpen(true)}
          >
            Create Repository
          </Button>
        </div>
      ) : (
        <Row gutter={[16, 16]}>
          {repos.map((repo) => (
            <Col key={repo.id} xs={24} sm={12} lg={8} xl={6}>
              <RepoCard repo={repo} onDelete={handleDelete} />
            </Col>
          ))}
        </Row>
      )}

      <CreateRepoModal
        open={createModalOpen}
        onClose={() => setCreateModalOpen(false)}
      />
    </div>
  );
};

export default Dashboard;
