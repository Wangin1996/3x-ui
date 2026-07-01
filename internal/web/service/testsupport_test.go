package service

import (
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func initTrafficTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	return database.GetDB()
}

func readTraffic(t *testing.T, db *gorm.DB, email string) xray.ClientTraffic {
	t.Helper()
	var ct xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).First(&ct).Error; err != nil {
		t.Fatalf("read client_traffics %q: %v", email, err)
	}
	return ct
}
