import React from 'react';
import { Outlet } from 'react-router-dom';
import { Layout, Spin, Alert } from 'antd';
import Sidebar from './Sidebar';
import { useAppStore } from '../../store/appStore';

const { Content } = Layout;

const AppLayout: React.FC = () => {
  const loading = useAppStore((s) => s.loading);
  const error = useAppStore((s) => s.error);
  const clearError = useAppStore((s) => s.clearError);

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sidebar />
      <Layout style={{ marginLeft: 240 }}>
        <Content style={{ margin: 24, minHeight: 280 }}>
          {error && (
            <Alert
              message={error}
              type="error"
              closable
              onClose={clearError}
              style={{ marginBottom: 16 }}
            />
          )}
          <Spin spinning={loading} size="large">
            <Outlet />
          </Spin>
        </Content>
      </Layout>
    </Layout>
  );
};

export default AppLayout;
