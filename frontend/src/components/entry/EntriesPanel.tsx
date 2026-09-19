import React, { useEffect, useState } from 'react';
import {
  Button,
  Space,
  Typography,
  Tag,
  Collapse,
  Modal,
  Radio,
  Empty,
  Alert,
  message,
  Tooltip,
  Badge,
  Checkbox,
  Input,
  List,
  Popconfirm,
} from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  LinkOutlined,
  DeleteOutlined,
  ToolOutlined,
  CloudDownloadOutlined,
  SafetyCertificateOutlined,
  ImportOutlined,
  DisconnectOutlined,
  DesktopOutlined,
  FolderOpenOutlined,
} from '@ant-design/icons';
import { Trans, useTranslation } from 'react-i18next';
import { useAppStore } from '../../store/appStore';
import type { Entry, Link, LinkState, ApplyResult, AdoptRequest, DetachMode } from '../../types';
import AdoptModal from './AdoptModal';
import DirectoryPickerModal from '../common/DirectoryPickerModal';

const { Text, Paragraph } = Typography;

function entryBaseName(repoPath?: string): string {
  if (!repoPath) return '';
  return (
    repoPath.replace(/\\/g, '/').replace(/\/+$/, '').split('/').filter(Boolean).pop() || ''
  );
}

// Backslash is a legal POSIX filename character, so only normalize clear Windows paths.
function isWindowsLocalPath(value: string): boolean {
  return /^[A-Za-z]:[\\/]/.test(value) || value.startsWith('\\\\');
}

function joinLocalPath(parent: string, name: string): string {
  const value = parent.trim();
  if (!value) return name;
  const windowsPath = isWindowsLocalPath(value);
  const separator = windowsPath ? '\\' : '/';
  const normalized = windowsPath ? value.replace(/\//g, '\\') : value;
  const withoutTrailingSeparators = windowsPath
    ? normalized.replace(/\\+$/, '')
    : normalized.replace(/\/+$/, '');
  if (!withoutTrailingSeparators && separator === '/') return `/${name}`;
  return `${withoutTrailingSeparators}${separator}${name}`;
}

function localParentPath(path?: string): string | undefined {
  if (!path) return undefined;
  const raw = path.trim();
  const windowsPath = isWindowsLocalPath(raw);
  const normalized = windowsPath ? raw.replace(/\//g, '\\') : raw;
  const value = windowsPath
    ? normalized.replace(/\\+$/, '')
    : normalized.replace(/\/+$/, '');
  if (!value) return '/';
  const lastSeparator = windowsPath ? value.lastIndexOf('\\') : value.lastIndexOf('/');
  if (lastSeparator < 0) return undefined;
  if (lastSeparator === 0) return value.slice(0, 1);
  if (windowsPath && /^[A-Za-z]:/.test(value) && lastSeparator === 2) {
    return value.slice(0, 3);
  }
  return value.slice(0, lastSeparator);
}

/** 链接状态 → 展示样式。 */
const STATE_META: Record<LinkState, { color: string; labelKey: string }> = {
  ok: { color: 'green', labelKey: 'entries.state.ok' },
  missing: { color: 'gold', labelKey: 'entries.state.missing' },
  wrong_target: { color: 'orange', labelKey: 'entries.state.wrongTarget' },
  replaced: { color: 'volcano', labelKey: 'entries.state.replaced' },
  dangling: { color: 'red', labelKey: 'entries.state.dangling' },
  occupied: { color: 'red', labelKey: 'entries.state.occupied' },
  disabled: { color: 'default', labelKey: 'entries.state.disabled' },
  not_current: { color: 'default', labelKey: 'entries.state.otherDevice' },
};

/** 可以就地修复的状态。 */
const REPAIRABLE: LinkState[] = ['missing', 'wrong_target'];

interface EntriesPanelProps {
  repoId: string;
}

/**
 * 条目与链接面板。
 *
 * 每个被备份的文件/目录是一个条目，可以有 0..N 条链接。
 * 所有链接完全等价 —— 都是指向 data/<repo_path> 的软链接，
 * 因此通过任何一条编辑都等于编辑被备份对象本身。
 */
const EntriesPanel: React.FC<EntriesPanelProps> = ({ repoId }) => {
  const { t } = useTranslation();
  const entries = useAppStore((s) => s.entries);
  const devices = useAppStore((s) => s.devices);
  const currentDevice = useAppStore((s) => s.currentDevice);
  const fetchEntries = useAppStore((s) => s.fetchEntries);
  const fetchDevices = useAppStore((s) => s.fetchDevices);
  const fetchCurrentDevice = useAppStore((s) => s.fetchCurrentDevice);
  const adoptEntry = useAppStore((s) => s.adoptEntry);
  const addLink = useAppStore((s) => s.addLink);
  const bulkLink = useAppStore((s) => s.bulkLink);
  const repairLink = useAppStore((s) => s.repairLink);
  const readoptLink = useAppStore((s) => s.readoptLink);
  const detachDevice = useAppStore((s) => s.detachDevice);
  const removeLink = useAppStore((s) => s.removeLink);
  const removeEntry = useAppStore((s) => s.removeEntry);
  const applyDevice = useAppStore((s) => s.applyDevice);
  const registerDevice = useAppStore((s) => s.registerDevice);
  const renameDevice = useAppStore((s) => s.renameDevice);
  const deleteDevice = useAppStore((s) => s.deleteDevice);
  const audit = useAppStore((s) => s.audit);
  const auditLoading = useAppStore((s) => s.auditLoading);
  const runAudit = useAppStore((s) => s.runAudit);
  const repairConsistency = useAppStore((s) => s.repairConsistency);
  const loading = useAppStore((s) => s.loading);

  const [adoptOpen, setAdoptOpen] = useState(false);
  const [addLinkFor, setAddLinkFor] = useState<Entry | null>(null);
  const [addLinkPath, setAddLinkPath] = useState('');
  const [addLinkPickerOpen, setAddLinkPickerOpen] = useState(false);
  const [removeEntryFor, setRemoveEntryFor] = useState<Entry | null>(null);
  const [removeMode, setRemoveMode] = useState<'unlink' | 'move_back' | 'purge'>('unlink');
  const [plan, setPlan] = useState<ApplyResult | null>(null);
  const [applying, setApplying] = useState(false);
  const [auditOpen, setAuditOpen] = useState(false);
  const [detachOpen, setDetachOpen] = useState(false);
  const [detachMode, setDetachMode] = useState<DetachMode>('unlink');
  const [devicesOpen, setDevicesOpen] = useState(false);
  const [bulkOpen, setBulkOpen] = useState(false);
  const [bulkRoot, setBulkRoot] = useState('');
  const [bulkSelected, setBulkSelected] = useState<string[]>([]);
  const [bulkSubmitting, setBulkSubmitting] = useState(false);
  const [rootPickerOpen, setRootPickerOpen] = useState(false);
  const [purgeConfirmText, setPurgeConfirmText] = useState('');

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const info = await fetchCurrentDevice();
      const [deviceList] = await Promise.all([fetchDevices(repoId), fetchEntries(repoId)]);
      // FR-26：打开仓库时自动登记本机设备（幂等）
      if (!cancelled && info && !deviceList.some((d) => d.fingerprint === info.fingerprint)) {
        await registerDevice(repoId, info.hostname);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [repoId, fetchCurrentDevice, fetchEntries, fetchDevices, registerDevice]);

  useEffect(() => {
    setAddLinkFor(null);
    setAddLinkPath('');
    setAddLinkPickerOpen(false);
  }, [repoId]);

  const refresh = () => {
    fetchEntries(repoId);
    fetchDevices(repoId);
  };

  const fingerprint = currentDevice?.fingerprint || '';
  const planHasActions = !!plan && (plan.created.length > 0 || plan.repaired.length > 0);

  const handleAdopt = async (req: AdoptRequest) => {
    await adoptEntry(repoId, req);
    message.success(t('entries.message.entryCreated'));
  };

  const handleAddLink = async () => {
    if (!addLinkFor || !addLinkPath.trim()) return;
    try {
      await addLink(repoId, addLinkFor.id, { local_path: addLinkPath.trim() });
      message.success(t('entries.message.linkAdded'));
      setAddLinkFor(null);
      setAddLinkPath('');
      setAddLinkPickerOpen(false);
    } catch (err) {
      message.error(err instanceof Error ? err.message : t('entries.error.addLink'));
    }
  };

  // 先 dry-run 拿计划，确认后再执行 —— apply 从不覆盖已占用的路径
  const handlePlan = async () => {
    try {
      setPlan(await applyDevice(repoId, fingerprint, true));
    } catch (err) {
      message.error(err instanceof Error ? err.message : t('entries.error.buildPlan'));
    }
  };

  const handleApply = async () => {
    setApplying(true);
    try {
      const result = await applyDevice(repoId, fingerprint, false);
      setPlan(null);
      const n = result.created.length + result.repaired.length;
      message.success(
        n > 0 ? t('entries.message.applied', { count: n }) : t('entries.message.alreadyConverged')
      );
    } catch (err) {
      message.error(err instanceof Error ? err.message : t('entries.error.apply'));
    } finally {
      setApplying(false);
    }
  };

  // 巡检：先出结论再决定是否修复
  const handleAudit = async () => {
    try {
      await runAudit(repoId);
      setAuditOpen(true);
    } catch (err) {
      message.error(err instanceof Error ? err.message : t('entries.error.audit'));
    }
  };

  const handleRepair = async () => {
    try {
      const res = await repairConsistency(repoId);
      message.success(
        res.remaining_errors > 0
          ? t('entries.message.repairedWithErrors', {
              count: res.remaining_errors,
              repairedCount: res.repaired_count,
            })
          : t('entries.message.repaired', { count: res.repaired_count })
      );
      refresh();
    } catch (err) {
      message.error(err instanceof Error ? err.message : t('entries.error.repair'));
    }
  };

  // 卸载本机：unlink 删除本机软链接，keep 只停止管理
  const handleDetach = async () => {
    try {
      const res = await detachDevice(repoId, fingerprint, detachMode);
      message.success(
        detachMode === 'keep'
          ? t('entries.message.stoppedManaging')
          : t('entries.message.detached', { count: res.removed.length })
      );
      setDetachOpen(false);
    } catch (err) {
      message.error(err instanceof Error ? err.message : t('entries.error.detach'));
    }
  };

  const handleRemoveEntry = async () => {
    if (!removeEntryFor) return;
    try {
      await removeEntry(repoId, removeEntryFor.id, removeMode);
      message.success(t('entries.message.entryRemoved'));
      setRemoveEntryFor(null);
      setRemoveMode('unlink');
      setPurgeConfirmText('');
    } catch (err) {
      message.error(err instanceof Error ? err.message : t('entries.error.removeEntry'));
    }
  };

  // 打开设备列表时重新拉取一次，链接计数才是最新的
  const openDevices = async () => {
    setDevicesOpen(true);
    await fetchDevices(repoId);
  };

  // FR-9：把一个本地根目录下的多个条目一次性链接到本机
  const openBulkLink = () => {
    setBulkSelected(entries.map((e) => e.id));
    setBulkRoot('');
    setBulkOpen(true);
  };

  const handleBulkLink = async () => {
    if (!bulkRoot.trim() || bulkSelected.length === 0) return;
    setBulkSubmitting(true);
    try {
      const created = await bulkLink(repoId, {
        local_root: bulkRoot.trim(),
        entry_ids: bulkSelected,
      });
      message.success(t('entries.message.linksCreated', { count: created.length }));
      setBulkOpen(false);
      setBulkRoot('');
      setBulkSelected([]);
    } catch (err) {
      message.error(err instanceof Error ? err.message : t('entries.error.createLinks'));
      // 批量创建可能部分成功，刷新真实状态
      refresh();
    } finally {
      setBulkSubmitting(false);
    }
  };

  const purgeTarget = removeEntryFor?.repo_path || '';
  const otherDeviceLinks = (removeEntryFor?.links || []).filter((l) => !l.is_current).length;

  const renderLink = (entry: Entry, link: Link) => {
    const meta = STATE_META[link.state];
    return (
      <div
        key={link.id}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          padding: '6px 0',
          borderTop: '1px solid #f0f0f0',
        }}
      >
        <Tag color={meta.color} style={{ margin: 0 }}>
          {t(meta.labelKey)}
        </Tag>
        <Tooltip title={link.device}>
          <Tag style={{ margin: 0 }}>{link.device_name || link.device.slice(0, 8)}</Tag>
        </Tooltip>
        <Text code style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {link.local_path}
        </Text>
        {link.state_note && (
          <Tooltip title={link.state_note}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              ?
            </Text>
          </Tooltip>
        )}
        <Space size={4}>
          {REPAIRABLE.includes(link.state) && link.is_current && (
            <Tooltip title={t('entries.tooltip.recreateLink')}>
              <Button
                size="small"
                type="text"
                icon={<ToolOutlined />}
                onClick={() => repairLink(repoId, entry.id, link.id)}
              />
            </Tooltip>
          )}
          {link.state === 'replaced' && link.is_current && (
            <Tooltip title={t('entries.tooltip.reAdopt')}>
              <Button
                size="small"
                type="text"
                icon={<ImportOutlined />}
                onClick={() => readoptLink(repoId, entry.id, link.id)}
              >
                {t('entries.action.reAdopt')}
              </Button>
            </Tooltip>
          )}
          {link.is_current && (
            <Tooltip title={t('entries.tooltip.removeLink')}>
              <Button
                size="small"
                type="text"
                danger
                icon={<DeleteOutlined />}
                onClick={() => removeLink(repoId, entry.id, link.id)}
              />
            </Tooltip>
          )}
        </Space>
      </div>
    );
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        <Badge status="processing" />
        <Text strong>
          {currentDevice?.name || currentDevice?.hostname || t('entries.thisDevice')}
        </Text>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {t('entries.deviceCount', { count: devices.length })}
        </Text>
        <Button icon={<ReloadOutlined />} onClick={refresh} loading={loading}>
          {t('entries.action.refresh')}
        </Button>
        <Button icon={<PlusOutlined />} type="primary" onClick={() => setAdoptOpen(true)}>
          {t('entries.action.newEntry')}
        </Button>
        <Button icon={<LinkOutlined />} onClick={openBulkLink}>
          {t('entries.action.bulkLink')}
        </Button>
        <Button icon={<DesktopOutlined />} onClick={openDevices}>
          {t('entries.action.devices')}
        </Button>
        <Button
          icon={<CloudDownloadOutlined />}
          onClick={handlePlan}
          disabled={!fingerprint}
        >
          {t('entries.action.apply')}
        </Button>
        <Button
          icon={<SafetyCertificateOutlined />}
          onClick={handleAudit}
          loading={auditLoading}
          danger={!!audit && audit.errors > 0}
        >
          {t('entries.action.audit')}
          {audit && (audit.errors > 0 || audit.warnings > 0) && (
            <Tag
              color={audit.errors > 0 ? 'red' : 'gold'}
              style={{ marginInlineStart: 6, marginInlineEnd: 0 }}
            >
              {audit.errors + audit.warnings}
            </Tag>
          )}
        </Button>
        <Button
          icon={<DisconnectOutlined />}
          onClick={() => setDetachOpen(true)}
          disabled={!fingerprint}
        >
          {t('entries.action.detach')}
        </Button>
      </Space>

      {entries.length === 0 ? (
        <Empty description={t('entries.empty')} />
      ) : (
        <Collapse
          accordion={false}
          items={entries.map((entry) => ({
            key: entry.id,
            label: (
              <Space wrap>
                <Text strong>{entry.repo_path}</Text>
                <Tag>{t(`entries.kind.${entry.kind}`)}</Tag>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {t('entries.linkCount', { count: entry.links.length })}
                </Text>
              </Space>
            ),
            extra: (
              <Button
                size="small"
                danger
                type="text"
                icon={<DeleteOutlined />}
                onClick={(e) => {
                  e.stopPropagation();
                  setRemoveEntryFor(entry);
                }}
              >
                {t('entries.action.remove')}
              </Button>
            ),
            children: (
              <div>
                {entry.links.length === 0 ? (
                  <Paragraph type="secondary" style={{ margin: 0 }}>
                    {t('entries.noLinks')}
                  </Paragraph>
                ) : (
                  entry.links.map((l) => renderLink(entry, l))
                )}
                <Button
                  size="small"
                  type="dashed"
                  icon={<LinkOutlined />}
                  style={{ marginTop: 12 }}
                  onClick={() => {
                    setAddLinkFor(entry);
                    setAddLinkPath('');
                    setAddLinkPickerOpen(false);
                  }}
                >
                  {t('entries.action.addLink')}
                </Button>
              </div>
            ),
          }))}
        />
      )}

      <AdoptModal
        key={repoId}
        open={adoptOpen}
        repoId={repoId}
        onClose={() => setAdoptOpen(false)}
        onSubmit={handleAdopt}
      />

      {/* 添加链接：可填写完整路径，或选择父目录后自动追加条目名称。 */}
      <Modal
        title={t('entries.addLink.title', { repoPath: addLinkFor?.repo_path || '' })}
        open={!!addLinkFor}
        onCancel={() => {
          setAddLinkFor(null);
          setAddLinkPickerOpen(false);
        }}
        onOk={handleAddLink}
        okText={t('entries.action.add')}
        cancelText={t('entries.action.cancel')}
        okButtonProps={{ disabled: !addLinkPath.trim() }}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={t('entries.addLink.info')}
        />
        <Text>{t('entries.addLink.localPath')}</Text>
        <Input
          style={{ marginTop: 4 }}
          placeholder={t('entries.addLink.placeholder', {
            name: entryBaseName(addLinkFor?.repo_path) || 'notes.txt',
          })}
          value={addLinkPath}
          onChange={(e) => setAddLinkPath(e.target.value)}
          onPressEnter={() => void handleAddLink()}
          addonAfter={
            <Tooltip title={t('entries.addLink.chooseParent')}>
              <Button
                type="text"
                size="small"
                icon={<FolderOpenOutlined />}
                aria-label={t('entries.addLink.chooseParent')}
                onClick={() => setAddLinkPickerOpen(true)}
              />
            </Tooltip>
          }
        />
        <Text type="secondary" style={{ display: 'block', marginTop: 6, fontSize: 12 }}>
          {t('entries.addLink.pathHelp')}
        </Text>

        <DirectoryPickerModal
          open={addLinkPickerOpen}
          mode="directory"
          title={t('entries.addLink.pickerTitle')}
          initialPath={localParentPath(addLinkPath)}
          onClose={() => setAddLinkPickerOpen(false)}
          onSelect={(parent) => {
            const name = entryBaseName(addLinkFor?.repo_path);
            if (!name) return;
            setAddLinkPath(joinLocalPath(parent, name));
            setAddLinkPickerOpen(false);
          }}
        />
      </Modal>

      {/* 移除条目 */}
      <Modal
        title={t('entries.remove.title', { repoPath: removeEntryFor?.repo_path || '' })}
        open={!!removeEntryFor}
        onCancel={() => {
          setRemoveEntryFor(null);
          setPurgeConfirmText('');
        }}
        onOk={handleRemoveEntry}
        okText={t('entries.action.remove')}
        cancelText={t('entries.action.cancel')}
        okButtonProps={{
          danger: true,
          disabled: removeMode === 'purge' && purgeConfirmText.trim() !== purgeTarget,
        }}
      >
        <Radio.Group
          value={removeMode}
          onChange={(e) => {
            setRemoveMode(e.target.value);
            setPurgeConfirmText('');
          }}
        >
          <Space direction="vertical">
            <Radio value="unlink">{t('entries.remove.mode.unlink')}</Radio>
            <Radio value="move_back">{t('entries.remove.mode.moveBack')}</Radio>
            <Radio value="purge">{t('entries.remove.mode.purge')}</Radio>
          </Space>
        </Radio.Group>

        {/* NFR-4：删除内容必须键入 repo_path 二次确认 */}
        {removeMode === 'purge' && (
          <div style={{ marginTop: 16 }}>
            <Alert
              type="error"
              showIcon
              style={{ marginBottom: 12 }}
              message={t('entries.remove.purgeWarning')}
              description={
                otherDeviceLinks > 0
                  ? t('entries.remove.otherDeviceLinks', { count: otherDeviceLinks })
                  : undefined
              }
            />
            <Text type="secondary" style={{ fontSize: 12 }}>
              <Trans
                i18nKey="entries.remove.confirm"
                values={{ repoPath: purgeTarget }}
                components={{ code: <Text code /> }}
              />
            </Text>
            <Input
              style={{ marginTop: 8 }}
              placeholder={purgeTarget}
              value={purgeConfirmText}
              onChange={(e) => setPurgeConfirmText(e.target.value)}
            />
          </div>
        )}
      </Modal>

      {/* 一致性巡检结论 */}
      <Modal
        title={t('entries.audit.title')}
        open={auditOpen}
        onCancel={() => setAuditOpen(false)}
        width={720}
        footer={[
          <Button key="close" onClick={() => setAuditOpen(false)}>
            {t('entries.action.close')}
          </Button>,
          <Button
            key="repair"
            type="primary"
            loading={auditLoading}
            disabled={!audit || (audit.findings ?? []).length === 0}
            onClick={handleRepair}
          >
            {t('entries.action.repair')}
          </Button>,
        ]}
      >
        {!audit || audit.findings.length === 0 ? (
          <Alert type="success" showIcon message={t('entries.audit.noInconsistency')} />
        ) : (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Text>
              <Text type="danger">
                {t('entries.audit.errorCount', { count: audit.errors })}
              </Text>
              {' · '}
              <Text type="warning">
                {t('entries.audit.warningCount', { count: audit.warnings })}
              </Text>
              {' · '}
              <Text type="secondary">
                {t('entries.audit.entryCount', { count: audit.entry_count })}
                {' · '}
                {t('entries.audit.linkCount', { count: audit.link_count })}
              </Text>
            </Text>
            {(audit.findings ?? []).map((f, i) => (
              <div
                key={`${f.code}-${f.link_id || f.repo_path || i}`}
                style={{ borderTop: '1px solid #f0f0f0', paddingTop: 8 }}
              >
                <Space wrap size={4}>
                  <Tag color={f.severity === 'error' ? 'red' : 'gold'} style={{ margin: 0 }}>
                    {t(`entries.severity.${f.severity}`)}
                  </Tag>
                  <Text code>{f.code}</Text>
                  {f.repairable && (
                    <Tag color="blue">{t('entries.audit.repairable')}</Tag>
                  )}
                </Space>
                <div>
                  <Text>{f.message}</Text>
                </div>
                {f.repo_path && (
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {t('entries.audit.repoPath', { path: f.repo_path })}
                  </Text>
                )}
                {f.local_path && (
                  <div>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {f.local_path}
                    </Text>
                  </div>
                )}
              </div>
            ))}
          </Space>
        )}
      </Modal>

      {/* 卸载本机 */}
      <Modal
        title={t('entries.detach.title', {
          device: currentDevice?.name || t('entries.thisDevice'),
        })}
        open={detachOpen}
        onCancel={() => setDetachOpen(false)}
        onOk={handleDetach}
        okText={t('entries.action.detach')}
        cancelText={t('entries.action.cancel')}
        okButtonProps={{ danger: detachMode === 'unlink' }}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={t('entries.detach.info')}
        />
        <Radio.Group value={detachMode} onChange={(e) => setDetachMode(e.target.value)}>
          <Space direction="vertical">
            <Radio value="unlink">{t('entries.detach.unlink')}</Radio>
            <Radio value="keep">{t('entries.detach.keep')}</Radio>
          </Space>
        </Radio.Group>
      </Modal>

      {/* apply 计划确认 */}
      <Modal
        title={t('entries.apply.title')}
        open={!!plan}
        onCancel={() => setPlan(null)}
        onOk={planHasActions ? handleApply : () => setPlan(null)}
        confirmLoading={applying}
        okText={t(planHasActions ? 'entries.action.apply' : 'entries.action.close')}
        cancelText={t('entries.action.cancel')}
        cancelButtonProps={{ style: planHasActions ? undefined : { display: 'none' } }}
      >
        {plan && (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Text>
              <Trans
                i18nKey="entries.apply.summary"
                values={{
                  created: plan.created.length,
                  repaired: plan.repaired.length,
                  skipped: plan.skipped.length,
                  conflicts: plan.conflicts.length,
                  orphans: plan.orphans.length,
                }}
                components={{ strong: <b /> }}
              />
            </Text>
            {plan.conflicts.map((a) => (
              <Text key={a.link_id} type="danger" style={{ fontSize: 12 }}>
                {t('entries.apply.conflict', { path: a.local_path, reason: a.reason })}
              </Text>
            ))}
            {plan.orphans.map((a) => (
              <Text key={a.link_id} type="warning" style={{ fontSize: 12 }}>
                {t('entries.apply.orphan', { path: a.repo_path })}
              </Text>
            ))}
            <Text type="secondary" style={{ fontSize: 12 }}>
              {t('entries.apply.note')}
            </Text>
          </Space>
        )}
      </Modal>

      {/* 设备管理（FR-26 / FR-29） */}
      <Modal
        title={t('entries.devices.title')}
        open={devicesOpen}
        onCancel={() => setDevicesOpen(false)}
        footer={
          <Button onClick={() => setDevicesOpen(false)}>{t('entries.action.close')}</Button>
        }
        width={620}
      >
        <Text type="secondary" style={{ fontSize: 12 }}>
          {t('entries.devices.description')}
        </Text>
        {!devices.some((d) => d.is_current) && (
          <Alert
            type="info"
            showIcon
            style={{ margin: '12px 0' }}
            message={t('entries.devices.unregistered')}
            action={
              <Button
                size="small"
                type="primary"
                onClick={() => registerDevice(repoId, currentDevice?.hostname)}
              >
                {t('entries.action.register')}
              </Button>
            }
          />
        )}
        <List
          size="small"
          style={{ marginTop: 12 }}
          dataSource={devices}
          locale={{ emptyText: t('entries.devices.empty') }}
          renderItem={(d) => (
            <List.Item
              actions={
                d.is_current
                  ? []
                  : [
                      <Popconfirm
                        key="delete"
                        title={t('entries.devices.deleteTitle')}
                        description={t('entries.devices.deleteDescription')}
                        okText={t('entries.action.delete')}
                        cancelText={t('entries.action.cancel')}
                        okButtonProps={{ danger: true }}
                        onConfirm={() => deleteDevice(repoId, d.fingerprint)}
                      >
                        <Button size="small" type="text" danger icon={<DeleteOutlined />} />
                      </Popconfirm>,
                    ]
              }
            >
              <List.Item.Meta
                avatar={
                  <DesktopOutlined
                    style={{ fontSize: 18, color: d.is_current ? '#1677ff' : '#999' }}
                  />
                }
                title={
                  <Space wrap size={4}>
                    <Text
                      strong
                      editable={{
                        tooltip: t('entries.action.rename'),
                        onChange: (value) => {
                          const name = value.trim();
                          if (name && name !== d.name) {
                            renameDevice(repoId, d.fingerprint, name);
                          }
                        },
                      }}
                    >
                      {d.name || d.hostname || d.fingerprint.slice(0, 8)}
                    </Text>
                    {d.is_current && (
                      <Tag color="blue" style={{ margin: 0 }}>
                        {t('entries.devices.current')}
                      </Tag>
                    )}
                  </Space>
                }
                description={
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {t('entries.linkCount', { count: d.link_count })}
                    {d.hostname ? ` · ${d.hostname}` : ''}
                    {d.os ? ` · ${d.os}` : ''}
                  </Text>
                }
              />
            </List.Item>
          )}
        />
      </Modal>

      {/* 批量链接（FR-9）：把多个条目一次性挂到某个本地根目录下 */}
      <Modal
        title={t('entries.bulk.title')}
        open={bulkOpen}
        onCancel={() => setBulkOpen(false)}
        onOk={handleBulkLink}
        okText={t('entries.action.createLinks')}
        cancelText={t('entries.action.cancel')}
        confirmLoading={bulkSubmitting}
        okButtonProps={{ disabled: !bulkRoot.trim() || bulkSelected.length === 0 }}
        width={640}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={t('entries.bulk.info')}
        />
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <div>
            <Text strong>{t('entries.bulk.localRoot')}</Text>
            <Input
              style={{ marginTop: 4 }}
              placeholder={t('entries.bulk.rootPlaceholder')}
              value={bulkRoot}
              onChange={(e) => setBulkRoot(e.target.value)}
              addonAfter={
                <Button
                  type="text"
                  size="small"
                  icon={<FolderOpenOutlined />}
                  onClick={() => setRootPickerOpen(true)}
                />
              }
            />
          </div>
          <div>
            <Space style={{ marginBottom: 4 }}>
              <Text strong>{t('entries.bulk.entries')}</Text>
              <Button
                size="small"
                type="link"
                onClick={() => setBulkSelected(entries.map((e) => e.id))}
              >
                {t('entries.action.selectAll')}
              </Button>
              <Button size="small" type="link" onClick={() => setBulkSelected([])}>
                {t('entries.action.clear')}
              </Button>
            </Space>
            <Checkbox.Group
              style={{ display: 'flex', flexDirection: 'column', gap: 6 }}
              value={bulkSelected}
              onChange={(values) => setBulkSelected(values as string[])}
              options={entries.map((e) => ({
                value: e.id,
                label: (
                  <Space size={4}>
                    <Text code style={{ fontSize: 12 }}>
                      {bulkRoot.trim()
                        ? `${bulkRoot.replace(/\/+$/, '')}/${e.repo_path}`
                        : e.repo_path}
                    </Text>
                    <Tag style={{ margin: 0 }}>{t(`entries.kind.${e.kind}`)}</Tag>
                  </Space>
                ),
              }))}
            />
          </div>
        </Space>

        {/* Nested to inherit the parent Modal's z-index context. */}
        <DirectoryPickerModal
          open={rootPickerOpen}
          mode="directory"
          title={t('entries.bulk.pickerTitle')}
          onClose={() => setRootPickerOpen(false)}
          onSelect={(path) => setBulkRoot(path)}
        />
      </Modal>
    </div>
  );
};

export default EntriesPanel;
