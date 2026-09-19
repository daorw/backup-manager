import React from 'react';
import { Modal, Typography, Alert, Space, Tag } from 'antd';
import { RollbackOutlined, WarningOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

interface RollbackConfirmModalProps {
  open: boolean;
  commitHash: string;
  commitMessage: string;
  commitDate: string;
  fileCount: number;
  isFullRollback: boolean;
  onCancel: () => void;
  onConfirm: () => void;
  loading: boolean;
}

const RollbackConfirmModal: React.FC<RollbackConfirmModalProps> = ({
  open,
  commitHash,
  commitMessage,
  commitDate,
  fileCount,
  isFullRollback,
  onCancel,
  onConfirm,
  loading,
}) => {
  const { t } = useTranslation();
  const scopeKey = isFullRollback
    ? 'rollback.confirm.scopeFull'
    : 'rollback.confirm.scopeSelected';

  return (
    <Modal
      title={
        <Space>
          <RollbackOutlined />
          <span>
            {isFullRollback
              ? t('rollback.confirm.fullTitle')
              : t('rollback.confirm.title')}
          </span>
        </Space>
      }
      open={open}
      onCancel={onCancel}
      onOk={onConfirm}
      confirmLoading={loading}
      okText={
        isFullRollback
          ? t('rollback.confirm.fullAction')
          : t('rollback.confirm.action')
      }
      okButtonProps={{ danger: true }}
      cancelText={t('rollback.confirm.cancel')}
      width={560}
    >
      <div style={{ marginBottom: 16 }}>
        <Typography.Text strong>{t('rollback.confirm.targetCommit')}</Typography.Text>
        <div style={{ marginTop: 8, padding: '8px 12px', background: '#f5f5f5', borderRadius: 6 }}>
          <Space direction="vertical" size={2}>
            <Space>
              <Tag color="blue">{commitHash.substring(0, 8)}</Tag>
              <Typography.Text>{commitDate}</Typography.Text>
            </Space>
            <Typography.Text code>{commitMessage}</Typography.Text>
          </Space>
        </div>
      </div>

      <div style={{ marginBottom: 16 }}>
        <Typography.Text strong>{t('rollback.confirm.scope')}</Typography.Text>
        <div style={{ marginTop: 4 }}>
          <Typography.Text>{t(scopeKey, { count: fileCount })}</Typography.Text>
        </div>
      </div>

      <Alert
        type="warning"
        icon={<WarningOutlined />}
        showIcon
        message={t('rollback.confirm.warning')}
        description={
          <ul style={{ margin: 0, paddingLeft: 20 }}>
            <li>{t('rollback.confirm.rewrite')}</li>
            <li>{t('rollback.confirm.uncommitted')}</li>
            <li>{t('rollback.confirm.history')}</li>
            <li>{t('rollback.confirm.backupAfter')}</li>
          </ul>
        }
      />
    </Modal>
  );
};

export default RollbackConfirmModal;
