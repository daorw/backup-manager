import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Tree, Typography, Space, Tag, Empty, Spin, Alert, Button, Tooltip } from 'antd';
import type { DataNode } from 'antd/es/tree';
import {
  FileOutlined,
  FolderOutlined,
  ReloadOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { fetchTree, previewFile, saveFile } from '../../api/client';
import type { ContentEntry, PreviewResult } from '../../types';
import { useAppStore } from '../../store/appStore';
import TextPreview from '../preview/TextPreview';
import MarkdownPreview from '../preview/MarkdownPreview';
import BinaryInfo from '../preview/BinaryInfo';

interface FilesPanelProps {
  repoId: string;
}

interface TreeMeta {
  entryId?: string;
  mountedHere: boolean;
  drift: boolean;
  notBackedUp: boolean;
}

type TreeNode = DataNode & TreeMeta;

/**
 * 浏览标签页：展示仓库 data/ 下的真实内容并支持预览与编辑。
 *
 * 内容只存在于 data/，本机路径只是指向它的软链接 —— 因此保存一次即可让该条目的
 * 所有链接立即反映，不存在双写与同步步骤。
 */
const FilesPanel: React.FC<FilesPanelProps> = ({ repoId }) => {
  const entries = useAppStore((s) => s.entries);
  const fetchEntries = useAppStore((s) => s.fetchEntries);
  const currentRepo = useAppStore((s) => s.currentRepo);

  const [treeData, setTreeData] = useState<TreeNode[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [preview, setPreview] = useState<PreviewResult | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  // 条目索引：repo_path → 元数据，用于在树上打「已备份 / 漂移」徽标
  const entryIndex = useMemo(() => {
    const map = new Map<string, TreeMeta>();
    for (const e of entries) {
      const currentLinks = e.links.filter((l) => l.is_current);
      map.set(e.repo_path, {
        entryId: e.id,
        mountedHere: currentLinks.length > 0,
        drift: currentLinks.some((l) => l.state !== 'ok' && l.state !== 'not_current'),
        notBackedUp: false,
      });
    }
    return map;
  }, [entries]);

  /** 节点是否被某个条目覆盖：本身是条目、位于条目之内、或包含条目。 */
  const isCovered = useCallback(
    (path: string) =>
      entries.some(
        (en) =>
          en.repo_path === path ||
          en.repo_path.startsWith(`${path}/`) ||
          path.startsWith(`${en.repo_path}/`)
      ),
    [entries]
  );

  const toNode = useCallback(
    (e: ContentEntry): TreeNode => {
      const meta = entryIndex.get(e.path);
      const isDir = e.type === 'directory';
      const notBackedUp = !meta && !isCovered(e.path);
      return {
        key: e.path,
        title: (
          <Space size={4}>
            <span>{e.name}</span>
            {meta && <Tag color="blue" style={{ marginInlineStart: 4 }}>entry</Tag>}
            {notBackedUp && (
              <Tooltip title="No entry covers this path. Create an entry to make it a backed-up member.">
                <Tag style={{ marginInlineStart: 4 }}>not backed up</Tag>
              </Tooltip>
            )}
            {meta?.drift && (
              <Tooltip title="A link of this entry needs repair">
                <WarningOutlined style={{ color: '#faad14' }} />
              </Tooltip>
            )}
          </Space>
        ),
        icon: isDir ? <FolderOutlined /> : <FileOutlined />,
        isLeaf: !isDir,
        entryId: meta?.entryId,
        mountedHere: !!meta?.mountedHere,
        drift: !!meta?.drift,
        notBackedUp,
      };
    },
    [entryIndex, isCovered]
  );

  const loadChildren = useCallback(
    async (path: string) => {
      const list = await fetchTree(repoId, path);
      setTreeData((prev) => updateChildren(prev, path, list.map(toNode)));
      return list;
    },
    [repoId, toNode]
  );

  const loadRoot = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await fetchTree(repoId, '');
      setTreeData(list.map(toNode));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load the repository content');
    } finally {
      setLoading(false);
    }
  }, [repoId, toNode]);

  useEffect(() => {
    fetchEntries(repoId);
    loadRoot();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [repoId]);

  // 条目状态变化后重建树，让徽标刷新
  useEffect(() => {
    if (treeData.length > 0) {
      loadRoot();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entryIndex]);

  const handleSelect = async (path: string, isLeaf: boolean) => {
    if (!isLeaf) return;
    setSelected(path);
    setPreviewLoading(true);
    setPreview(null);
    try {
      setPreview(await previewFile(repoId, path));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to preview the file');
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleSave = async (content: string) => {
    if (!selected) return;
    setSaving(true);
    try {
      await saveFile(repoId, { path: selected, content });
      setPreview(await previewFile(repoId, selected));
    } finally {
      setSaving(false);
    }
  };

  const renderPreview = () => {
    if (previewLoading) {
      return (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Spin />
        </div>
      );
    }
    if (!preview || !selected) {
      return <Empty description="Select a file to preview" />;
    }
    const name = selected.split('/').pop() || '';
    if (!preview.text) {
      return <BinaryInfo preview={preview} fileName={name} />;
    }
    // 截断的文件不允许编辑，避免把不完整内容写回去
    const editable = !preview.truncated;
    if (name.toLowerCase().endsWith('.md')) {
      return (
        <MarkdownPreview
          content={preview.content || ''}
          repoId={repoId}
          filePath={selected}
          editable={editable}
          onSave={handleSave}
          saving={saving}
        />
      );
    }
    return (
      <TextPreview
        content={preview.content || ''}
        fileName={name}
        truncated={!!preview.truncated}
        editable={editable}
        onSave={handleSave}
        saving={saving}
      />
    );
  };

  return (
    <div style={{ display: 'flex', gap: 16, minHeight: 0, flex: 1 }}>
      <div style={{ width: 320, flexShrink: 0, display: 'flex', flexDirection: 'column' }}>
        <Space style={{ marginBottom: 8 }}>
          <Typography.Text strong>data/</Typography.Text>
          <Button size="small" icon={<ReloadOutlined />} onClick={loadRoot}>
            Refresh
          </Button>
        </Space>
        {error && <Alert type="error" showIcon message={error} style={{ marginBottom: 8 }} />}
        {loading ? (
          <Spin />
        ) : treeData.length === 0 ? (
          <Empty description="No content yet" />
        ) : (
          <Tree
            showIcon
            treeData={treeData}
            loadData={async (node) => {
              if (node.isLeaf) return;
              await loadChildren(String(node.key));
            }}
            onSelect={(_, info) => handleSelect(String(info.node.key), !!info.node.isLeaf)}
          />
        )}
      </div>
      <div style={{ flex: 1, minWidth: 0, overflow: 'auto' }}>{renderPreview()}</div>
    </div>
  );
};

/** 把子节点挂到树中对应节点上。 */
function updateChildren(nodes: TreeNode[], parentPath: string, children: TreeNode[]): TreeNode[] {
  return nodes.map((n) => {
    if (String(n.key) === parentPath) {
      return { ...n, children };
    }
    if (n.children && n.children.length > 0) {
      return { ...n, children: updateChildren(n.children as TreeNode[], parentPath, children) };
    }
    return n;
  });
}

export default FilesPanel;
