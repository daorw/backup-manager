import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import {
  Form,
  Input,
  Switch,
  Button,
  Card,
  Typography,
  Space,
  Select,
  Divider,
  Popconfirm,
  message,
  InputNumber,
  Spin,
} from 'antd';
import {
  GithubOutlined,
  KeyOutlined,
  SafetyOutlined,
  DeleteOutlined,
  SaveOutlined,
  UserOutlined,
  ClockCircleOutlined,
  LinkOutlined,
  FolderOpenOutlined,
} from '@ant-design/icons';
import { useAppStore } from '../../store/appStore';
import type { GitAuthType, SetAuthRequest } from '../../types';
import DirectoryPickerModal from '../common/DirectoryPickerModal';

interface ConfigPanelProps {
  repoId: string;
}

const ConfigPanel: React.FC<ConfigPanelProps> = ({ repoId }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const currentRepo = useAppStore((s) => s.currentRepo);
  const currentAuth = useAppStore((s) => s.currentAuth);
  const loading = useAppStore((s) => s.loading);
  const updateRepoConfig = useAppStore((s) => s.updateRepoConfig);
  const fetchAuth = useAppStore((s) => s.fetchAuth);
  const setAuth = useAppStore((s) => s.setAuth);
  const clearAuth = useAppStore((s) => s.clearAuth);
  const deleteRepo = useAppStore((s) => s.deleteRepo);
  const fetchRepo = useAppStore((s) => s.fetchRepo);

  const [configForm] = Form.useForm();
  const [authForm] = Form.useForm();
  const [savingConfig, setSavingConfig] = useState(false);
  const [savingAuth, setSavingAuth] = useState(false);
  const [authType, setAuthType] = useState<GitAuthType>('none');
  const [sshPickerOpen, setSshPickerOpen] = useState(false);

  useEffect(() => {
    if (repoId) {
      fetchRepo(repoId);
      fetchAuth(repoId);
    }
  }, [repoId, fetchRepo, fetchAuth]);

  useEffect(() => {
    if (currentRepo) {
      configForm.setFieldsValue({
        remote_url: currentRepo.remote_url || '',
        branch: currentRepo.branch || 'main',
        auto_backup: currentRepo.auto_backup || false,
        auto_backup_interval: currentRepo.auto_backup_interval || '',
        git_user_name: currentRepo.git_user_name || '',
        git_user_email: currentRepo.git_user_email || '',
      });
    }
  }, [currentRepo, configForm]);

  useEffect(() => {
    if (currentAuth) {
      setAuthType(currentAuth.auth_type);
      authForm.setFieldsValue({
        auth_type: currentAuth.auth_type,
        ssh_private_key_path: currentAuth.ssh_private_key_path || '',
        username: currentAuth.username || '',
        password: '',
      });
    } else {
      setAuthType('none');
      authForm.resetFields();
    }
  }, [currentAuth, authForm]);

  const handleSaveConfig = async () => {
    try {
      const values = await configForm.validateFields();
      setSavingConfig(true);
      await updateRepoConfig(repoId, {
        remote_url: values.remote_url || undefined,
        branch: values.branch || undefined,
        auto_backup: values.auto_backup,
        auto_backup_interval: values.auto_backup_interval || undefined,
        git_user_name: values.git_user_name || undefined,
        git_user_email: values.git_user_email || undefined,
      });
      message.success(t('config.messages.configurationSaved'));
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setSavingConfig(false);
    }
  };

  const handleSaveAuth = async () => {
    try {
      const values = await authForm.validateFields();
      setSavingAuth(true);
      const authData: SetAuthRequest = {
        auth_type: values.auth_type,
      };
      if (values.auth_type === 'ssh_key') {
        authData.ssh_private_key_path = values.ssh_private_key_path;
      } else if (values.auth_type === 'password') {
        authData.username = values.username;
        authData.password = values.password;
      }
      await setAuth(repoId, authData);
      message.success(t('config.messages.authenticationSaved'));
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setSavingAuth(false);
    }
  };

  const handleClearAuth = async () => {
    try {
      await clearAuth(repoId);
      message.success(t('config.messages.authenticationCleared'));
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    }
  };

  const handleDeleteRepo = async () => {
    try {
      await deleteRepo(repoId);
      message.success(t('config.messages.repositoryDeleted'));
      navigate('/');
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    }
  };

  const handleNavigateHome = () => {
    navigate('/');
  };

  if (!currentRepo) {
    return (
      <div style={{ textAlign: 'center', padding: 48 }}>
        <Spin size="large" />
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 700 }}>
      <Card
        title={
          <Space>
            <LinkOutlined />
            <span>{t('config.remote.title')}</span>
          </Space>
        }
        style={{ marginBottom: 16 }}
        size="small"
      >
        <Form form={configForm} layout="vertical">
          <Form.Item name="remote_url" label={t('config.remote.urlLabel')}>
            <Input
              placeholder={t('config.remote.urlPlaceholder')}
              prefix={<GithubOutlined />}
            />
          </Form.Item>
          <Form.Item name="branch" label={t('config.remote.branchLabel')}>
            <Input placeholder={t('config.remote.branchPlaceholder')} />
          </Form.Item>
          <Divider />
          <Form.Item name="git_user_name" label={t('config.remote.gitUserNameLabel')}>
            <Input
              placeholder={t('config.remote.gitUserNamePlaceholder')}
              prefix={<UserOutlined />}
            />
          </Form.Item>
          <Form.Item name="git_user_email" label={t('config.remote.gitUserEmailLabel')}>
            <Input
              placeholder={t('config.remote.gitUserEmailPlaceholder')}
              prefix={<UserOutlined />}
            />
          </Form.Item>
          <Divider />
          <Form.Item
            name="auto_backup"
            label={t('config.remote.automaticBackupLabel')}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            noStyle
            shouldUpdate={(prev, curr) => prev.auto_backup !== curr.auto_backup}
          >
            {({ getFieldValue }) =>
              getFieldValue('auto_backup') ? (
                <Form.Item
                  name="auto_backup_interval"
                  label={t('config.remote.backupIntervalLabel')}
                  rules={[
                    {
                      required: true,
                      message: t('config.remote.backupIntervalRequired'),
                    },
                  ]}
                >
                  <Input
                    placeholder={t('config.remote.cronPlaceholder')}
                    prefix={<ClockCircleOutlined />}
                  />
                </Form.Item>
              ) : null
            }
          </Form.Item>
          <Form.Item>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              onClick={handleSaveConfig}
              loading={savingConfig}
            >
              {t('config.remote.saveButton')}
            </Button>
          </Form.Item>
        </Form>
      </Card>

      <Card
        title={
          <Space>
            <KeyOutlined />
            <span>{t('config.auth.title')}</span>
          </Space>
        }
        style={{ marginBottom: 16 }}
        size="small"
      >
        <Form form={authForm} layout="vertical">
          <Form.Item name="auth_type" label={t('config.auth.typeLabel')}>
            <Select
              onChange={(value: GitAuthType) => setAuthType(value)}
              options={[
                { value: 'none', label: t('config.auth.typeNone') },
                { value: 'ssh_key', label: t('config.auth.typeSshKey') },
                { value: 'password', label: t('config.auth.typePassword') },
              ]}
            />
          </Form.Item>
          {authType === 'ssh_key' && (
            <Form.Item
              name="ssh_private_key_path"
              label={t('config.auth.sshPrivateKeyPathLabel')}
              rules={[
                {
                  required: true,
                  message: t('config.auth.sshPrivateKeyPathRequired'),
                },
              ]}
            >
              <Input
                placeholder={t('config.auth.sshPrivateKeyPathPlaceholder')}
                suffix={
                  <Button
                    type="text"
                    size="small"
                    icon={<FolderOpenOutlined />}
                    onClick={() => setSshPickerOpen(true)}
                    style={{ padding: '0 4px' }}
                  >
                    {t('config.auth.browseButton')}
                  </Button>
                }
              />
            </Form.Item>
          )}
          {authType === 'password' && (
            <>
              <Form.Item
                name="username"
                label={t('config.auth.usernameLabel')}
                rules={[
                  { required: true, message: t('config.auth.usernameRequired') },
                ]}
              >
                <Input placeholder={t('config.auth.usernamePlaceholder')} />
              </Form.Item>
              <Form.Item
                name="password"
                label={t('config.auth.passwordLabel')}
                rules={[
                  { required: true, message: t('config.auth.passwordRequired') },
                ]}
              >
                <Input.Password placeholder={t('config.auth.passwordPlaceholder')} />
              </Form.Item>
            </>
          )}
          <Form.Item>
            <Space>
              <Button
                type="primary"
                icon={<SaveOutlined />}
                onClick={handleSaveAuth}
                loading={savingAuth}
              >
                {t('config.auth.saveButton')}
              </Button>
              {currentAuth && currentAuth.auth_type !== 'none' && (
                <Popconfirm
                  title={t('config.auth.clearConfirmTitle')}
                  description={t('config.auth.clearConfirmDescription')}
                  onConfirm={handleClearAuth}
                  okText={t('config.auth.clearButton')}
                  cancelText={t('config.auth.cancelButton')}
                >
                  <Button danger icon={<DeleteOutlined />}>
                    {t('config.auth.clearButton')}
                  </Button>
                </Popconfirm>
              )}
            </Space>
          </Form.Item>
        </Form>
      </Card>

      <Card
        title={
          <Space>
            <SafetyOutlined />
            <span>{t('config.danger.title')}</span>
          </Space>
        }
        size="small"
        styles={{ header: { background: '#fff2f0', borderColor: '#ffccc7' } }}
      >
        <Typography.Paragraph type="danger">
          {t('config.danger.descriptionBeforeData')}
          <Typography.Text code>data/</Typography.Text>
          {t('config.danger.descriptionBetweenPaths')}
          <Typography.Text code>.backup-manager/manifest.json</Typography.Text>
          {t('config.danger.descriptionAfterManifest')}
        </Typography.Paragraph>
        <Space>
          <Popconfirm
            title={t('config.danger.deleteConfirmTitle')}
            description={t('config.danger.deleteConfirmDescription')}
            onConfirm={handleDeleteRepo}
            okText={t('config.danger.deleteButton')}
            cancelText={t('config.danger.cancelButton')}
            okButtonProps={{ danger: true }}
          >
            <Button danger icon={<DeleteOutlined />}>
              {t('config.danger.deleteRepositoryButton')}
            </Button>
          </Popconfirm>
          <Button onClick={handleNavigateHome}>
            {t('config.danger.backToDashboardButton')}
          </Button>
        </Space>
      </Card>
      <DirectoryPickerModal
        open={sshPickerOpen}
        onClose={() => setSshPickerOpen(false)}
        onSelect={(path) => {
          authForm.setFieldsValue({ ssh_private_key_path: path });
        }}
        mode="file"
        title={t('config.auth.pickerTitle')}
        initialPath={authForm.getFieldValue('ssh_private_key_path') || '~/.ssh'}
      />
    </div>
  );
};

export default ConfigPanel;
