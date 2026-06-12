import React from 'react';
import { Modal, Typography, Alert, Space, Tag } from 'antd';
import { RollbackOutlined, WarningOutlined } from '@ant-design/icons';

interface RollbackConfirmModalProps {
  open: boolean;
  commitHash: string;
  commitMessage: string;
  commitDate: string;
  symlinkCount: number;
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
  symlinkCount,
  isFullRollback,
  onCancel,
  onConfirm,
  loading,
}) => {
  return (
    <Modal
      title={
        <Space>
          <RollbackOutlined />
          <span>{isFullRollback ? 'Full Rollback Confirmation' : 'Rollback Confirmation'}</span>
        </Space>
      }
      open={open}
      onCancel={onCancel}
      onOk={onConfirm}
      confirmLoading={loading}
      okText={isFullRollback ? 'Full Rollback' : 'Rollback'}
      okButtonProps={{ danger: true }}
      cancelText="Cancel"
      width={560}
    >
      <div style={{ marginBottom: 16 }}>
        <Typography.Text strong>Target Commit:</Typography.Text>
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
        <Typography.Text strong>Scope:</Typography.Text>
        <div style={{ marginTop: 4 }}>
          {isFullRollback ? (
            <Typography.Text>
              All <strong>{symlinkCount}</strong> changed file(s) / symlink(s) will be rolled back
            </Typography.Text>
          ) : (
            <Typography.Text>
              <strong>{symlinkCount}</strong> symlink(s) will be rolled back
            </Typography.Text>
          )}
        </div>
      </div>

      <Alert
        type="warning"
        icon={<WarningOutlined />}
        showIcon
        message="This operation will OVERWRITE your source files with the version from the selected commit."
        description={
          <ul style={{ margin: 0, paddingLeft: 20 }}>
            <li>Source files at their original locations will be overwritten.</li>
            <li>This action cannot be undone. Consider backing up important files first.</li>
            <li>The <code>data/</code> backup directory will NOT be modified.</li>
            <li>Run a backup after rollback to sync changes.</li>
          </ul>
        }
      />
    </Modal>
  );
};

export default RollbackConfirmModal;
