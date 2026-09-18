import React, { useState, useCallback, useEffect, useRef } from 'react';
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
  Switch,
  Tooltip,
} from 'antd';
import {
  FolderOutlined,
  FileOutlined,
  ReloadOutlined,
  ArrowUpOutlined,
  EyeOutlined,
} from '@ant-design/icons';
import type { DataNode } from 'antd/es/tree';
import { browsePath, fetchHomeDir } from '../../api/client';
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

function toNodes(entries: BrowseEntry[]): FileBrowserNode[] {
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

async function loadChildren(
  nodePath: string,
  includeHidden: boolean
): Promise<FileBrowserNode[]> {
  const entries: BrowseEntry[] = await browsePath(nodePath, includeHidden);
  return toNodes(entries);
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
  const [homeDir, setHomeDir] = useState<string>('/');
  const [currentPathInput, setCurrentPathInput] = useState<string>('');
  const [showHidden, setShowHidden] = useState(false);
  const [pathWarning, setPathWarning] = useState<string>('');
  // 供 loadRoot 读取，避免因 showHidden 变化触发 useEffect 重新初始化
  const showHiddenRef = useRef(false);

  useEffect(() => {
    showHiddenRef.current = showHidden;
  }, [showHidden]);

  /** 载入某个目录的内容，作为树的根。失败时保留用户输入的路径（仍可手动使用）。 */
  const loadDir = useCallback(
    async (target: string, includeHidden: boolean) => {
      setLoading(true);
      try {
        const nodes = await loadChildren(target, includeHidden);
        setTreeData(nodes);
        setExpandedKeys([]);
        setPathWarning('');
        return true;
      } catch (err) {
        setTreeData([]);
        setPathWarning(
          err instanceof Error ? err.message : 'Cannot open this directory'
        );
        return false;
      } finally {
        setLoading(false);
      }
    },
    []
  );

  const loadRoot = useCallback(async () => {
    let home = '/';
    try {
      home = await fetchHomeDir();
    } catch {
      home = '/';
    }
    setHomeDir(home);

    const startPath = initialPath && initialPath !== '' ? initialPath : home;
    setCurrentPathInput(startPath);
    await loadDir(startPath, showHiddenRef.current);
  }, [initialPath, loadDir]);

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
      const children = await loadChildren(node.path, showHidden);
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

  /** 方式 1：跳转到输入框里的路径（Go / 回车）。 */
  const handleNavigateToPath = async () => {
    const target = currentPathInput.trim();
    if (!target) return;
    setCurrentPathInput(target);
    await loadDir(target, showHidden);
  };

  /** 方式 1：直接使用用户填写的完整路径，不要求能在树里打开（路径可能尚不存在）。 */
  const useTypedPath = () => {
    const target = currentPathInput.trim();
    if (!target) {
      message.warning('Please enter a path first');
      return;
    }
    onSelect(target);
    onClose();
  };

  const handleGoUp = async () => {
    if (!currentPathInput || currentPathInput === '/') return;
    const parentPath =
      currentPathInput.substring(0, currentPathInput.lastIndexOf('/')) || '/';
    setCurrentPathInput(parentPath);
    const ok = await loadDir(parentPath, showHidden);
    if (ok) setSelectedPath(parentPath);
  };

  const handleGoHome = async () => {
    setCurrentPathInput(homeDir);
    const ok = await loadDir(homeDir, showHidden);
    if (ok) setSelectedPath(homeDir);
  };

  /** 隐藏文件开关：切换后按当前目录重新载入。 */
  const toggleHidden = async (checked: boolean) => {
    setShowHidden(checked);
    const target = currentPathInput.trim() || homeDir;
    await loadDir(target, checked);
  };

  const handleConfirm = () => {
    if (!selectedPath) {
      message.warning('Please select a path first');
      return;
    }
    onSelect(selectedPath);
    onClose();
  };

  return (
    <Modal
      title={title}
      open={open}
      onCancel={onClose}
      width={680}
      footer={
        <Space>
          <Button onClick={onClose}>Cancel</Button>
          <Tooltip title="Use the path typed above as-is (it does not need to exist yet)">
            <Button onClick={useTypedPath} disabled={!currentPathInput.trim()}>
              Use Typed Path
            </Button>
          </Tooltip>
          <Button type="primary" onClick={handleConfirm} disabled={!selectedPath}>
            Select
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        <Space.Compact style={{ width: '100%' }}>
          <Tooltip title="Parent directory">
            <Button icon={<ArrowUpOutlined />} onClick={handleGoUp} />
          </Tooltip>
          <Tooltip title={`Home (${homeDir})`}>
            <Button onClick={handleGoHome}>~</Button>
          </Tooltip>
          <Input
            value={currentPathInput}
            onChange={(e) => setCurrentPathInput(e.target.value)}
            onPressEnter={handleNavigateToPath}
            placeholder="Type a full path (e.g. /Users/me/notes) and press Enter"
          />
          <Button onClick={handleNavigateToPath}>Go</Button>
          <Tooltip title="Reload">
            <Button
              icon={<ReloadOutlined />}
              onClick={() => loadDir(currentPathInput.trim() || homeDir, showHidden)}
            />
          </Tooltip>
        </Space.Compact>

        <Space>
          <Switch
            checked={showHidden}
            onChange={toggleHidden}
            checkedChildren={<EyeOutlined />}
            unCheckedChildren={<EyeOutlined />}
            size="small"
          />
          <Typography.Text style={{ fontSize: 12 }}>Show hidden files</Typography.Text>
          {pathWarning && (
            <Typography.Text type="warning" style={{ fontSize: 12 }}>
              {pathWarning}
            </Typography.Text>
          )}
        </Space>

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
            <Empty
              description={
                pathWarning ? 'Directory cannot be listed' : 'No entries found'
              }
            />
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
          Two ways to choose: type a full path above and click &quot;Use Typed Path&quot;, or
          browse the tree and click a node then &quot;Select&quot;.
          {mode === 'directory' && ' (directories only)'}
          {mode === 'file' && ' (files only)'}
        </Typography.Text>
      </Space>
    </Modal>
  );
};

export default DirectoryPickerModal;
