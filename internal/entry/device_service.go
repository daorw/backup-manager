package entry

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"backup-manager/internal/model"
	"backup-manager/internal/util"
)

// CurrentDeviceInfo 返回当前机器的标识信息（指纹由本机派生，无需配置）。
func (s *Service) CurrentDeviceInfo() *CurrentDeviceInfo {
	return &CurrentDeviceInfo{
		Fingerprint: util.MachineFingerprint(),
		Hostname:    util.Hostname(),
		OS:          runtime.GOOS,
		Name:        util.Hostname(),
	}
}

// ListDevices 列出仓库的全部设备。
func (s *Service) ListDevices(repoID string) ([]*DeviceView, error) {
	_, m, err := s.load(repoID)
	if err != nil {
		return nil, err
	}
	return buildDeviceViews(m, util.MachineFingerprint()), nil
}

// RegisterDevice 登记当前设备；已存在则只更新名称与在线时间（幂等）。
func (s *Service) RegisterDevice(repoID, name string) (*DeviceView, error) {
	defer s.lock(repoID)()
	repo, m, err := s.load(repoID)
	if err != nil {
		return nil, err
	}

	fingerprint := util.MachineFingerprint()
	d := ensureDevice(m, fingerprint)
	if name != "" {
		d.Name = name
	}
	now := time.Now().UTC()
	d.LastSeenAt = &now

	if err := s.save(repo, m, "device: register "+d.Name); err != nil {
		return nil, err
	}
	for _, v := range buildDeviceViews(m, fingerprint) {
		if v.Fingerprint == fingerprint {
			return v, nil
		}
	}
	return nil, fmt.Errorf("device not found: %s", fingerprint)
}

// RenameDevice 重命名设备。
func (s *Service) RenameDevice(repoID, fingerprint, name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	defer s.lock(repoID)()
	repo, m, err := s.load(repoID)
	if err != nil {
		return err
	}
	d := m.FindDevice(fingerprint)
	if d == nil {
		return fmt.Errorf("device not found: %s", fingerprint)
	}
	d.Name = name
	return s.save(repo, m, "device: rename "+name)
}

// DeleteDevice 删除设备及其链接定义。
// 若某条目因此失去 in 链接但仍有其他链接，自动把最早的一条提升为 in，使跟踪得以延续。
func (s *Service) DeleteDevice(repoID, fingerprint string) error {
	if fingerprint == util.MachineFingerprint() {
		return fmt.Errorf("cannot delete the current device")
	}
	defer s.lock(repoID)()
	repo, m, err := s.load(repoID)
	if err != nil {
		return err
	}
	if m.FindDevice(fingerprint) == nil {
		return fmt.Errorf("device not found: %s", fingerprint)
	}

	keptDevices := m.Devices[:0]
	for _, d := range m.Devices {
		if d.Fingerprint != fingerprint {
			keptDevices = append(keptDevices, d)
		}
	}
	m.Devices = keptDevices

	for _, e := range m.Entries {
		kept := e.Links[:0]
		for _, l := range e.Links {
			if l.Device != fingerprint {
				kept = append(kept, l)
			}
		}
		e.Links = kept
		// 失去 in 链接时自动兜底提升一条，保持跟踪不中断
		if len(e.Links) > 0 && e.InLink() == nil {
			e.Links[0].Type = model.LinkTypeIn
		}
	}
	return s.save(repo, m, "device: delete "+fingerprint[:8])
}

// Apply 让本机与清单收敛：按需创建/修复本机软链接，从不覆盖已占用的路径。
// 幂等，且只操作软链接，不触碰内容。
func (s *Service) Apply(repoID, deviceID string, dryRun bool) (*ApplyResult, error) {
	fingerprint := util.MachineFingerprint()
	if deviceID == "" {
		deviceID = fingerprint
	}
	if deviceID != fingerprint {
		return nil, fmt.Errorf("apply is only allowed for the current device")
	}

	defer s.lock(repoID)()
	repo, m, err := s.load(repoID)
	if err != nil {
		return nil, err
	}

	result := &ApplyResult{
		Device:      fingerprint,
		DryRun:      dryRun,
		Created:     []ApplyAction{},
		Repaired:    []ApplyAction{},
		Skipped:     []ApplyAction{},
		Conflicts:   []ApplyAction{},
		Orphans:     []ApplyAction{},
		CompletedAt: time.Now().UTC(),
	}

	// 先出计划，再执行（dry run 时只到计划为止）
	type step struct {
		entry *model.Entry
		link  *model.Link
		state model.LinkState
	}
	var plan []step

	for _, e := range m.Entries {
		for _, l := range e.Links {
			if l.Device != fingerprint {
				continue
			}
			state, _ := Diagnose(repo.Path, e, l, fingerprint)
			plan = append(plan, step{entry: e, link: l, state: state})
		}
	}

	action := func(st step, name, reason string) ApplyAction {
		return ApplyAction{
			EntryID:   st.entry.ID,
			LinkID:    st.link.ID,
			RepoPath:  st.entry.RepoPath,
			LocalPath: st.link.LocalPath,
			Action:    name,
			Reason:    reason,
		}
	}

	for _, st := range plan {
		switch st.state {
		case model.LinkStateMissing, model.LinkStateWrongTarget:
			if !dryRun {
				if err := applyLink(repo.Path, st.entry, st.link, st.state); err != nil {
					result.Conflicts = append(result.Conflicts, action(st, "conflict", err.Error()))
					continue
				}
			}
			if st.state == model.LinkStateMissing {
				result.Created = append(result.Created, action(st, "create", ""))
			} else {
				result.Repaired = append(result.Repaired, action(st, "repair", ""))
			}
		case model.LinkStateDangling:
			result.Orphans = append(result.Orphans, action(st, "orphan", "repository content is missing"))
		case model.LinkStateReplaced:
			result.Conflicts = append(result.Conflicts, action(st, "conflict",
				"local path is a real file or directory"))
		case model.LinkStateOccupied:
			result.Conflicts = append(result.Conflicts, action(st, "conflict", "path is occupied"))
		default:
			result.Skipped = append(result.Skipped, action(st, "skip", string(st.state)))
		}
	}

	// 记录本机在线时间（dry run 不落盘）
	if !dryRun {
		d := ensureDevice(m, fingerprint)
		now := time.Now().UTC()
		d.LastSeenAt = &now
		if err := s.save(repo, m, "device: apply "+d.Name); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// applyLink 按诊断出的状态重建一条链接。
func applyLink(repoRoot string, e *model.Entry, l *model.Link, state model.LinkState) error {
	if state == model.LinkStateWrongTarget {
		if info, err := os.Lstat(l.LocalPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
			os.Remove(l.LocalPath)
		}
	}
	return createSymlink(l.LocalPath, repoRoot, e.RepoPath)
}
