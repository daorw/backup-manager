import React from 'react';
import { Modal, Typography, Space, Tag, List, Alert } from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import type { RollbackResult } from '../../types';

interface RollbackResultModalProps {
  open: boolean;
  result: RollbackResult | null;
  onClose: () => void;
}

const RollbackResultModal: React.FC<RollbackResultModalProps> = ({
  open,
  result,
  onClose,
}) => {
  if (!result) return null;

  const allSuccess = result.failed === 0 && result.skipped === 0 && result.total > 0;

  const icon = allSuccess ? (
    <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 48 }} />
  ) : result.success > 0 ? (
    <WarningOutlined style={{ color: '#faad14', fontSize: 48 }} />
  ) : (
    <CloseCircleOutlined style={{ color: '#ff4d4f', fontSize: 48 }} />
  );

  return (
    <Modal
      title="Rollback Result"
      open={open}
      onCancel={onClose}
      onOk={onClose}
      okText="Close"
      width={520}
    >
      <div style={{ textAlign: 'center', marginBottom: 20 }}>
        {icon}
        <Typography.Title level={4} style={{ marginTop: 12 }}>
          {allSuccess
            ? 'Rollback completed successfully'
            : result.success > 0
            ? 'Rollback completed with issues'
            : 'Rollback failed'}
        </Typography.Title>
      </div>

      <div
        style={{
          display: 'flex',
          justifyContent: 'center',
          gap: 24,
          marginBottom: 16,
        }}
      >
        <div style={{ textAlign: 'center' }}>
          <Typography.Title level={3} style={{ color: '#52c41a', margin: 0 }}>
            {result.success}
          </Typography.Title>
          <Typography.Text type="secondary">Restored</Typography.Text>
        </div>
        <div style={{ textAlign: 'center' }}>
          <Typography.Title level={3} style={{ color: '#faad14', margin: 0 }}>
            {result.skipped}
          </Typography.Title>
          <Typography.Text type="secondary">Skipped</Typography.Text>
        </div>
        <div style={{ textAlign: 'center' }}>
          <Typography.Title level={3} style={{ color: '#ff4d4f', margin: 0 }}>
            {result.failed}
          </Typography.Title>
          <Typography.Text type="secondary">Failed</Typography.Text>
        </div>
      </div>

      <Space style={{ marginBottom: 12 }}>
        <Typography.Text type="secondary">Commit:</Typography.Text>
        <Tag color="blue">{result.commit_hash.substring(0, 8)}</Tag>
        <Typography.Text type="secondary">Total: {result.total}</Typography.Text>
      </Space>

      {result.failures && result.failures.length > 0 && (
        <div>
          <Typography.Text strong style={{ color: '#ff4d4f' }}>
            Failure Details:
          </Typography.Text>
          <List
            size="small"
            dataSource={result.failures}
            renderItem={(item) => (
              <List.Item>
                <Space>
                  <Typography.Text code>{item.relative_path}</Typography.Text>
                  <Typography.Text type="danger">{item.error}</Typography.Text>
                </Space>
              </List.Item>
            )}
            style={{ marginTop: 8 }}
          />
        </div>
      )}

      <Alert
        type="info"
        showIcon
        message="Next Steps"
        description="The rollback updated your source files. Run a backup to sync these changes to the data/ directory and create a new commit."
        style={{ marginTop: 16 }}
      />
    </Modal>
  );
};

export default RollbackResultModal;
