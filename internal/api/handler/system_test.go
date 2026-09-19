package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backup-manager/internal/appconfig"

	"github.com/gin-gonic/gin"
)

func TestSystemSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager, err := appconfig.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	handler := NewSystemHandler(manager)
	router := gin.New()
	router.GET("/settings", handler.Settings)
	router.PUT("/settings", handler.UpdateSettings)

	getResponse := performRequest(router, http.MethodGet, "/settings", "")
	if getResponse.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d; body=%s", getResponse.Code, http.StatusOK, getResponse.Body.String())
	}
	if language := responseLanguage(t, getResponse); language != appconfig.LanguageEnglish {
		t.Fatalf("GET language = %q, want %q", language, appconfig.LanguageEnglish)
	}

	putResponse := performRequest(router, http.MethodPut, "/settings", `{"language":"zh-CN"}`)
	if putResponse.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d; body=%s", putResponse.Code, http.StatusOK, putResponse.Body.String())
	}
	if language := responseLanguage(t, putResponse); language != appconfig.LanguageChineseSimplified {
		t.Fatalf("PUT language = %q, want %q", language, appconfig.LanguageChineseSimplified)
	}
	if manager.Get().Language != appconfig.LanguageChineseSimplified {
		t.Fatalf("manager language = %q, want %q", manager.Get().Language, appconfig.LanguageChineseSimplified)
	}
}

func TestSystemUpdateSettingsRejectsInvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ""},
		{name: "missing language", body: `{}`},
		{name: "null language", body: `{"language":null}`},
		{name: "unsupported language", body: `{"language":"fr"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager, err := appconfig.NewManager(t.TempDir())
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}
			handler := NewSystemHandler(manager)
			router := gin.New()
			router.PUT("/settings", handler.UpdateSettings)

			response := performRequest(router, http.MethodPut, "/settings", test.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("PUT status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			if got := manager.Get().Language; got != appconfig.LanguageEnglish {
				t.Fatalf("language after rejected request = %q, want %q", got, appconfig.LanguageEnglish)
			}
		})
	}
}

func performRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func responseLanguage(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Data struct {
			Language string `json:"language"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload.Data.Language
}
