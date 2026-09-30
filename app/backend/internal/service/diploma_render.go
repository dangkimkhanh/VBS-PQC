package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
)

// diplomaRenderData builds the values a diploma template can use. Every
// generation path (single, bulk, zip) must go through here so the same record
// always renders the same way.
//
// Keys: SoHieu, SoVaoSo, HoTen, NgaySinh, TenTruong, Nganh, XepLoai,
// HinhThucDaoTao, Khoa, NgayCap, NgayCapChu, LoaiVanBang, ChucDanhKy, TenNguoiKy,
// ConDau (the digital-signature stamp, ready-to-embed SVG).
// algorithm is the ML-DSA parameter set of the key that will sign the diploma.
func diplomaRenderData(ed *models.EDiploma, user *models.User, university *models.University, faculty *models.Faculty, issueDate time.Time, algorithm string) map[string]interface{} {
	fullName := strings.TrimSpace(ed.FullName)
	dob := ""
	if user != nil {
		if strings.TrimSpace(user.FullName) != "" {
			fullName = strings.TrimSpace(user.FullName)
		}
		// Dates of birth are stored as dd/mm/yyyy; older records may be ISO.
		if t, err := utils.ParseDate(user.DateOfBirth); err == nil && !t.IsZero() {
			dob = t.Format("02/01/2006")
		}
	}

	universityName, signerName := "", ""
	if university != nil {
		universityName = strings.TrimSpace(university.UniversityName)
		signerName = strings.TrimSpace(university.SignerName)
	}
	major := ""
	if faculty != nil {
		major = strings.TrimSpace(faculty.FacultyName)
	}

	issued, issuedWords := "", ""
	if !issueDate.IsZero() {
		issued = issueDate.Format("02/01/2006")
		issuedWords = fmt.Sprintf("Ngày %02d tháng %02d năm %d", issueDate.Day(), int(issueDate.Month()), issueDate.Year())
	}

	return map[string]interface{}{
		"SoHieu":         strings.TrimSpace(ed.SerialNumber),
		"SoVaoSo":        strings.TrimSpace(ed.RegistrationNumber),
		"HoTen":          fullName,
		"NgaySinh":       dob,
		"TenTruong":      universityName,
		"Nganh":          major,
		"XepLoai":        strings.TrimSpace(ed.GraduationRank),
		"HinhThucDaoTao": strings.TrimSpace(ed.EducationType),
		"Khoa":           strings.TrimSpace(ed.Course),
		"NgayCap":        issued,
		"NgayCapChu":     issuedWords,
		"LoaiVanBang":    diplomaTitle(ed),
		"ChucDanhKy":     signerTitle(universityName),
		"TenNguoiKy":     signerName,
		"ConDau":         diplomaSealSVG(universityName, algorithm),
	}
}

// diplomaTitle returns the printed title, e.g. "BẰNG CỬ NHÂN" or "BẰNG KỸ SƯ".
func diplomaTitle(ed *models.EDiploma) string {
	for _, candidate := range []string{ed.CertificateType, ed.Name} {
		value := strings.ToUpper(strings.TrimSpace(candidate))
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "BẰNG") {
			return value
		}
		return "BẰNG " + value
	}
	return "BẰNG TỐT NGHIỆP"
}

// signerTitle follows Vietnamese usage: academies and national/regional
// universities are headed by a "Giám đốc", other institutions by a "Hiệu trưởng".
func signerTitle(universityName string) string {
	name := strings.ToLower(strings.TrimSpace(universityName))
	if strings.HasPrefix(name, "học viện") || strings.HasPrefix(name, "đại học") {
		return "GIÁM ĐỐC"
	}
	return "HIỆU TRƯỞNG"
}
