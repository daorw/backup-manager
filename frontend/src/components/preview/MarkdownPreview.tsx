import React, { useState, useEffect } from 'react';
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

const textareaStyle: React.CSSProperties = {
  flex: 1,
  minHeight: 200,
  width: '100%',
  overflow: 'auto',
  resize: 'none',
  fontFamily: "'SF Mono', 'Monaco', 'Inconsolata', 'Fira Code', monospace",
  fontSize: 13,
  lineHeight: 1.5,
  padding: 16,
  border: '1px solid #d9d9d9',
  borderRadius: 6,
};

const previewContainerStyle: React.CSSProperties = {
  padding: 16,
  border: '1px solid #f0f0f0',
  borderRadius: 6,
  flex: 1,
  minHeight: 0,
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

  useEffect(() => {
    if (activeTab !== 'edit') {
      setEditContent(content);
    }
  }, [content, activeTab]);

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

  const markdownElement = content ? (
    <div className="markdown-preview" style={previewContainerStyle}>
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{displayContent}</ReactMarkdown>
    </div>
  ) : (
    <div className="markdown-preview" style={{ ...previewContainerStyle, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <Typography.Text type="secondary">Empty file</Typography.Text>
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
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
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
            style={textareaStyle}
          />
        </div>
      ),
    },
  ];

  return (
    <Tabs
      className="markdown-preview-tabs"
      animated={false}
      activeKey={activeTab}
      onChange={setActiveTab}
      items={tabItems}
      style={{ flex: 1, minHeight: 0 }}
    />
  );
};

export default MarkdownPreview;
