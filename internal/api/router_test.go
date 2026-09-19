package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backup-manager/internal/api/handler"
	"backup-manager/internal/appconfig"
)

func TestSetupRouterRegistersSettingsRoutes(t *testing.T) {
	manager, err := appconfig.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	router := SetupRouter(
		handler.NewRepoHandler(nil),
		handler.NewEntryHandler(nil),
		handler.NewLinkHandler(nil),
		handler.NewDeviceHandler(nil),
		handler.NewConsistencyHandler(nil),
		handler.NewBrowseHandler(nil),
		handler.NewContentHandler(nil),
		handler.NewBackupHandler(nil),
		handler.NewAuthHandler(nil),
		handler.NewSystemHandler(manager),
		handler.NewRollbackHandler(nil),
	)

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d; body=%s", getResponse.Code, http.StatusOK, getResponse.Body.String())
	}

	putRequest := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(`{"language":"zh-CN"}`))
	putRequest.Header.Set("Content-Type", "application/json")
	putResponse := httptest.NewRecorder()
	router.ServeHTTP(putResponse, putRequest)
	if putResponse.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d; body=%s", putResponse.Code, http.StatusOK, putResponse.Body.String())
	}
	if got := manager.Get().Language; got != appconfig.LanguageChineseSimplified {
		t.Fatalf("language after PUT = %q, want %q", got, appconfig.LanguageChineseSimplified)
	}
}
