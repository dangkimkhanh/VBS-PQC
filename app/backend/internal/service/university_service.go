package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/common"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/repository"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const activationTTL = 72 * time.Hour

// activationResendCooldown stops repeated "resend" clicks from sending several
// links at once; each new link invalidates the previous one.
const activationResendCooldown = 5 * time.Minute

var (
	emailDomainPattern = regexp.MustCompile(`^[a-z0-9-]+(\.[a-z0-9-]+)+$`)
	phonePattern       = regexp.MustCompile(`^\+?[0-9][0-9 .-]{7,18}$`)

	ErrInvalidEmailDomain   = errors.New("Tên miền email không hợp lệ (ví dụ: actvn.edu.vn)")
	ErrInvalidPhone         = errors.New("Số điện thoại không hợp lệ")
	ErrAccountAlreadyActive = errors.New("Tài khoản quản trị đã được kích hoạt")
	ErrUniversityLocked     = errors.New("Trường đang bị khóa")
	// ErrActivationRecentlySent is wrapped with the time a new link can be sent.
	ErrActivationRecentlySent = errors.New("Link kích hoạt vừa được gửi")
)

type UniversityService interface {
	// CreateUniversity returns the created university and whether the activation email was sent.
	CreateUniversity(ctx context.Context, req *models.CreateUniversityRequest) (*models.University, bool, error)
	UpdateUniversity(ctx context.Context, id primitive.ObjectID, req *models.UpdateUniversityRequest) (bool, error)
	LockUniversity(ctx context.Context, id primitive.ObjectID) error
	UnlockUniversity(ctx context.Context, id primitive.ObjectID) error
	ResendActivation(ctx context.Context, universityID primitive.ObjectID) error
	ListUniversities(ctx context.Context, filter models.UniversityListFilter) ([]models.UniversityListItem, int64, error)
	Overview(ctx context.Context) (*models.PlatformOverview, error)
	RecentAdminEvents(ctx context.Context, limit int) ([]models.AdminAuditEvent, error)
	GetUniversityResponse(ctx context.Context, id primitive.ObjectID) (*models.UniversityResponse, error)
	GetUniversityByID(ctx context.Context, id primitive.ObjectID) (*models.University, error)
	GetUniversityByCode(ctx context.Context, code string) (*models.University, error)
}

type universityService struct {
	universityRepo repository.UniversityRepository
	authRepo       repository.AuthRepository
	adminRepo      repository.AdminRepository
	emailSender    utils.EmailSender
}

func NewUniversityService(
	universityRepo repository.UniversityRepository,
	authRepo repository.AuthRepository,
	adminRepo repository.AdminRepository,
	emailSender utils.EmailSender,
) UniversityService {
	return &universityService{
		universityRepo: universityRepo,
		authRepo:       authRepo,
		adminRepo:      adminRepo,
		emailSender:    emailSender,
	}
}

func normalizeDomain(domain string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), "@")
}

func validateContact(domain, phone string) error {
	if !emailDomainPattern.MatchString(domain) {
		return ErrInvalidEmailDomain
	}
	if !phonePattern.MatchString(strings.TrimSpace(phone)) {
		return ErrInvalidPhone
	}
	return nil
}

func conflictError(field string) error {
	switch field {
	case "university_name":
		return common.ErrUniversityNameExists
	case "email_domain":
		return common.ErrUniversityEmailDomainExists
	case "university_code":
		return common.ErrUniversityCodeExists
	}
	return nil
}

func (s *universityService) GetUniversityByID(ctx context.Context, id primitive.ObjectID) (*models.University, error) {
	university, err := s.universityRepo.FindByID(ctx, id)
	if err != nil || university == nil {
		return nil, common.ErrUniversityNotFound
	}
	return university, nil
}

func (s *universityService) GetUniversityByCode(ctx context.Context, code string) (*models.University, error) {
	return s.universityRepo.GetUniversityByCode(ctx, code)
}

func (s *universityService) CreateUniversity(ctx context.Context, req *models.CreateUniversityRequest) (*models.University, bool, error) {
	domain := normalizeDomain(req.EmailDomain)
	code := strings.ToUpper(strings.TrimSpace(req.UniversityCode))
	name := strings.TrimSpace(req.UniversityName)
	adminEmail := utils.NormalizeEmail(req.AdminEmail)
	if err := validateContact(domain, req.AdminPhone); err != nil {
		return nil, false, err
	}

	field, err := s.universityRepo.CheckUniversityConflicts(ctx, primitive.NilObjectID, name, domain, code)
	if err != nil {
		return nil, false, err
	}
	if err := conflictError(field); err != nil {
		return nil, false, err
	}
	taken, err := s.authRepo.IsLoginEmailTaken(ctx, adminEmail, primitive.NilObjectID)
	if err != nil {
		return nil, false, err
	}
	if taken {
		return nil, false, common.ErrAccountUniversityAlreadyExists
	}

	now := time.Now()
	uni := &models.University{
		ID:               primitive.NewObjectID(),
		UniversityName:   name,
		Address:          strings.TrimSpace(req.Address),
		EmailDomain:      domain,
		UniversityCode:   code,
		Website:          strings.TrimSpace(req.Website),
		AdminEmail:       adminEmail,
		AdminName:        strings.TrimSpace(req.AdminName),
		AdminPhone:       strings.TrimSpace(req.AdminPhone),
		SignerName:       strings.TrimSpace(req.SignerName),
		VerificationNote: strings.TrimSpace(req.VerificationNote),
		Description:      strings.TrimSpace(req.Description),
		Status:           models.UniversityActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.universityRepo.CreateUniversity(ctx, uni); err != nil {
		return nil, false, err
	}

	rawToken := utils.GenerateRandomCode(64)
	expiresAt := now.Add(activationTTL)
	account := &models.Account{
		ID: primitive.NewObjectID(), UniversityID: uni.ID, PersonalEmail: adminEmail, Status: models.AccountPending,
		ActivationTokenHash: utils.HashOpaqueToken(rawToken), ActivationExpiresAt: &expiresAt, CreatedAt: now, Role: "university_admin",
	}
	if err := s.authRepo.CreateAccount(ctx, account); err != nil {
		// Roll back so the admin can retry with the same name and code.
		if delErr := s.universityRepo.DeleteByID(ctx, uni.ID); delErr != nil {
			log.Printf("rollback of university %s failed: %v", uni.ID.Hex(), delErr)
		}
		return nil, false, err
	}

	if err := s.sendActivationEmail(uni, adminEmail, rawToken, expiresAt); err != nil {
		log.Printf("activation email for university %s failed: %v", uni.ID.Hex(), err)
		s.allowImmediateResend(ctx, account.ID, account.ActivationTokenHash, expiresAt)
		return uni, false, nil
	}
	return uni, true, nil
}

func (s *universityService) UpdateUniversity(ctx context.Context, id primitive.ObjectID, req *models.UpdateUniversityRequest) (bool, error) {
	university, err := s.GetUniversityByID(ctx, id)
	if err != nil {
		return false, err
	}
	domain := normalizeDomain(req.EmailDomain)
	name := strings.TrimSpace(req.UniversityName)
	adminEmail := utils.NormalizeEmail(req.AdminEmail)
	if err := validateContact(domain, req.AdminPhone); err != nil {
		return false, err
	}
	field, err := s.universityRepo.CheckUniversityConflicts(ctx, id, name, domain, "")
	if err != nil {
		return false, err
	}
	if err := conflictError(field); err != nil {
		return false, err
	}

	account, err := s.authRepo.FindByUniversityID(ctx, id)
	if err != nil || account == nil {
		return false, common.ErrAccountUniversityNotFound
	}
	emailChanged := adminEmail != utils.NormalizeEmail(account.PersonalEmail)
	if emailChanged {
		taken, err := s.authRepo.IsLoginEmailTaken(ctx, adminEmail, account.ID)
		if err != nil {
			return false, err
		}
		if taken {
			return false, common.ErrAccountUniversityAlreadyExists
		}
	}

	if err := s.universityRepo.UpdateDetails(ctx, id, bson.M{
		"university_name":   name,
		"address":           strings.TrimSpace(req.Address),
		"email_domain":      domain,
		"website":           strings.TrimSpace(req.Website),
		"admin_email":       adminEmail,
		"admin_name":        strings.TrimSpace(req.AdminName),
		"admin_phone":       strings.TrimSpace(req.AdminPhone),
		"signer_name":       strings.TrimSpace(req.SignerName),
		"verification_note": strings.TrimSpace(req.VerificationNote),
		"description":       strings.TrimSpace(req.Description),
	}); err != nil {
		return false, err
	}
	if !emailChanged {
		return false, nil
	}

	// A new person takes over: revoke the old login and send them an activation link.
	if err := s.authRepo.UpdateUniversityAdminEmail(ctx, account.ID, adminEmail); err != nil {
		return false, err
	}
	university.AdminEmail = adminEmail
	university.AdminName = strings.TrimSpace(req.AdminName)
	university.UniversityName = name
	if university.IsLocked() {
		// Keep the account locked; the link is sent when the university is unlocked.
		return false, s.authRepo.SetStatus(ctx, account.ID, models.AccountLocked)
	}
	return s.issueActivation(ctx, university, account.ID, adminEmail) == nil, nil
}

func (s *universityService) LockUniversity(ctx context.Context, id primitive.ObjectID) error {
	if _, err := s.GetUniversityByID(ctx, id); err != nil {
		return err
	}
	account, err := s.authRepo.FindByUniversityID(ctx, id)
	if err != nil || account == nil {
		return common.ErrAccountUniversityNotFound
	}
	if err := s.universityRepo.UpdateStatus(ctx, id, models.UniversityLocked); err != nil {
		return err
	}
	// Locking the account also invalidates any token already issued (see ValidateSession).
	return s.authRepo.SetStatus(ctx, account.ID, models.AccountLocked)
}

func (s *universityService) UnlockUniversity(ctx context.Context, id primitive.ObjectID) error {
	university, err := s.GetUniversityByID(ctx, id)
	if err != nil {
		return err
	}
	account, err := s.authRepo.FindByUniversityID(ctx, id)
	if err != nil || account == nil {
		return common.ErrAccountUniversityNotFound
	}
	if err := s.universityRepo.UpdateStatus(ctx, id, models.UniversityActive); err != nil {
		return err
	}
	if account.PasswordHash != "" {
		return s.authRepo.SetStatus(ctx, account.ID, models.AccountActive)
	}
	// Never activated (or the admin email changed while locked): send a fresh link.
	return s.issueActivation(ctx, university, account.ID, account.PersonalEmail)
}

func (s *universityService) ResendActivation(ctx context.Context, universityID primitive.ObjectID) error {
	university, err := s.GetUniversityByID(ctx, universityID)
	if err != nil {
		return err
	}
	if university.IsLocked() {
		return ErrUniversityLocked
	}
	account, err := s.authRepo.FindByUniversityID(ctx, universityID)
	if err != nil || account == nil {
		return common.ErrAccountUniversityNotFound
	}
	if account.Status == models.AccountActive {
		return ErrAccountAlreadyActive
	}
	if at := activationResendAt(account.ActivationExpiresAt); at != nil && time.Now().Before(*at) {
		wait := int(time.Until(*at).Minutes()) + 1
		return fmt.Errorf("%w lúc %s. Vui lòng đợi khoảng %d phút rồi gửi lại, và dùng email mới nhất",
			ErrActivationRecentlySent, at.Add(-activationResendCooldown).In(vietnamLocation()).Format("15:04"), wait)
	}
	return s.issueActivation(ctx, university, account.ID, account.PersonalEmail)
}

// activationResendAt is when another activation link may be sent, derived from
// the current link's expiry (links are valid for activationTTL from sending).
func activationResendAt(expiresAt *time.Time) *time.Time {
	if expiresAt == nil {
		return nil
	}
	at := expiresAt.Add(-activationTTL + activationResendCooldown)
	return &at
}

func (s *universityService) issueActivation(ctx context.Context, university *models.University, accountID primitive.ObjectID, email string) error {
	rawToken := utils.GenerateRandomCode(64)
	expiresAt := time.Now().Add(activationTTL)
	tokenHash := utils.HashOpaqueToken(rawToken)
	if err := s.authRepo.SetActivationToken(ctx, accountID, tokenHash, expiresAt); err != nil {
		return err
	}
	if err := s.sendActivationEmail(university, email, rawToken, expiresAt); err != nil {
		log.Printf("activation email for university %s failed: %v", university.ID.Hex(), err)
		s.allowImmediateResend(ctx, accountID, tokenHash, expiresAt)
		return common.ErrUniversityApprovalEmailFailed
	}
	return nil
}

// allowImmediateResend is used when the activation email could not be sent: the
// resend cooldown is derived from the link's expiry, so moving the expiry back
// by the cooldown lets the admin retry at once.
func (s *universityService) allowImmediateResend(ctx context.Context, accountID primitive.ObjectID, tokenHash string, expiresAt time.Time) {
	if err := s.authRepo.SetActivationToken(ctx, accountID, tokenHash, expiresAt.Add(-activationResendCooldown)); err != nil {
		log.Printf("could not reopen activation resend for account %s: %v", accountID.Hex(), err)
	}
}

func (s *universityService) sendActivationEmail(university *models.University, email, rawToken string, expiresAt time.Time) error {
	activationURL := fmt.Sprintf("%s/auth/activate?token=%s", appURL(), rawToken)
	loc := vietnamLocation()
	sentAt := expiresAt.Add(-activationTTL).In(loc)
	support := ""
	if contact := strings.TrimSpace(os.Getenv("SUPPORT_CONTACT")); contact != "" {
		support = "\n\nNếu cần hỗ trợ, vui lòng liên hệ: " + contact
	}
	body := fmt.Sprintf(`Xin chào %s,

Tài khoản quản trị của %s đã được tạo trên hệ thống Văn bằng số PQC.

- Email đăng nhập: %s
- Liên kết kích hoạt: %s
- Gửi lúc: %s
- Hiệu lực đến: %s

Hãy mở liên kết để tự đặt mật khẩu. Liên kết chỉ dùng được một lần; nếu bạn nhận được nhiều email kích hoạt, chỉ liên kết trong email mới nhất còn hiệu lực. Hệ thống không bao giờ gửi mật khẩu qua email.%s`,
		university.AdminName, university.UniversityName, email, activationURL, sentAt.Format("15:04 02/01/2006"), expiresAt.In(loc).Format("15:04 02/01/2006"), support)
	// The send time in the subject keeps each email in its own Gmail thread.
	subject := fmt.Sprintf("Kích hoạt tài khoản quản trị trường - VBS PQC (%s)", sentAt.Format("15:04 02/01"))
	return s.emailSender.SendEmail(email, subject, body)
}

func (s *universityService) toResponse(ctx context.Context, u *models.University, loc *time.Location) models.UniversityResponse {
	accountStatus := ""
	var activationExpiresAt *time.Time
	if account, err := s.authRepo.FindByUniversityID(ctx, u.ID); err == nil && account != nil {
		accountStatus = account.Status
		activationExpiresAt = account.ActivationExpiresAt
	}
	return universityResponse(u, accountStatus, activationExpiresAt, loc)
}

func universityResponse(u *models.University, accountStatus string, activationExpiresAt *time.Time, loc *time.Location) models.UniversityResponse {
	status := models.UniversityActive
	if u.IsLocked() {
		status = models.UniversityLocked
	}
	resendAt := ""
	if at := activationResendAt(activationExpiresAt); at != nil && accountStatus == models.AccountPending {
		resendAt = at.UTC().Format(time.RFC3339)
	}
	return models.UniversityResponse{
		ActivationResendAt: resendAt,
		ID:                 u.ID.Hex(),
		UniversityName:     u.UniversityName,
		UniversityCode:     u.UniversityCode,
		EmailDomain:        u.EmailDomain,
		Address:            u.Address,
		Website:            u.Website,
		AdminEmail:         u.AdminEmail,
		AdminName:          u.AdminName,
		AdminPhone:         u.AdminPhone,
		SignerName:         u.SignerName,
		VerificationNote:   u.VerificationNote,
		Status:             status,
		AdminAccountStatus: accountStatus,
		Description:        u.Description,
		CreatedAt:          u.CreatedAt.In(loc).Format("2006-01-02 15:04:05"),
		UpdatedAt:          u.UpdatedAt.In(loc).Format("2006-01-02 15:04:05"),
	}
}

func vietnamLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("ICT", 7*60*60)
	}
	return loc
}

func (s *universityService) ListUniversities(ctx context.Context, filter models.UniversityListFilter) ([]models.UniversityListItem, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	rows, total, err := s.adminRepo.SearchUniversities(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.withStats(ctx, rows)
	return items, total, err
}

func (s *universityService) withStats(ctx context.Context, rows []repository.UniversityWithAccount) ([]models.UniversityListItem, error) {
	ids := make([]primitive.ObjectID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	stats, err := s.adminRepo.StatsByUniversity(ctx, ids)
	if err != nil {
		return nil, err
	}
	loc := vietnamLocation()
	items := make([]models.UniversityListItem, 0, len(rows))
	for i := range rows {
		items = append(items, models.UniversityListItem{
			UniversityResponse: universityResponse(&rows[i].University, rows[i].AdminAccountStatus, rows[i].AdminActivationExpiresAt, loc),
			Stats:              stats[rows[i].ID],
		})
	}
	return items, nil
}

func (s *universityService) Overview(ctx context.Context) (*models.PlatformOverview, error) {
	overview := &models.PlatformOverview{}
	if err := s.adminRepo.PlatformCounts(ctx, overview); err != nil {
		return nil, err
	}

	topIDs, err := s.adminRepo.TopIssuingUniversities(ctx, 5)
	if err != nil {
		return nil, err
	}
	top := make([]repository.UniversityWithAccount, 0, len(topIDs))
	for _, id := range topIDs {
		if u, err := s.universityRepo.FindByID(ctx, id); err == nil && u != nil {
			top = append(top, repository.UniversityWithAccount{University: *u})
		}
	}
	if overview.TopUniversities, err = s.withStats(ctx, top); err != nil {
		return nil, err
	}

	pending, _, err := s.adminRepo.SearchUniversities(ctx, models.UniversityListFilter{Status: "pending", Page: 1, PageSize: 10})
	if err != nil {
		return nil, err
	}
	loc := vietnamLocation()
	overview.PendingActivations = make([]models.UniversityResponse, 0, len(pending))
	for i := range pending {
		overview.PendingActivations = append(overview.PendingActivations, universityResponse(&pending[i].University, pending[i].AdminAccountStatus, pending[i].AdminActivationExpiresAt, loc))
	}
	return overview, nil
}

// RecentAdminEvents lists platform-admin actions on universities, newest first.
func (s *universityService) RecentAdminEvents(ctx context.Context, limit int) ([]models.AdminAuditEvent, error) {
	events, err := s.adminRepo.RecentAdminEvents(ctx, limit)
	if err != nil {
		return nil, err
	}
	loc := vietnamLocation()
	names := map[string]*models.University{}
	result := make([]models.AdminAuditEvent, 0, len(events))
	for _, e := range events {
		event := models.AdminAuditEvent{}
		event.Action, _ = e["action"].(string)
		event.Method, _ = e["method"].(string)
		event.Success, _ = e["success"].(bool)
		event.UniversityID, _ = e["university_id"].(string)
		if at, ok := e["at"].(primitive.DateTime); ok {
			event.At = at.Time().In(loc).Format("2006-01-02 15:04:05")
		}
		if _, seen := names[event.UniversityID]; !seen {
			names[event.UniversityID] = nil
			if id, err := primitive.ObjectIDFromHex(event.UniversityID); err == nil {
				if u, err := s.universityRepo.FindByID(ctx, id); err == nil {
					names[event.UniversityID] = u
				}
			}
		}
		if u := names[event.UniversityID]; u != nil {
			event.UniversityName, event.UniversityCode = u.UniversityName, u.UniversityCode
		}
		result = append(result, event)
	}
	return result, nil
}

func (s *universityService) GetUniversityResponse(ctx context.Context, id primitive.ObjectID) (*models.UniversityResponse, error) {
	university, err := s.GetUniversityByID(ctx, id)
	if err != nil {
		return nil, err
	}
	res := s.toResponse(ctx, university, vietnamLocation())
	return &res, nil
}
