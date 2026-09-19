import React, { useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Layout, Menu, Segmented, Space, Tooltip, Typography, message } from 'antd';
import {
  DashboardOutlined,
  DatabaseOutlined,
  GlobalOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useAppStore } from '../../store/appStore';
import { updateSettings } from '../../api/client';
import { isAppLanguage, setAppLanguage } from '../../i18n';
import type { AppLanguage } from '../../types';

const { Sider } = Layout;

const Sidebar: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { t, i18n } = useTranslation();
  const repos = useAppStore((s) => s.repos);
  const [savingLanguage, setSavingLanguage] = useState(false);
  const resolvedLanguage = i18n.resolvedLanguage || '';
  const language: AppLanguage = isAppLanguage(resolvedLanguage) ? resolvedLanguage : 'en';

  const handleLanguageChange = async (nextLanguage: AppLanguage) => {
    if (nextLanguage === language) return;

    const previousLanguage = language;
    setSavingLanguage(true);
    try {
      await setAppLanguage(nextLanguage);
      await updateSettings({ language: nextLanguage });
    } catch {
      await setAppLanguage(previousLanguage);
      message.error(t('language.saveFailed'));
    } finally {
      setSavingLanguage(false);
    }
  };

  const menuItems = [
    {
      key: '/',
      icon: <DashboardOutlined />,
      label: t('nav.dashboard'),
    },
    ...(repos.length > 0
      ? [
          {
            key: 'repos-group',
            type: 'group' as const,
            label: t('nav.repositories'),
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
      className="app-sidebar"
      width={240}
      theme="dark"
      style={{
        height: '100vh',
        position: 'fixed',
        left: 0,
        top: 0,
        bottom: 0,
        overflow: 'hidden',
      }}
    >
      <div
        style={{
          height: 64,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderBottom: '1px solid rgba(255,255,255,0.1)',
          flexShrink: 0,
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
        style={{ borderRight: 0, flex: 1, overflowY: 'auto' }}
      />
      <div
        style={{
          padding: 12,
          borderTop: '1px solid rgba(255,255,255,0.1)',
          flexShrink: 0,
        }}
      >
        <Tooltip title={t('language.label')} placement="right">
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            <Space size={6}>
              <GlobalOutlined style={{ color: 'rgba(255,255,255,0.72)' }} />
              <Typography.Text style={{ color: 'rgba(255,255,255,0.72)', fontSize: 12 }}>
                {t('language.label')}
              </Typography.Text>
            </Space>
            <Segmented
              block
              size="small"
              value={language}
              disabled={savingLanguage}
              options={[
                { label: t('language.english'), value: 'en' },
                { label: t('language.chinese'), value: 'zh-CN' },
              ]}
              onChange={(value) => handleLanguageChange(value as AppLanguage)}
            />
          </Space>
        </Tooltip>
      </div>
    </Sider>
  );
};

export default Sidebar;
