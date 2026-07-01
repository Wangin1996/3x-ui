package middleware

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestNodeAgentValidationTag(t *testing.T) {
	agent := &model.Node{Name: "x", Mode: "agent", Address: "", ApiToken: ""}
	if err := validate.Struct(agent); err != nil {
		t.Fatalf("agent node with empty address/token should pass validation, got: %v", err)
	}

	push := &model.Node{Name: "x", Mode: "push", Address: "", ApiToken: "tok", Port: 443, TlsVerifyMode: "verify"}
	if err := validate.Struct(push); err == nil {
		t.Fatalf("push node with empty address should FAIL validation")
	}
}
