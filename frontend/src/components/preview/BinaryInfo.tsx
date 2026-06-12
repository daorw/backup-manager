import React from 'react';
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
  return (
    <div style={{ padding: 24, textAlign: 'center' }}>
      <FileExclamationOutlined
        style={{ fontSize: 48, color: '#faad14', marginBottom: 16 }}
      />
      <Typography.Title level={5} type="secondary">
        Binary File
      </Typography.Title>
      <Typography.Paragraph type="secondary">
        <Typography.Text>
          {fileName} is a binary file and cannot be previewed as text.
        </Typography.Text>
      </Typography.Paragraph>
      <Descriptions
        column={1}
        bordered
        size="small"
        style={{ maxWidth: 400, margin: '0 auto' }}
      >
        <Descriptions.Item label="File Name">{fileName}</Descriptions.Item>
        <Descriptions.Item label="MIME Type">
          <Tag>{preview.mime_type || 'application/octet-stream'}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label="Size">
          {formatSize(preview.size)}
        </Descriptions.Item>
        <Descriptions.Item label="Encoding">
          binary
        </Descriptions.Item>
      </Descriptions>
    </div>
  );
};

export default BinaryInfo;
