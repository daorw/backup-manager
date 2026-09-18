import React, { useEffect, useState } from 'react';
import { Modal, Form, Input, Checkbox, Alert, Space, Button, Typography } from 'antd';
import { FolderOpenOutlined } from '@ant-design/icons';
import DirectoryPickerModal from '../common/DirectoryPickerModal';
import type { AdoptRequest } from '../../types';

interface AdoptModalProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (req: AdoptRequest) => Promise<void>;
}

/**
 * 校验仓库内路径：起点固定为仓库的 data/，只能是相对路径且不得出现 ".."。
 * 这是新建备份文件/目录时「不得突破 data/」的前端约束，后端还会再校验一次。
 */
function validateRepoPath(value?: string): string | null {
  if (!value) return null;
  const v = value.replace(/\\/g, '/');
  if (v.startsWith('/')) {
    return 'Must be a path relative to data/ (no leading "/")';
  }
  if (v.split('/').some((seg) => seg === '..')) {
    return 'Must stay inside data/ (".." is not allowed)';
  }
  return null;
}

/**
 * 归一化仓库内路径：输入框左侧恒显示 "data/"，用户若又手输了 data/ 或 ./ 前缀，
 * 去掉它，避免内容被写进 data/data/。
 */
function normalizeRepoPath(value?: string): string | undefined {
  if (!value) return undefined;
  let v = value.replace(/\\/g, '/').trim();
  v = v.replace(/^\.\//, '');
  v = v.replace(/^data\//, '');
  return v || undefined;
}

/**
 * 创建条目（adopt）对话框。
 *
 * adopt 会把本机内容「移动」进仓库 data/<repo_path>，并把原位置替换为软链接 ——
 * 该软链接即条目的 in 链接，因此这里必须明确提示用户。
 */
const AdoptModal: React.FC<AdoptModalProps> = ({ open, onClose, onSubmit }) => {
  const [form] = Form.useForm();
  const [pickerOpen, setPickerOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      form.resetFields();
      form.setFieldsValue({ follow_symlinks: false });
    }
  }, [open, form]);

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
        title="New Entry"
        open={open}
        onCancel={onClose}
        onOk={handleOk}
        confirmLoading={submitting}
        okText="Create"
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item
            label="Local Path"
            name="local_path"
            rules={[{ required: true, message: 'Please pick the file or directory to back up' }]}
          >
            <Input
              placeholder="e.g. ~/.config/opencode/opencode.json"
              addonAfter={
                <Button
                  type="text"
                  size="small"
                  icon={<FolderOpenOutlined />}
                  onClick={() => setPickerOpen(true)}
                />
              }
            />
          </Form.Item>

          <Form.Item
            label="Repo Path"
            name="repo_path"
            tooltip="内容在仓库 data/ 下的路径。留空则取文件名。不得与其它条目重叠。"
            extra="起点固定为当前仓库的 data/：只能填相对路径，不能出现 ..，也无法跳出该目录。"
            rules={[
              {
                validator: (_, value) => {
                  const err = validateRepoPath(value);
                  return err ? Promise.reject(new Error(err)) : Promise.resolve();
                },
              },
            ]}
          >
            <Input placeholder="e.g. opencode/opencode.json" addonBefore="data/" />
          </Form.Item>

          <Form.Item name="follow_symlinks" valuePropName="checked">
            <Checkbox>Follow symlinks found inside the directory</Checkbox>
          </Form.Item>

          <Alert
            type="warning"
            showIcon
            message="The content will be moved into the repository"
            description={
              <Typography.Text style={{ fontSize: 12 }}>
                The original location will be replaced by a symlink pointing at
                <Typography.Text code>data/&lt;repo_path&gt;</Typography.Text>. Nothing is copied, so
                the repository becomes the single owner of the content.
              </Typography.Text>
            }
          />
          <Space />
        </Form>
      </Modal>

      <DirectoryPickerModal
        open={pickerOpen}
        mode="both"
        title="Select the file or directory to back up"
        onClose={() => setPickerOpen(false)}
        onSelect={(path) => {
          form.setFieldsValue({ local_path: path });
          // 默认 repo_path 取文件名
          const base = path.split('/').filter(Boolean).pop();
          if (base && !form.getFieldValue('repo_path')) {
            form.setFieldsValue({ repo_path: base });
          }
          setPickerOpen(false);
        }}
      />
    </>
  );
};

export default AdoptModal;
