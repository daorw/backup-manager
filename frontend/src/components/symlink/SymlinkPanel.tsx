import React, { useEffect, useState, useCallback } from 'react';
import {
  Tree,
  Button,
  Space,
  Typography,
  Dropdown,
  message,
  Input,
  Modal,
  Empty,
  Spin,
} from 'antd';
import type { MenuProps } from 'antd';
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
import type { Symlink, SymlinkTreeNode } from '../../types';
import SymlinkAddModal from './SymlinkAddModal';

interface SymlinkPanelProps {
  repoId: string;
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
          isLeaf: isLast,
          symlink: isLast ? sym : undefined,
          children: isLast ? undefined : [],
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

  const [addModalOpen, setAddModalOpen] = useState(false);
  const [selectedNode, setSelectedNode] = useState<SymlinkTreeNode | null>(null);
  const [editingPath, setEditingPath] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');

  useEffect(() => {
    if (repoId) {
      fetchSymlinks(repoId);
    }
  }, [repoId, fetchSymlinks]);

  const treeData = buildTree(symlinks);

  const handleAddSymlink = useCallback(
    async (targetPath: string, relativePath: string) => {
      await createSymlink(repoId, targetPath, relativePath);
    },
    [repoId, createSymlink]
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
            message.success('Symlink deleted');
          } catch (err) {
            if (err instanceof Error) {
              message.error(err.message);
            }
          }
        },
      });
    },
    [repoId, deleteSymlink]
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

  const titleRenderer = (node: SymlinkTreeNode) => {
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

  const convertToAntdTreeData = (nodes: SymlinkTreeNode[]): any[] => {
    return nodes.map((node) => ({
      key: node.key,
      title: titleRenderer(node),
      isLeaf: node.isLeaf,
      icon: node.isLeaf ? <FileOutlined /> : <FolderOutlined />,
      children: node.children
        ? convertToAntdTreeData(node.children)
        : undefined,
    }));
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
            onClick={() => fetchSymlinks(repoId)}
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
              treeData={convertToAntdTreeData(treeData)}
              defaultExpandAll
              showIcon
              onSelect={(keys, info) => {
                const node = info.node as any;
                if (node?.symlink) {
                  setSelectedNode(node.symlink);
                }
              }}
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
