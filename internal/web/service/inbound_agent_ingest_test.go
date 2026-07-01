package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func seedClientTraffic(t *testing.T, email string) {
	t.Helper()
	if err := database.GetDB().Create(&xray.ClientTraffic{Email: email, Enable: true}).Error; err != nil {
		t.Fatalf("seed client_traffics %q: %v", email, err)
	}
}

func loadClientTraffic(t *testing.T, email string) xray.ClientTraffic {
	t.Helper()
	var row xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("load client_traffics %q: %v", email, err)
	}
	return row
}

func TestIngestAgentTraffic_AccumulatesDeltaByEmail(t *testing.T) {
	initTrafficTestDB(t)
	svc := &InboundService{}
	seedClientTraffic(t, "alice")

	if err := svc.IngestAgentTraffic(7, []*xray.ClientTraffic{{Email: "alice", Up: 100, Down: 200}}); err != nil {
		t.Fatalf("ingest 1: %v", err)
	}
	if got := loadClientTraffic(t, "alice"); got.Up != 0 || got.Down != 0 {
		t.Fatalf("after baseline report up/down = %d/%d, want 0/0", got.Up, got.Down)
	}

	if err := svc.IngestAgentTraffic(7, []*xray.ClientTraffic{{Email: "alice", Up: 250, Down: 500}}); err != nil {
		t.Fatalf("ingest 2: %v", err)
	}
	if got := loadClientTraffic(t, "alice"); got.Up != 150 || got.Down != 300 {
		t.Fatalf("after delta report up/down = %d/%d, want 150/300", got.Up, got.Down)
	}
}

func TestIngestAgentTraffic_AgentRestartResetsBaseline(t *testing.T) {
	initTrafficTestDB(t)
	svc := &InboundService{}
	seedClientTraffic(t, "alice")

	if err := svc.IngestAgentTraffic(7, []*xray.ClientTraffic{{Email: "alice", Up: 100, Down: 200}}); err != nil {
		t.Fatalf("ingest baseline: %v", err)
	}
	if err := svc.IngestAgentTraffic(7, []*xray.ClientTraffic{{Email: "alice", Up: 300, Down: 600}}); err != nil {
		t.Fatalf("ingest growth: %v", err)
	}
	if got := loadClientTraffic(t, "alice"); got.Up != 200 || got.Down != 400 {
		t.Fatalf("before restart up/down = %d/%d, want 200/400", got.Up, got.Down)
	}

	if err := svc.IngestAgentTraffic(7, []*xray.ClientTraffic{{Email: "alice", Up: 10, Down: 20}}); err != nil {
		t.Fatalf("ingest restart: %v", err)
	}
	if got := loadClientTraffic(t, "alice"); got.Up != 200 || got.Down != 400 {
		t.Fatalf("after restart clamp up/down = %d/%d, want 200/400 (negative delta clamped)", got.Up, got.Down)
	}

	if err := svc.IngestAgentTraffic(7, []*xray.ClientTraffic{{Email: "alice", Up: 60, Down: 120}}); err != nil {
		t.Fatalf("ingest post-restart: %v", err)
	}
	if got := loadClientTraffic(t, "alice"); got.Up != 250 || got.Down != 500 {
		t.Fatalf("after post-restart growth up/down = %d/%d, want 250/500", got.Up, got.Down)
	}
}

func TestIngestAgentTraffic_MasterAuthoritativeNoRowCreation(t *testing.T) {
	initTrafficTestDB(t)
	svc := &InboundService{}

	if err := svc.IngestAgentTraffic(7, []*xray.ClientTraffic{{Email: "ghost", Up: 100, Down: 200}}); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	var ctCount, ibCount int64
	database.GetDB().Model(&xray.ClientTraffic{}).Count(&ctCount)
	database.GetDB().Model(&model.Inbound{}).Count(&ibCount)
	if ctCount != 0 {
		t.Fatalf("client_traffics rows = %d, want 0 (unknown email must not create a client row)", ctCount)
	}
	if ibCount != 0 {
		t.Fatalf("inbounds rows = %d, want 0 (agent ingest must never create inbounds)", ibCount)
	}
}
