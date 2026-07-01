package service

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"

	"github.com/google/uuid"
)

// SetLoginPassword sets, or clears when plain is empty, the end-user portal
// login password for the client with the given email. The bcrypt hash lives in
// the clients.password_hash column, kept separate from the protocol credential
// columns (Password/Auth). Clearing it disables portal login for that client.
func (s *ClientService) SetLoginPassword(email, plain string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return err
	}
	hash := ""
	if plain != "" {
		h, herr := crypto.HashPasswordAsBcrypt(plain)
		if herr != nil {
			return herr
		}
		hash = h
	}
	return database.GetDB().Model(&model.ClientRecord{}).
		Where("id = ?", rec.Id).
		Update("password_hash", hash).Error
}

// CheckLoginPassword verifies an end-user portal login and returns the matching
// client record. A client with no password set cannot log in. Every failure
// returns the same opaque error so the caller can present one generic message.
func (s *ClientService) CheckLoginPassword(email, plain string) (*model.ClientRecord, error) {
	email = strings.TrimSpace(email)
	if email == "" || plain == "" {
		return nil, common.NewError("invalid credentials")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return nil, common.NewError("invalid credentials")
	}
	if rec.PasswordHash == "" || !crypto.CheckPasswordHash(rec.PasswordHash, plain) {
		return nil, common.NewError("invalid credentials")
	}
	return rec, nil
}

// RotateSubID assigns the client a fresh subscription id, updating both the
// clients table and every attached inbound's settings JSON — the subscription
// server matches subId against both (getInboundsBySubId on the table,
// matchingClients on the JSON). Old subscription URLs stop resolving by design.
func (s *ClientService) RotateSubID(inboundSvc *InboundService, email string) (string, error) {
	rec, err := s.GetRecordByEmail(nil, strings.TrimSpace(email))
	if err != nil {
		return "", err
	}
	client := rec.ToClient()
	client.SubID = uuid.NewString()
	if _, err := s.Update(inboundSvc, rec.Id, *client); err != nil {
		return "", err
	}
	return client.SubID, nil
}
