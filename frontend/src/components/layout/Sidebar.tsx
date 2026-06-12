import React from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Layout, Menu, Typography } from 'antd';
import {
  DashboardOutlined,
  DatabaseOutlined,
} from '@ant-design/icons';
import { useAppStore } from '../../store/appStore';

const { Sider } = Layout;

const Sidebar: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const repos = useAppStore((s) => s.repos);

  const menuItems = [
    {
      key: '/',
      icon: <DashboardOutlined />,
      label: 'Dashboard',
    },
    ...(repos.length > 0
      ? [
          {
            key: 'repos-group',
            type: 'group' as const,
            label: 'Repositories',
            children: repos.map((repo) => ({
              key: `/repos/${repo.id}`,
              icon: <DatabaseOutlined />,
              label: repo.name,
            })),
          },
        ]
      : []),
  ];

  const selectedKey = location.pathname;

  return (
    <Sider
      width={240}
      theme="dark"
      style={{
        height: '100vh',
        position: 'fixed',
        left: 0,
        top: 0,
        bottom: 0,
        overflow: 'auto',
      }}
    >
      <div
        style={{
          height: 64,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderBottom: '1px solid rgba(255,255,255,0.1)',
        }}
      >
        <Typography.Text
          strong
          style={{ color: '#fff', fontSize: 18, whiteSpace: 'nowrap' }}
        >
          Backup Manager
        </Typography.Text>
      </div>
      <Menu
        theme="dark"
        mode="inline"
        selectedKeys={[selectedKey]}
        items={menuItems}
        onClick={({ key }) => navigate(key)}
        style={{ borderRight: 0 }}
      />
    </Sider>
  );
};

export default Sidebar;
