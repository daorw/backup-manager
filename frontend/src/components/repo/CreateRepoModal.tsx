import React, { useState } from 'react';
import { Modal, Form, Input, Typography, message } from 'antd';
import { FolderOpenOutlined } from '@ant-design/icons';
import { useAppStore } from '../../store/appStore';

interface CreateRepoModalProps {
  open: boolean;
  onClose: () => void;
}

const CreateRepoModal: React.FC<CreateRepoModalProps> = ({ open, onClose }) => {
  const [form] = Form.useForm();
  const createRepo = useAppStore((s) => s.createRepo);
  const [submitting, setSubmitting] = useState(false);

  const handleOk = async () => {
    try {
      const values = await form.validateFields();
      setSubmitting(true);
      await createRepo(values.name, values.path);
      form.resetFields();
      message.success('Repository created successfully');
      onClose();
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleCancel = () => {
    form.resetFields();
    onClose();
  };

  return (
    <Modal
      title="Create Repository"
      open={open}
      onOk={handleOk}
      onCancel={handleCancel}
      confirmLoading={submitting}
      okText="Create"
      width={520}
    >
      <Form
        form={form}
        layout="vertical"
        initialValues={{ name: '', path: '' }}
      >
        <Form.Item
          name="name"
          label="Repository Name"
          rules={[
            { required: true, message: 'Please enter a repository name' },
            { min: 1, max: 100, message: 'Name must be 1-100 characters' },
            {
              pattern: /^[a-zA-Z0-9_\-\s]+$/,
              message:
                'Name can only contain letters, numbers, spaces, underscores and hyphens',
            },
          ]}
        >
          <Input placeholder="My Backups" />
        </Form.Item>
        <Form.Item
          name="path"
          label="Repository Path"
          rules={[
            { required: true, message: 'Please enter a repository path' },
            {
              pattern: /^\/|^~\/|^\.\.\/|^\.\//,
              message: 'Please enter an absolute path starting with /',
            },
          ]}
        >
          <Input
            placeholder="/home/user/backups/my-repo"
            prefix={<FolderOpenOutlined />}
          />
        </Form.Item>
        <Form.Item>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            The repository will be created at the specified path. A{' '}
            <Typography.Text code>.links/</Typography.Text> directory and Git
            repository will be initialized automatically.
          </Typography.Text>
        </Form.Item>
      </Form>
    </Modal>
  );
};

export default CreateRepoModal;
