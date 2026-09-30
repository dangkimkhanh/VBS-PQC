package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/repository"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func seedAdminAccount(db *mongo.Database) {
	adminEmail := utils.NormalizeEmail(os.Getenv("ADMIN_EMAIL"))
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	if adminEmail == "" || adminPassword == "" {
		log.Fatal("ADMIN_EMAIL hoặc ADMIN_PASSWORD chưa được cấu hình trong .env")
	}

	collection := db.Collection("accounts")

	// The platform admin is created once, on the first run. Changing ADMIN_EMAIL or
	// ADMIN_PASSWORD later has no effect; use "Đổi mật khẩu" or "Quên mật khẩu" instead.
	count, err := collection.CountDocuments(context.TODO(), bson.M{"role": "admin"})
	if err != nil {
		log.Fatalf("Lỗi khi kiểm tra tài khoản admin: %v", err)
	}
	if count > 0 {
		log.Println("Tài khoản admin đã tồn tại, không cần tạo thêm.")
		return
	}

	if len(adminPassword) < 12 {
		log.Fatal("ADMIN_PASSWORD phải có ít nhất 12 ký tự")
	}
	passwordHash, err := utils.HashPassword(adminPassword)
	if err != nil {
		log.Fatalf("Lỗi khi hash mật khẩu: %v", err)
	}

	admin := models.Account{
		ID:            primitive.NewObjectID(),
		StudentID:     primitive.NilObjectID,
		StudentEmail:  "",
		PersonalEmail: adminEmail,
		PasswordHash:  passwordHash,
		CreatedAt:     time.Now(),
		Role:          "admin",
		Status:        models.AccountActive,
	}

	_, err = collection.InsertOne(context.TODO(), admin)
	if err != nil {
		log.Fatalf("Lỗi khi tạo tài khoản admin: %v", err)
	}

	log.Println("Tài khoản admin đã được tạo thành công.")
}

// SeedTemplateSamples installs the built-in diploma layouts and keeps them in
// step with the files shipped in pkg/database. Built-in samples belong to the
// platform, so a newer layout replaces the stored one, and every diploma
// template that uses it is re-pinned to the new content. PDFs already issued
// are stored and hashed separately and are not affected.
func SeedTemplateSamples(ctx context.Context, repo *repository.TemplateSampleRepo, templateRepo repository.TemplateRepository) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	builtIn := []struct{ name, file string }{
		{"Mẫu 1", "template1.html"},
		{"Mẫu 2", "template2.html"},
		{"Mẫu 3", "template3.html"},
	}

	for _, b := range builtIn {
		data, err := os.ReadFile(filepath.Join(cwd, "pkg", "database", b.file))
		if err != nil {
			return fmt.Errorf("read %s: %w", b.file, err)
		}
		html := string(data)

		existing, err := repo.FindSystemByName(ctx, b.name)
		if err != nil {
			return err
		}
		if existing == nil {
			now := time.Now()
			sample := models.TemplateSample{Name: b.name, HTMLContent: html, UniversityID: primitive.NilObjectID, CreatedAt: now, UpdatedAt: now}
			if _, err := repo.Create(ctx, &sample); err != nil {
				return err
			}
			continue
		}
		if existing.HTMLContent == html {
			continue
		}

		existing.HTMLContent = html
		if err := repo.Update(ctx, existing); err != nil {
			return err
		}
		repinned, err := templateRepo.UpdateHashBySampleID(ctx, existing.ID, utils.ComputeSHA256(data))
		if err != nil {
			return err
		}
		log.Printf("Đã cập nhật giao diện %s (%d mẫu văn bằng dùng giao diện này)", b.name, repinned)
	}

	return nil
}
