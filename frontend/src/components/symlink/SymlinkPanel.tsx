import React, { useEffect, useState, useCallback } from 'react';
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
} from '@ant-design/icons';
import { useAppStore } from '../../store/appStore';
import type { Symlink, SymlinkTreeNode, SymlinkDirEntry } from '../../types';
import SymlinkAddModal from './SymlinkAddModal';

interface SymlinkPanelProps {
  repoId: string;
}

interface ExtendedDataNode extends DataNode {
  symlink?: Symlink;
  isSymlinkLeaf?: boolean;
  linkId?: string;      // For directory symlinks: the symlink ID
  browseRelPath?: string; // For directory entries: relative path within the symlink
}

function buildTree(symlinks: Symlink[]): SymlinkTreeNode[] {
  const root: SymlinkTreeNode[] = [];
  const map = new Map<string, SymlinkTreeNode>();

  // Sort by relative_path for deterministic order
  const sorted = [...symlinks].sort((a, b) =>
    a.relative_path.localeCompare(b.relative_path)
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
          if (parent && parent.children) {
            parent.children.push(node);
          }
        }
      }
    }
  }

  return root;
}

const SymlinkPanel: React.FC<SymlinkPanelProps> = ({ repoId }) => {
  const symlinks = useAppStore((s) => s.symlinks);
  const loading = useAppStore((s) => s.loading);
  const error = useAppStore((s) => s.error);
  const fetchSymlinks = useAppStore((s) => s.fetchSymlinks);
  const createSymlink = useAppStore((s) => s.createSymlink);
  const deleteSymlink = useAppStore((s) => s.deleteSymlink);
  const updateSymlink = useAppStore((s) => s.updateSymlink);
  const fetchDirEntries = useAppStore((s) => s.fetchDirEntries);
  const clearDirEntryCache = useAppStore((s) => s.clearDirEntryCache);

  const [addModalOpen, setAddModalOpen] = useState(false);
  const [editingPath, setEditingPath] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');
  // Track which directory nodes have been expanded and their loaded children
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]);
  const [dynamicChildren, setDynamicChildren] = useState<Record<string, ExtendedDataNode[]>>({});

  useEffect(() => {
    if (repoId) {
      fetchSymlinks(repoId);
      clearDirEntryCache();
      setDynamicChildren({});
      setExpandedKeys([]);
    }
  }, [repoId]); // eslint-disable-line react-hooks/exhaustive-deps

  const treeData = buildTree(symlinks);

  const handleAddSymlink = useCallback(
    async (targetPath: string, relativePath: string) => {
      await createSymlink(repoId, targetPath, relativePath);
      clearDirEntryCache();
      setDynamicChildren({});
    },
    [repoId, createSymlink, clearDirEntryCache]
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
    [repoId, deleteSymlink, clearDirEntryCache]
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
    [repoId, updateSymlink, editValue]
  );

  const getContextMenuItems = (node: SymlinkTreeNode): MenuProps['items'] => {
    const items: MenuProps['items'] = [];
    if (node.symlink) {
      items.push({
        key: 'edit',
        icon: <EditOutlined />,
        label: 'Edit target',
        onClick: () => {
          setEditingPath(node.key);
          setEditValue(node.symlink!.target_path);
        },
      });
      items.push({
        key: 'delete',
        icon: <DeleteOutlined />,
        label: 'Delete',
        danger: true,
        onClick: () => handleDeleteSymlink(node.symlink!),
      });
    }
    return items.length > 0 ? items : undefined;
  };

  const handleTitleRenderer = (node: SymlinkTreeNode) => {
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
              <Typography.Text
                type="secondary"
                style={{ fontSize: 11, marginLeft: 8 }}
              >
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

  // Load directory contents when a directory-type symlink node is expanded
  const handleLoadData = async (treeNode: ExtendedDataNode): Promise<void> => {
    const { key, linkId, browseRelPath } = treeNode;
    if (!linkId) return;

    const cacheKey = `${linkId}:${browseRelPath || ''}`;
    if (dynamicChildren[cacheKey]) return; // Already loaded

    try {
      const entries = await fetchDirEntries(repoId, linkId, browseRelPath || '');
      const children: ExtendedDataNode[] = entries.map((entry: SymlinkDirEntry) => {
        const nodeKey = `${key}/${entry.name}`;

        // Handle nested symlink entries
        if (entry.is_nested_symlink) {
          const isError = entry.type === 'symlink_error';
          const isCycle = entry.has_cycle;
          const isDir = entry.type === 'symlink_directory';
          const isFile = entry.type === 'symlink_file';

          const iconColor = isError || isCycle ? '#ff4d4f' : isDir ? '#1890ff' : '#52c41a';
          const linkIcon = <LinkOutlined style={{ color: iconColor }} />;

          const titleContent = (
            <Typography.Text>
              {linkIcon}{' '}
              {entry.name}
              {isCycle && (
                <Tag color="red" style={{ marginLeft: 6, fontSize: 10, lineHeight: '16px' }}>cycle</Tag>
              )}
              {isError && !isCycle && (
                <Tag color="red" style={{ marginLeft: 6, fontSize: 10, lineHeight: '16px' }}>depth limit</Tag>
              )}
              {entry.nested_target && (
                <Typography.Text
                  type="secondary"
                  style={{ fontSize: 11, marginLeft: 8 }}
                >
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

          // Nested symlink directories are expandable
          if (isDir && !isError) {
            return {
              key: nodeKey,
              title: titleContent,
              isLeaf: false,
              icon: <FolderOutlined style={{ color: '#1890ff' }} />,
              linkId: linkId,
              browseRelPath: browseRelPath
                ? `${browseRelPath}/${entry.name}`
                : entry.name,
            };
          }

          // Nested symlink files and errors are leaf nodes
          return {
            key: nodeKey,
            title: titleContent,
            isLeaf: true,
            icon: isFile ? <FileOutlined style={{ color: '#52c41a' }} /> : <LinkOutlined style={{ color: '#ff4d4f' }} />,
            linkId: linkId,
            browseRelPath: browseRelPath
              ? `${browseRelPath}/${entry.name}`
              : entry.name,
          };
        }

        // Regular directory entries
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
          };
        }

        // Regular file entries
        return {
          key: nodeKey,
          title: (
            <Typography.Text>
              {entry.name}
              {entry.is_new && (
                <Tag color="green" style={{ marginLeft: 6, fontSize: 10, lineHeight: '16px' }}>new</Tag>
              )}
              <Typography.Text
                type="secondary"
                style={{ fontSize: 11, marginLeft: 8 }}
              >
                {entry.size > 0 ? `(${(entry.size / 1024).toFixed(1)} KB)` : ''}
              </Typography.Text>
            </Typography.Text>
          ),
          isLeaf: true,
          icon: <FileOutlined />,
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

  // Convert SymlinkTreeNode to Ant Design tree nodes with dynamic loading support
  const convertToAntdTreeData = (nodes: SymlinkTreeNode[]): ExtendedDataNode[] => {
    return nodes.map((node) => {
      const isDirectorySymlink =
        node.symlink && node.symlink.type === 'directory';

      const antdNode: ExtendedDataNode = {
        key: node.key,
        title: handleTitleRenderer(node),
        isLeaf: isDirectorySymlink ? false : node.isLeaf,
        icon: node.isLeaf ? (
          <FileOutlined />
        ) : isDirectorySymlink ? (
          <FolderOutlined />
        ) : (
          <FolderOutlined />
        ),
        symlink: node.symlink,
        isSymlinkLeaf: node.isLeaf,
      };

      // For directory symlinks, attach loading info
      if (isDirectorySymlink && node.symlink) {
        antdNode.linkId = node.symlink.id;
        antdNode.browseRelPath = '';
      }

      // Convert static children
      if (node.children && node.children.length > 0) {
        antdNode.children = convertToAntdTreeData(node.children);
      } else if (!node.isLeaf && isDirectorySymlink) {
        // Directory symlink: children will be loaded dynamically
        const cacheKey = `${node.symlink!.id}:`;
        if (dynamicChildren[cacheKey]) {
          antdNode.children = dynamicChildren[cacheKey];
        } else {
          antdNode.children = [];
        }
      } else if (!node.isLeaf) {
        antdNode.children = [];
      }

      return antdNode;
    });
  };

  // Merge dynamic children into tree data (recursive)
  const mergeTreeData = (nodes: ExtendedDataNode[]): ExtendedDataNode[] => {
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
      return { ...node, children: processedChildren };
    });
  };

  const finalTreeData = mergeTreeData(convertToAntdTreeData(treeData));

  const handleExpand = async (keys: React.Key[], info: { expanded: boolean; node: ExtendedDataNode }) => {
    setExpandedKeys(keys);
    if (info.expanded && info.node.linkId) {
      await handleLoadData(info.node);
    }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16, justifyContent: 'space-between', width: '100%' }}>
        <Typography.Title level={5} style={{ margin: 0 }}>
          Symlinks ({symlinks.length})
        </Typography.Title>
        <Space>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => {
              fetchSymlinks(repoId);
              clearDirEntryCache();
              setDynamicChildren({});
              setExpandedKeys([]);
            }}
          >
            Refresh
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setAddModalOpen(true)}
          >
            Add Symlink
          </Button>
        </Space>
      </Space>

      {error && (
        <Typography.Text type="danger" style={{ display: 'block', marginBottom: 8 }}>
          {error}
        </Typography.Text>
      )}

      <Spin spinning={loading}>
        {treeData.length === 0 ? (
          <Empty
            description="No symlinks yet. Add files to start backing up."
            style={{ marginTop: 48 }}
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
              maxHeight: 500,
              overflow: 'auto',
            }}
          >
            <Tree
              treeData={finalTreeData}
              expandedKeys={expandedKeys}
              defaultExpandAll={false}
              showIcon
              onSelect={(keys, info) => {
                const node = info.node as ExtendedDataNode;
                // For directory entries, we just report the selection
                if (node?.linkId && node?.isLeaf) {
                  message.info(`Selected: ${node.key}`);
                }
              }}
              onExpand={handleExpand}
              loadData={undefined} // We handle loading via onExpand
            />
          </div>
        )}
      </Spin>

      <SymlinkAddModal
        open={addModalOpen}
        onClose={() => setAddModalOpen(false)}
        onSubmit={handleAddSymlink}
      />
    </div>
  );
};

export default SymlinkPanel;
