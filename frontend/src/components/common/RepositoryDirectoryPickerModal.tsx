import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Alert,
  Breadcrumb,
  Button,
  Empty,
  Input,
  List,
  Modal,
  Space,
  Spin,
  Switch,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import {
  ArrowUpOutlined,
  CheckOutlined,
  CloseOutlined,
  EyeOutlined,
  FolderOpenOutlined,
  FolderOutlined,
  PlusOutlined,
  ReloadOutlined,
  RightOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { fetchTree } from '../../api/client';

interface RepositoryDirectoryPickerModalProps {
  open: boolean;
  repoId: string;
  initialPath?: string;
  onClose: () => void;
  onSelect: (path: string) => void;
}

interface DirectoryItem {
  name: string;
  path: string;
  pending: boolean;
}

function normalizeDirectoryPath(value?: string): string {
  if (!value) return '';
  let normalized = value.trim().replace(/\\/g, '/');
  normalized = normalized.replace(/^\.\//, '').replace(/^\/+|\/+$/g, '');
  if (normalized.split('/').some((segment) => segment === '..')) return '';
  return normalized
    .split('/')
    .filter((segment) => segment && segment !== '.')
    .join('/');
}

function parentPath(path: string): string {
  const index = path.lastIndexOf('/');
  return index < 0 ? '' : path.slice(0, index);
}

function baseName(path: string): string {
  const index = path.lastIndexOf('/');
  return index < 0 ? path : path.slice(index + 1);
}

function isPendingPath(path: string, pendingPaths: Set<string>): boolean {
  for (const pendingPath of pendingPaths) {
    if (path === pendingPath || path.startsWith(`${pendingPath}/`)) return true;
  }
  return false;
}

function pendingChildren(path: string, pendingPaths: Set<string>): DirectoryItem[] {
  const children: DirectoryItem[] = [];
  for (const pendingPath of pendingPaths) {
    if (parentPath(pendingPath) === path) {
      children.push({ name: baseName(pendingPath), path: pendingPath, pending: true });
    }
  }
  return children;
}

const RepositoryDirectoryPickerModal: React.FC<RepositoryDirectoryPickerModalProps> = ({
  open,
  repoId,
  initialPath,
  onClose,
  onSelect,
}) => {
  const { t, i18n } = useTranslation();
  const locale = i18n.resolvedLanguage || i18n.language;
  const pendingPathsRef = useRef<Set<string>>(new Set());
  const navigationRequestRef = useRef(0);
  const showHiddenRef = useRef(false);

  const [currentPath, setCurrentPath] = useState('');
  const [showHidden, setShowHidden] = useState(false);
  const [directories, setDirectories] = useState<DirectoryItem[]>([]);
  const [occupiedNames, setOccupiedNames] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [pathWarning, setPathWarning] = useState('');
  const [creating, setCreating] = useState(false);
  const [newDirectoryName, setNewDirectoryName] = useState('');
  const [creatingDirectory, setCreatingDirectory] = useState(false);

  const readDirectory = useCallback(
    async (path: string): Promise<{ items: DirectoryItem[]; names: Set<string> }> => {
      const pendingPaths = pendingPathsRef.current;
      const entries = isPendingPath(path, pendingPaths)
        ? []
        : await fetchTree(repoId, path, true);
      const names = new Set(entries.map((entry) => entry.name));
      const byPath = new Map<string, DirectoryItem>();

      for (const entry of entries) {
        if (
          entry.type === 'directory' &&
          (showHiddenRef.current || !entry.name.startsWith('.'))
        ) {
          byPath.set(entry.path, { name: entry.name, path: entry.path, pending: false });
        }
      }
      for (const item of pendingChildren(path, pendingPaths)) {
        names.add(item.name);
        if (
          (showHiddenRef.current || !item.name.startsWith('.')) &&
          !byPath.has(item.path)
        ) {
          byPath.set(item.path, item);
        }
      }

      const items = Array.from(byPath.values()).sort((a, b) =>
        a.name.localeCompare(b.name, locale)
      );
      return { items, names };
    },
    [locale, repoId]
  );

  const navigate = useCallback(
    async (targetPath: string, fallbackToRoot = false) => {
      const requestID = ++navigationRequestRef.current;
      const normalized = normalizeDirectoryPath(targetPath);
      setLoading(true);
      try {
        const result = await readDirectory(normalized);
        if (requestID !== navigationRequestRef.current) return;
        setCurrentPath(normalized);
        setDirectories(result.items);
        setOccupiedNames(result.names);
        setPathWarning('');
        setCreating(false);
        setNewDirectoryName('');
      } catch (err) {
        if (requestID !== navigationRequestRef.current) return;
        if (fallbackToRoot && normalized) {
          try {
            const root = await readDirectory('');
            if (requestID !== navigationRequestRef.current) return;
            setCurrentPath('');
            setDirectories(root.items);
            setOccupiedNames(root.names);
            setPathWarning(t('adopt.repoPicker.initialPathUnavailable'));
            return;
          } catch {
            // Fall through to the regular error below.
          }
        }
        if (requestID !== navigationRequestRef.current) return;
        setDirectories([]);
        setOccupiedNames(new Set());
        setPathWarning(
          err instanceof Error ? err.message : t('adopt.repoPicker.loadDirectoryFailed')
        );
      } finally {
        if (requestID === navigationRequestRef.current) setLoading(false);
      }
    },
    [readDirectory, t]
  );

  useEffect(() => {
    pendingPathsRef.current.clear();
    navigationRequestRef.current += 1;
    showHiddenRef.current = false;
    setCurrentPath('');
    setShowHidden(false);
    setDirectories([]);
    setOccupiedNames(new Set());
    setPathWarning('');
    setCreating(false);
    setNewDirectoryName('');
    setCreatingDirectory(false);
  }, [repoId]);

  useEffect(() => {
    if (!open) {
      navigationRequestRef.current += 1;
      return;
    }
    void navigate(normalizeDirectoryPath(initialPath), true);
    return () => {
      navigationRequestRef.current += 1;
    };
  }, [initialPath, navigate, open]);

  const breadcrumbs = useMemo(() => {
    const segments = currentPath ? currentPath.split('/') : [];
    return [
      { label: 'data', path: '' },
      ...segments.map((segment, index) => ({
        label: segment,
        path: segments.slice(0, index + 1).join('/'),
      })),
    ];
  }, [currentPath]);

  const handleCreateDirectory = async () => {
    const name = newDirectoryName.trim();
    if (!name || name === '.' || name === '..' || /[\\/\0]/.test(name)) {
      message.warning(t('adopt.repoPicker.invalidDirectoryName'));
      return;
    }
    if (occupiedNames.has(name)) {
      message.warning(t('adopt.repoPicker.directoryExists', { name }));
      return;
    }

    const path = currentPath ? `${currentPath}/${name}` : name;
    setCreatingDirectory(true);
    try {
      pendingPathsRef.current.add(path);
      await navigate(path);
    } finally {
      setCreatingDirectory(false);
    }
  };

  const handleToggleHidden = (checked: boolean) => {
    showHiddenRef.current = checked;
    setShowHidden(checked);
    void navigate(currentPath);
  };

  const handleConfirm = () => {
    if (loading || creatingDirectory) return;
    onSelect(currentPath);
    onClose();
  };

  const currentPathLabel = currentPath ? `data/${currentPath}` : 'data/';
  const currentPathPending = isPendingPath(currentPath, pendingPathsRef.current);

  return (
    <Modal
      title={t('adopt.repoPicker.title')}
      open={open}
      onCancel={onClose}
      width={680}
      footer={
        <Space>
          <Button onClick={onClose}>{t('adopt.action.cancel')}</Button>
          <Button
            type="primary"
            disabled={loading || creatingDirectory}
            onClick={handleConfirm}
          >
            {t('adopt.repoPicker.selectCurrent')}
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Space wrap style={{ width: '100%', justifyContent: 'space-between' }}>
          <Space size="small" wrap>
            <Tooltip title={t('adopt.repoPicker.parentDirectory')}>
              <Button
                icon={<ArrowUpOutlined />}
                disabled={!currentPath || loading}
                onClick={() => void navigate(parentPath(currentPath))}
              />
            </Tooltip>
            <Tooltip title={t('adopt.repoPicker.reload')}>
              <Button
                icon={<ReloadOutlined />}
                disabled={loading}
                onClick={() => void navigate(currentPath)}
              />
            </Tooltip>
            <Button
              icon={<PlusOutlined />}
              disabled={loading}
              onClick={() => setCreating(true)}
            >
              {t('adopt.repoPicker.newDirectory')}
            </Button>
          </Space>
          <Space size="small" wrap>
            <Switch
              checked={showHidden}
              onChange={handleToggleHidden}
              checkedChildren={<EyeOutlined />}
              unCheckedChildren={<EyeOutlined />}
              size="small"
            />
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {t('picker.showHidden')}
            </Typography.Text>
            {currentPathPending && <Tag color="gold">{t('adopt.repoPicker.pending')}</Tag>}
          </Space>
        </Space>

        <Breadcrumb
          items={breadcrumbs.map((item, index) => ({
            title:
              index === breadcrumbs.length - 1 ? (
                <Typography.Text strong>{item.label}</Typography.Text>
              ) : (
                <Button
                  type="link"
                  size="small"
                  style={{ height: 'auto', padding: 0 }}
                  disabled={loading}
                  onClick={() => void navigate(item.path)}
                >
                  {item.label}
                </Button>
              ),
          }))}
        />

        <Typography.Text code>{currentPathLabel}</Typography.Text>

        {creating && (
          <Space.Compact style={{ width: '100%' }}>
            <Input
              autoFocus
              value={newDirectoryName}
              placeholder={t('adopt.repoPicker.newDirectoryPlaceholder')}
              onChange={(event) => setNewDirectoryName(event.target.value)}
              onPressEnter={() => void handleCreateDirectory()}
            />
            <Tooltip title={t('adopt.repoPicker.confirmNewDirectory')}>
              <Button
                icon={<CheckOutlined />}
                loading={creatingDirectory}
                onClick={() => void handleCreateDirectory()}
              />
            </Tooltip>
            <Tooltip title={t('adopt.repoPicker.cancelNewDirectory')}>
              <Button
                icon={<CloseOutlined />}
                onClick={() => {
                  setCreating(false);
                  setNewDirectoryName('');
                }}
              />
            </Tooltip>
          </Space.Compact>
        )}

        {pathWarning && <Alert type="warning" showIcon message={pathWarning} />}

        <div
          style={{
            border: '1px solid #d9d9d9',
            borderRadius: 6,
            minHeight: 300,
            maxHeight: 400,
            overflow: 'auto',
          }}
        >
          {loading ? (
            <div style={{ padding: 56, textAlign: 'center' }}>
              <Spin />
            </div>
          ) : directories.length === 0 ? (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={t('adopt.repoPicker.emptyDirectory')}
              style={{ marginBlock: 72 }}
            />
          ) : (
            <List
              dataSource={directories}
              renderItem={(item) => (
                <List.Item style={{ paddingInline: 12 }}>
                  <Button
                    type="text"
                    icon={item.pending ? <FolderOpenOutlined /> : <FolderOutlined />}
                    onClick={() => void navigate(item.path)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'flex-start',
                      minWidth: 0,
                      flex: 1,
                      textAlign: 'start',
                    }}
                  >
                    <span
                      style={{ display: 'block', overflow: 'hidden', textOverflow: 'ellipsis' }}
                    >
                      {item.name}
                    </span>
                  </Button>
                  <Space size="small">
                    {item.pending && <Tag color="gold">{t('adopt.repoPicker.pending')}</Tag>}
                    <Tooltip title={t('adopt.repoPicker.openDirectory')}>
                      <Button
                        type="text"
                        icon={<RightOutlined />}
                        onClick={() => void navigate(item.path)}
                      />
                    </Tooltip>
                  </Space>
                </List.Item>
              )}
            />
          )}
        </div>
      </Space>
    </Modal>
  );
};

export default RepositoryDirectoryPickerModal;
