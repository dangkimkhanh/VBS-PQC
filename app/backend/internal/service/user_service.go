package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/common"
	"github.com/vnkmasc/Kmasc/app/backend/internal/mapper"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/repository"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserService interface {
	GetUserByID(ctx context.Context, id primitive.ObjectID) (*models.UserResponse, error)
	SearchUsers(ctx context.Context, params models.SearchUserParams) ([]models.UserResponse, int64, error)
	CreateUser(ctx context.Context, claims *utils.CustomClaims, req *models.CreateUserRequest) (*models.UserResponse, error)
	DeleteUser(ctx context.Context, id primitive.ObjectID) error
	UpdateUser(ctx context.Context, id primitive.ObjectID, req models.UpdateUserRequest) error
	GetMyProfile(ctx context.Context) (*models.UserResponse, error)
}

type userService struct {
	userRepo       repository.UserRepository
	universityRepo repository.UniversityRepository
	facultyRepo    repository.FacultyRepository
	authRepo       repository.AuthRepository
	ediplomaRepo   repository.EDiplomaRepository
}

var (
	ErrStudentEmailDomain = errors.New("Email sinh viên phải thuộc tên miền email của trường")
	ErrStudentHasDiplomas = errors.New("Sinh viên đã có văn bằng, không thể xóa hồ sơ")
)

func NewUserService(
	userRepo repository.UserRepository,
	universityRepo repository.UniversityRepository,
	facultyRepo repository.FacultyRepository,
	authRepo repository.AuthRepository,
	ediplomaRepo repository.EDiplomaRepository,
) UserService {
	return &userService{
		userRepo:       userRepo,
		universityRepo: universityRepo,
		facultyRepo:    facultyRepo,
		authRepo:       authRepo,
		ediplomaRepo:   ediplomaRepo,
	}
}

func (s *userService) GetUserByID(ctx context.Context, id primitive.ObjectID) (*models.UserResponse, error) {
	user, err := s.userRepo.GetUserByID(ctx, id)
	if err != nil || user == nil {
		return nil, common.ErrUserNotExisted
	}

	faculty, err := s.facultyRepo.FindByID(ctx, user.FacultyID)
	if err != nil || faculty == nil {
		return nil, common.ErrFacultyNotFound
	}

	university, err := s.universityRepo.FindByID(ctx, user.UniversityID)
	if err != nil || university == nil {
		return nil, common.ErrUniversityNotFound
	}

	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		log.Printf("Error loading timezone: %v. Fallback to UTC", err)
		loc = time.UTC
	}

	resp := mapper.MapUserToResponse(user, faculty, university, loc)
	return &resp, nil
}

func (s *userService) SearchUsers(ctx context.Context, params models.SearchUserParams) ([]models.UserResponse, int64, error) {
	claimsVal := ctx.Value(utils.ClaimsContextKey)
	claims, ok := claimsVal.(*utils.CustomClaims)
	if !ok || claims == nil {
		return nil, 0, common.ErrUnauthorized
	}
	universityID, err := primitive.ObjectIDFromHex(claims.UniversityID)
	if err != nil {
		return nil, 0, common.ErrInvalidToken
	}
	params.UniversityID = universityID

	users, total, err := s.userRepo.SearchUsers(ctx, params)
	if err != nil {
		return nil, 0, err
	}

	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		log.Printf("Error loading timezone Asia/Ho_Chi_Minh: %v. Using UTC instead.", err)
		loc = time.UTC
	}

	var responses []models.UserResponse
	for _, u := range users {
		faculty, _ := s.facultyRepo.FindByID(ctx, u.FacultyID)
		university, _ := s.universityRepo.FindByID(ctx, u.UniversityID)

		resp := mapper.MapUserToResponse(u, faculty, university, loc)
		responses = append(responses, resp)
	}

	return responses, total, nil
}

func (s *userService) CreateUser(ctx context.Context, claims *utils.CustomClaims, req *models.CreateUserRequest) (*models.UserResponse, error) {
	universityID, err := primitive.ObjectIDFromHex(claims.UniversityID)
	if err != nil {
		return nil, common.ErrInvalidToken
	}

	exists, err := s.userRepo.ExistsByStudentCodeAndUniversityID(ctx, req.StudentCode, universityID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, common.ErrStudentIDExists
	}

	university, err := s.universityRepo.FindByID(ctx, universityID)
	if err != nil || university == nil {
		return nil, common.ErrUniversityNotFound
	}

	req.Email = utils.NormalizeEmail(req.Email)
	if err := s.checkStudentEmail(ctx, req.Email, university, primitive.NilObjectID); err != nil {
		return nil, err
	}

	faculty, err := s.facultyRepo.FindByCodeAndUniversityID(ctx, req.FacultyCode, universityID)
	if err != nil || faculty == nil {
		return nil, common.ErrFacultyNotFound
	}

	user := &models.User{
		ID:              primitive.NewObjectID(),
		StudentCode:     req.StudentCode,
		FullName:        req.FullName,
		Email:           req.Email,
		FacultyID:       faculty.ID,
		UniversityID:    universityID,
		Course:          req.Course,
		Status:          0,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		CitizenIdNumber: req.CitizenIdNumber,
		Gender:          req.Gender,
		DateOfBirth:     req.DateOfBirth,
		Ethnicity:       req.Ethnicity,
		CurrentAddress:  req.CurrentAddress,
		BirthAddress:    req.BirthAddress,
		UnionJoinDate:   req.UnionJoinDate,
		PartyJoinDate:   req.PartyJoinDate,
		Description:     req.Description,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	if err := s.authRepo.CreateAccount(ctx, &models.Account{
		ID: primitive.NewObjectID(), StudentID: user.ID, UniversityID: universityID,
		StudentEmail: user.Email, CreatedAt: time.Now(), Role: "student", Status: "pending",
	}); err != nil {
		_ = s.userRepo.DeleteUser(ctx, user.ID)
		return nil, err
	}

	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		log.Printf("Error loading location: %v, fallback to UTC", err)
		loc = time.UTC
	}

	resp := mapper.MapUserToResponse(user, faculty, university, loc)
	return &resp, nil

}

func (s *userService) UpdateUser(ctx context.Context, id primitive.ObjectID, req models.UpdateUserRequest) error {
	update := bson.M{}

	claimsVal := ctx.Value(utils.ClaimsContextKey)
	claims, ok := claimsVal.(*utils.CustomClaims)
	if !ok || claims == nil {
		return common.ErrUnauthorized
	}

	universityID, err := primitive.ObjectIDFromHex(claims.UniversityID)
	if err != nil {
		return common.ErrInvalidToken
	}

	if req.StudentCode != nil {
		studentCode := strings.TrimSpace(*req.StudentCode)
		if studentCode != "" {
			exist, err := s.userRepo.FindByStudentCodeAndUniversityID(ctx, studentCode, universityID)
			if err != nil {
				return err
			}
			if exist != nil && exist.ID != id {
				return common.ErrStudentIDExists
			}
			update["student_code"] = studentCode
		}
	}

	if req.Email != nil {
		email := utils.NormalizeEmail(*req.Email)
		if email != "" {
			university, err := s.universityRepo.FindByID(ctx, universityID)
			if err != nil || university == nil {
				return common.ErrUniversityNotFound
			}
			if err := s.checkStudentEmail(ctx, email, university, id); err != nil {
				return err
			}
			update["email"] = email
		}
	}

	if req.FullName != nil {
		fullName := strings.TrimSpace(*req.FullName)
		if fullName != "" {
			update["full_name"] = fullName
		}
	}

	if req.Course != nil {
		course := strings.TrimSpace(*req.Course)
		if course != "" {
			update["course"] = course
		}
	}

	if req.FacultyCode != nil {
		facultyCode := strings.TrimSpace(*req.FacultyCode)
		if facultyCode != "" {
			faculty, err := s.facultyRepo.FindByCodeAndUniversityID(ctx, facultyCode, universityID)
			if err != nil {
				return err
			}
			if faculty == nil {
				return common.ErrFacultyNotFound
			}
			update["faculty_id"] = faculty.ID
		}
	}

	update["updated_at"] = time.Now()

	if len(update) == 1 {
		return errors.New("không có trường nào để cập nhật")
	}

	if err := s.userRepo.UpdateUser(ctx, id, update); err != nil {
		return err
	}
	if email, ok := update["email"].(string); ok {
		return s.authRepo.UpdateStudentEmail(ctx, id, email)
	}
	return nil
}

// checkStudentEmail requires the school email to be unique and to belong to the
// university's email domain (subdomains allowed), since OTP activation relies on it.
func (s *userService) checkStudentEmail(ctx context.Context, email string, university *models.University, exceptUserID primitive.ObjectID) error {
	domain := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(university.EmailDomain)), "@")
	if i := strings.LastIndex(domain, "@"); i >= 0 {
		domain = domain[i+1:]
	}
	if domain != "" && !strings.HasSuffix(email, "@"+domain) && !strings.HasSuffix(email, "."+domain) {
		return fmt.Errorf("%w (@%s)", ErrStudentEmailDomain, domain)
	}
	existing, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return err
	}
	if existing != nil && existing.ID != exceptUserID {
		return common.ErrEmailExists
	}
	exceptAccount := primitive.NilObjectID
	if exceptUserID != primitive.NilObjectID {
		if account, _ := s.authRepo.FindPersonalAccountByUserID(ctx, exceptUserID); account != nil {
			exceptAccount = account.ID
		}
	}
	taken, err := s.authRepo.IsLoginEmailTaken(ctx, email, exceptAccount)
	if err != nil {
		return err
	}
	if taken {
		return common.ErrEmailExists
	}
	return nil
}

// DeleteUser removes a student record and its account. Students who already
// have diplomas are kept: those records back published, signed documents.
func (s *userService) DeleteUser(ctx context.Context, id primitive.ObjectID) error {
	diplomas, err := s.ediplomaRepo.GetByUserID(ctx, id)
	if err != nil {
		return err
	}
	if len(diplomas) > 0 {
		return ErrStudentHasDiplomas
	}
	if err := s.userRepo.DeleteUser(ctx, id); err != nil {
		return err
	}
	return s.authRepo.DeleteByStudentID(ctx, id)
}

func (s *userService) GetMyProfile(ctx context.Context) (*models.UserResponse, error) {
	claimsVal := ctx.Value(utils.ClaimsContextKey)
	claims, ok := claimsVal.(*utils.CustomClaims)
	if !ok || claims == nil {
		return nil, common.ErrUnauthorized
	}
	userID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return nil, common.ErrInvalidToken
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.ErrUserNotExisted
	}

	faculty, err := s.facultyRepo.FindByID(ctx, user.FacultyID)
	if err != nil {
		return nil, err
	}
	university, err := s.universityRepo.FindByID(ctx, user.UniversityID)
	if err != nil {
		return nil, err
	}

	return &models.UserResponse{
		ID:             user.ID,
		StudentCode:    user.StudentCode,
		FullName:       user.FullName,
		Email:          user.Email,
		FacultyCode:    faculty.FacultyCode,
		FacultyName:    faculty.FacultyName,
		UniversityCode: university.UniversityCode,
		UniversityName: university.UniversityName,
		Course:         user.Course,
		Status:         user.Status,
	}, nil
}
