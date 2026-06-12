// Package resolver provides path resolution from Git tree paths
// to source file target paths for rollback operations.
package resolver

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"backup-manager/internal/git"
	"backup-manager/internal/model"
)

// DataDirPrefix is the "data/" prefix used in Git tree paths.
var DataDirPrefix = git.DataDirName + "/"

// ResolveResult contains the resolved target path information for a single file.
type ResolveResult struct {
	Symlink    *model.Symlink // The matching symlink record
	TargetPath string         // Source file absolute path to write to
	GitRelPath string         // Relative path without "data/" prefix
}

// SymlinkResolver resolves Git tree paths to source file target paths.
type SymlinkResolver struct {
	symlinks []*model.Symlink // Sorted by relative_path length descending
}

// NewSymlinkResolver creates a resolver with pre-sorted symlinks.
// Sorting by longest relative_path first ensures correct prefix matching
// (longest prefix wins when multiple directory symlinks could match).
func NewSymlinkResolver(symlinks []*model.Symlink) *SymlinkResolver {
	sorted := make([]*model.Symlink, len(symlinks))
	copy(sorted, symlinks)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i].RelativePath) > len(sorted[j].RelativePath)
	})
	return &SymlinkResolver{symlinks: sorted}
}

// Resolve maps a Git tree path to a source file target path.
// Returns nil if no matching symlink is found.
//
// Matching priority:
//  1. Exact match (symlink.relative_path == relPath) — covers file-type symlinks
//  2. Longest prefix match (relPath starts with symlink.relative_path + "/") — covers directory-type
//  3. No match → error (file has no corresponding symlink in current DB)
func (r *SymlinkResolver) Resolve(gitTreePath string) (*ResolveResult, error) {
	// Step 1: Strip "data/" prefix
	relPath, ok := strings.CutPrefix(gitTreePath, DataDirPrefix)
	if !ok {
		return nil, fmt.Errorf("path %q is not under data/ directory", gitTreePath)
	}

	// Step 2: Try exact match first, then prefix match
	for _, sym := range r.symlinks {
		// Exact match
		if sym.RelativePath == relPath {
			return &ResolveResult{
				Symlink:    sym,
				TargetPath: sym.TargetPath,
				GitRelPath: relPath,
			}, nil
		}

		// Prefix match (only directory symlinks have sub-files)
		if sym.Type == model.SymlinkTypeDirectory &&
			strings.HasPrefix(relPath, sym.RelativePath+"/") {

			suffix := strings.TrimPrefix(relPath, sym.RelativePath+"/")
			return &ResolveResult{
				Symlink:    sym,
				TargetPath: filepath.Join(sym.TargetPath, suffix),
				GitRelPath: relPath,
			}, nil
		}
	}

	return nil, fmt.Errorf("no matching symlink for %q (repo data may be inconsistent)", relPath)
}

// ResolveCommitFiles resolves all files from a commit diff and groups them by symlink ID.
// Files that cannot be resolved are skipped with a log warning.
func (r *SymlinkResolver) ResolveCommitFiles(gitTreePaths []string) map[string][]*ResolveResult {
	grouped := make(map[string][]*ResolveResult)
	for _, path := range gitTreePaths {
		result, err := r.Resolve(path)
		if err != nil {
			log.Printf("[resolver] %v — skipping", err)
			continue
		}
		grouped[result.Symlink.ID] = append(grouped[result.Symlink.ID], result)
	}
	return grouped
}
