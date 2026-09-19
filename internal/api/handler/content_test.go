package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"backup-manager/internal/git"
	"backup-manager/internal/model"
	"backup-manager/internal/service"
	"backup-manager/internal/store"
	"backup-manager/internal/util"

	"github.com/gin-gonic/gin"
)

func TestContentTreeIncludeHiddenQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := store.OpenDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repoRoot := t.TempDir()
	for _, name := range []string{"visible", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(repoRoot, "data", name), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
	}

	ds := store.NewStore(db)
	repo := &model.Repo{ID: "r1", Name: "test", Path: repoRoot, Status: model.RepoStatusActive}
	if err := ds.CreateRepo(repo); err != nil {
		t.Fatalf("create repo: %v", err)
	}

	handler := NewContentHandler(service.NewContentService(
		ds,
		git.NewGitEngine(),
		util.NewRepoMutexManager(),
	))
	router := gin.New()
	router.GET("/repos/:id/tree", handler.Tree)

	visible := performRequest(router, http.MethodGet, "/repos/r1/tree", "")
	if visible.Code != http.StatusOK {
		t.Fatalf("default tree status = %d; body=%s", visible.Code, visible.Body.String())
	}
	if names := decodeContentEntryNames(t, visible.Body.Bytes()); len(names) != 1 || names[0] != "visible" {
		t.Fatalf("default tree entries = %v, want [visible]", names)
	}

	all := performRequest(router, http.MethodGet, "/repos/r1/tree?include_hidden=true", "")
	if all.Code != http.StatusOK {
		t.Fatalf("hidden tree status = %d; body=%s", all.Code, all.Body.String())
	}
	if names := decodeContentEntryNames(t, all.Body.Bytes()); len(names) != 2 || names[0] != ".hidden" || names[1] != "visible" {
		t.Fatalf("hidden tree entries = %v, want [.hidden visible]", names)
	}

	invalid := performRequest(router, http.MethodGet, "/repos/r1/tree?include_hidden=not-a-bool", "")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid include_hidden status = %d, want %d", invalid.Code, http.StatusBadRequest)
	}
}

func decodeContentEntryNames(t *testing.T, body []byte) []string {
	t.Helper()
	var payload struct {
		Data []service.ContentEntry `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode tree response: %v", err)
	}
	names := make([]string, 0, len(payload.Data))
	for _, entry := range payload.Data {
		names = append(names, entry.Name)
	}
	return names
}
