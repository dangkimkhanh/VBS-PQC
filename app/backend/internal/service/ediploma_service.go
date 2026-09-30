package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/common"
	"github.com/vnkmasc/Kmasc/app/backend/internal/mapper"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/repository"
	"github.com/vnkmasc/Kmasc/app/backend/pkg/database"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type EDiplomaService interface {
	GetSimpleEDiplomasByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.EDiplomaSimpleResponse, error)
	GetEDiplomaFileByUniversityCode(ctx context.Context, ediplomaID primitive.ObjectID, universityCode string) (io.ReadCloser, string, error)
	GetEDiplomaFile(ctx context.Context, ediplomaID, universityID primitive.ObjectID) (io.ReadCloser, string, error)
	GetEDiplomaDTOByID(ctx context.Context, id primitive.ObjectID) (*models.EDiplomaResponse, error)
	GetByID(ctx context.Context, id string) (*models.EDiploma, error)
	CreateEDiploma(ctx context.Context, claims *utils.CustomClaims, req *models.CreateEDiplomaRequest) (*models.EDiploma, error)
	GenerateEDiploma(ctx context.Context, ediplomaIDStr, templateIDStr string) (*models.EDiploma, error)
	GenerateBulkEDiplomas(ctx context.Context, facultyIDStr, templateIDStr string) ([]*models.EDiploma, error)
	UploadLocalEDiplomas(ctx context.Context) []map[string]interface{}
	ProcessZip(ctx context.Context, zipPath string, universityID primitive.ObjectID) ([]*models.EDiploma, error)
	SearchEDiplomaDTOs(ctx context.Context, filter models.EDiplomaSearchFilter) ([]*models.EDiplomaResponse, int64, error)
	GenerateBulkEDiplomasZip(ctx context.Context, facultyIDStr, certificateType, course string, issued *bool, templateIDStr string) (string, error)
	RevokeEDiploma(ctx context.Context, universityID, diplomaID primitive.ObjectID, reason string) error

	CreateRound(ctx context.Context, universityID primitive.ObjectID, req models.CreateIssuanceRoundRequest) (*models.IssuanceRound, error)
	SearchRounds(ctx context.Context, universityID primitive.ObjectID, query, facultyID string, limit int) ([]models.IssuanceRound, error)
	RequireRound(ctx context.Context, universityID primitive.ObjectID, roundID string) (*models.IssuanceRound, error)
	RoundScope(ctx context.Context, universityID primitive.ObjectID, facultyID, roundID string) (bson.M, error)
	AssignRound(ctx context.Context, universityID primitive.ObjectID, roundID, facultyID, course, certificateType string) (int64, error)
	RevokeRound(ctx context.Context, universityID primitive.ObjectID, facultyID, roundID, reason string) (int64, error)
}

func (s *eDiplomaService) RevokeEDiploma(ctx context.Context, universityID, diplomaID primitive.ObjectID, reason string) error {
	diploma, err := s.repo.FindByID(ctx, diplomaID)
	if err != nil || diploma == nil {
		return fmt.Errorf("ediploma not found")
	}
	if diploma.UniversityID != universityID {
		return fmt.Errorf("access denied")
	}
	if !diploma.Issued {
		return fmt.Errorf("only issued diplomas can be revoked")
	}
	if diploma.Revoked {
		return fmt.Errorf("ediploma is already revoked")
	}
	reason = strings.TrimSpace(reason)
	if len(reason) < 5 {
		return fmt.Errorf("revocation reason must be at least 5 characters")
	}
	revokedAt := time.Now()
	if err := s.repo.Revoke(ctx, diplomaID, reason, revokedAt); err != nil {
		return err
	}
	s.notifier.NotifyRevoked(ctx, []*models.EDiploma{diploma}, reason, revokedAt)
	return nil
}

type eDiplomaService struct {
	templateSampleRepo repository.TemplateSampleRepo
	universityRepo     repository.UniversityRepository
	majorRepo          repository.MajorRepository
	facultyRepo        repository.FacultyRepository
	repo               repository.EDiplomaRepository
	templateRepo       repository.TemplateRepository
	userRepo           repository.UserRepository
	minioClient        *database.MinioClient
	templateEngine     *models.TemplateEngine
	pdfGenerator       *utils.PDFGenerator
	pqcKeys            repository.PQCKeyRepository
	rounds             repository.IssuanceRoundRepository
	notifier           *StudentNotifier
}

func NewEDiplomaService(
	templateSampleRepo repository.TemplateSampleRepo,
	universityRepo repository.UniversityRepository,
	majorRepo repository.MajorRepository,
	facultyRepo repository.FacultyRepository,
	repo repository.EDiplomaRepository,
	templateRepo repository.TemplateRepository,
	userRepo repository.UserRepository,
	minioClient *database.MinioClient,
	templateEngine *models.TemplateEngine,
	pdfGenerator *utils.PDFGenerator,
	pqcKeys repository.PQCKeyRepository,
	rounds repository.IssuanceRoundRepository,
	notifier *StudentNotifier,
) *eDiplomaService {
	return &eDiplomaService{
		templateSampleRepo: templateSampleRepo,
		universityRepo:     universityRepo,
		majorRepo:          majorRepo,
		facultyRepo:        facultyRepo,
		repo:               repo,
		templateRepo:       templateRepo,
		userRepo:           userRepo,
		minioClient:        minioClient,
		templateEngine:     templateEngine,
		pdfGenerator:       pdfGenerator,
		pqcKeys:            pqcKeys,
		rounds:             rounds,
		notifier:           notifier,
	}
}

// sealAlgorithm returns the parameter set of the university's active signing key,
// which is printed in the PDF stamp. A PDF cannot be rendered without one.
func (s *eDiplomaService) sealAlgorithm(ctx context.Context, universityID primitive.ObjectID) (string, error) {
	key, err := s.pqcKeys.FindActive(ctx, universityID)
	if errors.Is(err, mongo.ErrNoDocuments) || (err == nil && key == nil) {
		return "", ErrNoActivePQCKey
	}
	if err != nil {
		return "", err
	}
	return key.Algorithm, nil
}

func (s *eDiplomaService) GetEDiplomaDTOByID(ctx context.Context, id primitive.ObjectID) (*models.EDiplomaResponse, error) {
	ediploma, err := s.repo.FindByID(ctx, id)
	if err != nil || ediploma == nil {
		return nil, fmt.Errorf("ediploma not found")
	}
	if claims, ok := ctx.Value(utils.ClaimsContextKey).(*utils.CustomClaims); ok {
		if claims.UniversityID != ediploma.UniversityID.Hex() || (claims.Role == "student" && claims.UserID != ediploma.UserID.Hex()) {
			return nil, fmt.Errorf("access denied")
		}
	}

	university, _ := s.universityRepo.FindByID(ctx, ediploma.UniversityID)
	faculty, _ := s.facultyRepo.FindByID(ctx, ediploma.FacultyID)
	template, _ := s.templateRepo.GetByID(ctx, ediploma.TemplateID)
	user, _ := s.userRepo.GetUserByID(ctx, ediploma.UserID)

	dto := mapper.MapEDiplomaToDTO(ediploma, university, faculty, template, user)
	s.attachRounds(ctx, []*models.EDiploma{ediploma}, []*models.EDiplomaResponse{dto})
	return dto, nil
}

func (s *eDiplomaService) GetByID(ctx context.Context, id string) (*models.EDiploma, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid diploma id")
	}
	return s.repo.FindByID(ctx, objID)
}

func (s *eDiplomaService) CreateEDiploma(ctx context.Context, claims *utils.CustomClaims, req *models.CreateEDiplomaRequest) (*models.EDiploma, error) {
	universityID, err := primitive.ObjectIDFromHex(claims.UniversityID)
	if err != nil {
		return nil, common.ErrInvalidToken
	}

	if strings.TrimSpace(req.StudentCode) == "" || strings.TrimSpace(req.Name) == "" || req.IssueDate.IsZero() ||
		strings.TrimSpace(req.SerialNumber) == "" || strings.TrimSpace(req.RegistrationNumber) == "" {
		return nil, common.ErrMissingRequiredFieldsForEDiploma
	}

	// Every diploma belongs to an issuance round of the same university.
	if req.RoundID.IsZero() {
		return nil, ErrRoundRequired
	}
	if _, err := s.RequireRound(ctx, universityID, req.RoundID.Hex()); err != nil {
		return nil, err
	}

	user, err := s.userRepo.FindByStudentCodeAndUniversityID(ctx, strings.TrimSpace(req.StudentCode), universityID)
	if err != nil || user == nil {
		return nil, common.ErrUserNotExisted
	}

	existed, err := s.repo.FindByDynamicFilter(ctx, bson.M{
		"university_id":       universityID,
		"serial_number":       strings.TrimSpace(req.SerialNumber),
		"registration_number": strings.TrimSpace(req.RegistrationNumber),
	})
	if err != nil {
		return nil, err
	}
	if len(existed) > 0 {
		return nil, common.ErrEDiplomaAlreadyExists
	}

	now := time.Now()
	ediploma := &models.EDiploma{
		ID:                 primitive.NewObjectID(),
		Name:               strings.TrimSpace(req.Name),
		UniversityID:       universityID,
		FacultyID:          user.FacultyID,
		UserID:             user.ID,
		StudentCode:        strings.TrimSpace(req.StudentCode),
		FullName:           user.FullName,
		CertificateType:    strings.TrimSpace(req.CertificateType),
		Course:             strings.TrimSpace(req.Course),
		EducationType:      strings.TrimSpace(req.EducationType),
		GPA:                req.GPA,
		GraduationRank:     strings.TrimSpace(req.GraduationRank),
		IssueDate:          req.IssueDate,
		SerialNumber:       strings.TrimSpace(req.SerialNumber),
		RegistrationNumber: strings.TrimSpace(req.RegistrationNumber),
		Issued:             false,
		Signed:             false,
		SignedAt:           time.Time{},
		DataEncrypted:      false,
		OnBlockchain:       false,
		Description:        strings.TrimSpace(req.Description),
		RoundID:            req.RoundID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := s.repo.Save(ctx, ediploma); err != nil {
		return nil, err
	}

	return ediploma, nil
}

func (s *eDiplomaService) GenerateEDiploma(ctx context.Context, ediplomaIDStr, templateIDStr string) (*models.EDiploma, error) {
	ediplomaID, err := primitive.ObjectIDFromHex(ediplomaIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid eDiploma ID")
	}
	templateID, err := primitive.ObjectIDFromHex(templateIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid template ID")
	}

	ediploma, err := s.repo.FindByID(ctx, ediplomaID)
	if err != nil || ediploma == nil {
		return nil, fmt.Errorf("ediploma not found")
	}
	if err := canIssueDiploma(ctx, ediploma); err != nil {
		return nil, err
	}

	template, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found")
	}
	if template.UniversityID != ediploma.UniversityID || template.FacultyID != ediploma.FacultyID {
		return nil, fmt.Errorf("template does not belong to the diploma faculty")
	}

	// Lấy TemplateSample từ Template
	sample, err := s.templateSampleRepo.GetByID(ctx, template.TemplateSampleID)
	if err != nil {
		return nil, fmt.Errorf("template sample not found")
	}

	// Kiểm tra hash HTMLContent
	calculatedHash := utils.ComputeSHA256([]byte(sample.HTMLContent))
	if calculatedHash != template.HashTemplate {
		return nil, fmt.Errorf("template content hash mismatch - data may be corrupted")
	}

	user, err := s.userRepo.GetUserByID(ctx, ediploma.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	university, err := s.universityRepo.FindByID(ctx, ediploma.UniversityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get university: %w", err)
	}

	faculty, err := s.facultyRepo.FindByID(ctx, ediploma.FacultyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get faculty: %w", err)
	}

	algorithm, err := s.sealAlgorithm(ctx, ediploma.UniversityID)
	if err != nil {
		return nil, err
	}

	data := diplomaRenderData(ediploma, user, university, faculty, ediploma.IssueDate, algorithm)

	// Render PDF từ sample HTML
	renderedHTML, err := s.templateEngine.Render(sample.HTMLContent, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render HTML: %w", err)
	}

	pdfBytes, err := s.pdfGenerator.ConvertHTMLToPDF(renderedHTML)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	hash := utils.ComputeSHA256(pdfBytes)
	pdfPath := fmt.Sprintf("ediplomas/%s.pdf", primitive.NewObjectID().Hex())
	if err := s.minioClient.UploadFile(ctx, pdfPath, pdfBytes, "application/pdf"); err != nil {
		return nil, fmt.Errorf("failed to upload PDF: %w", err)
	}

	if err := s.templateRepo.LockTemplate(ctx, templateID); err != nil {
		log.Printf("Failed to lock template: %v", err)
	}

	now := time.Now()
	updates := bson.M{
		"template_id":         templateID,
		"ediploma_file_link":  pdfPath,
		"ediploma_file_hash":  hash,
		"issued":              true,
		"seal_algorithm":      algorithm,
		"signature_of_uni":    template.SignatureOfUni,
		"signature_of_minedu": template.SignatureOfMinEdu,
		"updated_at":          now,
	}
	if err := s.repo.UpdateFields(ctx, ediploma.ID, updates); err != nil {
		return nil, fmt.Errorf("failed to update EDiploma: %w", err)
	}
	if err := s.markStudentGraduated(ctx, ediploma.UserID); err != nil {
		return nil, fmt.Errorf("failed to update student graduation status: %w", err)
	}

	updated, err := s.repo.FindByID(ctx, ediploma.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load updated EDiploma: %w", err)
	}

	return updated, nil
}

func (s *eDiplomaService) SearchEDiplomaDTOs(ctx context.Context, filter models.EDiplomaSearchFilter) ([]*models.EDiplomaResponse, int64, error) {
	ediplomas, total, err := s.repo.SearchByFilters(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	dtoList := make([]*models.EDiplomaResponse, 0, len(ediplomas))
	for _, ed := range ediplomas {
		university, _ := s.universityRepo.FindByID(ctx, ed.UniversityID)
		faculty, _ := s.facultyRepo.FindByID(ctx, ed.FacultyID)

		template, _ := s.templateRepo.GetByID(ctx, ed.TemplateID)
		user, _ := s.userRepo.GetUserByID(ctx, ed.UserID)

		dto := mapper.MapEDiplomaToDTO(ed, university, faculty, template, user)
		dtoList = append(dtoList, dto)
	}
	s.attachRounds(ctx, ediplomas, dtoList)

	return dtoList, total, nil
}

func (s *eDiplomaService) GenerateBulkEDiplomas(ctx context.Context, facultyIDStr, templateIDStr string) ([]*models.EDiploma, error) {
	// Parse ID
	facultyID, err := primitive.ObjectIDFromHex(facultyIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid faculty ID")
	}
	templateID, err := primitive.ObjectIDFromHex(templateIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid template ID")
	}

	// 1. Load template từ MongoDB
	template, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found")
	}
	if template.FacultyID != facultyID {
		return nil, errors.New("template does not belong to the given faculty")
	}

	// 2. Lấy TemplateSample
	sample, err := s.templateSampleRepo.GetByID(ctx, template.TemplateSampleID)
	if err != nil {
		return nil, fmt.Errorf("template sample not found: %w", err)
	}

	// 3. Xác minh hash của HTMLContent
	calculatedHash := utils.ComputeSHA256([]byte(sample.HTMLContent))
	if calculatedHash != template.HashTemplate {
		return nil, fmt.Errorf("template content hash mismatch - data may be corrupted")
	}

	// 4. Load tất cả eDiploma của faculty
	ediplomas, err := s.repo.GetByFacultyID(ctx, facultyID)
	if err != nil {
		return nil, fmt.Errorf("failed to load eDiplomas: %w", err)
	}

	algorithm, err := s.sealAlgorithm(ctx, template.UniversityID)
	if err != nil {
		return nil, err
	}

	result := make([]*models.EDiploma, 0, len(ediplomas))
	now := time.Now()
	for _, ed := range ediplomas {
		// Load university
		if canIssueDiploma(ctx, ed) != nil || template.UniversityID != ed.UniversityID || template.FacultyID != ed.FacultyID {
			continue
		}
		university, err := s.universityRepo.FindByID(ctx, ed.UniversityID)
		if err != nil {
			log.Printf("Load university failed for eDiploma %s: %v", ed.ID.Hex(), err)
			continue
		}

		// Load user
		user, err := s.userRepo.GetUserByID(ctx, ed.UserID)
		if err != nil {
			log.Printf("Load user failed for eDiploma %s: %v", ed.ID.Hex(), err)
			continue
		}

		faculty, err := s.facultyRepo.FindByID(ctx, ed.FacultyID)
		if err != nil {
			log.Printf("Load faculty failed for eDiploma %s: %v", ed.ID.Hex(), err)
			continue
		}

		data := diplomaRenderData(ed, user, university, faculty, now, algorithm)

		// Render HTML từ TemplateSample
		renderedHTML, err := s.templateEngine.Render(sample.HTMLContent, data)
		if err != nil {
			log.Printf("Render failed for eDiploma %s: %v", ed.ID.Hex(), err)
			continue
		}

		// Convert sang PDF
		pdfBytes, err := s.pdfGenerator.ConvertHTMLToPDF(renderedHTML)
		if err != nil {
			log.Printf("PDF generation failed for eDiploma %s: %v", ed.ID.Hex(), err)
			continue
		}

		// Hash PDF
		hash := utils.ComputeSHA256(pdfBytes)

		// Lưu PDF ở MinIO
		pdfPath := fmt.Sprintf("ediplomas/%s/%s.pdf", university.UniversityCode, ed.StudentCode)
		if err := s.minioClient.UploadFile(ctx, pdfPath, pdfBytes, "application/pdf"); err != nil {
			log.Printf("Upload failed for eDiploma %s: %v", ed.ID.Hex(), err)
			continue
		}

		updates := bson.M{
			"template_id":         templateID,
			"ediploma_file_link":  pdfPath,
			"ediploma_file_hash":  hash,
			"signature_of_uni":    template.SignatureOfUni,
			"signature_of_minedu": template.SignatureOfMinEdu,
			"issue_date":          now,
			"issued":              true,
			"seal_algorithm":      algorithm,
			"updated_at":          now,
		}
		if err := s.repo.UpdateFields(ctx, ed.ID, updates); err != nil {
			log.Printf("Update failed for eDiploma %s: %v", ed.ID.Hex(), err)
			continue
		}
		if err := s.markStudentGraduated(ctx, ed.UserID); err != nil {
			log.Printf("Student status update failed for eDiploma %s: %v", ed.ID.Hex(), err)
			continue
		}

		// Cập nhật struct hiện tại và lưu
		ed.TemplateID = templateID
		ed.EDiplomaFileLink = pdfPath
		ed.EDiplomaFileHash = hash
		ed.SignatureOfUni = template.SignatureOfUni
		ed.SignatureOfMinEdu = template.SignatureOfMinEdu
		ed.IssueDate = now
		ed.Issued = true
		ed.UpdatedAt = now
		result = append(result, ed)
	}

	return result, nil
}

func (s *eDiplomaService) UploadLocalEDiplomas(ctx context.Context) []map[string]interface{} {
	localFolder := os.Getenv("EDIPLOMA_LOCAL_FOLDER")
	if localFolder == "" {
		return []map[string]interface{}{
			{"error": "EDIPLOMA_LOCAL_FOLDER not set in .env"},
		}
	}

	files, err := os.ReadDir(localFolder)
	if err != nil {
		return []map[string]interface{}{
			{"error": fmt.Sprintf("failed to read folder: %v", err)},
		}
	}

	var results []map[string]interface{}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(localFolder, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			results = append(results, map[string]interface{}{
				"file":  file.Name(),
				"error": fmt.Sprintf("read failed: %v", err),
			})
			continue
		}

		hash := utils.ComputeSHA256(data) // SHA256 -> string

		// Upload file lên MinIO
		minioPath := fmt.Sprintf("ediplomas/%s", file.Name())
		if err := s.minioClient.UploadFile(ctx, minioPath, data, "application/pdf"); err != nil {
			results = append(results, map[string]interface{}{
				"file":  file.Name(),
				"error": fmt.Sprintf("upload failed: %v", err),
			})
			continue
		}

		// Lưu vào DB
		ediploma := &models.EDiploma{
			ID:               primitive.NewObjectID(),
			EDiplomaFileLink: minioPath,
			EDiplomaFileHash: hash,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		if err := s.repo.Save(ctx, ediploma); err != nil {
			results = append(results, map[string]interface{}{
				"file":  file.Name(),
				"error": fmt.Sprintf("DB save failed: %v", err),
			})
			continue
		}

		results = append(results, map[string]interface{}{
			"file":   file.Name(),
			"hash":   hash,
			"link":   minioPath,
			"status": "uploaded",
		})
	}

	return results
}
func (s *eDiplomaService) ProcessZip(ctx context.Context, zipPath string, universityID primitive.ObjectID) ([]*models.EDiploma, error) {
	university, err := s.universityRepo.FindByID(ctx, universityID)
	if err != nil || university == nil {
		return nil, fmt.Errorf("university not found")
	}

	universityCode := university.UniversityCode

	// Thư mục tạm để giải nén
	extractDir := filepath.Join(os.TempDir(), fmt.Sprintf("unzipped_%d", time.Now().Unix()))
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create extract dir: %w", err)
	}
	defer os.RemoveAll(extractDir) // Cleanup tự động khi function kết thúc

	if err := utils.Unzip(zipPath, extractDir); err != nil {
		return nil, fmt.Errorf("failed to unzip: %w", err)
	}

	files, err := os.ReadDir(extractDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read extracted folder: %w", err)
	}

	var results []*models.EDiploma

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(extractDir, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		hash := utils.ComputeSHA256(data)
		minioPath := fmt.Sprintf("ediplomas/%s/%s.pdf", universityCode, primitive.NewObjectID().Hex())

		filename := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		studentCode := strings.TrimSuffix(filename, "_signed")

		ediploma, err := s.repo.FindByStudentCode(ctx, studentCode)
		if err != nil || ediploma == nil {
			continue
		}
		if ediploma.UniversityID != universityID || ediploma.Revoked || ediploma.PQCProof != nil || ediploma.OnBlockchain || ediploma.Signed {
			continue
		}
		if err := s.minioClient.UploadFile(ctx, minioPath, data, "application/pdf"); err != nil {
			continue
		}

		updates := bson.M{
			"ediploma_file_hash": hash,
			"ediploma_file_link": minioPath,
			"data_encrypted":     true,
			"updated_at":         time.Now(),
		}

		if err := s.repo.UpdateFields(ctx, ediploma.ID, updates); err != nil {
			continue
		}
		if err := s.markStudentGraduated(ctx, ediploma.UserID); err != nil {
			continue
		}

		updatedEDiploma, err := s.repo.FindByID(ctx, ediploma.ID)
		if err != nil || updatedEDiploma == nil {
			continue
		}

		results = append(results, updatedEDiploma)
	}

	return results, nil
}

func (s *eDiplomaService) markStudentGraduated(ctx context.Context, userID primitive.ObjectID) error {
	return s.userRepo.UpdateUser(ctx, userID, bson.M{
		"status":     1,
		"updated_at": time.Now(),
	})
}

func (s *eDiplomaService) GenerateBulkEDiplomasZip(
	ctx context.Context,
	facultyIDStr, certificateType, course string,
	issued *bool,
	templateIDStr string,
) (string, error) {

	// 1. Convert templateID
	templateID, err := primitive.ObjectIDFromHex(templateIDStr)
	if err != nil {
		return "", fmt.Errorf("invalid template ID")
	}

	// 2. Lấy template
	template, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		return "", common.ErrTemplateNotFound
	}

	// 3. Lấy TemplateSample
	sample, err := s.templateSampleRepo.GetByID(ctx, template.TemplateSampleID)
	if err != nil || sample.HTMLContent == "" {
		return "", errors.New("template sample not found or empty")
	}

	// 4. Convert facultyID nếu có
	var facultyID primitive.ObjectID
	if facultyIDStr != "" {
		facultyID, err = primitive.ObjectIDFromHex(facultyIDStr)
		if err != nil {
			return "", fmt.Errorf("invalid faculty_id")
		}
	}

	// 5. Build dynamic filter
	filter := bson.M{}
	if !facultyID.IsZero() {
		filter["faculty_id"] = facultyID
	}
	if certificateType != "" {
		filter["certificate_type"] = bson.M{"$regex": regexp.QuoteMeta(certificateType), "$options": "i"}
	}
	if course != "" {
		filter["course"] = bson.M{"$regex": regexp.QuoteMeta(course), "$options": "i"}
	}
	if issued != nil {
		filter["issued"] = *issued
	}

	// 6. Lấy danh sách eDiplomas
	ediplomas, err := s.repo.FindByDynamicFilter(ctx, filter)
	if err != nil {
		return "", fmt.Errorf("failed to load eDiplomas: %w", err)
	}
	if len(ediplomas) == 0 {
		return "", nil
	}

	// 7. Tạo thư mục tạm
	tmpDir, err := os.MkdirTemp("", "ediplomas_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	// Do not clean up temp dir here because handler still needs to stream this file.

	var generatedFilePaths []string
	algorithm, err := s.sealAlgorithm(ctx, template.UniversityID)
	if err != nil {
		return "", err
	}
	now := time.Now() // Gọi 1 lần ngoài loop
	for _, ed := range ediplomas {
		user, _ := s.userRepo.GetUserByID(ctx, ed.UserID)
		if canIssueDiploma(ctx, ed) != nil || template.UniversityID != ed.UniversityID || template.FacultyID != ed.FacultyID {
			continue
		}
		university, _ := s.universityRepo.FindByID(ctx, ed.UniversityID)
		faculty, _ := s.facultyRepo.FindByID(ctx, ed.FacultyID)

		data := diplomaRenderData(ed, user, university, faculty, now, algorithm)

		renderedHTML, err := s.templateEngine.Render(sample.HTMLContent, data)
		if err != nil {
			continue
		}

		// Convert HTML → PDF
		pdfBytes, err := s.pdfGenerator.ConvertHTMLToPDF(renderedHTML)
		if err != nil {
			continue
		}

		// Ghi file
		fileName := fmt.Sprintf("%s.pdf", ed.StudentCode)
		filePath := filepath.Join(tmpDir, fileName)
		if err := os.WriteFile(filePath, pdfBytes, 0644); err != nil {
			continue
		}

		// Update eDiploma
		ed.TemplateID = templateID
		ed.SignatureOfUni = template.SignatureOfUni
		ed.SignatureOfMinEdu = template.SignatureOfMinEdu
		ed.EDiplomaFileLink = filePath
		ed.EDiplomaFileHash = utils.ComputeSHA256(pdfBytes)
		ed.IssueDate = now
		ed.Issued = true
		ed.SealAlgorithm = algorithm
		ed.UpdatedAt = now
		if err := s.repo.Update(ctx, ed.ID, ed); err != nil {
			continue
		}
		if err := s.markStudentGraduated(ctx, ed.UserID); err != nil {
			continue
		}

		generatedFilePaths = append(generatedFilePaths, filePath)
	}

	// 8. Tạo file zip từ các PDF
	zipFilePath := filepath.Join(tmpDir, "ediplomas.zip")
	if err := utils.CreateZipFromFiles(zipFilePath, generatedFilePaths); err != nil {
		return "", fmt.Errorf("failed to create zip: %w", err)
	}

	if len(generatedFilePaths) > 0 {
		if err := s.templateRepo.LockTemplate(ctx, template.ID); err != nil {
			// Log lỗi nhưng không block trả zip
			log.Printf("Failed to lock template: %v", err)
		}
	}

	return zipFilePath, nil
}

// service/eDiplomaService.go
func (s *eDiplomaService) GetEDiplomaFile(ctx context.Context, ediplomaID, universityID primitive.ObjectID) (io.ReadCloser, string, error) {
	// Lấy bản ghi EDiploma
	ediploma, err := s.repo.FindByID(ctx, ediplomaID)
	if err != nil || ediploma == nil {
		return nil, "", fmt.Errorf("EDiploma not found")
	}

	// Kiểm tra quyền truy cập theo university
	if ediploma.UniversityID != universityID {
		return nil, "", fmt.Errorf("access denied")
	}
	if claims, ok := ctx.Value(utils.ClaimsContextKey).(*utils.CustomClaims); ok && claims.Role == "student" && claims.UserID != ediploma.UserID.Hex() {
		return nil, "", fmt.Errorf("access denied")
	}

	// Lấy thông tin university để lấy mã trường
	university, err := s.universityRepo.FindByID(ctx, universityID)
	if err != nil || university == nil {
		return nil, "", fmt.Errorf("university not found")
	}

	stream, contentType, err := s.downloadEDiplomaFileWithFallback(ctx, ediploma, university.UniversityCode)
	if err != nil {
		return nil, "", err
	}

	return stream, contentType, nil
}

func (s *eDiplomaService) GetEDiplomaFileByUniversityCode(ctx context.Context, ediplomaID primitive.ObjectID, universityCode string) (io.ReadCloser, string, error) {
	ediploma, err := s.repo.FindByID(ctx, ediplomaID)
	if err != nil || ediploma == nil {
		return nil, "", fmt.Errorf("EDiploma not found")
	}

	university, err := s.universityRepo.FindByCode(ctx, universityCode)
	if err != nil || university == nil {
		return nil, "", fmt.Errorf("university not found")
	}

	if ediploma.UniversityID != university.ID {
		return nil, "", fmt.Errorf("EDiploma does not belong to university")
	}

	stream, contentType, err := s.downloadEDiplomaFileWithFallback(ctx, ediploma, university.UniversityCode)
	if err != nil {
		return nil, "", err
	}

	return stream, contentType, nil
}

func (s *eDiplomaService) downloadEDiplomaFileWithFallback(ctx context.Context, ediploma *models.EDiploma, universityCode string) (io.ReadCloser, string, error) {
	candidates := buildEDiplomaObjectCandidates(ediploma, universityCode)
	var lastErr error

	for _, objectKey := range candidates {
		stream, contentType, err := s.minioClient.DownloadFileStream(ctx, objectKey)
		if err == nil {
			return stream, contentType, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = errors.New("no candidate object key")
	}

	return nil, "", fmt.Errorf("failed to get file from MinIO: %w", lastErr)
}

func buildEDiplomaObjectCandidates(ediploma *models.EDiploma, universityCode string) []string {
	// Prefer persisted link in DB, then try legacy key formats.
	rawLink := strings.TrimSpace(ediploma.EDiplomaFileLink)
	legacyPlain := fmt.Sprintf("ediplomas/%s/%s.pdf", universityCode, ediploma.StudentCode)
	legacySigned := fmt.Sprintf("ediplomas/%s/%s_signed.pdf", universityCode, ediploma.StudentCode)

	seen := make(map[string]struct{}, 3)
	add := func(keys *[]string, key string) {
		if key == "" {
			return
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		*keys = append(*keys, key)
	}

	keys := make([]string, 0, 3)
	add(&keys, rawLink)
	add(&keys, legacyPlain)
	add(&keys, legacySigned)

	return keys
}
func (s *eDiplomaService) GetSimpleEDiplomasByUserID(
	ctx context.Context,
	userID primitive.ObjectID,
) ([]*models.EDiplomaSimpleResponse, error) {

	ediplomas, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Students only see diplomas that were actually issued, not drafts.
	responses := []*models.EDiplomaSimpleResponse{}
	for _, ed := range ediplomas {
		if !ed.Issued {
			continue
		}
		responses = append(responses, &models.EDiplomaSimpleResponse{
			ID:                 ed.ID.Hex(),
			Name:               ed.Name,
			Revoked:            ed.Revoked,
			OnBlockchainVerify: ed.OnBlockchainVerify,
		})
	}

	return responses, nil
}
