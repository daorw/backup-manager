import React, { useMemo, useState } from 'react';
import { Typography, Button, Space, Tabs } from 'antd';
import { SaveOutlined, CloseOutlined } from '@ant-design/icons';

interface MarkdownPreviewProps {
  content: string;
  repoId: string;
  filePath: string;
  editable?: boolean;
  onSave?: (content: string) => Promise<void>;
  saving?: boolean;
}

function simpleMarkdownToHtml(md: string, repoId: string, filePath: string): string {
  const basePath = filePath.substring(0, filePath.lastIndexOf('/') + 1);
  const apiBase = `/api/v1/repos/${repoId}/preview?path=`;

  let html = md
    // Escape HTML tags
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    // Headers
    .replace(/^###### (.*)$/gm, '<h6>$1</h6>')
    .replace(/^##### (.*)$/gm, '<h5>$1</h5>')
    .replace(/^#### (.*)$/gm, '<h4>$1</h4>')
    .replace(/^### (.*)$/gm, '<h3>$1</h3>')
    .replace(/^## (.*)$/gm, '<h2>$1</h2>')
    .replace(/^# (.*)$/gm, '<h1>$1</h1>')
    // Bold and italic
    .replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/___(.+?)___/g, '<strong><em>$1</em></strong>')
    .replace(/__(.+?)__/g, '<strong>$1</strong>')
    .replace(/_(.+?)_/g, '<em>$1</em>')
    // Strikethrough
    .replace(/~~(.+?)~~/g, '<del>$1</del>')
    // Inline code
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    // Code blocks
    .replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code class="language-$1">$2</code></pre>')
    // Images - rewrite local paths to API
    .replace(/!\[([^\]]*)\]\(([^)]+)\)/g, (_match, alt, src) => {
      const resolvedSrc = resolveImageSrc(src, basePath, apiBase);
      return `<img src="${resolvedSrc}" alt="${alt}" style="max-width:100%" />`;
    })
    // Links
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>')
    // Horizontal rules
    .replace(/^---$/gm, '<hr />')
    .replace(/^\*\*\*$/gm, '<hr />')
    .replace(/^___$/gm, '<hr />')
    // Blockquotes
    .replace(/^> (.*)$/gm, '<blockquote>$1</blockquote>')
    // Unordered lists
    .replace(/^[\*\-] (.*)$/gm, '<li>$1</li>')
    // Ordered lists
    .replace(/^\d+\. (.*)$/gm, '<li>$1</li>')
    // Paragraphs - wrap remaining text
    .replace(/^(?!<[hHLlpbBcodh]|<li|<bl|<\/)(.+)$/gm, '<p>$1</p>');

  // Wrap consecutive <li> tags in <ul>
  html = html.replace(/((?:<li>.*?<\/li>\s*)+)/g, '<ul>$1</ul>');

  // Wrap consecutive <blockquote> tags
  html = html.replace(/((?:<blockquote>.*?<\/blockquote>\s*)+)/g, (match) => {
    return match;
  });

  return html;
}

function resolveImageSrc(src: string, basePath: string, apiBase: string): string {
  // Absolute URL - return as-is
  if (src.startsWith('http://') || src.startsWith('https://')) {
    return src;
  }
  // Absolute path from root
  if (src.startsWith('/')) {
    return `${apiBase}${encodeURIComponent(src)}`;
  }
  // Data URI - return as-is
  if (src.startsWith('data:')) {
    return src;
  }
  // Relative path - resolve relative to the markdown file
  const resolvedPath = basePath + src;
  return `${apiBase}${encodeURIComponent(resolvedPath)}`;
}

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

  // Compute markdown HTML unconditionally at the top level
  const displayContent = editable && activeTab === 'edit' ? editContent : content;
  const html = useMemo(
    () => simpleMarkdownToHtml(displayContent, repoId, filePath),
    [displayContent, repoId, filePath]
  );

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

  if (!editable) {
    return (
      <div
        className="markdown-preview"
        style={{
          padding: 16,
          border: '1px solid #f0f0f0',
          borderRadius: 6,
          maxHeight: 600,
          overflow: 'auto',
        }}
        dangerouslySetInnerHTML={{ __html: html }}
      />
    );
  }

  const tabItems = [
    {
      key: 'preview',
      label: 'Preview',
      children: (
        <div
          className="markdown-preview"
          style={{
            padding: 16,
            border: '1px solid #f0f0f0',
            borderRadius: 6,
            maxHeight: 600,
            overflow: 'auto',
          }}
          dangerouslySetInnerHTML={{ __html: html }}
        />
      ),
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
