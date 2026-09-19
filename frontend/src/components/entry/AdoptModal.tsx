import React, { useEffect, useState } from 'react';
import { Modal, Form, Input, Checkbox, Alert, Space, Button, Typography } from 'antd';
import { FolderOpenOutlined } from '@ant-design/icons';
import { Trans, useTranslation } from 'react-i18next';
import DirectoryPickerModal from '../common/DirectoryPickerModal';
import RepositoryDirectoryPickerModal from '../common/RepositoryDirectoryPickerModal';
import type { AdoptRequest } from '../../types';

interface AdoptModalProps {
  open: boolean;
  repoId: string;
  onClose: () => void;
  onSubmit: (req: AdoptRequest) => Promise<void>;
}

/**
 * 校验仓库内路径：起点固定为仓库的 data/，只能是相对路径且不得出现 ".."。
 * 这是新建备份文件/目录时「不得突破 data/」的前端约束，后端还会再校验一次。
 */
type RepoPathValidationKey =
  | 'adopt.validation.relativePath'
  | 'adopt.validation.insideData';

function pathBaseName(value?: string): string {
  if (!value) return '';
  const windowsPath = /^[A-Za-z]:[\\/]/.test(value) || value.startsWith('\\\\');
  const normalized = windowsPath ? value.replace(/\\/g, '/') : value;
  const withoutTrailingSeparators = normalized.replace(/\/+$/, '');
  return withoutTrailingSeparators.split('/').filter(Boolean).pop() || '';
}

function validateRepoPath(value?: string): RepoPathValidationKey | null {
  if (!value) return null;
  const v = value.replace(/\\/g, '/');
  if (v.startsWith('/')) {
    return 'adopt.validation.relativePath';
  }
  if (v.split('/').some((seg) => seg === '..')) {
    return 'adopt.validation.insideData';
  }
  return null;
}

/**
 * 归一化仓库内路径。输入框左侧已单独显示 "data/"，字段值本身始终是
 * 相对该根目录的路径；不能剥离合法的首级 data 目录。
 */
function normalizeRepoPath(value?: string): string | undefined {
  if (!value) return undefined;
  const v = value.replace(/\\/g, '/').trim().replace(/^\.\//, '');
  return v || undefined;
}

function joinRepoPath(parent: string, name: string): string {
  const normalizedParent = normalizeRepoPath(parent)?.replace(/\/+$/, '') || '';
  return normalizedParent ? `${normalizedParent}/${name}` : name;
}

function repoParentPath(targetPath?: string): string {
  const normalized = normalizeRepoPath(targetPath) || '';
  const index = normalized.lastIndexOf('/');
  return index < 0 ? '' : normalized.slice(0, index);
}

/**
 * 创建条目（adopt）对话框。
 *
 * adopt 会把本机内容「移动」进仓库 data/<repo_path>，并把原位置替换为软链接 ——
 * 该软链接就是条目的第一条链接（所有链接等价，不存在 in/out 之分），因此这里必须明确提示用户。
 */
const AdoptModal: React.FC<AdoptModalProps> = ({ open, repoId, onClose, onSubmit }) => {
  const { t } = useTranslation();
  const [form] = Form.useForm();
  const [localPickerOpen, setLocalPickerOpen] = useState(false);
  const [repoPickerOpen, setRepoPickerOpen] = useState(false);
  const [repoPickerInitialPath, setRepoPickerInitialPath] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      form.resetFields();
      form.setFieldsValue({ follow_symlinks: false });
      setLocalPickerOpen(false);
      setRepoPickerOpen(false);
      setRepoPickerInitialPath('');
    }
  }, [open, form, repoId]);

  const openRepoPicker = async () => {
    try {
      await form.validateFields(['local_path']);
    } catch {
      return;
    }

    const sourceName = pathBaseName(form.getFieldValue('local_path'));
    if (!sourceName) return;
    const currentTarget = normalizeRepoPath(form.getFieldValue('repo_path')) || sourceName;
    setRepoPickerInitialPath(repoParentPath(currentTarget));
    setRepoPickerOpen(true);
  };

  const handleOk = async () => {
    const values = await form.validateFields();
    setSubmitting(true);
    try {
      await onSubmit({
        local_path: values.local_path,
        repo_path: normalizeRepoPath(values.repo_path),
        follow_symlinks: !!values.follow_symlinks,
      });
      onClose();
    } catch {
      // 错误已由上层统一提示
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <Modal
        title={t('adopt.title')}
        open={open}
        onCancel={onClose}
        onOk={handleOk}
        confirmLoading={submitting}
        okText={t('adopt.action.create')}
        cancelText={t('adopt.action.cancel')}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item
            label={t('adopt.field.localPath')}
            name="local_path"
            rules={[{ required: true, message: t('adopt.validation.localPathRequired') }]}
          >
            <Input
              placeholder={t('adopt.placeholder.localPath')}
              addonAfter={
                <Button
                  type="text"
                  size="small"
                  icon={<FolderOpenOutlined />}
                  onClick={() => setLocalPickerOpen(true)}
                />
              }
            />
          </Form.Item>

          <Form.Item
            label={t('adopt.field.repoPath')}
            name="repo_path"
            tooltip={t('adopt.tooltip.repoPath')}
            extra={t('adopt.extra.repoPath')}
            rules={[
              {
                validator: (_, value) => {
                  const errorKey = validateRepoPath(value);
                  return errorKey ? Promise.reject(new Error(t(errorKey))) : Promise.resolve();
                },
              },
            ]}
          >
            <Input
              placeholder={t('adopt.placeholder.repoPath')}
              addonBefore="data/"
              addonAfter={
                <Button
                  type="text"
                  size="small"
                  icon={<FolderOpenOutlined />}
                  title={t('adopt.repoPicker.open')}
                  onClick={() => void openRepoPicker()}
                />
              }
            />
          </Form.Item>

          <Form.Item name="follow_symlinks" valuePropName="checked">
            <Checkbox>{t('adopt.followSymlinks')}</Checkbox>
          </Form.Item>

          <Alert
            type="warning"
            showIcon
            message={t('adopt.warning.title')}
            description={
              <Typography.Text style={{ fontSize: 12 }}>
                <Trans
                  i18nKey="adopt.warning.description"
                  values={{ path: 'data/<repo_path>' }}
                  components={{ code: <Typography.Text code /> }}
                />
              </Typography.Text>
            }
          />
          <Space />
        </Form>

        {/* Nested to inherit the parent Modal's z-index context. */}
        <DirectoryPickerModal
          open={localPickerOpen}
          mode="both"
          title={t('adopt.pickerTitle')}
          onClose={() => setLocalPickerOpen(false)}
          onSelect={(path) => {
            form.setFieldsValue({ local_path: path });
            // 默认 repo_path 取文件名
            const base = pathBaseName(path);
            if (base && !form.getFieldValue('repo_path')) {
              form.setFieldsValue({ repo_path: base });
            }
            void form.validateFields(['repo_path']);
            setLocalPickerOpen(false);
          }}
        />

        <RepositoryDirectoryPickerModal
          open={repoPickerOpen}
          repoId={repoId}
          initialPath={repoPickerInitialPath}
          onClose={() => setRepoPickerOpen(false)}
          onSelect={(parent) => {
            const sourceName = pathBaseName(form.getFieldValue('local_path'));
            if (!sourceName) return;
            form.setFieldsValue({ repo_path: joinRepoPath(parent, sourceName) });
            void form.validateFields(['repo_path']);
            setRepoPickerOpen(false);
          }}
        />
      </Modal>
    </>
  );
};

export default AdoptModal;
