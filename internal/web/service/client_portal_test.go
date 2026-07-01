package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func seedPortalClient(t *testing.T, svc *ClientService, port int, email, subID string) *model.Inbound {
	t.Helper()
	c := model.Client{Email: email, ID: "11111111-1111-1111-1111-111111111111", SubID: subID, Enable: true}
	ib := mkInbound(t, port, model.VLESS, clientsSettings(t, []model.Client{c}))
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{c}); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, email, 0, 0, 0, 0, true)
	return ib
}

func TestSetAndCheckLoginPassword(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	email := "pw@x"
	seedPortalClient(t, svc, 54101, email, "sp")

	if _, err := svc.CheckLoginPassword(email, "whatever"); err == nil {
		t.Fatal("no password set: expected login failure")
	}
	if err := svc.SetLoginPassword(email, "s3cret"); err != nil {
		t.Fatalf("SetLoginPassword: %v", err)
	}
	rec, err := svc.CheckLoginPassword(email, "s3cret")
	if err != nil {
		t.Fatalf("CheckLoginPassword correct: %v", err)
	}
	if rec.Email != email {
		t.Fatalf("returned rec email = %q, want %q", rec.Email, email)
	}
	if _, err := svc.CheckLoginPassword(email, "wrong"); err == nil {
		t.Fatal("wrong password: expected failure")
	}
	if _, err := svc.CheckLoginPassword(email, ""); err == nil {
		t.Fatal("empty password: expected failure")
	}
	if err := svc.SetLoginPassword(email, ""); err != nil {
		t.Fatalf("clear password: %v", err)
	}
	if _, err := svc.CheckLoginPassword(email, "s3cret"); err == nil {
		t.Fatal("cleared password: expected failure")
	}
}

func TestRotateSubID(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	email := "rot@x"
	ib := seedPortalClient(t, svc, 54102, email, "old-sub")

	newSub, err := svc.RotateSubID(inboundSvc, email)
	if err != nil {
		t.Fatalf("RotateSubID: %v", err)
	}
	if newSub == "" || newSub == "old-sub" {
		t.Fatalf("new subId = %q, want a fresh value", newSub)
	}
	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.SubID != newSub {
		t.Fatalf("record sub_id = %q, want %q", rec.SubID, newSub)
	}
	if rec.Email != email {
		t.Fatalf("email changed to %q", rec.Email)
	}
	if got := jsonClientSubID(t, inboundSvc, ib.Id, email); got != newSub {
		t.Fatalf("inbound JSON subId = %q, want %q", got, newSub)
	}
}

func jsonClientSubID(t *testing.T, inboundSvc *InboundService, inboundId int, email string) string {
	t.Helper()
	ib, err := inboundSvc.GetInbound(inboundId)
	if err != nil {
		t.Fatalf("GetInbound: %v", err)
	}
	var s struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &s); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	for _, c := range s.Clients {
		if c.Email == email {
			return c.SubID
		}
	}
	t.Fatalf("client %q not found in inbound %d settings", email, inboundId)
	return ""
}
