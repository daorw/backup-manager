import React, { useEffect, useState, useCallback } from 'react';
import { Tree, Spin, Empty, Typography, Space, Button } from 'antd';
import {
  FileOutlined,
  FolderOutlined,
  ReloadOutlined,
  FileTextOutlined,
} from '@ant-design/icons';
import type { DataNode } from 'antd/es/tree';
import { useAppStore } from '../../store/appStore';
import { previewFile } from '../../api/client';
import type { PreviewResult, Symlink } from '../../types';
import TextPreview from './TextPreview';
import MarkdownPreview from './MarkdownPreview';
import BinaryInfo from './BinaryInfo';

interface PreviewPanelProps {
  repoId: string;
}

interface PreviewTreeNode extends DataNode {
  symlink?: Symlink;
}

function buildPreviewTree(symlinks: Symlink[]): PreviewTreeNode[] {
  const root: PreviewTreeNode[] = [];
  const map = new Map<string, PreviewTreeNode>();
  const sorted = [...symlinks]
    .filter((s) => s.type === 'file')
    .sort((a, b) => a.relative_path.localeCompare(b.relative_path));

  for (const sym of sorted) {
    const parts = sym.relative_path.split('/');
    let currentPath = '';
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      currentPath = currentPath ? `${currentPath}/${part}` : part;
      const isLast = i === parts.length - 1;
      if (!map.has(currentPath)) {
        const node: PreviewTreeNode = {
          key: currentPath,
          title: part,
          isLeaf: isLast,
          symlink: isLast ? sym : undefined,
          icon: isLast ? <FileTextOutlined /> : <FolderOutlined />,
          children: isLast ? undefined : [],
        };
        map.set(currentPath, node);
        if (i === 0) {
          root.push(node);
        } else {
          const parentPath = parts.slice(0, i).join('/');
          const parent = map.get(parentPath);
          if (parent && parent.children) {
            parent.children.push(node);
          }
        }
      }
    }
  }
  return root;
}

const TEXT_EXTENSIONS = new Set([
  'txt', 'md', 'json', 'xml', 'yaml', 'yml', 'toml', 'ini', 'cfg', 'conf',
  'env', 'log', 'csv', 'tsv',
  'js', 'jsx', 'ts', 'tsx', 'py', 'rb', 'java', 'go', 'rs', 'cpp', 'c',
  'h', 'hpp', 'cs', 'php', 'swift', 'kt', 'scala',
  'html', 'css', 'scss', 'less',
  'sql', 'sh', 'bash', 'zsh', 'bat', 'ps1',
  'makefile', 'dockerfile',
]);

function isTextExtension(filename: string): boolean {
  const lower = filename.toLowerCase();
  if (lower === 'makefile' || lower === 'dockerfile') return true;
  const ext = lower.split('.').pop() || '';
  return TEXT_EXTENSIONS.has(ext);
}

const PreviewPanel: React.FC<PreviewPanelProps> = ({ repoId }) => {
  const symlinks = useAppStore((s) => s.symlinks);
  const fetchSymlinks = useAppStore((s) => s.fetchSymlinks);

  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [previewResult, setPreviewResult] = useState<PreviewResult | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);

  useEffect(() => {
    if (repoId) {
      fetchSymlinks(repoId);
    }
  }, [repoId, fetchSymlinks]);

  const treeData = buildPreviewTree(symlinks);

  const handleFileSelect = useCallback(
    async (path: string) => {
      setSelectedFile(path);
      setPreviewLoading(true);
      setPreviewError(null);
      setPreviewResult(null);
      try {
        const result = await previewFile(repoId, path);
        setPreviewResult(result);
      } catch (err) {
        setPreviewError(
          err instanceof Error ? err.message : 'Failed to preview file'
        );
      } finally {
        setPreviewLoading(false);
      }
    },
    [repoId]
  );

  const selectedSymlink = symlinks.find(
    (s) => s.relative_path === selectedFile
  );

  const renderPreview = () => {
    if (previewLoading) {
      return (
        <div style={{ textAlign: 'center', padding: 48 }}>
          <Spin size="large" />
        </div>
      );
    }

    if (previewError) {
      return (
        <div style={{ textAlign: 'center', padding: 48 }}>
          <Typography.Text type="danger">{previewError}</Typography.Text>
        </div>
      );
    }

    if (!previewResult || !selectedFile) {
      return (
        <div style={{ textAlign: 'center', padding: 64 }}>
          <FileOutlined style={{ fontSize: 48, color: '#d9d9d9' }} />
          <Typography.Paragraph type="secondary" style={{ marginTop: 16 }}>
            Select a file from the tree to preview its contents
          </Typography.Paragraph>
        </div>
      );
    }

    if (!previewResult.text) {
      return (
        <BinaryInfo preview={previewResult} fileName={selectedFile.split('/').pop() || selectedFile} />
      );
    }

    const fileName = selectedFile.split('/').pop() || selectedFile;
    const ext = fileName.toLowerCase().split('.').pop();

    const content = previewResult.content || '';
    if (ext === 'md' || ext === 'markdown') {
      return (
        <MarkdownPreview
          content={content}
          repoId={repoId}
          filePath={selectedFile}
        />
      );
    }

    return (
      <TextPreview
        content={content}
        fileName={fileName}
        truncated={previewResult.truncated || false}
      />
    );
  };

  return (
    <div style={{ display: 'flex', gap: 16, minHeight: 400 }}>
      <div
        style={{
          width: 280,
          flexShrink: 0,
          border: '1px solid #f0f0f0',
          borderRadius: 6,
          padding: 12,
          maxHeight: 600,
          overflow: 'auto',
        }}
      >
        <Space
          style={{
            marginBottom: 8,
            justifyContent: 'space-between',
            width: '100%',
          }}
        >
          <Typography.Text strong>Files</Typography.Text>
          <Button
            size="small"
            icon={<ReloadOutlined />}
            onClick={() => fetchSymlinks(repoId)}
          />
        </Space>
        {treeData.length === 0 ? (
          <Empty
            description="No files to preview"
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          />
        ) : (
          <Tree
            treeData={treeData}
            defaultExpandAll
            showIcon
            selectedKeys={selectedFile ? [selectedFile] : []}
            onSelect={(keys) => {
              if (keys.length > 0) {
                const key = keys[0] as string;
                const sym = symlinks.find((s) => s.relative_path === key);
                if (sym && sym.type === 'file') {
                  handleFileSelect(key);
                }
              }
            }}
          />
        )}
      </div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{
            border: '1px solid #f0f0f0',
            borderRadius: 6,
            padding: 12,
            minHeight: 400,
          }}
        >
          {selectedFile && selectedSymlink && (
            <Space style={{ marginBottom: 12 }}>
              <Typography.Text strong>
                {selectedFile}
              </Typography.Text>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                → {selectedSymlink.target_path}
              </Typography.Text>
            </Space>
          )}
          {renderPreview()}
        </div>
      </div>
    </div>
  );
};

export default PreviewPanel;
