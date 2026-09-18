package entry

import (
	"os"
	"path/filepath"

	"backup-manager/internal/model"
)

// Diagnose 计算一条链接在本机的实际状态。
// 状态按需计算，从不持久化 —— 因此不存在「状态字段与文件系统不一致」的问题。
func Diagnose(repoRoot string, e *model.Entry, l *model.Link, currentDevice string) (model.LinkState, string) {
	if !l.Enabled {
		return model.LinkStateDisabled, ""
	}
	if l.Device != currentDevice {
		return model.LinkStateNotCurrent, ""
	}

	// 仓库内容缺失：链接再对也无意义
	content := DataPath(repoRoot, e.RepoPath)
	if _, err := os.Stat(content); err != nil {
		return model.LinkStateDangling, "repository content is missing"
	}

	info, err := os.Lstat(l.LocalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return model.LinkStateMissing, ""
		}
		return model.LinkStateOccupied, err.Error()
	}

	// 不是软链接 → 被原子写替换成了真实文件/目录
	if info.Mode()&os.ModeSymlink == 0 {
		return model.LinkStateReplaced, "local path is a real file or directory, not a symlink"
	}

	actual, err := os.Readlink(l.LocalPath)
	if err != nil {
		return model.LinkStateOccupied, err.Error()
	}
	if !filepath.IsAbs(actual) {
		actual = filepath.Join(filepath.Dir(l.LocalPath), actual)
	}
	if filepath.Clean(actual) != filepath.Clean(content) {
		return model.LinkStateWrongTarget, "points to " + actual
	}

	if _, err := os.Stat(l.LocalPath); err != nil {
		return model.LinkStateDangling, "symlink target does not resolve"
	}
	return model.LinkStateOK, ""
}

// buildEntryViews 把清单转换为对外视图，逐链接附带状态。
func (s *Service) buildEntryViews(repoRoot string, m *model.Manifest, currentDevice string) []*EntryView {
	views := make([]*EntryView, 0, len(m.Entries))
	for _, e := range m.Entries {
		v := &EntryView{
			ID:        e.ID,
			RepoPath:  e.RepoPath,
			Kind:      string(e.Kind),
			CreatedAt: e.CreatedAt,
			Links:     make([]*LinkView, 0, len(e.Links)),
		}
		for _, l := range e.Links {
			state, note := Diagnose(repoRoot, e, l, currentDevice)
			name := ""
			if d := m.FindDevice(l.Device); d != nil {
				name = d.Name
			}
			v.Links = append(v.Links, &LinkView{
				ID:         l.ID,
				EntryID:    e.ID,
				Device:     l.Device,
				DeviceName: name,
				LocalPath:  l.LocalPath,
				Enabled:    l.Enabled,
				IsCurrent:  l.Device == currentDevice,
				State:      string(state),
				StateNote:  note,
				CreatedAt:  l.CreatedAt,
			})
		}
		views = append(views, v)
	}
	return views
}

// buildDeviceViews 把设备列表转换为对外视图。
func buildDeviceViews(m *model.Manifest, currentDevice string) []*DeviceView {
	views := make([]*DeviceView, 0, len(m.Devices))
	for _, d := range m.Devices {
		count := 0
		for _, e := range m.Entries {
			for _, l := range e.Links {
				if l.Device == d.Fingerprint {
					count++
				}
			}
		}
		views = append(views, &DeviceView{
			Fingerprint: d.Fingerprint,
			Name:        d.Name,
			Hostname:    d.Hostname,
			OS:          d.OS,
			IsCurrent:   d.Fingerprint == currentDevice,
			LastSeenAt:  d.LastSeenAt,
			LinkCount:   count,
		})
	}
	return views
}
