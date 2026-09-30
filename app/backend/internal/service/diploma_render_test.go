package service

import (
	"strings"
	"testing"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
)

func TestDiplomaRenderDataUsesStoredDateFormatAndMajor(t *testing.T) {
	ed := &models.EDiploma{Name: "Bằng tốt nghiệp", CertificateType: "Kỹ sư", Course: "2021", EducationType: "Chính quy", GraduationRank: "Giỏi", SerialNumber: "333", RegistrationNumber: "4444"}
	user := &models.User{FullName: "Đặng Kim Khánh", DateOfBirth: "11/05/2003"}
	university := &models.University{UniversityName: "Học viện Kỹ thuật Mật mã", SignerName: "Đặng Kim Khánh"}
	faculty := &models.Faculty{FacultyName: "An toàn thông tin"}

	data := diplomaRenderData(ed, user, university, faculty, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), "ML-DSA-65")

	want := map[string]string{
		"NgaySinh":    "11/05/2003",
		"Nganh":       "An toàn thông tin",
		"LoaiVanBang": "BẰNG KỸ SƯ",
		"ChucDanhKy":  "GIÁM ĐỐC",
		"TenNguoiKy":  "Đặng Kim Khánh",
		"NgayCap":     "16/09/2026",
		"NgayCapChu":  "Ngày 16 tháng 09 năm 2026",
	}
	for key, value := range want {
		if data[key] != value {
			t.Errorf("%s = %q, want %q", key, data[key], value)
		}
	}
}

func TestDiplomaRenderDataLeavesUnknownValuesBlank(t *testing.T) {
	data := diplomaRenderData(&models.EDiploma{}, &models.User{DateOfBirth: ""}, &models.University{UniversityName: "Trường Đại học Bách khoa"}, nil, time.Time{}, "ML-DSA-65")

	for _, key := range []string{"NgaySinh", "Nganh", "NgayCap", "NgayCapChu"} {
		if data[key] != "" {
			t.Errorf("%s = %q, want blank", key, data[key])
		}
	}
	if data["LoaiVanBang"] != "BẰNG TỐT NGHIỆP" {
		t.Errorf("LoaiVanBang = %q", data["LoaiVanBang"])
	}
	if data["ChucDanhKy"] != "HIỆU TRƯỞNG" {
		t.Errorf("ChucDanhKy = %q", data["ChucDanhKy"])
	}
}

func TestTemplateEngineEscapesStudentData(t *testing.T) {
	out, err := models.NewTemplateEngine().Render(`<p>{{ .HoTen }}</p>`, map[string]interface{}{"HoTen": `<img src=x onerror=alert(1)>`})
	if err != nil {
		t.Fatal(err)
	}
	if out != `<p>&lt;img src=x onerror=alert(1)&gt;</p>` {
		t.Fatalf("student data was not escaped: %s", out)
	}
}

func TestDiplomaSealShowsUniversityAndAlgorithmSafely(t *testing.T) {
	seal := string(diplomaSealSVG(`Học viện <script>alert(1)</script>`, "ML-DSA-87"))
	if !strings.Contains(seal, "HỌC VIỆN &lt;SCRIPT&gt;") || strings.Contains(seal, "<script>") {
		t.Fatalf("university name not escaped: %s", seal)
	}
	for _, want := range []string{"KÝ SỐ HẬU LƯỢNG TỬ", ">ML-DSA-87<"} {
		if !strings.Contains(seal, want) {
			t.Errorf("seal is missing %q", want)
		}
	}
	if diplomaSealSVG("  ", "ML-DSA-65") != "" {
		t.Error("expected no seal without a university name")
	}
}
