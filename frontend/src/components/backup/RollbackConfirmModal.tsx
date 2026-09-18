import React from 'react';
import { Modal, Typography, Alert, Space, Tag } from 'antd';
import { RollbackOutlined, WarningOutlined } from '@ant-design/icons';

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
              All <strong>{fileCount}</strong> file(s) under <code>data/</code> will be restored to
              this commit's version
            </Typography.Text>
          ) : (
            <Typography.Text>
              <strong>{fileCount}</strong> file(s) under <code>data/</code> will be restored to this
              commit's version
            </Typography.Text>
          )}
        </div>
      </div>

      <Alert
        type="warning"
        icon={<WarningOutlined />}
        showIcon
        message="This operation will OVERWRITE the current content under data/ with the version from the selected commit."
        description={
          <ul style={{ margin: 0, paddingLeft: 20 }}>
            <li>
              Files under <code>data/</code> are rewritten in place. Every link points there, so all
              local paths reflect the change immediately.
            </li>
            <li>
              Uncommitted changes under <code>data/</code> are lost — run a backup first if you may
              need them.
            </li>
            <li>Git history is not modified; the selected commit stays as it is.</li>
            <li>Run a backup afterwards to record the rollback in a new commit.</li>
          </ul>
        }
      />
    </Modal>
  );
};

export default RollbackConfirmModal;
