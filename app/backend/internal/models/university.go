package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	UniversityActive = "active"
	UniversityLocked = "locked"
)

type University struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"`
	UniversityName   string             `bson:"university_name"`
	UniversityCode   string             `bson:"university_code"`
	Address          string             `bson:"address"`
	EmailDomain      string             `bson:"email_domain"`
	Website          string             `bson:"website,omitempty"`
	AdminEmail       string             `bson:"admin_email,omitempty"`
	AdminName        string             `bson:"admin_name,omitempty"`
	AdminPhone       string             `bson:"admin_phone,omitempty"`
	SignerName       string             `bson:"signer_name,omitempty"` // Printed under the signer title on diplomas
	VerificationNote string             `bson:"verification_note,omitempty"`
	Status           string             `bson:"status"` // "active" | "locked" (older records may say "approved")
	Description      string             `bson:"description"`
	CreatedAt        time.Time          `bson:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at"`
}

// IsLocked treats legacy statuses ("approved", "pending") as not locked.
func (u *University) IsLocked() bool {
	return u.Status == UniversityLocked
}

type CreateUniversityRequest struct {
	UniversityName   string `json:"university_name" binding:"required,max=200"`
	UniversityCode   string `json:"university_code" binding:"required,max=30"`
	Address          string `json:"address" binding:"required,max=300"`
	EmailDomain      string `json:"email_domain" binding:"required,max=100"`
	Website          string `json:"website" binding:"omitempty,url"`
	AdminEmail       string `json:"admin_email" binding:"required,email"`
	AdminName        string `json:"admin_name" binding:"required,max=100"`
	AdminPhone       string `json:"admin_phone" binding:"required,max=20"`
	SignerName       string `json:"signer_name" binding:"max=100"`
	VerificationNote string `json:"verification_note" binding:"max=1000"`
	Description      string `json:"description" binding:"max=1000"`
}

// UpdateUniversityRequest cannot change university_code: it is embedded in
// issued diplomas and file paths.
type UpdateUniversityRequest struct {
	UniversityName   string `json:"university_name" binding:"required,max=200"`
	Address          string `json:"address" binding:"required,max=300"`
	EmailDomain      string `json:"email_domain" binding:"required,max=100"`
	Website          string `json:"website" binding:"omitempty,url"`
	AdminEmail       string `json:"admin_email" binding:"required,email"`
	AdminName        string `json:"admin_name" binding:"required,max=100"`
	AdminPhone       string `json:"admin_phone" binding:"required,max=20"`
	SignerName       string `json:"signer_name" binding:"max=100"`
	VerificationNote string `json:"verification_note" binding:"max=1000"`
	Description      string `json:"description" binding:"max=1000"`
}

type UniversityResponse struct {
	ID                 string `json:"id"`
	UniversityName     string `json:"university_name"`
	UniversityCode     string `json:"university_code"`
	EmailDomain        string `json:"email_domain"`
	Website            string `json:"website,omitempty"`
	AdminEmail         string `json:"admin_email,omitempty"`
	AdminName          string `json:"admin_name,omitempty"`
	AdminPhone         string `json:"admin_phone,omitempty"`
	SignerName         string `json:"signer_name,omitempty"`
	VerificationNote   string `json:"verification_note,omitempty"`
	Address            string `json:"address"`
	Status             string `json:"status"`
	AdminAccountStatus string `json:"admin_account_status"`
	// ActivationResendAt (RFC 3339) is when the activation link may be resent; set while pending.
	ActivationResendAt string `json:"activation_resend_at,omitempty"`
	Description        string `json:"description"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}
