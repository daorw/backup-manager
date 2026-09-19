import React, { useState } from 'react';
import { Modal, Form, Input, Typography, message, Button } from 'antd';
import { FolderOpenOutlined } from '@ant-design/icons';
import { Trans, useTranslation } from 'react-i18next';
import { useAppStore } from '../../store/appStore';
import DirectoryPickerModal from '../common/DirectoryPickerModal';

interface CreateRepoModalProps {
  open: boolean;
  onClose: () => void;
}

const CreateRepoModal: React.FC<CreateRepoModalProps> = ({ open, onClose }) => {
  const { t } = useTranslation();
  const [form] = Form.useForm();
  const createRepo = useAppStore((s) => s.createRepo);
  const [submitting, setSubmitting] = useState(false);
  const [pickerOpen, setPickerOpen] = useState(false);

  const handleOk = async () => {
    try {
      const values = await form.validateFields();
      setSubmitting(true);
      await createRepo(values.name, values.path);
      form.resetFields();
      message.success(t('repo.create.success'));
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

  const handlePickDirectory = (path: string) => {
    form.setFieldsValue({ path });
  };

  return (
    <>
      <Modal
        title={t('repo.create.title')}
        open={open}
        onOk={handleOk}
        onCancel={handleCancel}
        confirmLoading={submitting}
        okText={t('repo.create.submit')}
        width={520}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{ name: '', path: '' }}
        >
          <Form.Item
            name="name"
            label={t('repo.create.name')}
            rules={[
              { required: true, message: t('repo.create.nameRequired') },
              { min: 1, max: 100, message: t('repo.create.nameLength') },
              {
                pattern: /^[a-zA-Z0-9_\-\s]+$/,
                message: t('repo.create.namePattern'),
              },
            ]}
          >
            <Input placeholder={t('repo.create.namePlaceholder')} />
          </Form.Item>
          <Form.Item
            name="path"
            label={t('repo.create.path')}
            rules={[
              { required: true, message: t('repo.create.pathRequired') },
              {
                pattern: /^\/|^~\/|^\.\.\/|^\.\//,
                message: t('repo.create.pathAbsolute'),
              },
            ]}
          >
            <Input
              placeholder="/home/user/backups/my-repo"
              prefix={<FolderOpenOutlined />}
              suffix={
                <Button
                  type="text"
                  size="small"
                  icon={<FolderOpenOutlined />}
                  onClick={() => setPickerOpen(true)}
                  style={{ padding: '0 4px' }}
                >
                  {t('repo.create.browse')}
                </Button>
              }
            />
          </Form.Item>
          <Form.Item>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              <Trans
                i18nKey="repo.create.description"
                components={{
                  data: <Typography.Text code />,
                  manifest: <Typography.Text code />,
                }}
              />
            </Typography.Text>
          </Form.Item>
        </Form>

        {/* Nested to inherit the parent Modal's z-index context. */}
        <DirectoryPickerModal
          open={pickerOpen}
          onClose={() => setPickerOpen(false)}
          onSelect={handlePickDirectory}
          mode="directory"
          title={t('repo.create.selectDirectory')}
        />
      </Modal>
    </>
  );
};

export default CreateRepoModal;
