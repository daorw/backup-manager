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
} from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  LinkOutlined,
  DeleteOutlined,
  ToolOutlined,
  SwapOutlined,
  CloudDownloadOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons';
import { useAppStore } from '../../store/appStore';
import type { Entry, Link, LinkState, ApplyResult, AdoptRequest } from '../../types';
import AdoptModal from './AdoptModal';

const { Text, Paragraph } = Typography;

/** 链接状态 → 展示样式。 */
const STATE_META: Record<LinkState, { color: string; label: string }> = {
  ok: { color: 'green', label: 'ok' },
  missing: { color: 'gold', label: 'missing' },
  wrong_target: { color: 'orange', label: 'wrong target' },
  replaced: { color: 'volcano', label: 'replaced' },
  dangling: { color: 'red', label: 'dangling' },
  occupied: { color: 'red', label: 'occupied' },
  disabled: { color: 'default', label: 'disabled' },
  not_current: { color: 'default', label: 'other device' },
};

/** 可以就地修复的状态。 */
const REPAIRABLE: LinkState[] = ['missing', 'wrong_target'];

interface EntriesPanelProps {
  repoId: string;
}

/**
 * 条目与链接面板。
 *
 * 每个被备份的文件/目录是一个条目：恰好一条 in 链接（跟踪链接）+ 0..N 条 out 链接。
 * in 是 out 的特例，两者物理形态相同，所以「指定新的 in」是纯元数据变更。
 */
const EntriesPanel: React.FC<EntriesPanelProps> = ({ repoId }) => {
  const entries = useAppStore((s) => s.entries);
  const devices = useAppStore((s) => s.devices);
  const currentDevice = useAppStore((s) => s.currentDevice);
  const fetchEntries = useAppStore((s) => s.fetchEntries);
  const fetchDevices = useAppStore((s) => s.fetchDevices);
  const fetchCurrentDevice = useAppStore((s) => s.fetchCurrentDevice);
  const adoptEntry = useAppStore((s) => s.adoptEntry);
  const addLink = useAppStore((s) => s.addLink);
  const switchTrackedLink = useAppStore((s) => s.switchTrackedLink);
  const repairLink = useAppStore((s) => s.repairLink);
  const removeLink = useAppStore((s) => s.removeLink);
  const removeEntry = useAppStore((s) => s.removeEntry);
  const applyDevice = useAppStore((s) => s.applyDevice);
  const audit = useAppStore((s) => s.audit);
  const auditLoading = useAppStore((s) => s.auditLoading);
  const runAudit = useAppStore((s) => s.runAudit);
  const repairConsistency = useAppStore((s) => s.repairConsistency);
  const loading = useAppStore((s) => s.loading);

  const [adoptOpen, setAdoptOpen] = useState(false);
  const [addLinkFor, setAddLinkFor] = useState<Entry | null>(null);
  const [addLinkPath, setAddLinkPath] = useState('');
  const [removeEntryFor, setRemoveEntryFor] = useState<Entry | null>(null);
  const [removeMode, setRemoveMode] = useState<'unlink' | 'move_back' | 'purge'>('unlink');
  const [plan, setPlan] = useState<ApplyResult | null>(null);
  const [applying, setApplying] = useState(false);
  const [auditOpen, setAuditOpen] = useState(false);

  useEffect(() => {
    fetchCurrentDevice();
    fetchEntries(repoId);
    fetchDevices(repoId);
  }, [repoId, fetchCurrentDevice, fetchEntries, fetchDevices]);

  const refresh = () => {
    fetchEntries(repoId);
    fetchDevices(repoId);
  };

  const fingerprint = currentDevice?.fingerprint || '';

  const handleAdopt = async (req: AdoptRequest) => {
    await adoptEntry(repoId, req);
    message.success('Entry created');
  };

  const handleAddLink = async () => {
    if (!addLinkFor || !addLinkPath.trim()) return;
    try {
      await addLink(repoId, addLinkFor.id, { local_path: addLinkPath.trim() });
      message.success('Link added');
      setAddLinkFor(null);
      setAddLinkPath('');
    } catch (err) {
      message.error(err instanceof Error ? err.message : 'Failed to add the link');
    }
  };

  // 先 dry-run 拿计划，确认后再执行 —— apply 从不覆盖已占用的路径
  const handlePlan = async () => {
    try {
      setPlan(await applyDevice(repoId, fingerprint, true));
    } catch (err) {
      message.error(err instanceof Error ? err.message : 'Failed to build the plan');
    }
  };

  const handleApply = async () => {
    setApplying(true);
    try {
      const result = await applyDevice(repoId, fingerprint, false);
      setPlan(null);
      const n = result.created.length + result.repaired.length;
      message.success(n > 0 ? `Applied: ${n} link(s) converged` : 'Already converged');
    } catch (err) {
      message.error(err instanceof Error ? err.message : 'Failed to apply');
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
      message.error(err instanceof Error ? err.message : 'Audit failed');
    }
  };

  const handleRepair = async () => {
    try {
      const res = await repairConsistency(repoId);
      message.success(
        res.remaining_errors > 0
          ? `Repaired ${res.repaired_count}; ${res.remaining_errors} error(s) still need manual resolution`
          : `Repaired ${res.repaired_count}`
      );
      refresh();
    } catch (err) {
      message.error(err instanceof Error ? err.message : 'Repair failed');
    }
  };

  const handleRemoveEntry = async () => {
    if (!removeEntryFor) return;
    try {
      await removeEntry(repoId, removeEntryFor.id, removeMode);
      message.success('Entry removed');
      setRemoveEntryFor(null);
      setRemoveMode('unlink');
    } catch (err) {
      message.error(err instanceof Error ? err.message : 'Failed to remove the entry');
    }
  };

  const renderLink = (entry: Entry, link: Link) => {
    const meta = STATE_META[link.state];
    const isIn = link.type === 'in';
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
        <Tag color={isIn ? 'blue' : 'default'} style={{ margin: 0, width: 46, textAlign: 'center' }}>
          {link.type}
        </Tag>
        <Tag color={meta.color} style={{ margin: 0 }}>
          {meta.label}
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
          {!isIn && link.is_current && (
            <Tooltip title="Designate as the tracked (in) link. Metadata-only — the filesystem is not touched.">
              <Button
                size="small"
                type="text"
                icon={<SwapOutlined />}
                onClick={() => switchTrackedLink(repoId, entry.id, link.id)}
              >
                Set as tracked
              </Button>
            </Tooltip>
          )}
          {REPAIRABLE.includes(link.state) && link.is_current && (
            <Tooltip title="Recreate the local symlink">
              <Button
                size="small"
                type="text"
                icon={<ToolOutlined />}
                onClick={() => repairLink(repoId, entry.id, link.id)}
              />
            </Tooltip>
          )}
          {link.is_current && (
            <Tooltip title="Remove this link. The repository content is kept.">
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

  const unboundCount = entries.filter((e) => e.unbound).length;

  return (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        <Badge status="processing" />
        <Text strong>{currentDevice?.name || currentDevice?.hostname || 'this device'}</Text>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {devices.length} device(s)
        </Text>
        <Button icon={<ReloadOutlined />} onClick={refresh} loading={loading}>
          Refresh
        </Button>
        <Button icon={<PlusOutlined />} type="primary" onClick={() => setAdoptOpen(true)}>
          New Entry
        </Button>
        <Button
          icon={<CloudDownloadOutlined />}
          onClick={handlePlan}
          disabled={!fingerprint}
        >
          Apply
        </Button>
        <Button
          icon={<SafetyCertificateOutlined />}
          onClick={handleAudit}
          loading={auditLoading}
          danger={!!audit && audit.errors > 0}
        >
          Audit
          {audit && (audit.errors > 0 || audit.warnings > 0) && (
            <Tag
              color={audit.errors > 0 ? 'red' : 'gold'}
              style={{ marginInlineStart: 6, marginInlineEnd: 0 }}
            >
              {audit.errors + audit.warnings}
            </Tag>
          )}
        </Button>
      </Space>

      {unboundCount > 0 && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
          message={`${unboundCount} entry(ies) have no in link`}
          description="This is normal on a freshly initialised device. Designate one of their links with “Set as tracked” to bind them."
        />
      )}

      {entries.length === 0 ? (
        <Empty description="No entries yet. Create one to move content into the repository." />
      ) : (
        <Collapse
          accordion={false}
          items={entries.map((entry) => ({
            key: entry.id,
            label: (
              <Space wrap>
                <Text strong>{entry.repo_path}</Text>
                <Tag>{entry.kind}</Tag>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {entry.links.length} link(s)
                </Text>
                {entry.unbound && <Tag color="warning">no in link</Tag>}
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
                Remove
              </Button>
            ),
            children: (
              <div>
                {entry.links.length === 0 ? (
                  <Paragraph type="secondary" style={{ margin: 0 }}>
                    No links. Add one to make this content reachable at a local path.
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
                  }}
                >
                  Add Link
                </Button>
              </div>
            ),
          }))}
        />
      )}

      <AdoptModal
        open={adoptOpen}
        onClose={() => setAdoptOpen(false)}
        onSubmit={handleAdopt}
      />

      {/* 添加 out 链接：目标路径必须不存在，所以用自由输入 */}
      <Modal
        title={`Add Link — ${addLinkFor?.repo_path || ''}`}
        open={!!addLinkFor}
        onCancel={() => setAddLinkFor(null)}
        onOk={handleAddLink}
        okText="Add"
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="A symlink pointing at this entry's content will be created here. Nothing is copied."
        />
        <Text>Local path</Text>
        <input
          className="ant-input"
          style={{ marginTop: 4 }}
          placeholder="/Users/you/Desktop/notes.txt"
          value={addLinkPath}
          onChange={(e) => setAddLinkPath(e.target.value)}
        />
      </Modal>

      {/* 移除条目 */}
      <Modal
        title={`Remove Entry — ${removeEntryFor?.repo_path || ''}`}
        open={!!removeEntryFor}
        onCancel={() => setRemoveEntryFor(null)}
        onOk={handleRemoveEntry}
        okText="Remove"
        okButtonProps={{ danger: true }}
      >
        <Radio.Group value={removeMode} onChange={(e) => setRemoveMode(e.target.value)}>
          <Space direction="vertical">
            <Radio value="unlink">
              Remove this device's links only — the entry and repository content stay
            </Radio>
            <Radio value="move_back">
              Move the content back to a local path, then remove the entry
            </Radio>
            <Radio value="purge">
              Delete the repository content as well — the previous commit can restore it
            </Radio>
          </Space>
        </Radio.Group>
      </Modal>

      {/* 一致性巡检结论 */}
      <Modal
        title="Consistency Audit"
        open={auditOpen}
        onCancel={() => setAuditOpen(false)}
        width={720}
        footer={[
          <Button key="close" onClick={() => setAuditOpen(false)}>
            Close
          </Button>,
          <Button
            key="repair"
            type="primary"
            loading={auditLoading}
            disabled={!audit || (audit.findings ?? []).length === 0}
            onClick={handleRepair}
          >
            Repair
          </Button>,
        ]}
      >
        {!audit || audit.findings.length === 0 ? (
          <Alert type="success" showIcon message="No inconsistency found" />
        ) : (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Text>
              <Text type="danger">{audit.errors} error(s)</Text>
              {' · '}
              <Text type="warning">{audit.warnings} warning(s)</Text>
              {' · '}
              <Text type="secondary">
                {audit.entry_count} entr(ies), {audit.link_count} link(s)
              </Text>
            </Text>
            {(audit.findings ?? []).map((f, i) => (
              <div
                key={`${f.code}-${f.link_id || f.repo_path || i}`}
                style={{ borderTop: '1px solid #f0f0f0', paddingTop: 8 }}
              >
                <Space wrap size={4}>
                  <Tag color={f.severity === 'error' ? 'red' : 'gold'} style={{ margin: 0 }}>
                    {f.severity}
                  </Tag>
                  <Text code>{f.code}</Text>
                  {f.repairable && <Tag color="blue">repairable</Tag>}
                </Space>
                <div>
                  <Text>{f.message}</Text>
                </div>
                {f.repo_path && (
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    repo: {f.repo_path}
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

      {/* apply 计划确认 */}
      <Modal
        title="Apply Plan"
        open={!!plan}
        onCancel={() => setPlan(null)}
        onOk={handleApply}
        confirmLoading={applying}
        okText="Apply"
      >
        {plan && (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Text>
              create <b>{plan.created.length}</b> · repair <b>{plan.repaired.length}</b> · skip{' '}
              <b>{plan.skipped.length}</b> · conflict <b>{plan.conflicts.length}</b> · orphan{' '}
              <b>{plan.orphans.length}</b>
            </Text>
            {plan.conflicts.map((a) => (
              <Text key={a.link_id} type="danger" style={{ fontSize: 12 }}>
                conflict: {a.local_path} — {a.reason}
              </Text>
            ))}
            {plan.orphans.map((a) => (
              <Text key={a.link_id} type="warning" style={{ fontSize: 12 }}>
                orphan: {a.repo_path} — repository content is missing
              </Text>
            ))}
            <Text type="secondary" style={{ fontSize: 12 }}>
              Conflicting paths are reported, never overwritten.
            </Text>
          </Space>
        )}
      </Modal>
    </div>
  );
};

export default EntriesPanel;
