import React, { useState, useCallback, useEffect } from 'react';
import {
  Modal,
  Form,
  Input,
  Tree,
  Spin,
  message,
  Typography,
  Space,
  Button,
  Empty,
} from 'antd';
import {
  FolderOutlined,
  FileOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import type { DataNode } from 'antd/es/tree';
import { browsePath } from '../../api/client';

interface SymlinkAddModalProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (targetPath: string, relativePath: string) => Promise<void>;
  repoPath: string;
}

interface FileBrowserNode extends DataNode {
  path: string;
  isLeaf: boolean;
  nodeType: 'file' | 'directory';
}

async function loadChildren(nodePath: string): Promise<FileBrowserNode[]> {
  const entries = await browsePath(nodePath);
  return entries.map((entry) => ({
    key: entry.path,
    title: entry.name,
    path: entry.path,
    isLeaf: entry.type === 'file',
    nodeType: entry.type,
    icon: entry.type === 'directory' ? <FolderOutlined /> : <FileOutlined />,
    children: entry.type === 'directory' ? [] : undefined,
  }));
}

const SymlinkAddModal: React.FC<SymlinkAddModalProps> = ({
  open,
  onClose,
  onSubmit,
  repoPath,
}) => {
  const [form] = Form.useForm();
  const [treeData, setTreeData] = useState<FileBrowserNode[]>([]);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [selectedPath, setSelectedPath] = useState<string>('');
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]);

  const loadRoot = useCallback(async () => {
    setLoading(true);
    try {
      // Browse the repo's data/ directory to show currently backed-up files
      const dataPath = repoPath ? `${repoPath}/data` : '/';
      const nodes = await loadChildren(dataPath);
      setTreeData(nodes);
    } catch {
      // Fallback: try OS home directory
      try {
        const nodes = await loadChildren('/');
        setTreeData(nodes);
      } catch {
        setTreeData([]);
      }
    } finally {
      setLoading(false);
    }
  }, [repoPath]);

  useEffect(() => {
    if (open) {
      loadRoot();
      form.resetFields();
      setSelectedPath('');
    }
  }, [open, loadRoot, form]);

  const onLoadData = async (node: FileBrowserNode): Promise<void> => {
    if (node.children && node.children.length > 0) {
      return;
    }
    const children = await loadChildren(node.path);
    setTreeData((prev) => updateTreeNode(prev, node.key, children));
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
      if (node.nodeType === 'file' || node.nodeType === 'directory') {
        setSelectedPath(node.path);
        form.setFieldsValue({ target_path: node.path });
      }
    }
  };

  const handleOk = async () => {
    try {
      const values = await form.validateFields();
      setSubmitting(true);
      await onSubmit(values.target_path, values.relative_path);
      form.resetFields();
      message.success('Symlink created successfully');
      onClose();
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Add Symlink"
      open={open}
      onOk={handleOk}
      onCancel={onClose}
      confirmLoading={submitting}
      okText="Add"
      width={700}
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="target_path"
          label="Source Path"
          rules={[{ required: true, message: 'Please select a source file or directory' }]}
        >
          <Input
            placeholder="Please enter the file or directories you want to back up, e.g: ~/.config/opencode/opencode.json"
            value={selectedPath}
            onChange={(e) => setSelectedPath(e.target.value)}
          />
        </Form.Item>
        <Form.Item
          name="relative_path"
          label="Link Name (in .links/)"
          rules={[
            { required: true, message: 'Please enter a link name' },
            {
              pattern: /^[a-zA-Z0-9_\/\-\.]+$/,
              message: 'Link name can only contain letters, numbers, /, -, _, .',
            },
          ]}
        >
          <Input placeholder="Please enter a file path relative to the repository for storing the backup files, e.g: data/opencode/opencode.json" />
        </Form.Item>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          The content below in File Browser shows the currently backed-up files, which are stored under the relative root directory specified by Link Name.
        </Typography.Text>
      </Form>

      <div style={{ marginTop: 16 }}>
        <Space style={{ marginBottom: 8 }}>
          <Typography.Text strong>File Browser</Typography.Text>
          <Button
            size="small"
            icon={<ReloadOutlined />}
            onClick={loadRoot}
            loading={loading}
          >
            Refresh
          </Button>
        </Space>
        <div
          style={{
            border: '1px solid #d9d9d9',
            borderRadius: 6,
            padding: 8,
            maxHeight: 300,
            overflow: 'auto',
          }}
        >
          {loading ? (
            <div style={{ textAlign: 'center', padding: 24 }}>
              <Spin />
            </div>
          ) : treeData.length === 0 ? (
            <Empty description="No files found" />
          ) : (
            <Tree
              treeData={treeData}
              loadData={onLoadData as any}
              onSelect={handleSelect as any}
              expandedKeys={expandedKeys}
              onExpand={(keys) => setExpandedKeys(keys)}
              showIcon
              defaultExpandParent={false}
            />
          )}
        </div>
      </div>
    </Modal>
  );
};

export default SymlinkAddModal;
