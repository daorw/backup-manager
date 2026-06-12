import React, { useEffect, useState } from 'react';
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
} from '@ant-design/icons';
import { useAppStore } from '../../store/appStore';
import type { GitAuthType, SetAuthRequest } from '../../types';

interface ConfigPanelProps {
  repoId: string;
}

const ConfigPanel: React.FC<ConfigPanelProps> = ({ repoId }) => {
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
      message.success('Configuration saved');
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
      message.success('Authentication configuration saved');
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
      message.success('Authentication cleared');
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    }
  };

  const handleDeleteRepo = async () => {
    try {
      await deleteRepo(repoId);
      message.success('Repository deleted');
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
            <span>Git Remote Configuration</span>
          </Space>
        }
        style={{ marginBottom: 16 }}
        size="small"
      >
        <Form form={configForm} layout="vertical">
          <Form.Item name="remote_url" label="Remote URL">
            <Input
              placeholder="git@github.com:user/repo.git or https://github.com/user/repo.git"
              prefix={<GithubOutlined />}
            />
          </Form.Item>
          <Form.Item name="branch" label="Branch">
            <Input placeholder="main" />
          </Form.Item>
          <Divider />
          <Form.Item name="git_user_name" label="Git User Name">
            <Input placeholder="Your Name" prefix={<UserOutlined />} />
          </Form.Item>
          <Form.Item name="git_user_email" label="Git User Email">
            <Input placeholder="your@email.com" prefix={<UserOutlined />} />
          </Form.Item>
          <Divider />
          <Form.Item
            name="auto_backup"
            label="Automatic Backup"
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
                  label="Backup Interval (cron expression)"
                  rules={[
                    { required: true, message: 'Please enter a cron expression' },
                  ]}
                >
                  <Input placeholder="0 */6 * * *" prefix={<ClockCircleOutlined />} />
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
              Save Configuration
            </Button>
          </Form.Item>
        </Form>
      </Card>

      <Card
        title={
          <Space>
            <KeyOutlined />
            <span>Git Authentication</span>
          </Space>
        }
        style={{ marginBottom: 16 }}
        size="small"
      >
        <Form form={authForm} layout="vertical">
          <Form.Item name="auth_type" label="Authentication Type">
            <Select
              onChange={(value: GitAuthType) => setAuthType(value)}
              options={[
                { value: 'none', label: 'None' },
                { value: 'ssh_key', label: 'SSH Key' },
                { value: 'password', label: 'Password / Token' },
              ]}
            />
          </Form.Item>
          {authType === 'ssh_key' && (
            <Form.Item
              name="ssh_private_key_path"
              label="SSH Private Key Path"
              rules={[
                { required: true, message: 'Please enter SSH key path' },
              ]}
            >
              <Input placeholder="~/.ssh/id_rsa" />
            </Form.Item>
          )}
          {authType === 'password' && (
            <>
              <Form.Item
                name="username"
                label="Username"
                rules={[
                  { required: true, message: 'Please enter username' },
                ]}
              >
                <Input placeholder="GitHub username" />
              </Form.Item>
              <Form.Item
                name="password"
                label="Password / Token"
                rules={[
                  { required: true, message: 'Please enter password or token' },
                ]}
              >
                <Input.Password placeholder="Personal Access Token" />
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
                Save Authentication
              </Button>
              {currentAuth && currentAuth.auth_type !== 'none' && (
                <Popconfirm
                  title="Clear authentication?"
                  description="This will remove all stored authentication data."
                  onConfirm={handleClearAuth}
                  okText="Clear"
                  cancelText="Cancel"
                >
                  <Button danger icon={<DeleteOutlined />}>
                    Clear
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
            <span>Danger Zone</span>
          </Space>
        }
        size="small"
        styles={{ header: { background: '#fff2f0', borderColor: '#ffccc7' } }}
      >
        <Typography.Paragraph type="danger">
          Deleting the repository will permanently remove all data, including
          symlinks, backup data, and Git history. This action cannot be undone.
        </Typography.Paragraph>
        <Space>
          <Popconfirm
            title="Delete this repository?"
            description="All data will be permanently deleted. This cannot be undone."
            onConfirm={handleDeleteRepo}
            okText="Delete"
            cancelText="Cancel"
            okButtonProps={{ danger: true }}
          >
            <Button danger icon={<DeleteOutlined />}>
              Delete Repository
            </Button>
          </Popconfirm>
          <Button onClick={handleNavigateHome}>Back to Dashboard</Button>
        </Space>
      </Card>
    </div>
  );
};

export default ConfigPanel;
