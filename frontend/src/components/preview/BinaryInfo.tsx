import React from 'react';
import { useTranslation } from 'react-i18next';
import { Descriptions, Tag, Typography } from 'antd';
import {
  FileExclamationOutlined,
} from '@ant-design/icons';
import type { PreviewResult } from '../../types';

interface BinaryInfoProps {
  preview: PreviewResult;
  fileName: string;
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const k = 1024;
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + units[i];
}

const BinaryInfo: React.FC<BinaryInfoProps> = ({ preview, fileName }) => {
  const { t } = useTranslation();

  return (
    <div style={{ padding: 24, textAlign: 'center' }}>
      <FileExclamationOutlined
        style={{ fontSize: 48, color: '#faad14', marginBottom: 16 }}
      />
      <Typography.Title level={5} type="secondary">
        {t('preview.binary.title')}
      </Typography.Title>
      <Typography.Paragraph type="secondary">
        <Typography.Text>
          {t('preview.binary.description', { fileName })}
        </Typography.Text>
      </Typography.Paragraph>
      <Descriptions
        column={1}
        bordered
        size="small"
        style={{ maxWidth: 400, margin: '0 auto' }}
      >
        <Descriptions.Item label={t('preview.binary.fileNameLabel')}>
          {fileName}
        </Descriptions.Item>
        <Descriptions.Item label={t('preview.binary.mimeTypeLabel')}>
          <Tag>{preview.mime_type || 'application/octet-stream'}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label={t('preview.binary.sizeLabel')}>
          {formatSize(preview.size)}
        </Descriptions.Item>
        <Descriptions.Item label={t('preview.binary.encodingLabel')}>
          binary
        </Descriptions.Item>
      </Descriptions>
    </div>
  );
};

export default BinaryInfo;
