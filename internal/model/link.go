package model

import "time"

// LinkType 链接类型。
//
// in 是 out 的特例：两者物理形态完全相同（都是指向 <repo>/data/<repo_path> 的软链接），
// 仅角色不同 —— in 负责把内容移入仓库并创建条目，每个条目至多一个；out 只做分发，0..N 个。
type LinkType string

const (
	LinkTypeIn  LinkType = "in"  // 条目的跟踪链接
	LinkTypeOut LinkType = "out" // 额外的分发链接
)

// EntryKind 条目类型。
type EntryKind string

const (
	EntryKindFile EntryKind = "file"
	EntryKindDir  EntryKind = "dir"
)

// LinkState 链接在本机的实际状态。按需从文件系统计算，不持久化。
type LinkState string

const (
	LinkStateOK          LinkState = "ok"           // 链接存在且指向正确
	LinkStateMissing     LinkState = "missing"      // 本机链接不存在
	LinkStateWrongTarget LinkState = "wrong_target" // 链接存在但指向别处
	LinkStateReplaced    LinkState = "replaced"     // 本机路径被真实文件/目录占用
	LinkStateDangling    LinkState = "dangling"     // 链接存在但仓库内容缺失
	LinkStateOccupied    LinkState = "occupied"     // 路径被无关对象占用，不可覆盖
	LinkStateDisabled    LinkState = "disabled"     // 已禁用
	LinkStateNotCurrent  LinkState = "not_current"  // 属于其他设备
)

// Link 一条链接：把一个本机路径绑定到一个条目。
type Link struct {
	ID        string    `json:"id"`
	Type      LinkType  `json:"type"`
	Device    string    `json:"device"`     // 所属设备的指纹
	LocalPath string    `json:"local_path"` // 本机绝对路径
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// Entry 一个被备份的文件/目录：data/ 下的一个 repo_path，加上所有指向它的链接。
type Entry struct {
	ID        string    `json:"id"`
	RepoPath  string    `json:"repo_path"` // 相对 data/ 的路径，即内容的身份
	Kind      EntryKind `json:"kind"`
	CreatedAt time.Time `json:"created_at"`
	Links     []*Link   `json:"links"`
}

// InLink 返回条目的 in 链接；条目未绑定时返回 nil。
func (e *Entry) InLink() *Link {
	for _, l := range e.Links {
		if l.Type == LinkTypeIn {
			return l
		}
	}
	return nil
}

// FindLink 按 id 查找链接，未找到返回 nil。
func (e *Entry) FindLink(id string) *Link {
	for _, l := range e.Links {
		if l.ID == id {
			return l
		}
	}
	return nil
}

// RemoveLink 从条目中移除指定链接。
func (e *Entry) RemoveLink(id string) {
	kept := e.Links[:0]
	for _, l := range e.Links {
		if l.ID != id {
			kept = append(kept, l)
		}
	}
	e.Links = kept
}

// Device 一台引用该仓库的机器。它是元数据，链接引用它时使用指纹。
type Device struct {
	Fingerprint string     `json:"fingerprint"`
	Name        string     `json:"name"` // 默认取 hostname，可重命名
	Hostname    string     `json:"hostname,omitempty"`
	OS          string     `json:"os,omitempty"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
}

// Manifest 仓库清单：设备 + 条目（链接内嵌于条目）。
// 它存放在 <repo-root>/.backup-manager/manifest.json，由 Git 跟踪，
// 因此新机器仅凭 git clone 即可获知所有设备的链接。
type Manifest struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	Devices   []*Device `json:"devices"`
	Entries   []*Entry  `json:"entries"`
}

// FindEntry 按 id 查找条目。
func (m *Manifest) FindEntry(id string) *Entry {
	for _, e := range m.Entries {
		if e.ID == id {
			return e
		}
	}
	return nil
}

// FindEntryByRepoPath 按 repo_path 查找条目。
func (m *Manifest) FindEntryByRepoPath(repoPath string) *Entry {
	for _, e := range m.Entries {
		if e.RepoPath == repoPath {
			return e
		}
	}
	return nil
}

// FindDevice 按指纹查找设备。
func (m *Manifest) FindDevice(fingerprint string) *Device {
	for _, d := range m.Devices {
		if d.Fingerprint == fingerprint {
			return d
		}
	}
	return nil
}

// Links 展平返回所有条目上的全部链接。
func (m *Manifest) Links() []*Link {
	var out []*Link
	for _, e := range m.Entries {
		out = append(out, e.Links...)
	}
	return out
}
