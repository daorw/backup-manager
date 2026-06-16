import React, { useState } from 'react';
import { Typography, Button, Space } from 'antd';
import { EditOutlined, SaveOutlined, CloseOutlined } from '@ant-design/icons';

interface TextPreviewProps {
  content: string;
  fileName: string;
  truncated: boolean;
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
  whiteSpace: 'pre-wrap',
  wordBreak: 'break-all',
};

const LANGUAGE_MAP: Record<string, string> = {
  js: 'javascript',
  jsx: 'javascript',
  ts: 'typescript',
  tsx: 'typescript',
  py: 'python',
  rb: 'ruby',
  java: 'java',
  go: 'go',
  rs: 'rust',
  cpp: 'cpp',
  c: 'c',
  h: 'c',
  hpp: 'cpp',
  cs: 'csharp',
  php: 'php',
  swift: 'swift',
  kt: 'kotlin',
  scala: 'scala',
  html: 'html',
  css: 'css',
  scss: 'scss',
  less: 'less',
  json: 'json',
  xml: 'xml',
  yaml: 'yaml',
  yml: 'yaml',
  toml: 'toml',
  md: 'markdown',
  sql: 'sql',
  sh: 'bash',
  bash: 'bash',
  zsh: 'bash',
  dockerfile: 'dockerfile',
  conf: 'conf',
  ini: 'ini',
  cfg: 'ini',
  env: 'dotenv',
  txt: 'text',
  log: 'text',
};

function getLanguage(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase() || '';
  const name = filename.toLowerCase();
  if (name === 'dockerfile') return 'dockerfile';
  if (name === 'makefile') return 'makefile';
  return LANGUAGE_MAP[ext] || 'text';
}

const TextPreview: React.FC<TextPreviewProps> = ({
  content,
  fileName,
  truncated,
  editable = false,
  onSave,
  saving = false,
}) => {
  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState(content);

  const language = getLanguage(fileName);

  const handleStartEdit = () => {
    setEditContent(content);
    setIsEditing(true);
  };

  const handleCancel = () => {
    setEditContent(content);
    setIsEditing(false);
  };

  const handleSave = async () => {
    if (!onSave) return;
    await onSave(editContent);
    setIsEditing(false);
  };

  if (isEditing) {
    return (
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
        {truncated && (
          <Typography.Text type="warning" style={{ display: 'block', marginBottom: 8 }}>
            File was truncated. Only the first 10MB is shown.
          </Typography.Text>
        )}
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
    );
  }

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
      {editable && !truncated && (
        <Button
          type="text"
          icon={<EditOutlined />}
          onClick={handleStartEdit}
          style={{ marginBottom: 8 }}
        >
          Edit
        </Button>
      )}
      {truncated && (
        <Typography.Text type="warning" style={{ display: 'block', marginBottom: 8 }}>
          File was truncated. Only the first 10MB is shown.
        </Typography.Text>
      )}
      <pre
        style={{
          background: '#f6f8fa',
          border: '1px solid #e1e4e8',
          borderRadius: 6,
          padding: 16,
          overflow: 'auto',
          flex: 1,
          fontSize: 13,
          lineHeight: 1.5,
          fontFamily: "'SF Mono', 'Monaco', 'Inconsolata', 'Fira Code', monospace",
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
          margin: 0,
        }}
      >
        <code className={`language-${language}`}>{content}</code>
      </pre>
    </div>
  );
};

export default TextPreview;
