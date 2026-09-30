package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	AccountPending = "pending"
	AccountActive  = "active"
	AccountLocked  = "locked"
)

type Account struct {
	ID                       primitive.ObjectID `bson:"_id,omitempty"`
	StudentID                primitive.ObjectID `bson:"student_id"`
	UniversityID             primitive.ObjectID `bson:"university_id,omitempty"`
	StudentEmail             string             `bson:"student_email"`  // Email do trường cấp, dùng để xác thực OTP
	PersonalEmail            string             `bson:"personal_email"` // Email đăng nhập (cá nhân với sinh viên, quản trị với trường)
	PasswordHash             string             `bson:"password_hash"`
	CreatedAt                time.Time          `bson:"created_at"`
	Role                     string             `bson:"role"`
	Status                   string             `bson:"status"`
	ActivationTokenHash      string             `bson:"activation_token_hash,omitempty"`
	ActivationExpiresAt      *time.Time         `bson:"activation_expires_at,omitempty"`
	ActivatedAt              *time.Time         `bson:"activated_at,omitempty"`
	PasswordChangedAt        *time.Time         `bson:"password_changed_at,omitempty"`
	PasswordResetTokenHash   string             `bson:"password_reset_token_hash,omitempty"`
	PasswordResetExpiresAt   *time.Time         `bson:"password_reset_expires_at,omitempty"`
	OTPVerificationTokenHash string             `bson:"otp_verification_token_hash,omitempty"`
	OTPVerificationExpiresAt *time.Time         `bson:"otp_verification_expires_at,omitempty"`
}

type AccountResponse struct {
	ID            primitive.ObjectID  `json:"id"`
	StudentID     *primitive.ObjectID `json:"student_id,omitempty"`
	UniversityID  *primitive.ObjectID `json:"university_id,omitempty"`
	StudentEmail  string              `json:"student_email,omitempty"`
	PersonalEmail string              `json:"personal_email"`
	CreatedAt     string              `json:"created_at"`
	Role          string              `json:"role"`
	Status        string              `json:"status"`
}

// OTP is stored in its own collection; only the hash of the code is kept.
type OTP struct {
	Email     string    `bson:"email"`
	CodeHash  string    `bson:"code_hash"`
	Attempts  int       `bson:"attempts"`
	CreatedAt time.Time `bson:"created_at"`
	ExpiresAt time.Time `bson:"expires_at"`
}

type RequestOTPInput struct {
	StudentEmail string `json:"student_email" binding:"required,email"`
}

type VerifyOTPRequest struct {
	StudentEmail string `json:"student_email" binding:"required,email"`
	OTP          string `json:"otp" binding:"required,len=6,numeric"`
}

type VerifyOTPResponse struct {
	UserID            string `json:"user_id"`
	VerificationToken string `json:"verification_token"`
}

type RegisterRequest struct {
	UserID        string `json:"user_id" binding:"required"`
	PersonalEmail string `json:"personal_email" binding:"required,email"`
	Password      string `json:"password" binding:"required,min=8,max=72"`
	OTPToken      string `json:"otp_token" binding:"required"`
}
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
	Role  string `json:"role"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

type ActivateAccountRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}
type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}
