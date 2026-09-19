import React, { useEffect, useState } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { ConfigProvider, Spin, theme } from 'antd';
import enUS from 'antd/locale/en_US';
import zhCN from 'antd/locale/zh_CN';
import { useTranslation } from 'react-i18next';
import AppLayout from './components/layout/AppLayout';
import Dashboard from './routes/Dashboard';
import RepoDetail from './routes/RepoDetail';
import { fetchSettings } from './api/client';
import { isAppLanguage, setAppLanguage } from './i18n';
import './App.css';

const App: React.FC = () => {
  const { i18n } = useTranslation();
  const [settingsReady, setSettingsReady] = useState(false);

  useEffect(() => {
    let cancelled = false;

    void fetchSettings()
      .then(async ({ language }) => {
        if (isAppLanguage(language)) {
          await setAppLanguage(language);
        }
      })
      .catch(async () => {
        await setAppLanguage('en');
      })
      .finally(() => {
        if (!cancelled) {
          setSettingsReady(true);
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  const language = isAppLanguage(i18n.resolvedLanguage || '') ? i18n.resolvedLanguage : 'en';

  return (
    <ConfigProvider
      locale={language === 'zh-CN' ? zhCN : enUS}
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          colorPrimary: '#1677ff',
          borderRadius: 6,
        },
      }}
    >
      {settingsReady ? (
        <BrowserRouter>
          <Routes>
            <Route element={<AppLayout />}>
              <Route path="/" element={<Dashboard />} />
              <Route path="/repos/:id" element={<RepoDetail />} />
            </Route>
          </Routes>
        </BrowserRouter>
      ) : (
        <div
          style={{
            minHeight: '100vh',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Spin size="large" />
        </div>
      )}
    </ConfigProvider>
  );
};

export default App;
