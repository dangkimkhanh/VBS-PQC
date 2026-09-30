package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/common"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/repository"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	otpLength       = 6
	otpTTL          = 5 * time.Minute
	otpMaxAttempts  = 5
	otpSessionTTL   = 15 * time.Minute
	resetTokenTTL   = 20 * time.Minute
	emailRateWindow = time.Minute
)

var (
	ErrInvalidCredentials = errors.New("Email hoặc mật khẩu không đúng")
	ErrAccountInactive    = errors.New("Tài khoản chưa được kích hoạt hoặc đã bị khóa")
	ErrOTPInvalid         = errors.New("Mã OTP không đúng hoặc đã hết hạn")
	ErrOTPTooManyAttempts = errors.New("Bạn đã nhập sai quá nhiều lần. Vui lòng yêu cầu mã OTP mới")
	ErrOTPSessionInvalid  = errors.New("Phiên xác thực OTP không hợp lệ hoặc đã hết hạn. Vui lòng xác thực lại")
	ErrLoginEmailTaken    = errors.New("Email này đã được dùng cho một tài khoản khác")
	ErrSessionRevoked     = errors.New("session revoked")
)

// dummyHash keeps login timing similar whether or not the account exists.
var dummyHash, _ = utils.HashPassword("vbs-pqc-timing-equalizer")

type AuthService interface {
	RequestOTP(ctx context.Context, input models.RequestOTPInput) error
	VerifyOTP(ctx context.Context, req *models.VerifyOTPRequest) (*models.VerifyOTPResponse, error)
	Register(ctx context.Context, req models.RegisterRequest) error
	Login(ctx context.Context, email, password string) (*models.Account, error)
	ValidateSession(ctx context.Context, accountID string, issuedAt time.Time) error
	ChangePassword(ctx context.Context, accountID primitive.ObjectID, oldPass, newPass string) error
	GetAllAccounts(ctx context.Context, page, pageSize int) ([]*models.Account, int64, error)
	GetAccountByID(ctx context.Context, id primitive.ObjectID) (*models.Account, error)
	GetAccountsByRole(ctx context.Context, role string) ([]models.Account, error)
	ActivateAccount(ctx context.Context, req models.ActivateAccountRequest) error
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, req models.ResetPasswordRequest) error
}

type authService struct {
	authRepo    repository.AuthRepository
	userRepo    repository.UserRepository
	emailSender utils.EmailSender
	emailRateMu sync.Mutex
	emailRate   map[string]time.Time
}

func NewAuthService(authRepo repository.AuthRepository, userRepo repository.UserRepository, emailSender utils.EmailSender) AuthService {
	return &authService{
		authRepo:    authRepo,
		userRepo:    userRepo,
		emailSender: emailSender,
		emailRate:   make(map[string]time.Time),
	}
}

func (s *authService) GetAccountByID(ctx context.Context, id primitive.ObjectID) (*models.Account, error) {
	return s.authRepo.FindByID(ctx, id)
}

// RequestOTP never tells the caller whether the email exists or is already
// registered; the handler always answers with the same message.
func (s *authService) RequestOTP(ctx context.Context, input models.RequestOTPInput) error {
	email := utils.NormalizeEmail(input.StudentEmail)
	if !s.allowEmail("otp:" + email) {
		return common.ErrRateLimited
	}
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("lỗi khi tìm user theo email: %w", err)
	}
	if user == nil {
		return common.ErrUserNotExisted
	}
	account, err := s.authRepo.FindPersonalAccountByUserID(ctx, user.ID)
	if err != nil {
		return common.ErrCheckingPersonalAccount
	}
	if account != nil && account.Status != models.AccountPending {
		return common.ErrPersonalAccountAlreadyExist
	}

	code := utils.GenerateNumericCode(otpLength)
	now := time.Now()
	if err := s.authRepo.SaveOTP(ctx, models.OTP{
		Email: email, CodeHash: utils.HashOpaqueToken(code), CreatedAt: now, ExpiresAt: now.Add(otpTTL),
	}); err != nil {
		return fmt.Errorf("lỗi khi lưu OTP: %w", err)
	}

	body := fmt.Sprintf("Mã OTP kích hoạt tài khoản VBS PQC của bạn là: %s\n\nMã có hiệu lực trong %d phút và chỉ dùng được một lần. Không chia sẻ mã này với bất kỳ ai.", code, int(otpTTL.Minutes()))
	s.sendAsync(email, "Mã xác thực OTP - VBS PQC", body)
	return nil
}

func (s *authService) VerifyOTP(ctx context.Context, input *models.VerifyOTPRequest) (*models.VerifyOTPResponse, error) {
	email := utils.NormalizeEmail(input.StudentEmail)
	otp, err := s.authRepo.FindOTPByEmail(ctx, email)
	if err != nil || otp == nil || time.Now().After(otp.ExpiresAt) {
		return nil, ErrOTPInvalid
	}
	if otp.Attempts >= otpMaxAttempts {
		_ = s.authRepo.DeleteOTP(ctx, email)
		return nil, ErrOTPTooManyAttempts
	}
	if subtle.ConstantTimeCompare([]byte(utils.HashOpaqueToken(input.OTP)), []byte(otp.CodeHash)) != 1 {
		_ = s.authRepo.IncrementOTPAttempts(ctx, email)
		return nil, ErrOTPInvalid
	}
	if err := s.authRepo.DeleteOTP(ctx, email); err != nil {
		return nil, errors.New("không thể vô hiệu hóa OTP")
	}

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil || user == nil {
		return nil, ErrOTPInvalid
	}
	account, err := s.authRepo.FindPersonalAccountByUserID(ctx, user.ID)
	if err != nil {
		return nil, errors.New("không thể tải tài khoản sinh viên")
	}
	if account == nil {
		account = &models.Account{
			ID: primitive.NewObjectID(), StudentID: user.ID, UniversityID: user.UniversityID, StudentEmail: user.Email,
			CreatedAt: time.Now(), Role: "student", Status: models.AccountPending,
		}
		if err := s.authRepo.CreateAccount(ctx, account); err != nil {
			return nil, errors.New("không thể khởi tạo tài khoản sinh viên")
		}
	}
	if account.Status != models.AccountPending {
		return nil, ErrOTPInvalid
	}

	token := utils.GenerateRandomCode(64)
	if err := s.authRepo.SetOTPVerificationToken(ctx, account.ID, utils.HashOpaqueToken(token), time.Now().Add(otpSessionTTL)); err != nil {
		return nil, errors.New("không thể lưu phiên xác thực OTP")
	}
	return &models.VerifyOTPResponse{UserID: user.ID.Hex(), VerificationToken: token}, nil
}

func (s *authService) Register(ctx context.Context, req models.RegisterRequest) error {
	userID, err := primitive.ObjectIDFromHex(req.UserID)
	if err != nil {
		return ErrOTPSessionInvalid
	}
	account, err := s.authRepo.FindPersonalAccountByUserID(ctx, userID)
	if err != nil || account == nil || account.Status != models.AccountPending ||
		account.OTPVerificationTokenHash == "" || account.OTPVerificationExpiresAt == nil ||
		time.Now().After(*account.OTPVerificationExpiresAt) ||
		subtle.ConstantTimeCompare([]byte(utils.HashOpaqueToken(req.OTPToken)), []byte(account.OTPVerificationTokenHash)) != 1 {
		return ErrOTPSessionInvalid
	}

	personalEmail := utils.NormalizeEmail(req.PersonalEmail)
	taken, err := s.authRepo.IsLoginEmailTaken(ctx, personalEmail, account.ID)
	if err != nil {
		return fmt.Errorf("lỗi kiểm tra email: %w", err)
	}
	if taken {
		return ErrLoginEmailTaken
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return fmt.Errorf("lỗi hash mật khẩu: %w", err)
	}
	return s.authRepo.ActivateStudentAccount(ctx, account.ID, personalEmail, hash, time.Now())
}

func (s *authService) Login(ctx context.Context, email, password string) (*models.Account, error) {
	account, err := s.authRepo.FindByLoginEmail(ctx, email)
	if err != nil || account == nil || account.PasswordHash == "" {
		utils.ComparePassword(dummyHash, password)
		return nil, ErrInvalidCredentials
	}
	if !utils.ComparePassword(account.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	// Status is revealed only to someone who already knows the password.
	if account.Status != "" && account.Status != models.AccountActive {
		return nil, ErrAccountInactive
	}
	return account, nil
}

// ValidateSession rejects tokens of accounts that were locked, deleted, or
// whose password changed after the token was issued.
func (s *authService) ValidateSession(ctx context.Context, accountID string, issuedAt time.Time) error {
	id, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		return ErrSessionRevoked
	}
	account, err := s.authRepo.FindByID(ctx, id)
	if err != nil || account == nil {
		return ErrSessionRevoked
	}
	if account.Status != "" && account.Status != models.AccountActive {
		return ErrSessionRevoked
	}
	if account.PasswordChangedAt != nil && issuedAt.Before(account.PasswordChangedAt.Truncate(time.Second)) {
		return ErrSessionRevoked
	}
	return nil
}

func (s *authService) GetAllAccounts(ctx context.Context, page, pageSize int) ([]*models.Account, int64, error) {
	return s.authRepo.GetAllAccounts(ctx, page, pageSize)
}

func (s *authService) ChangePassword(ctx context.Context, accountID primitive.ObjectID, oldPass, newPass string) error {
	account, err := s.authRepo.FindByID(ctx, accountID)
	if err != nil || account == nil {
		return common.ErrAccountNotFound
	}
	if !utils.CheckPasswordHash(oldPass, account.PasswordHash) {
		return common.ErrInvalidOldPassword
	}
	newHash, err := utils.HashPassword(newPass)
	if err != nil {
		return err
	}
	return s.authRepo.UpdatePassword(ctx, accountID, newHash)
}

func (s *authService) GetAccountsByRole(ctx context.Context, role string) ([]models.Account, error) {
	return s.authRepo.FindByRole(ctx, role)
}

func (s *authService) ActivateAccount(ctx context.Context, req models.ActivateAccountRequest) error {
	account, err := s.authRepo.FindByActivationTokenHash(ctx, utils.HashOpaqueToken(req.Token))
	if err != nil || account == nil || account.Status != models.AccountPending ||
		account.ActivationExpiresAt == nil || time.Now().After(*account.ActivationExpiresAt) {
		return errors.New("Liên kết kích hoạt không hợp lệ hoặc đã hết hạn")
	}
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return err
	}
	return s.authRepo.ActivateAccount(ctx, account.ID, hash, time.Now())
}

// RequestPasswordReset always succeeds from the caller's point of view.
func (s *authService) RequestPasswordReset(ctx context.Context, email string) error {
	email = utils.NormalizeEmail(email)
	if !s.allowEmail("reset:" + email) {
		return nil
	}
	account, err := s.authRepo.FindByLoginEmail(ctx, email)
	if err != nil || account == nil || account.PersonalEmail == "" || account.Status == models.AccountLocked {
		return nil
	}
	// Students must finish OTP registration first; they have no password yet.
	if account.Role == "student" && account.Status != models.AccountActive {
		return nil
	}
	token := utils.GenerateRandomCode(64)
	if err := s.authRepo.SetPasswordResetToken(ctx, account.ID, utils.HashOpaqueToken(token), time.Now().Add(resetTokenTTL)); err != nil {
		return err
	}
	body := fmt.Sprintf("Xin chào,\n\nLiên kết đặt lại mật khẩu tài khoản VBS PQC: %s/auth/reset-password?token=%s\n\nLiên kết có hiệu lực trong %d phút và chỉ dùng một lần. Không chia sẻ liên kết này.\nNếu bạn không yêu cầu thao tác này, hãy bỏ qua email.", appURL(), token, int(resetTokenTTL.Minutes()))
	s.sendAsync(account.PersonalEmail, "Đặt lại mật khẩu tài khoản VBS PQC", body)
	return nil
}

func (s *authService) ResetPassword(ctx context.Context, req models.ResetPasswordRequest) error {
	account, err := s.authRepo.FindByPasswordResetTokenHash(ctx, utils.HashOpaqueToken(req.Token))
	if err != nil || account == nil || account.Status == models.AccountLocked ||
		account.PasswordResetExpiresAt == nil || time.Now().After(*account.PasswordResetExpiresAt) {
		return errors.New("Liên kết đặt lại mật khẩu không hợp lệ hoặc đã hết hạn")
	}
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return err
	}
	return s.authRepo.ResetPassword(ctx, account.ID, hash)
}

func (s *authService) allowEmail(key string) bool {
	s.emailRateMu.Lock()
	defer s.emailRateMu.Unlock()
	now := time.Now()
	for k, last := range s.emailRate {
		if now.Sub(last) > emailRateWindow {
			delete(s.emailRate, k)
		}
	}
	if last, ok := s.emailRate[key]; ok && now.Sub(last) < emailRateWindow {
		return false
	}
	s.emailRate[key] = now
	return true
}

// sendAsync keeps response time independent of whether an email was sent.
func (s *authService) sendAsync(to, subject, body string) {
	go func() {
		if err := s.emailSender.SendEmail(to, subject, body); err != nil {
			log.Printf("send email %q failed: %v", subject, err)
		}
	}()
}

func appURL() string {
	if url := os.Getenv("APP_URL"); url != "" {
		return url
	}
	return "http://localhost:3000"
}
