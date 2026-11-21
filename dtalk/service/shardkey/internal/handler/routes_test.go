package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAPIKeyAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", apiKeyAuthMiddleware(map[string]string{"app": "token"}), func(c *gin.Context) {
		c.JSON(http.StatusOK, successResponse("ok", nil))
	})

	cases := []struct {
		name   string
		appID  string
		token  string
		status int
		result int
	}{
		{"missing", "", "", http.StatusUnauthorized, ResultError},
		{"wrong", "app", "bad", http.StatusUnauthorized, ResultError},
		{"ok", "app", "token", http.StatusOK, ResultOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/x", nil)
			if tc.appID != "" {
				req.Header.Set("X-App-Id", tc.appID)
			}
			if tc.token != "" {
				req.Header.Set("X-App-Token", tc.token)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			var resp APIResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("invalid response: %v", err)
			}
			if resp.Result != tc.result {
				t.Fatalf("result = %d, want %d", resp.Result, tc.result)
			}
		})
	}
}

func TestValidAddress(t *testing.T) {
	if !isValidAddress("1FKxgaEh5fuSm7a35BfUnKYAmradowpiTR") {
		t.Error("1x address should be valid")
	}
	if !isValidAddress("0xabcdef") {
		t.Error("0x address should be valid")
	}
	if isValidAddress("abc") {
		t.Error("plain string should be invalid")
	}
}
