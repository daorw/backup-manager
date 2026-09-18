package entry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"backup-manager/internal/model"
	"backup-manager/internal/util"
)

// 巡检结论的严重级别。
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// 巡检结论码。
const (
	CodeInvalidEntry       = "invalid_entry"       // 条目结构非法（缺字段、id 重复、repo_path 非法）
	CodeInvalidLink        = "invalid_link"        // 链接结构非法（缺字段、id 重复）
	CodeUnknownDevice      = "unknown_device"      // 链接引用了未登记的设备
	CodeOverlappingEntries = "overlapping_entries" // 条目互相重叠（违反 R-2）
	CodeNestedLink         = "nested_link"         // 链接落在目录条目之内（违反 R-3）
	CodeLinkMissing        = "link_missing"        // 本机软链接不存在
	CodeLinkWrongTarget    = "link_wrong_target"   // 本机软链接指向别处
	CodeLinkReplaced       = "link_replaced"       // 本机路径被真实文件/目录替换
	CodeLinkOccupied       = "link_occupied"       // 本机路径被无关对象占用
	CodeContentMissing     = "content_missing"     // 仓库内容缺失，链接悬空
	CodeSymlinkInData      = "symlink_in_data"     // data/ 内出现软链接
	CodeUnmanagedLink      = "unmanaged_link"      // 扫描到指向 data/ 的未托管软链接
)

// Finding 一条巡检结论。
type Finding struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"`
	RepoPath   string `json:"repo_path,omitempty"`
	LinkID     string `json:"link_id,omitempty"`
	LocalPath  string `json:"local_path,omitempty"`
	Repairable bool   `json:"repairable"`
	Message    string `json:"message"`
}

// AuditResult 巡检结果。
type AuditResult struct {
	RepoID    string    `json:"repo_id"`
	Device    string    `json:"device"`
	Clean     bool      `json:"clean"`
	Errors    int       `json:"errors"`
	Warnings  int       `json:"warnings"`
	EntryCnt  int       `json:"entry_count"`
	LinkCnt   int       `json:"link_count"`
	Findings  []Finding `json:"findings"`
	AuditedAt time.Time `json:"audited_at"`
}

// RepairResult 巡检修复结果。
type RepairResult struct {
	RepoID          string        `json:"repo_id"`
	Repaired        []ApplyAction `json:"repaired"`
	Skipped         []Finding     `json:"skipped"`
	RepairedCount   int           `json:"repaired_count"`
	RemainingErrors int           `json:"remaining_errors"`
	CompletedAt     time.Time     `json:"completed_at"`
}

// Audit 巡检仓库：清单不变量 + 本机链接实际状态 + data/ 内的软链接 + 未托管软链接。
func (s *Service) Audit(repoID string) (*AuditResult, error) {
	repo, m, err := s.load(repoID)
	if err != nil {
		return nil, err
	}
	fingerprint := util.MachineFingerprint()

	findings := checkManifest(m)
	findings = append(findings, auditFilesystem(repo.Path, m, fingerprint)...)
	// 保证序列化为 [] 而不是 null，前端不必额外判空
	if findings == nil {
		findings = []Finding{}
	}

	sortFindings(findings)
	res := &AuditResult{
		RepoID:    repoID,
		Device:    fingerprint,
		EntryCnt:  len(m.Entries),
		LinkCnt:   len(m.Links()),
		Findings:  findings,
		AuditedAt: time.Now().UTC(),
	}
	for _, f := range findings {
		if f.Severity == SeverityError {
			res.Errors++
		} else {
			res.Warnings++
		}
	}
	res.Clean = res.Errors == 0 && res.Warnings == 0
	return res, nil
}

// RepairAll 收敛所有可自动修复的问题，其余只报告：
//
//	链接落在目录条目之内 → 禁用该链接（R-3）
//	本机链接缺失/指向错误 → 重建软链接
//
// 条目重叠（R-2）、内容缺失、data/ 内的软链接等无法安全自动处理，一律只报告。
func (s *Service) RepairAll(repoID string) (*RepairResult, error) {
	defer s.lock(repoID)()
	repo, m, err := s.load(repoID)
	if err != nil {
		return nil, err
	}
	fingerprint := util.MachineFingerprint()

	result := &RepairResult{
		RepoID:      repoID,
		Repaired:    []ApplyAction{},
		Skipped:     []Finding{},
		CompletedAt: time.Now().UTC(),
	}
	action := func(e *model.Entry, l *model.Link, name, reason string) ApplyAction {
		return ApplyAction{
			EntryID: e.ID, LinkID: l.ID, RepoPath: e.RepoPath,
			LocalPath: l.LocalPath, Action: name, Reason: reason,
		}
	}

	// 1) R-3：禁用落在目录条目之内的链接
	for _, dir := range m.Entries {
		if dir.Kind != model.EntryKindDir {
			continue
		}
		for _, owner := range dir.Links {
			if !owner.Enabled {
				continue
			}
			for _, e := range m.Entries {
				for _, l := range e.Links {
					if l == owner || !l.Enabled {
						continue
					}
					if localPathsOverlap(l.LocalPath, owner.LocalPath) {
						l.Enabled = false
						result.Repaired = append(result.Repaired,
							action(e, l, "disable", fmt.Sprintf("R-3: inside directory entry %q", dir.RepoPath)))
					}
				}
			}
		}
	}

	// 2) 本机链接状态：重建缺失或指向错误的软链接
	for _, e := range m.Entries {
		for _, l := range e.Links {
			if l.Device != fingerprint {
				continue
			}
			state, _ := Diagnose(repo.Path, e, l, fingerprint)
			switch state {
			case model.LinkStateWrongTarget:
				if info, err := os.Lstat(l.LocalPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
					os.Remove(l.LocalPath)
				}
				fallthrough
			case model.LinkStateMissing:
				if err := createSymlink(l.LocalPath, repo.Path, e.RepoPath); err != nil {
					result.Skipped = append(result.Skipped, Finding{
						Code: CodeLinkMissing, Severity: SeverityError,
						RepoPath: e.RepoPath, LinkID: l.ID, LocalPath: l.LocalPath,
						Message: err.Error(),
					})
					continue
				}
				result.Repaired = append(result.Repaired, action(e, l, "repair", ""))
			}
		}
	}

	result.RepairedCount = len(result.Repaired)

	// 收敛路径跳过不变量校验：否则已经不合规的清单永远修不好
	if err := s.saveConverging(repo, m, "consistency: repair"); err != nil {
		return nil, err
	}

	// 重新巡检，把所有仍存在的问题作为「未修复项」返回
	after, err := s.Audit(repoID)
	if err != nil {
		return nil, err
	}
	for _, f := range after.Findings {
		if f.Severity == SeverityError {
			result.Skipped = append(result.Skipped, f)
			result.RemainingErrors++
		}
	}
	return result, nil
}

// auditFilesystem 检查磁盘上的实际状态。
func auditFilesystem(repoRoot string, m *model.Manifest, currentDevice string) []Finding {
	var findings []Finding

	// 逐链接诊断（只看本机；其他设备的状态属于那台机器，本机无法判定）
	for _, e := range m.Entries {
		for _, l := range e.Links {
			if l.Device != currentDevice {
				continue
			}
			state, note := Diagnose(repoRoot, e, l, currentDevice)

			var code, severity, message string
			switch state {
			case model.LinkStateOK, model.LinkStateDisabled:
				continue
			case model.LinkStateMissing:
				code, severity, message = CodeLinkMissing, SeverityWarning, "本机软链接不存在，可用 Apply 或 Repair 重建"
			case model.LinkStateWrongTarget:
				code, severity, message = CodeLinkWrongTarget, SeverityWarning, "本机软链接指向别处"
			case model.LinkStateReplaced:
				code, severity, message = CodeLinkReplaced, SeverityWarning,
					"本机路径是真实文件/目录而非软链接（应用可能做了原子写），需重新纳入"
			case model.LinkStateOccupied:
				code, severity, message = CodeLinkOccupied, SeverityError, "本机路径被无关对象占用，需人工处理"
			case model.LinkStateDangling:
				code, severity, message = CodeContentMissing, SeverityError,
					"仓库内容缺失，链接悬空；可从 Git 历史回滚恢复"
			default:
				continue
			}
			if note != "" {
				message += "（" + note + "）"
			}
			findings = append(findings, Finding{
				Code: code, Severity: severity,
				RepoPath: e.RepoPath, LinkID: l.ID, LocalPath: l.LocalPath,
				Repairable: state == model.LinkStateMissing || state == model.LinkStateWrongTarget,
				Message:    message,
			})
		}
	}

	findings = append(findings, scanSymlinksInData(repoRoot)...)
	findings = append(findings, scanUnmanagedLinks(repoRoot, m)...)
	return findings
}

// scanSymlinksInData 扫描 data/ 内部是否出现软链接 —— 它会破坏「内容只存在于 data/」。
func scanSymlinksInData(repoRoot string) []Finding {
	dataDir := filepath.Join(repoRoot, "data")
	var findings []Finding

	_ = filepath.Walk(dataDir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil {
			return nil // 无法访问的子树跳过，不阻断巡检
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			return nil
		}
		rel, relErr := filepath.Rel(dataDir, p)
		if relErr != nil {
			rel = p
		}
		findings = append(findings, Finding{
			Code: CodeSymlinkInData, Severity: SeverityError,
			RepoPath: filepath.ToSlash(rel), LocalPath: p,
			Message: "data/ 内出现软链接；内容必须只存在于 data/，请改为普通文件/目录",
		})
		return nil
	})
	return findings
}

// scanUnmanagedLinks 扫描已注册链接的父目录，找出指向 data/ 但未被本工具托管的软链接。
//
// 这是 Issue 所禁止形态（对目录条目内部单个文件建链接）的直接探测手段：
// 无法扫描整个文件系统，因此范围限定在已注册链接的父目录，并按 warning 报告。
func scanUnmanagedLinks(repoRoot string, m *model.Manifest) []Finding {
	known := make(map[string]bool)
	parents := make(map[string]bool)
	for _, l := range m.Links() {
		if l.LocalPath == "" {
			continue
		}
		known[filepath.Clean(l.LocalPath)] = true
		parents[filepath.Dir(filepath.Clean(l.LocalPath))] = true
	}

	dataDir := filepath.Join(repoRoot, "data")
	var findings []Finding

	dirs := make([]string, 0, len(parents))
	for d := range parents {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			full := filepath.Join(dir, entry.Name())
			if known[full] {
				continue
			}
			info, err := os.Lstat(full)
			if err != nil || info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			// 只关心指向本仓库 data/ 的链接；解析到具体的仓库相对路径
			target, err := filepath.EvalSymlinks(full)
			if err != nil {
				continue
			}
			rel, err := filepath.Rel(dataDir, target)
			if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
				continue
			}
			findings = append(findings, Finding{
				Code: CodeUnmanagedLink, Severity: SeverityWarning,
				RepoPath: filepath.ToSlash(rel), LocalPath: full,
				Message: "发现未被托管的软链接；若要托管它，请为对应条目添加一条链接",
			})
		}
	}
	return findings
}

// sortFindings 让结论稳定有序：error 在前，然后按结论码、仓库路径、本机路径。
func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if (a.Severity == SeverityError) != (b.Severity == SeverityError) {
			return a.Severity == SeverityError
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.RepoPath != b.RepoPath {
			return a.RepoPath < b.RepoPath
		}
		return a.LocalPath < b.LocalPath
	})
}
