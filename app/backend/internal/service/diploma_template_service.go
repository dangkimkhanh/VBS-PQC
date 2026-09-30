package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/repository"
	"github.com/vnkmasc/Kmasc/app/backend/pkg/database"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrTemplateLocked = errors.New("template is locked and cannot be modified")
)

type TemplateService interface {
	GetTemplatesByUniversity(ctx context.Context, universityID primitive.ObjectID) ([]*models.DiplomaTemplate, error)
	UpdateDiplomaTemplate(ctx context.Context, templateID primitive.ObjectID, req models.UpdateDiplomaTemplateRequest) error
	CreateTemplate(ctx context.Context, universityID, facultyID, templateSampleID primitive.ObjectID, name, description string) (*models.DiplomaTemplate, error)
	GetTemplateByID(ctx context.Context, id string) (*models.DiplomaTemplate, error)
	GetTemplatesByFaculty(ctx context.Context, universityID, facultyID primitive.ObjectID) ([]*models.DiplomaTemplate, error)
	// UpdateTemplate(ctx context.Context, templateID, universityID primitive.ObjectID, name, description, htmlContent string) (*models.DiplomaTemplate, error)
}

type templateService struct {
	templateRepo          repository.TemplateRepository
	facultyRepo           repository.FacultyRepository
	universityRepo        repository.UniversityRepository
	facultyService        FacultyService
	templateSampleService TemplateSampleService
	minioClient           *database.MinioClient
}

func NewTemplateService(
	templateRepo repository.TemplateRepository,
	facultyRepo repository.FacultyRepository,
	universityRepo repository.UniversityRepository,
	facultyService FacultyService,
	templateSampleService TemplateSampleService,
	minioClient *database.MinioClient,
) TemplateService {
	return &templateService{
		templateRepo:          templateRepo,
		facultyRepo:           facultyRepo,
		universityRepo:        universityRepo,
		facultyService:        facultyService,
		templateSampleService: templateSampleService,
		minioClient:           minioClient,
	}
}

func (s *templateService) GetTemplatesByUniversity(ctx context.Context, universityID primitive.ObjectID) ([]*models.DiplomaTemplate, error) {
	return s.templateRepo.FindByUniversity(ctx, universityID)
}

func (s *templateService) UpdateDiplomaTemplate(
	ctx context.Context,
	templateID primitive.ObjectID,
	req models.UpdateDiplomaTemplateRequest,
) error {
	template, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		return err
	}

	if template.IsLocked {
		return fmt.Errorf("template is locked and cannot be edited")
	}

	if req.TemplateSampleID != template.TemplateSampleID {
		sample, err := s.templateSampleService.GetByID(ctx, req.TemplateSampleID)
		if err != nil {
			return fmt.Errorf("template sample not found: %v", err)
		}
		template.HashTemplate = utils.ComputeSHA256([]byte(sample.HTMLContent))
	}

	template.Name = req.Name
	template.Description = req.Description
	template.TemplateSampleID = req.TemplateSampleID
	template.FacultyID = req.FacultyID
	template.UpdatedAt = time.Now()

	return s.templateRepo.UpdateDiplomaTemplateByID(ctx, template)
}

func (s *templateService) GetTemplateByID(ctx context.Context, id string) (*models.DiplomaTemplate, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	return s.templateRepo.GetByID(ctx, objID)
}

func (s *templateService) CreateTemplate(
	ctx context.Context,
	universityID, facultyID, templateSampleID primitive.ObjectID,
	name, description string, // nhận name từ request
) (*models.DiplomaTemplate, error) {

	// 1. Kiểm tra khoa có thuộc trường không
	belongs, err := s.facultyService.CheckFacultyBelongsToUniversity(ctx, facultyID, universityID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify faculty ownership: %v", err)
	}
	if !belongs {
		return nil, errors.New("faculty does not belong to your university")
	}

	// 2. Lấy TemplateSample (chỉ dùng HTMLContent)
	sample, err := s.templateSampleService.GetByID(ctx, templateSampleID)
	if err != nil {
		return nil, fmt.Errorf("template sample not found: %v", err)
	}

	// 3. Tạo DiplomaTemplate dựa trên sample
	hash := utils.ComputeSHA256([]byte(sample.HTMLContent))

	template := &models.DiplomaTemplate{
		ID:               primitive.NewObjectID(),
		Name:             name,
		TemplateSampleID: templateSampleID,
		Description:      description,
		HashTemplate:     hash,
		Status:           "PENDING",
		IsLocked:         false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		UniversityID:     universityID,
		FacultyID:        facultyID,
	}

	if err := s.templateRepo.Create(ctx, template); err != nil {
		return nil, fmt.Errorf("failed to save template: %v", err)
	}

	return template, nil
}

func (s *templateService) GetTemplatesByFaculty(ctx context.Context, universityID, facultyID primitive.ObjectID) ([]*models.DiplomaTemplate, error) {
	// ✅ Tìm faculty theo facultyID và universityID để đảm bảo khoa thuộc đúng trường
	faculty, err := s.facultyRepo.FindByIDAndUniversityID(ctx, facultyID, universityID)
	if err != nil {
		return nil, fmt.Errorf("faculty does not belong to the university or not found: %v", err)
	}

	// ✅ Nếu tìm thấy thì truy vấn template
	return s.templateRepo.FindByUniversityAndFaculty(ctx, universityID, faculty.ID)
}

func (s *templateService) LockTemplate(ctx context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	return s.templateRepo.LockTemplate(ctx, objectID)
}
