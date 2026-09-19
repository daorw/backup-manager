import React, { useEffect, useState } from 'react';
import { Row, Col, Typography, Button, Space } from 'antd';
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useAppStore } from '../store/appStore';
import RepoCard from '../components/repo/RepoCard';
import CreateRepoModal from '../components/repo/CreateRepoModal';

const Dashboard: React.FC = () => {
  const { t } = useTranslation();
  const repos = useAppStore((s) => s.repos);
  const fetchRepos = useAppStore((s) => s.fetchRepos);
  const [createModalOpen, setCreateModalOpen] = useState(false);

  useEffect(() => {
    fetchRepos();
  }, [fetchRepos]);

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
            {t('dashboard.title')}
          </Typography.Title>
          <Typography.Text type="secondary">
            {t('dashboard.subtitle')}
          </Typography.Text>
        </div>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => fetchRepos()}>
            {t('dashboard.refresh')}
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateModalOpen(true)}
          >
            {t('dashboard.create')}
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
            {t('dashboard.emptyTitle')}
          </Typography.Title>
          <Typography.Paragraph type="secondary">
            {t('dashboard.emptyDescription')}
          </Typography.Paragraph>
          <Button
            type="primary"
            size="large"
            icon={<PlusOutlined />}
            onClick={() => setCreateModalOpen(true)}
          >
            {t('dashboard.create')}
          </Button>
        </div>
      ) : (
        <Row gutter={[16, 16]}>
          {repos.map((repo) => (
            <Col key={repo.id} xs={24} sm={12} lg={8} xl={6}>
              <RepoCard repo={repo} />
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
