package resolver

import (
	"testing"

	"backup-manager/internal/model"
)

func TestSymlinkResolver_ExactMatch_File(t *testing.T) {
	symlinks := []*model.Symlink{
		{ID: "s1", RelativePath: "notes/meeting.md", TargetPath: "/home/user/notes/meeting.md", Type: model.SymlinkTypeFile},
		{ID: "s2", RelativePath: "config", TargetPath: "/home/user/config", Type: model.SymlinkTypeDirectory},
	}

	r := NewSymlinkResolver(symlinks)
	result, err := r.Resolve("data/notes/meeting.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Symlink.ID != "s1" {
		t.Errorf("expected symlink s1, got %s", result.Symlink.ID)
	}
	if result.TargetPath != "/home/user/notes/meeting.md" {
		t.Errorf("expected /home/user/notes/meeting.md, got %s", result.TargetPath)
	}
}

func TestSymlinkResolver_PrefixMatch_Directory(t *testing.T) {
	symlinks := []*model.Symlink{
		{ID: "s1", RelativePath: "notes", TargetPath: "/home/user/Notes", Type: model.SymlinkTypeDirectory},
	}

	r := NewSymlinkResolver(symlinks)
	result, err := r.Resolve("data/notes/file1.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Symlink.ID != "s1" {
		t.Errorf("expected symlink s1, got %s", result.Symlink.ID)
	}
	if result.TargetPath != "/home/user/Notes/file1.md" {
		t.Errorf("expected /home/user/Notes/file1.md, got %s", result.TargetPath)
	}
}

func TestSymlinkResolver_LongestPrefixWins(t *testing.T) {
	symlinks := []*model.Symlink{
		{ID: "s1", RelativePath: "docs", TargetPath: "/home/user/Docs", Type: model.SymlinkTypeDirectory},
		{ID: "s2", RelativePath: "docs/sub", TargetPath: "/home/user/SubDocs", Type: model.SymlinkTypeDirectory},
	}

	r := NewSymlinkResolver(symlinks)

	// Should match the longer prefix "docs/sub" not just "docs"
	result, err := r.Resolve("data/docs/sub/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Symlink.ID != "s2" {
		t.Errorf("expected symlink s2 (longer prefix), got %s", result.Symlink.ID)
	}
	if result.TargetPath != "/home/user/SubDocs/file.txt" {
		t.Errorf("expected /home/user/SubDocs/file.txt, got %s", result.TargetPath)
	}
}

func TestSymlinkResolver_NoMatch(t *testing.T) {
	symlinks := []*model.Symlink{
		{ID: "s1", RelativePath: "notes", TargetPath: "/home/user/Notes", Type: model.SymlinkTypeDirectory},
	}

	r := NewSymlinkResolver(symlinks)
	_, err := r.Resolve("data/other/file.txt")
	if err == nil {
		t.Fatal("expected error for non-matching path")
	}
}

func TestSymlinkResolver_NotUnderData(t *testing.T) {
	symlinks := []*model.Symlink{}
	r := NewSymlinkResolver(symlinks)
	_, err := r.Resolve("outside/file.txt")
	if err == nil {
		t.Fatal("expected error for path outside data/")
	}
}

func TestSymlinkResolver_ExactMatchPreferredOverPrefix(t *testing.T) {
	// File symlink with the same path as a directory's sub-file
	symlinks := []*model.Symlink{
		{ID: "s1", RelativePath: "docs", TargetPath: "/home/user/Docs", Type: model.SymlinkTypeDirectory},
		{ID: "s2", RelativePath: "docs/file.txt", TargetPath: "/home/user/Special/file.txt", Type: model.SymlinkTypeFile},
	}

	r := NewSymlinkResolver(symlinks)

	// Exact match should win over prefix match
	result, err := r.Resolve("data/docs/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Symlink.ID != "s2" {
		t.Errorf("expected symlink s2 (exact match), got %s", result.Symlink.ID)
	}
	if result.TargetPath != "/home/user/Special/file.txt" {
		t.Errorf("expected /home/user/Special/file.txt, got %s", result.TargetPath)
	}
}

func TestSymlinkResolver_ResolveCommitFiles(t *testing.T) {
	symlinks := []*model.Symlink{
		{ID: "s1", RelativePath: "notes", TargetPath: "/home/user/Notes", Type: model.SymlinkTypeDirectory},
		{ID: "s2", RelativePath: "todo.txt", TargetPath: "/home/user/todo.txt", Type: model.SymlinkTypeFile},
	}

	r := NewSymlinkResolver(symlinks)
	paths := []string{
		"data/notes/file1.md",
		"data/notes/sub/file2.md",
		"data/todo.txt",
		"data/unknown/file.txt", // no match
	}

	grouped := r.ResolveCommitFiles(paths)

	// s1 should have 2 results
	if len(grouped["s1"]) != 2 {
		t.Errorf("expected 2 results for s1, got %d", len(grouped["s1"]))
	}

	// s2 should have 1 result
	if len(grouped["s2"]) != 1 {
		t.Errorf("expected 1 result for s2, got %d", len(grouped["s2"]))
	}

	// unknown should not be in the group
	if _, ok := grouped[""]; ok {
		t.Error("unmatched paths should not be in the group")
	}
}
