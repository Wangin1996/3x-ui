package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestBindNodeModeFromJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"edge","mode":"agent","address":"","port":0}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	n, ok := BindAndValidate[model.Node](c)
	if !ok {
		t.Fatalf("bind/validate failed: %s", w.Body.String())
	}
	if n.Mode != "agent" {
		t.Fatalf("Mode = %q, want agent (mode did not bind from JSON body)", n.Mode)
	}
}

func TestBindNodeModeFromForm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest("POST", "/", strings.NewReader("name=edge&mode=agent&address="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	n, ok := BindAndValidate[model.Node](c)
	if !ok {
		t.Fatalf("bind/validate failed: %s", w.Body.String())
	}
	if n.Mode != "agent" {
		t.Fatalf("Mode = %q, want agent (mode did not bind from form body)", n.Mode)
	}
}
