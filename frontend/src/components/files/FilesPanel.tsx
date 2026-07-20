import React, { useEffect, useState, useCallback, useMemo } from 'react';
import {
  Tree,
  Button,
  Space,
  Typography,
  Tag,
  Dropdown,
  message,
  Input,
  Modal,
  Empty,
  Spin,
} from 'antd';
import type { MenuProps } from 'antd';
import type { DataNode } from 'antd/es/tree';
import {
  PlusOutlined,
  FileOutlined,
  FolderOutlined,
  LinkOutlined,
  DeleteOutlined,
  EditOutlined,
  ReloadOutlined,
  FileTextOutlined,
} from '@ant-design/icons';
import { useAppStore } from '../../store/appStore';
import { previewFile, saveFile } from '../../api/client';
import type { Symlink, SymlinkTreeNode, SymlinkDirEntry, PreviewResult } from '../../types';
import SymlinkAddModal from '../symlink/SymlinkAddModal';
import TextPreview from '../preview/TextPreview';
import MarkdownPreview from '../preview/MarkdownPreview';
import BinaryInfo from '../preview/BinaryInfo';
import RepoOverviewCard from './RepoOverviewCard';

interface FilesPanelProps {
  repoId: string;
  repoPath: string;
}

interface ExtendedDataNode extends DataNode {
  symlink?: Symlink;
  isSymlinkLeaf?: boolean;
  linkId?: string;
  browseRelPath?: string;
}

// ── Tree Construction ──────────────────────────────────────────────────

function buildTree(symlinks: Symlink[]): SymlinkTreeNode[] {
  const root: SymlinkTreeNode[] = [];
  const map = new Map<string, SymlinkTreeNode>();
  const sorted = [...symlinks].sort((a, b) =>
    a.relative_path.localeCompare(b.relative_path),
  );

  for (const sym of sorted) {
    const parts = sym.relative_path.split('/');
    let currentPath = '';

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      const parentPath = currentPath;
      currentPath = currentPath ? `${currentPath}/${part}` : part;
      const isLast = i === parts.length - 1;

      if (!map.has(currentPath)) {
        const node: SymlinkTreeNode = {
          key: currentPath,
          title: part,
          isLeaf: isLast && sym.type === 'file',
          symlink: isLast ? sym : undefined,
          children: isLast && sym.type === 'file' ? undefined : [],
        };
        map.set(currentPath, node);

        if (parentPath === '') {
          root.push(node);
        } else {
          const parent = map.get(parentPath);
          if (parent?.children) {
            parent.children.push(node);
          }
        }
      }
    }
  }
  return root;
}

// ── Dynamic Children: Nested Symlink Entry → Tree Node ─────────────────

interface EntryContext {
  key: string;
  linkId: string;
  browseRelPath: string;
}

function entryToNode(entry: SymlinkDirEntry, ctx: EntryContext): ExtendedDataNode {
  const nodeKey = `${ctx.key}/${entry.name}`;
  const subCtx: EntryContext = {
    key: nodeKey,
    linkId: ctx.linkId,
    browseRelPath: ctx.browseRelPath ? `${ctx.browseRelPath}/${entry.name}` : entry.name,
  };

  if (entry.is_nested_symlink) {
    return nestedSymlinkToNode(entry, nodeKey, subCtx);
  }

  if (entry.type === 'directory') {
    return {
      title: entry.name,
      isLeaf: false,
      icon: <FolderOutlined />,
      ...subCtx,
    };
  }

  // Regular file
  return {
    title: (
      <Typography.Text>
        {entry.name}
        {entry.is_new && (
          <Tag color="green" style={{ marginLeft: 6, fontSize: 10, lineHeight: '16px' }}>new</Tag>
        )}
        <Typography.Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>
          {entry.size > 0 ? `(${(entry.size / 1024).toFixed(1)} KB)` : ''}
        </Typography.Text>
      </Typography.Text>
    ),
    isLeaf: true,
    icon: <FileOutlined />,
    ...subCtx,
  };
}

function nestedSymlinkToNode(
  entry: SymlinkDirEntry,
  nodeKey: string,
  ctx: EntryContext,
): ExtendedDataNode {
  const isError = entry.type === 'symlink_error';
  const isCycle = entry.has_cycle;
  const isDir = entry.type === 'symlink_directory';

  const iconColor = isError || isCycle ? '#ff4d4f' : isDir ? '#1890ff' : '#52c41a';

  const titleContent = (
    <Typography.Text>
      <LinkOutlined style={{ color: iconColor, marginRight: 4 }} />
      {entry.name}
      {isCycle && (
        <Tag color="red" style={{ marginLeft: 6, fontSize: 10, lineHeight: '16px' }}>cycle</Tag>
      )}
      {isError && !isCycle && (
        <Tag color="red" style={{ marginLeft: 6, fontSize: 10, lineHeight: '16px' }}>depth limit</Tag>
      )}
      {entry.nested_target && (
        <Typography.Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>
          → {entry.nested_target}
          {entry.nested_depth && entry.nested_depth > 1 && (
            <Typography.Text type="secondary" style={{ fontSize: 10 }}>
              {' '}(depth: {entry.nested_depth})
            </Typography.Text>
          )}
        </Typography.Text>
      )}
      {!isError && entry.is_new && (
        <Tag color="green" style={{ marginLeft: 6, fontSize: 10, lineHeight: '16px' }}>new</Tag>
      )}
    </Typography.Text>
  );

  // Nested symlink directory: expandable
  if (isDir && !isError) {
    return {
      title: titleContent,
      isLeaf: false,
      icon: <FolderOutlined style={{ color: '#1890ff' }} />,
      ...ctx,
    };
  }

  // Broken / cycle nested symlink: non-selectable, non-interactive
  if (isError) {
    return {
      key: nodeKey,
      title: titleContent,
      isLeaf: true,
      selectable: false,
      disabled: true,
      icon: <LinkOutlined style={{ color: '#ff4d4f' }} />,
    };
  }

  // Nested symlink file: selectable
  return {
    title: titleContent,
    isLeaf: true,
    icon: <FileOutlined style={{ color: '#52c41a' }} />,
    ...ctx,
  };
}

// ── Main Component ─────────────────────────────────────────────────────

const FilesPanel: React.FC<FilesPanelProps> = ({ repoId, repoPath }) => {
  // Store
  const symlinks = useAppStore((s) => s.symlinks);
  const currentRepo = useAppStore((s) => s.currentRepo);
  const loading = useAppStore((s) => s.loading);
  const error = useAppStore((s) => s.error);
  const fetchSymlinks = useAppStore((s) => s.fetchSymlinks);
  const createSymlink = useAppStore((s) => s.createSymlink);
  const deleteSymlink = useAppStore((s) => s.deleteSymlink);
  const updateSymlink = useAppStore((s) => s.updateSymlink);
  const fetchDirEntries = useAppStore((s) => s.fetchDirEntries);
  const clearDirEntryCache = useAppStore((s) => s.clearDirEntryCache);

  // Tree state
  const [addModalOpen, setAddModalOpen] = useState(false);
  const [editingPath, setEditingPath] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]);
  const [dynamicChildren, setDynamicChildren] = useState<Record<string, ExtendedDataNode[]>>({});

  // Preview state
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [previewResult, setPreviewResult] = useState<PreviewResult | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // ── Lifecycle ────────────────────────────────────────────────────────

  useEffect(() => {
    if (repoId) {
      fetchSymlinks(repoId);
      clearDirEntryCache();
      setDynamicChildren({});
      setExpandedKeys([]);
      setSelectedFile(null);
      setPreviewResult(null);
      setPreviewError(null);
    }
  }, [repoId]); // eslint-disable-line react-hooks/exhaustive-deps

  const rawTree = useMemo(() => buildTree(symlinks), [symlinks]);

  // ── File Preview ─────────────────────────────────────────────────────

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
        setPreviewError(err instanceof Error ? err.message : 'Failed to preview file');
      } finally {
        setPreviewLoading(false);
      }
    },
    [repoId],
  );

  const handleSave = useCallback(
    async (newContent: string) => {
      if (!selectedFile) return;
      setSaving(true);
      try {
        await saveFile(repoId, { path: selectedFile, content: newContent });
        message.success('File saved successfully');
        setPreviewResult((prev) => (prev ? { ...prev, content: newContent } : null));
      } catch (err) {
        message.error(err instanceof Error ? err.message : 'Failed to save file');
      } finally {
        setSaving(false);
      }
    },
    [repoId, selectedFile],
  );

  // ── Symlink CRUD ─────────────────────────────────────────────────────

  const handleAddSymlink = useCallback(
    async (targetPath: string, relativePath: string) => {
      await createSymlink(repoId, targetPath, relativePath);
      clearDirEntryCache();
      setDynamicChildren({});
    },
    [repoId, createSymlink, clearDirEntryCache],
  );

  const handleDeleteSymlink = useCallback(
    async (sym: Symlink) => {
      Modal.confirm({
        title: 'Delete symlink?',
        content: `Are you sure you want to delete "${sym.relative_path}"?`,
        okText: 'Delete',
        okButtonProps: { danger: true },
        cancelText: 'Cancel',
        onOk: async () => {
          try {
            if (selectedFile && selectedFile.startsWith(sym.relative_path)) {
              setSelectedFile(null);
              setPreviewResult(null);
              setPreviewError(null);
            }
            await deleteSymlink(repoId, sym.id);
            clearDirEntryCache(sym.id);
            setDynamicChildren({});
            message.success('Symlink deleted');
          } catch (err) {
            if (err instanceof Error) {
              message.error(err.message);
            }
          }
        },
      });
    },
    [repoId, deleteSymlink, clearDirEntryCache, selectedFile],
  );

  const handleEditSymlink = useCallback(
    async (sym: Symlink) => {
      if (!editValue.trim()) return;
      try {
        await updateSymlink(repoId, sym.id, editValue);
        message.success('Symlink updated');
        setEditingPath(null);
        setEditValue('');
      } catch (err) {
        if (err instanceof Error) {
          message.error(err.message);
        }
      }
    },
    [repoId, updateSymlink, editValue],
  );

  // ── Context Menu ─────────────────────────────────────────────────────

  const getContextMenuItems = (node: SymlinkTreeNode): MenuProps['items'] => {
    if (!node.symlink) return undefined;
    return [
      {
        key: 'edit',
        icon: <EditOutlined />,
        label: 'Edit target',
        onClick: () => {
          setEditingPath(node.key);
          setEditValue(node.symlink!.target_path);
        },
      },
      {
        key: 'delete',
        icon: <DeleteOutlined />,
        label: 'Delete',
        danger: true,
        onClick: () => handleDeleteSymlink(node.symlink!),
      },
    ];
  };

  // ── Tree Node Renderer ───────────────────────────────────────────────

  const renderNodeTitle = (node: SymlinkTreeNode) => {
    if (editingPath === node.key && node.symlink) {
      return (
        <Input
          size="small"
          value={editValue}
          onChange={(e) => setEditValue(e.target.value)}
          onPressEnter={() => handleEditSymlink(node.symlink!)}
          onBlur={() => {
            setEditingPath(null);
            setEditValue('');
          }}
          style={{ width: 300 }}
          autoFocus
        />
      );
    }

    return (
      <Dropdown
        menu={{ items: getContextMenuItems(node) }}
        trigger={['contextMenu']}
        disabled={!node.symlink}
      >
        <span>
          {node.symlink ? (
            <Typography.Text>
              {node.title}
              {node.symlink.is_new && (
                <Tag color="green" style={{ marginLeft: 6, fontSize: 10, lineHeight: '16px' }}>new</Tag>
              )}
              <Typography.Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>
                <LinkOutlined /> {node.symlink.target_path}
              </Typography.Text>
            </Typography.Text>
          ) : (
            <Typography.Text strong>{node.title}</Typography.Text>
          )}
        </span>
      </Dropdown>
    );
  };

  // ── Dynamic Loading ──────────────────────────────────────────────────

  const loadDirContents = async (treeNode: ExtendedDataNode): Promise<void> => {
    const { key, linkId, browseRelPath } = treeNode;
    if (!linkId) return;

    const cacheKey = `${linkId}:${browseRelPath || ''}`;
    if (dynamicChildren[cacheKey]) return;

    try {
      const entries = await fetchDirEntries(repoId, linkId, browseRelPath || '');
      const children: ExtendedDataNode[] = entries.map((entry) =>
        entryToNode(entry, {
          key: String(key),
          linkId,
          browseRelPath: browseRelPath || '',
        }),
      );

      setDynamicChildren((prev) => ({ ...prev, [cacheKey]: children }));
    } catch (err) {
      message.error(err instanceof Error ? err.message : 'Failed to load directory contents');
    }
  };

  // ── Tree Data ────────────────────────────────────────────────────────

  const convertToAntdTreeData = (nodes: SymlinkTreeNode[]): ExtendedDataNode[] => {
    return nodes.map((node) => {
      const isDirSymlink = node.symlink?.type === 'directory';
      const antdNode: ExtendedDataNode = {
        key: node.key,
        title: renderNodeTitle(node),
        isLeaf: isDirSymlink ? false : node.isLeaf,
        icon: node.isLeaf ? <FileTextOutlined /> : <FolderOutlined />,
        symlink: node.symlink,
        isSymlinkLeaf: node.isLeaf,
      };

      if (isDirSymlink && node.symlink) {
        antdNode.linkId = node.symlink.id;
        antdNode.browseRelPath = '';
      }

      if (node.children && node.children.length > 0) {
        antdNode.children = convertToAntdTreeData(node.children);
      } else if (!node.isLeaf && isDirSymlink) {
        const cacheKey = `${node.symlink!.id}:`;
        antdNode.children = dynamicChildren[cacheKey] ?? [];
      } else if (!node.isLeaf) {
        antdNode.children = [];
      }

      return antdNode;
    });
  };

  const mergeTreeData = (nodes: ExtendedDataNode[]): ExtendedDataNode[] => {
    return nodes.map((node) => {
      let children = node.children;
      if (!node.isLeaf && node.linkId) {
        const cacheKey = `${node.linkId}:${node.browseRelPath || ''}`;
        if (dynamicChildren[cacheKey]) {
          children = dynamicChildren[cacheKey];
        }
      }
      if (children && children.length > 0) {
        return { ...node, children: mergeTreeData(children) };
      }
      return { ...node, children };
    });
  };

  const treeData = useMemo(
    () => mergeTreeData(convertToAntdTreeData(rawTree)),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [rawTree, dynamicChildren, editingPath, editValue],
  );

  // ── Tree Event Handlers ──────────────────────────────────────────────

  const handleExpand = async (
    keys: React.Key[],
    info: { expanded: boolean; node: ExtendedDataNode },
  ) => {
    setExpandedKeys(keys);
    if (info.expanded && info.node.linkId) {
      await loadDirContents(info.node);
    }
  };

  const handleSelect = (
    keys: React.Key[],
    info: { node: ExtendedDataNode },
  ) => {
    if (keys.length === 0) return;
    const node = info.node;

    // Only file leaf nodes trigger preview (exclude directory symlinks)
    if (node.isLeaf && !(node.symlink?.type === 'directory')) {
      handleFileSelect(keys[0] as string);
    }
  };

  // ── Resolved symlink info for preview header ─────────────────────────

  const selectedSymlink = useMemo(() => {
    if (!selectedFile) return undefined;
    const exact = symlinks.find((s) => s.relative_path === selectedFile);
    if (exact) return exact;
    const dirSym = symlinks
      .filter((s) => s.type === 'directory')
      .find((s) => selectedFile.startsWith(s.relative_path + '/'));
    return dirSym;
  }, [symlinks, selectedFile]);

  // ── Preview Rendering ────────────────────────────────────────────────

  const renderPreview = () => {
    // No file selected → overview card
    if (!selectedFile) {
      if (currentRepo) {
        return <RepoOverviewCard repo={currentRepo} symlinks={symlinks} />;
      }
      return (
        <div
          style={{
            textAlign: 'center',
            padding: 64,
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <FileOutlined style={{ fontSize: 48, color: '#d9d9d9' }} />
          <Typography.Paragraph type="secondary" style={{ marginTop: 16 }}>
            Select a file from the tree to preview its contents
          </Typography.Paragraph>
        </div>
      );
    }

    if (previewLoading) {
      return (
        <div
          style={{
            textAlign: 'center',
            padding: 48,
            flex: 1,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Spin size="large" />
        </div>
      );
    }

    if (previewError) {
      return (
        <div
          style={{
            textAlign: 'center',
            padding: 48,
            flex: 1,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Typography.Text type="danger">{previewError}</Typography.Text>
        </div>
      );
    }

    if (!previewResult) {
      return (
        <div
          style={{
            textAlign: 'center',
            padding: 64,
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <FileOutlined style={{ fontSize: 48, color: '#d9d9d9' }} />
          <Typography.Paragraph type="secondary" style={{ marginTop: 16 }}>
            Select a file from the tree to preview its contents
          </Typography.Paragraph>
        </div>
      );
    }

    const fileName = selectedFile.split('/').pop() || selectedFile;
    const content = previewResult.content || '';
    const isEditable = previewResult.text && !previewResult.truncated && selectedFile != null;

    // Binary file
    if (!previewResult.text) {
      return (
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
          <BinaryInfo preview={previewResult} fileName={fileName} />
        </div>
      );
    }

    const ext = fileName.toLowerCase().split('.').pop();

    // Markdown
    if (ext === 'md' || ext === 'markdown') {
      return (
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
          <MarkdownPreview
            key={selectedFile}
            content={content}
            repoId={repoId}
            filePath={selectedFile}
            editable={isEditable}
            onSave={handleSave}
            saving={saving}
          />
        </div>
      );
    }

    // Text
    return (
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
        <TextPreview
          key={selectedFile}
          content={content}
          fileName={fileName}
          truncated={previewResult.truncated || false}
          editable={isEditable}
          onSave={handleSave}
          saving={saving}
        />
      </div>
    );
  };

  // ── Render ───────────────────────────────────────────────────────────

  return (
    <div style={{ display: 'flex', gap: 16, flex: 1, minHeight: 0 }}>
      {/* ── Left Panel: File Tree ── */}
      <div
        style={{
          width: 280,
          maxWidth: '40%',
          flexShrink: 0,
          display: 'flex',
          flexDirection: 'column',
          minHeight: 0,
        }}
      >
        <Space
          style={{
            marginBottom: 12,
            justifyContent: 'space-between',
            width: '100%',
          }}
        >
          <Typography.Title level={5} style={{ margin: 0 }}>
            Files ({symlinks.length})
          </Typography.Title>
          <Space>
            <Button
              icon={<ReloadOutlined />}
              onClick={() => {
                fetchSymlinks(repoId);
                clearDirEntryCache();
                setDynamicChildren({});
                setExpandedKeys([]);
                setSelectedFile(null);
                setPreviewResult(null);
                setPreviewError(null);
              }}
            >
              Refresh
            </Button>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => setAddModalOpen(true)}
            >
              Add
            </Button>
          </Space>
        </Space>

        {error && (
          <Typography.Text type="danger" style={{ display: 'block', marginBottom: 8 }}>
            {error}
          </Typography.Text>
        )}

        <Spin
          spinning={loading}
          style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}
        >
          {rawTree.length === 0 ? (
            <Empty
              description="No symlinks yet. Add files to start backing up."
              style={{
                flex: 1,
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'flex-start',
                marginTop: 48,
              }}
            >
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => setAddModalOpen(true)}
              >
                Add Symlink
              </Button>
            </Empty>
          ) : (
            <div
              style={{
                border: '1px solid #f0f0f0',
                borderRadius: 6,
                padding: 12,
                flex: 1,
                minHeight: 200,
                overflow: 'auto',
              }}
            >
              <Tree
                treeData={treeData}
                expandedKeys={expandedKeys}
                defaultExpandAll={false}
                showIcon
                selectedKeys={selectedFile ? [selectedFile] : []}
                onSelect={handleSelect as any}
                onExpand={handleExpand as any}
              />
            </div>
          )}
        </Spin>
      </div>

      {/* ── Right Panel: Preview / Overview ── */}
      <div
        style={{
          flex: 1,
          minWidth: 0,
          display: 'flex',
          flexDirection: 'column',
          minHeight: 0,
        }}
      >
        <div
          style={{
            border: '1px solid #f0f0f0',
            borderRadius: 6,
            padding: 12,
            flex: 1,
            overflow: 'hidden',
            display: 'flex',
            flexDirection: 'column',
            minHeight: 0,
          }}
        >
          {/* Preview header */}
          {selectedFile && selectedSymlink && (
            <Space style={{ marginBottom: 12 }}>
              <Typography.Text strong>{selectedFile}</Typography.Text>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                → {selectedSymlink.target_path}
                {!symlinks.find((s) => s.relative_path === selectedFile) && (
                  <Tag style={{ marginLeft: 4 }}>via directory symlink</Tag>
                )}
              </Typography.Text>
            </Space>
          )}
          {selectedFile && !selectedSymlink && (
            <Space style={{ marginBottom: 12 }}>
              <Typography.Text strong>{selectedFile}</Typography.Text>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                (inside directory symlink)
              </Typography.Text>
            </Space>
          )}

          {/* Preview body */}
          <div
            style={{
              flex: 1,
              minHeight: 0,
              display: 'flex',
              flexDirection: 'column',
            }}
          >
            {renderPreview()}
          </div>
        </div>
      </div>

      {/* Add Symlink Modal */}
      <SymlinkAddModal
        open={addModalOpen}
        onClose={() => setAddModalOpen(false)}
        onSubmit={handleAddSymlink}
        repoPath={repoPath}
      />
    </div>
  );
};

export default FilesPanel;
