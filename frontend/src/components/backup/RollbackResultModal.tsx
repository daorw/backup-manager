import React from 'react';
import { Modal, Typography, Space, Tag, List, Alert } from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
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
  const { t } = useTranslation();

  if (!result) return null;

  const allSuccess = result.failed === 0 && result.total > 0;

  const icon = allSuccess ? (
    <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 48 }} />
  ) : result.success > 0 ? (
    <WarningOutlined style={{ color: '#faad14', fontSize: 48 }} />
  ) : (
    <CloseCircleOutlined style={{ color: '#ff4d4f', fontSize: 48 }} />
  );

  return (
    <Modal
      title={t('rollback.result.title')}
      open={open}
      onCancel={onClose}
      onOk={onClose}
      okText={t('rollback.result.close')}
      cancelText={t('rollback.result.cancel')}
      width={520}
    >
      <div style={{ textAlign: 'center', marginBottom: 20 }}>
        {icon}
        <Typography.Title level={4} style={{ marginTop: 12 }}>
          {allSuccess
            ? t('rollback.result.success')
            : result.success > 0
              ? t('rollback.result.issues')
              : t('rollback.result.failed')}
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
          <Typography.Text type="secondary">
            {t('rollback.result.restoredLabel')}
          </Typography.Text>
        </div>
        <div style={{ textAlign: 'center' }}>
          <Typography.Title level={3} style={{ color: '#ff4d4f', margin: 0 }}>
            {result.failed}
          </Typography.Title>
          <Typography.Text type="secondary">
            {t('rollback.result.failedLabel')}
          </Typography.Text>
        </div>
      </div>

      <Space style={{ marginBottom: 12 }}>
        <Typography.Text type="secondary">{t('rollback.result.commit')}</Typography.Text>
        <Tag color="blue">{result.commit_hash.substring(0, 8)}</Tag>
        <Typography.Text type="secondary">
          {t('rollback.result.total', { count: result.total })}
        </Typography.Text>
      </Space>

      {result.failures && result.failures.length > 0 && (
        <div>
          <Typography.Text strong style={{ color: '#ff4d4f' }}>
            {t('rollback.result.failureDetails')}
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
        message={t('rollback.result.nextSteps')}
        description={t('rollback.result.nextStepsDescription')}
        style={{ marginTop: 16 }}
      />
    </Modal>
  );
};

export default RollbackResultModal;
