import React, { useState, useCallback, useEffect } from 'react';
import {
  Modal,
  Tree,
  Spin,
  Button,
  Space,
  Input,
  message,
  Empty,
  Typography,
} from 'antd';
import {
  FolderOutlined,
  FileOutlined,
  ReloadOutlined,
  ArrowUpOutlined,
} from '@ant-design/icons';
import type { DataNode } from 'antd/es/tree';
import { browsePath, fetchAllowedRoots } from '../../api/client';
import type { BrowseEntry } from '../../types';

interface FileBrowserNode extends DataNode {
  path: string;
  isLeaf: boolean;
  nodeType: 'file' | 'directory';
}

export type PickerMode = 'directory' | 'file' | 'both';

interface DirectoryPickerModalProps {
  open: boolean;
  onClose: () => void;
  onSelect: (path: string) => void;
  mode?: PickerMode;
  title?: string;
  initialPath?: string;
}

async function loadChildren(nodePath: string): Promise<FileBrowserNode[]> {
  const entries: BrowseEntry[] = await browsePath(nodePath);
  return entries
    .map((entry) => ({
      key: entry.path,
      title: entry.name,
      path: entry.path,
      isLeaf: entry.type === 'file',
      nodeType: entry.type,
      icon: entry.type === 'directory' ? <FolderOutlined /> : <FileOutlined />,
      children: entry.type === 'directory' ? [] : undefined,
    }))
    .sort((a, b) => {
      if (a.nodeType === 'directory' && b.nodeType === 'file') return -1;
      if (a.nodeType === 'file' && b.nodeType === 'directory') return 1;
      return a.title.toString().localeCompare(b.title.toString());
    });
}

const DirectoryPickerModal: React.FC<DirectoryPickerModalProps> = ({
  open,
  onClose,
  onSelect,
  mode = 'directory',
  title = 'Select Directory',
  initialPath,
}) => {
  const [treeData, setTreeData] = useState<FileBrowserNode[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedPath, setSelectedPath] = useState<string>('');
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]);
  const [roots, setRoots] = useState<string[]>([]);
  const [currentPathInput, setCurrentPathInput] = useState<string>('');

  const loadRoot = useCallback(async () => {
    setLoading(true);
    try {
      const allowedRoots = await fetchAllowedRoots();
      setRoots(allowedRoots);

      let startPath = initialPath;
      if (!startPath || startPath === '') {
        startPath = allowedRoots.length > 0 ? allowedRoots[0] : '/';
      }

      setCurrentPathInput(startPath);

      try {
        const nodes = await loadChildren(startPath);
        setTreeData(nodes);
      } catch {
        if (allowedRoots.length > 0) {
          const rootNodes = allowedRoots.map((root) => ({
            key: root,
            title: root,
            path: root,
            isLeaf: false,
            nodeType: 'directory' as const,
            icon: <FolderOutlined />,
            children: [] as FileBrowserNode[],
          }));
          setTreeData(rootNodes);
        } else {
          setTreeData([]);
        }
      }
    } catch {
      setTreeData([]);
    } finally {
      setLoading(false);
    }
  }, [initialPath]);

  useEffect(() => {
    if (open) {
      loadRoot();
      setSelectedPath('');
      setExpandedKeys([]);
    }
  }, [open, loadRoot]);

  const onLoadData = async (node: FileBrowserNode): Promise<void> => {
    if (node.children && node.children.length > 0) {
      return;
    }
    try {
      const children = await loadChildren(node.path);
      setTreeData((prev) => updateTreeNode(prev, node.key, children));
      setCurrentPathInput(node.path);
    } catch (err) {
      message.error(err instanceof Error ? err.message : 'Failed to load directory');
    }
  };

  const updateTreeNode = (
    nodes: FileBrowserNode[],
    key: React.Key,
    children: FileBrowserNode[]
  ): FileBrowserNode[] => {
    return nodes.map((node) => {
      if (node.key === key) {
        return { ...node, children };
      }
      if (node.children) {
        return {
          ...node,
          children: updateTreeNode(node.children as FileBrowserNode[], key, children),
        };
      }
      return node;
    });
  };

  const handleSelect = (selectedKeys: React.Key[], info: { node: FileBrowserNode }) => {
    if (selectedKeys.length > 0) {
      const node = info.node;
      if (mode === 'directory' && node.nodeType === 'directory') {
        setSelectedPath(node.path);
      } else if (mode === 'file' && node.nodeType === 'file') {
        setSelectedPath(node.path);
      } else if (mode === 'both') {
        setSelectedPath(node.path);
      }
      setCurrentPathInput(node.path);
    }
  };

  const handleNavigateToPath = async () => {
    if (!currentPathInput.trim()) return;
    setLoading(true);
    try {
      const info = await browsePath(currentPathInput);
      if (Array.isArray(info)) {
        setTreeData(info.map((entry) => ({
          key: entry.path,
          title: entry.name,
          path: entry.path,
          isLeaf: entry.type === 'file',
          nodeType: entry.type,
          icon: entry.type === 'directory' ? <FolderOutlined /> : <FileOutlined />,
          children: entry.type === 'directory' ? [] : undefined,
        })));
        setExpandedKeys([]);
      }
    } catch (err) {
      message.error(err instanceof Error ? err.message : 'Invalid path');
    } finally {
      setLoading(false);
    }
  };

  const handleGoUp = async () => {
    if (!currentPathInput || currentPathInput === '/') return;
    const parentPath = currentPathInput.substring(0, currentPathInput.lastIndexOf('/')) || '/';
    setCurrentPathInput(parentPath);
    setLoading(true);
    try {
      const info = await browsePath(parentPath);
      if (Array.isArray(info)) {
        setTreeData(info.map((entry) => ({
          key: entry.path,
          title: entry.name,
          path: entry.path,
          isLeaf: entry.type === 'file',
          nodeType: entry.type,
          icon: entry.type === 'directory' ? <FolderOutlined /> : <FileOutlined />,
          children: entry.type === 'directory' ? [] : undefined,
        })));
        setExpandedKeys([]);
        setSelectedPath(parentPath);
      }
    } catch (err) {
      message.error(err instanceof Error ? err.message : 'Failed to navigate');
    } finally {
      setLoading(false);
    }
  };

  const handleConfirm = () => {
    if (!selectedPath) {
      message.warning('Please select a path first');
      return;
    }
    onSelect(selectedPath);
    onClose();
  };

  const selectCurrentDir = () => {
    if (!currentPathInput) {
      message.warning('No directory selected');
      return;
    }
    onSelect(currentPathInput);
    onClose();
  };

  return (
    <Modal
      title={title}
      open={open}
      onCancel={onClose}
      width={640}
      footer={
        <Space>
          <Button onClick={onClose}>Cancel</Button>
          {mode !== 'file' && (
            <Button onClick={selectCurrentDir}>Use Current Directory</Button>
          )}
          <Button type="primary" onClick={handleConfirm} disabled={!selectedPath}>
            Select
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        <Space.Compact style={{ width: '100%' }}>
          <Button icon={<ArrowUpOutlined />} onClick={handleGoUp} title="Go to parent directory" />
          <Input
            value={currentPathInput}
            onChange={(e) => setCurrentPathInput(e.target.value)}
            onPressEnter={handleNavigateToPath}
            placeholder="Enter path and press Enter to navigate"
          />
          <Button onClick={handleNavigateToPath}>Go</Button>
        </Space.Compact>

        <div
          style={{
            border: '1px solid #d9d9d9',
            borderRadius: 6,
            padding: 8,
            maxHeight: 400,
            minHeight: 300,
            overflow: 'auto',
          }}
        >
          {loading ? (
            <div style={{ textAlign: 'center', padding: 48 }}>
              <Spin />
            </div>
          ) : treeData.length === 0 ? (
            <Empty description="No entries found" />
          ) : (
            <Tree
              treeData={treeData}
              loadData={onLoadData as any}
              onSelect={handleSelect as any}
              expandedKeys={expandedKeys}
              onExpand={(keys) => setExpandedKeys(keys)}
              showIcon
              defaultExpandParent={false}
              selectedKeys={selectedPath ? [selectedPath] : []}
            />
          )}
        </div>

        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {mode === 'directory' && 'Click a directory to select it, or navigate to a directory and click "Use Current Directory".'}
          {mode === 'file' && 'Click a file to select it.'}
          {mode === 'both' && 'Click a file or directory to select it.'}
        </Typography.Text>
      </Space>
    </Modal>
  );
};

export default DirectoryPickerModal;
