import React, { useState } from 'react';
import { Typography, Button, Space, Tabs } from 'antd';
import { SaveOutlined, CloseOutlined } from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

interface MarkdownPreviewProps {
  content: string;
  repoId: string;
  filePath: string;
  editable?: boolean;
  onSave?: (content: string) => Promise<void>;
  saving?: boolean;
}

const previewContainerStyle: React.CSSProperties = {
  padding: 16,
  border: '1px solid #f0f0f0',
  borderRadius: 6,
  maxHeight: 600,
  overflow: 'auto',
};

const MarkdownPreview: React.FC<MarkdownPreviewProps> = ({
  content,
  repoId,
  filePath,
  editable = false,
  onSave,
  saving = false,
}) => {
  const [activeTab, setActiveTab] = useState<string>('preview');
  const [editContent, setEditContent] = useState(content);

  const displayContent = editable && activeTab === 'edit' ? editContent : content;

  const handleSave = async () => {
    if (!onSave) return;
    await onSave(editContent);
    setActiveTab('preview');
  };

  const handleCancel = () => {
    setEditContent(content);
    setActiveTab('preview');
  };

  if (!content) {
    return <Typography.Text type="secondary">Empty file</Typography.Text>;
  }

  const markdownElement = (
    <div className="markdown-preview" style={previewContainerStyle}>
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{displayContent}</ReactMarkdown>
    </div>
  );

  if (!editable) {
    return markdownElement;
  }

  const tabItems = [
    {
      key: 'preview',
      label: 'Preview',
      children: markdownElement,
    },
    {
      key: 'edit',
      label: 'Edit',
      children: (
        <div>
          <Space style={{ marginBottom: 8 }}>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              onClick={handleSave}
              loading={saving}
            >
              Save
            </Button>
            <Button
              icon={<CloseOutlined />}
              onClick={handleCancel}
              disabled={saving}
            >
              Cancel
            </Button>
          </Space>
          <textarea
            value={editContent}
            onChange={(e) => setEditContent(e.target.value)}
            disabled={saving}
            style={{
              width: '100%',
              minHeight: 400,
              maxHeight: 600,
              fontFamily: "'SF Mono', 'Monaco', 'Inconsolata', 'Fira Code', monospace",
              fontSize: 13,
              lineHeight: 1.5,
              padding: 16,
              border: '1px solid #d9d9d9',
              borderRadius: 6,
              resize: 'vertical',
            }}
          />
        </div>
      ),
    },
  ];

  return (
    <Tabs
      activeKey={activeTab}
      onChange={setActiveTab}
      items={tabItems}
    />
  );
};

export default MarkdownPreview;
