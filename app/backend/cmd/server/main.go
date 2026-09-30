package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/vnkmasc/Kmasc/app/backend/internal/handlers"
	"github.com/vnkmasc/Kmasc/app/backend/internal/middleware"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/repository"
	"github.com/vnkmasc/Kmasc/app/backend/internal/service"
	"github.com/vnkmasc/Kmasc/app/backend/pkg/blockchain"
	"github.com/vnkmasc/Kmasc/app/backend/pkg/database"
	"github.com/vnkmasc/Kmasc/app/backend/routes"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Không tìm thấy file .env, đang dùng biến môi trường hệ thống")
	}

	if err := database.ConnectMongo(); err != nil {
		log.Fatalf("Lỗi khi kết nối MongoDB: %v", err)
	}
	db := database.DB
	fabricCfg := blockchain.NewFabricConfigFromEnv()

	InitValidator()
	seedAdminAccount(db)
	if err := SeedTemplateSamples(context.Background(), repository.NewTemplateSampleRepo(db), repository.NewTemplateRepository(db)); err != nil {
		log.Printf("Không đồng bộ được giao diện mẫu: %v", err)
	}

	emailSender := utils.NewSMTPSender(
		os.Getenv("EMAIL_FROM"),
		os.Getenv("EMAIL_PASSWORD"),
		os.Getenv("EMAIL_HOST"),
		os.Getenv("EMAIL_PORT"),
	)

	useSSL := false
	if strings.ToLower(os.Getenv("MINIO_USE_SSL")) == "true" {
		useSSL = true
	}

	minioClient, err := database.NewMinioClient(
		os.Getenv("MINIO_ENDPOINT"),
		os.Getenv("MINIO_ACCESS_KEY"),
		os.Getenv("MINIO_SECRET_KEY"),
		os.Getenv("MINIO_BUCKET"),
		useSSL,
	)
	if err != nil {
		log.Fatalf("Không thể khởi tạo MinIO client: %v", err)
	}
	fabricClient, err := blockchain.NewFabricClient(fabricCfg)
	if err != nil {
		log.Println("⚠️ Không thể kết nối Fabric, chạy chế độ không blockchain:", err)
		fabricClient = nil
	}

	// Repository
	userRepo := repository.NewUserRepository(db)
	authRepo := repository.NewAuthRepository(db)
	if err := authRepo.EnsureIndexes(context.Background()); err != nil {
		log.Printf("Không thể tạo index cho OTP: %v", err)
	}

	universityRepo := repository.NewUniversityRepository(db)
	facultyRepo := repository.NewFacultyRepository(db)
	majorRepo := repository.NewMajorRepository(db)
	templateRepo := repository.NewTemplateRepository(db)
	ediplomaRepo := repository.NewEDiplomaRepository(db, facultyRepo)
	templateSampleRepo := repository.NewTemplateSampleRepo(db)
	pqcKeyRepo := repository.NewPQCKeyRepository(db)
	if err := pqcKeyRepo.EnsureIndexes(context.Background()); err != nil {
		log.Printf("Không thể tạo index quản lý khóa PQC: %v", err)
	}
	roundRepo := repository.NewIssuanceRoundRepository(db)
	if err := roundRepo.EnsureIndexes(context.Background()); err != nil {
		log.Printf("Không thể tạo index đợt cấp: %v", err)
	}

	// Services
	templateEngine := models.NewTemplateEngine() // giả định bạn có utils/template_engine.go
	pdfGenerator := utils.NewPDFGenerator()      // giả định bạn có utils/pdf_generator.go

	ediplomaService := service.NewEDiplomaService(
		*templateSampleRepo,
		universityRepo,
		majorRepo,
		facultyRepo,
		ediplomaRepo,
		templateRepo,
		userRepo,
		minioClient,
		templateEngine,
		pdfGenerator,
		pqcKeyRepo,
		roundRepo,
		service.NewStudentNotifier(userRepo, authRepo, emailSender),
	)

	userService := service.NewUserService(userRepo, universityRepo, facultyRepo, authRepo, ediplomaRepo)
	authService := service.NewAuthService(authRepo, userRepo, emailSender)
	middleware.SetAccountGuard(authService.ValidateSession)
	universityService := service.NewUniversityService(universityRepo, authRepo, repository.NewAdminRepository(db), emailSender)
	facultyService := service.NewFacultyService(universityRepo, facultyRepo)
	pqcTransactionSigner := service.NewPQCTransactionSigner(pqcKeyRepo)
	blockchainSvc := service.NewBlockchainService(templateRepo, ediplomaRepo, userRepo, authRepo, facultyRepo, universityRepo, fabricClient, minioClient, pqcTransactionSigner, emailSender)
	majorService := service.NewMajorService(majorRepo, facultyRepo)
	templateSampleService := service.NewTemplateSampleService(templateSampleRepo, templateRepo)
	pqcService := service.NewPQCService(pqcKeyRepo, ediplomaRepo, ediplomaService, blockchainSvc)

	templateService := service.NewTemplateService(
		templateRepo,
		facultyRepo,
		universityRepo,
		facultyService,
		*templateSampleService,
		minioClient,
	) // Handlers
	facultyHandler := handlers.NewFacultyHandler(facultyService)
	userHandler := handlers.NewUserHandler(userService)
	authHandler := handlers.NewAuthHandler(authService, universityService, userService, facultyService)
	universityHandler := handlers.NewUniversityHandler(universityService)
	majorHandler := handlers.NewMajorHandler(majorService)
	templateHandler := handlers.NewTemplateHandler(templateService, minioClient, facultyService)
	ediplomaHandler := handlers.NewEDiplomaHandler(ediplomaService)
	blockchainHandler := handlers.NewBlockchainHandler(blockchainSvc, ediplomaService)
	templateSampleHandler := handlers.NewTemplateSampleHandler(templateSampleService)
	pqcHandler := handlers.NewPQCHandler(pqcService)

	// Setup router
	r := routes.SetupRouter(
		db,
		userHandler,
		authHandler,
		universityHandler,
		facultyHandler,
		blockchainHandler,
		majorHandler,
		templateHandler,
		ediplomaHandler,
		templateSampleHandler,
		pqcHandler,
		&handlers.DegreePortal{DB: db, PQC: pqcService, Degrees: ediplomaService},
	)
	// Xử lý tín hiệu dừng
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		if fabricClient != nil {
			if err := fabricClient.Close(); err != nil {
				log.Printf("Lỗi khi đóng kết nối Fabric Gateway: %v", err)
			}
		}
		log.Println("Đang tắt server...")
		if err := database.CloseMongo(); err != nil {
			log.Printf("Lỗi khi đóng kết nối MongoDB: %v", err)
		}
		os.Exit(0)
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Không thể khởi động server: %v", err)
	}

}
