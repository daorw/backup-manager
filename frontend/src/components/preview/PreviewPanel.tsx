import React, { useEffect, useState, useCallback } from 'react';
import { Tree, Spin, Empty, Typography, Space, Button, message } from 'antd';
import {
  FileOutlined,
  FolderOutlined,
  ReloadOutlined,
  FileTextOutlined,
} from '@ant-design/icons';
import type { DataNode } from 'antd/es/tree';
import { useAppStore } from '../../store/appStore';
import { previewFile } from '../../api/client';
import type { PreviewResult, Symlink, BrowseEntry } from '../../types';
import TextPreview from './TextPreview';
import MarkdownPreview from './MarkdownPreview';
import BinaryInfo from './BinaryInfo';

interface PreviewPanelProps {
  repoId: string;
}

interface PreviewTreeNode extends DataNode {
  symlink?: Symlink;
  linkId?: string;       // For directory entries: the parent symlink ID
  browseRelPath?: string; // For directory entries: relative path from symlink root
}

function buildPreviewTree(symlinks: Symlink[]): PreviewTreeNode[] {
  const root: PreviewTreeNode[] = [];
  const map = new Map<string, PreviewTreeNode>();
  // Include all symlinks (both file and directory)
  const sorted = [...symlinks]
    .sort((a, b) => a.relative_path.localeCompare(b.relative_path));

  for (const sym of sorted) {
    const parts = sym.relative_path.split('/');
    let currentPath = '';
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      currentPath = currentPath ? `${currentPath}/${part}` : part;
      const isLast = i === parts.length - 1;
      if (!map.has(currentPath)) {
        const isDirectorySymlink = isLast && sym.type === 'directory';
        const node: PreviewTreeNode = {
          key: currentPath,
          title: part,
          isLeaf: isDirectorySymlink ? false : isLast,
          symlink: isLast ? sym : undefined,
          icon: isLast ? (isDirectorySymlink ? <FolderOutlined /> : <FileTextOutlined />) : <FolderOutlined />,
          children: isLast && !isDirectorySymlink ? undefined : [],
          // For directory symlinks, attach loading info
          linkId: isDirectorySymlink ? sym.id : undefined,
          browseRelPath: isDirectorySymlink ? '' : undefined,
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
  const fetchDirEntries = useAppStore((s) => s.fetchDirEntries);

  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [previewResult, setPreviewResult] = useState<PreviewResult | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);

  // Dynamic children for expanded directory symlinks
  const [dynamicChildren, setDynamicChildren] = useState<Record<string, PreviewTreeNode[]>>({});
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]);

  useEffect(() => {
    if (repoId) {
      fetchSymlinks(repoId);
      setDynamicChildren({});
      setExpandedKeys([]);
    }
  }, [repoId, fetchSymlinks]);

  const baseTreeData = buildPreviewTree(symlinks);

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

  // Load directory contents when expanded
  const handleLoadData = async (treeNode: PreviewTreeNode): Promise<void> => {
    const { key, linkId, browseRelPath } = treeNode;
    if (!linkId) return;

    const cacheKey = `${linkId}:${browseRelPath || ''}`;
    if (dynamicChildren[cacheKey]) return;

    try {
      const entries = await fetchDirEntries(repoId, linkId, browseRelPath || '');
      const children: PreviewTreeNode[] = entries.map((entry: BrowseEntry) => {
        const nodeKey = `${key}/${entry.name}`;
        if (entry.type === 'directory') {
          return {
            key: nodeKey,
            title: entry.name,
            isLeaf: false,
            icon: <FolderOutlined />,
            linkId: linkId,
            browseRelPath: browseRelPath
              ? `${browseRelPath}/${entry.name}`
              : entry.name,
            children: [],
          };
        }
        return {
          key: nodeKey,
          title: entry.name,
          isLeaf: true,
          icon: <FileTextOutlined />,
          linkId: linkId,
          browseRelPath: browseRelPath
            ? `${browseRelPath}/${entry.name}`
            : entry.name,
        };
      });

      setDynamicChildren((prev) => ({
        ...prev,
        [cacheKey]: children,
      }));
    } catch (err) {
      message.error(err instanceof Error ? err.message : 'Failed to load directory contents');
    }
  };

  // Merge dynamic children into tree data (recursive)
  const mergeTreeData = (nodes: PreviewTreeNode[]): PreviewTreeNode[] => {
    return nodes.map((node) => {
      let processedChildren = node.children;
      if (!node.isLeaf && node.linkId) {
        const cacheKey = `${node.linkId}:${node.browseRelPath || ''}`;
        if (dynamicChildren[cacheKey]) {
          processedChildren = dynamicChildren[cacheKey];
        }
      }
      if (processedChildren && processedChildren.length > 0) {
        return { ...node, children: mergeTreeData(processedChildren) };
      }
      return { ...node, children: processedChildren || [] };
    });
  };

  const treeData = mergeTreeData(baseTreeData);

  const selectedSymlink = symlinks.find(
    (s) => s.relative_path === selectedFile
  );

  const handleExpand = async (keys: React.Key[], info: { expanded: boolean; node: PreviewTreeNode }) => {
    setExpandedKeys(keys);
    if (info.expanded && info.node.linkId) {
      await handleLoadData(info.node);
    }
  };

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
            onClick={() => {
              fetchSymlinks(repoId);
              setDynamicChildren({});
              setExpandedKeys([]);
            }}
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
            expandedKeys={expandedKeys}
            defaultExpandAll={false}
            showIcon
            selectedKeys={selectedFile ? [selectedFile] : []}
            onSelect={(keys) => {
              if (keys.length > 0) {
                const key = keys[0] as string;
                // Find the node to check if it's a file (leaf)
                // Traverse treeData to find the node
                const findNode = (nodes: PreviewTreeNode[]): PreviewTreeNode | null => {
                  for (const n of nodes) {
                    if (n.key === key) return n;
                    if (n.children) {
                      const found = findNode(n.children as PreviewTreeNode[]);
                      if (found) return found;
                    }
                  }
                  return null;
                };
                const node = findNode(treeData);
                if (node && node.isLeaf) {
                  handleFileSelect(key);
                }
              }
            }}
            onExpand={handleExpand}
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
          {selectedFile && !selectedSymlink && (
            <Space style={{ marginBottom: 12 }}>
              <Typography.Text strong>
                {selectedFile}
              </Typography.Text>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                (inside directory symlink)
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
