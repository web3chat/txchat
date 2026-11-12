package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConsoleMounted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	mountConsole(engine)

	root := httptest.NewRecorder()
	engine.ServeHTTP(root, httptest.NewRequest(http.MethodGet, "/", nil))
	if root.Code != http.StatusFound {
		t.Fatalf("GET / status=%d, want %d", root.Code, http.StatusFound)
	}
	if loc := root.Header().Get("Location"); loc != "/console/" {
		t.Fatalf("GET / location=%q", loc)
	}

	page := httptest.NewRecorder()
	engine.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/console/", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("GET /console/ status=%d", page.Code)
	}
	body, err := io.ReadAll(page.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "ChatAdmin") {
		t.Fatalf("运营页面缺少标题")
	}

	missing := httptest.NewRecorder()
	engine.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("GET /swagger/doc.json status=%d, want 404", missing.Code)
	}
}
