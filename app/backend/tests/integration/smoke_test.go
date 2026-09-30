package integration

// End-to-end API tests: the real router and services against a throwaway
// MongoDB database (dropped afterwards). Run with TEST_MONGODB_URI set
// (e.g. mongodb://localhost:27017); skipped otherwise.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/vnkmasc/Kmasc/app/backend/internal/handlers"
	"github.com/vnkmasc/Kmasc/app/backend/internal/middleware"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/repository"
	"github.com/vnkmasc/Kmasc/app/backend/internal/service"
	"github.com/vnkmasc/Kmasc/app/backend/routes"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mail struct{ to, subject, body string }

type fakeMailer struct {
	mu   sync.Mutex
	sent []mail
}

func (f *fakeMailer) SendEmail(to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, mail{to, subject, body})
	return nil
}

func (f *fakeMailer) last(to string, t *testing.T) mail {
	t.Helper()
	for i := 0; i < 50; i++ {
		f.mu.Lock()
		for j := len(f.sent) - 1; j >= 0; j-- {
			if f.sent[j].to == to {
				m := f.sent[j]
				f.mu.Unlock()
				return m
			}
		}
		f.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no email sent to %s", to)
	return mail{}
}

func (f *fakeMailer) count(to string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, m := range f.sent {
		if m.to == to {
			n++
		}
	}
	return n
}

var (
	tokenRe = regexp.MustCompile(`token=([A-Z0-9]+)`)
	otpRe   = regexp.MustCompile(`là: (\d{6})`)
)

type client struct {
	t *testing.T
	r *gin.Engine
}

func (c client) do(method, path, token string, body interface{}) (int, map[string]interface{}) {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, "/api/v1"+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = fmt.Sprintf("10.0.%d.%d:1234", time.Now().UnixNano()%250, time.Now().UnixNano()%250)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	c.r.ServeHTTP(w, req)
	out := map[string]interface{}{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func (c client) expect(want int, method, path, token string, body interface{}) map[string]interface{} {
	c.t.Helper()
	code, out := c.do(method, path, token, body)
	if code != want {
		c.t.Fatalf("%s %s: want %d, got %d %v", method, path, want, code, out)
	}
	return out
}

func (c client) login(email, password string) string {
	c.t.Helper()
	out := c.expect(200, "POST", "/auth/login", "", gin.H{"email": email, "password": password})
	return out["token"].(string)
}

func TestSmoke(t *testing.T) {
	uri := os.Getenv("TEST_MONGODB_URI")
	if uri == "" {
		t.Skip("TEST_MONGODB_URI not set")
	}
	os.Setenv("JWT_SECRET", "smoke-secret")
	gin.SetMode(gin.TestMode)
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("courseyear", func(fl validator.FieldLevel) bool {
			return regexp.MustCompile(`^\d{4}$`).MatchString(fl.Field().String())
		})
		_ = v.RegisterValidation("dateformat", func(fl validator.FieldLevel) bool {
			_, err := time.Parse("02/01/2006", fl.Field().String())
			return err == nil
		})
		_ = v.RegisterValidation("citizenid", func(fl validator.FieldLevel) bool {
			return regexp.MustCompile(`^\d{12}$`).MatchString(fl.Field().String())
		})
	}

	ctx := context.Background()
	mc, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Skip("mongo unavailable:", err)
	}
	dbName := fmt.Sprintf("vbs_pqc_smoke_%d", time.Now().Unix())
	db := mc.Database(dbName)
	defer func() { _ = db.Drop(ctx); _ = mc.Disconnect(ctx) }()

	mailer := &fakeMailer{}
	userRepo := repository.NewUserRepository(db)
	authRepo := repository.NewAuthRepository(db)
	if err := authRepo.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	universityRepo := repository.NewUniversityRepository(db)
	facultyRepo := repository.NewFacultyRepository(db)
	majorRepo := repository.NewMajorRepository(db)
	templateRepo := repository.NewTemplateRepository(db)
	ediplomaRepo := repository.NewEDiplomaRepository(db, facultyRepo)
	templateSampleRepo := repository.NewTemplateSampleRepo(db)
	pqcKeyRepo := repository.NewPQCKeyRepository(db)
	if err := repository.NewIssuanceRoundRepository(db).EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}

	ediplomaService := service.NewEDiplomaService(*templateSampleRepo, universityRepo, majorRepo, facultyRepo, ediplomaRepo, templateRepo, userRepo, nil, models.NewTemplateEngine(), utils.NewPDFGenerator(), repository.NewPQCKeyRepository(db), repository.NewIssuanceRoundRepository(db), service.NewStudentNotifier(userRepo, authRepo, mailer))
	userService := service.NewUserService(userRepo, universityRepo, facultyRepo, authRepo, ediplomaRepo)
	authService := service.NewAuthService(authRepo, userRepo, mailer)
	middleware.SetAccountGuard(authService.ValidateSession)
	universityService := service.NewUniversityService(universityRepo, authRepo, repository.NewAdminRepository(db), mailer)
	facultyService := service.NewFacultyService(universityRepo, facultyRepo)
	blockchainSvc := service.NewBlockchainService(templateRepo, ediplomaRepo, userRepo, authRepo, facultyRepo, universityRepo, nil, nil, service.NewPQCTransactionSigner(pqcKeyRepo), mailer)
	templateSampleService := service.NewTemplateSampleService(templateSampleRepo, templateRepo)
	pqcService := service.NewPQCService(pqcKeyRepo, ediplomaRepo, ediplomaService, blockchainSvc)
	templateService := service.NewTemplateService(templateRepo, facultyRepo, universityRepo, facultyService, *templateSampleService, nil)

	r := routes.SetupRouter(db,
		handlers.NewUserHandler(userService),
		handlers.NewAuthHandler(authService, universityService, userService, facultyService),
		handlers.NewUniversityHandler(universityService),
		handlers.NewFacultyHandler(facultyService),
		handlers.NewBlockchainHandler(blockchainSvc, ediplomaService),
		handlers.NewMajorHandler(service.NewMajorService(majorRepo, facultyRepo)),
		handlers.NewTemplateHandler(templateService, nil, facultyService),
		handlers.NewEDiplomaHandler(ediplomaService),
		handlers.NewTemplateSampleHandler(templateSampleService),
		handlers.NewPQCHandler(pqcService),
		&handlers.DegreePortal{DB: db, PQC: pqcService, Degrees: ediplomaService},
	)
	c := client{t: t, r: r}

	// Platform admin (normally created by the seeder on first run).
	hash, _ := utils.HashPassword("SuperAdmin#2026")
	if err := authRepo.CreateAccount(ctx, &models.Account{ID: primitive.NewObjectID(), PersonalEmail: "root@vbs.test", PasswordHash: hash, Role: "admin", Status: "active", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	adminTok := c.login("ROOT@vbs.test", "SuperAdmin#2026")

	t.Run("anonymous and wrong login are rejected uniformly", func(t *testing.T) {
		c.expect(401, "GET", "/universities", "", nil)
		a := c.expect(401, "POST", "/auth/login", "", gin.H{"email": "nobody@vbs.test", "password": "x"})
		b := c.expect(401, "POST", "/auth/login", "", gin.H{"email": "root@vbs.test", "password": "wrong"})
		if a["error"] != b["error"] {
			t.Fatalf("login errors differ: %v vs %v", a, b)
		}
	})

	createUni := func(code, domain, adminEmail string) string {
		out := c.expect(201, "POST", "/universities", adminTok, gin.H{
			"university_name": "Trường " + code, "university_code": code, "address": "Hà Nội",
			"email_domain": domain, "admin_email": adminEmail, "admin_name": "QTV " + code, "admin_phone": "0912345678",
		})
		if out["email_sent"] != true {
			t.Fatalf("activation email not reported: %v", out)
		}
		return out["id"].(string)
	}
	activate := func(email, password string) {
		m := mailer.last(email, t)
		tok := tokenRe.FindStringSubmatch(m.body)
		if tok == nil {
			t.Fatalf("no activation token in %q", m.body)
		}
		c.expect(200, "POST", "/auth/activate-account", "", gin.H{"token": tok[1], "password": password})
		c.expect(400, "POST", "/auth/activate-account", "", gin.H{"token": tok[1], "password": password})
	}

	uniA := createUni("KMA", "@actvn.edu.vn", "admin@kma.test")
	c.expect(409, "POST", "/universities", adminTok, gin.H{
		"university_name": "trường kma", "university_code": "X1", "address": "HN", "email_domain": "x.edu.vn",
		"admin_email": "a@x.test", "admin_name": "A", "admin_phone": "0912345678",
	})
	c.expect(400, "POST", "/universities", adminTok, gin.H{
		"university_name": "Bad", "university_code": "BAD", "address": "HN", "email_domain": "not a domain",
		"admin_email": "b@x.test", "admin_name": "A", "admin_phone": "0912345678",
	})
	c.expect(401, "POST", "/auth/login", "", gin.H{"email": "admin@kma.test", "password": "SchoolA#123"})
	activate("admin@kma.test", "SchoolA#123")
	schoolA := c.login("admin@kma.test", "SchoolA#123")

	uniB := createUni("HUST", "hust.edu.vn", "admin@hust.test")
	activate("admin@hust.test", "SchoolB#123")
	schoolB := c.login("admin@hust.test", "SchoolB#123")

	t.Run("school admins cannot use platform admin APIs", func(t *testing.T) {
		c.expect(403, "GET", "/universities", schoolA, nil)
		c.expect(403, "GET", "/auth/accounts", schoolA, nil)
		c.expect(403, "POST", "/universities/"+uniB+"/lock", schoolA, nil)
	})

	fac := c.expect(201, "POST", "/faculties", schoolA, gin.H{"faculty_code": "CNTT", "faculty_name": "Công nghệ thông tin"})
	_ = fac
	facs := c.expect(200, "GET", "/faculties", schoolA, nil)["data"].([]interface{})
	facultyID := facs[0].(map[string]interface{})["id"].(string)

	student := gin.H{
		"student_code": "CT060346", "full_name": "Nguyễn Văn A", "email": "CT060346@actvn.edu.vn", "faculty_code": "CNTT",
		"course": "2021", "citizen_id_number": "001203000001", "date_of_birth": "01/01/2003",
	}
	t.Run("student email must match the school domain", func(t *testing.T) {
		bad := gin.H{}
		for k, v := range student {
			bad[k] = v
		}
		bad["email"] = "someone@gmail.com"
		c.expect(400, "POST", "/users", schoolA, bad)
	})
	created := c.expect(201, "POST", "/users", schoolA, student)["data"].(map[string]interface{})
	studentID := created["id"].(string)

	t.Run("cross-university access is hidden", func(t *testing.T) {
		c.expect(404, "GET", "/users/"+studentID, schoolB, nil)
		c.expect(404, "PUT", "/users/"+studentID, schoolB, gin.H{"full_name": "Hacked"})
		c.expect(404, "DELETE", "/users/"+studentID, schoolB, nil)
		c.expect(404, "DELETE", "/faculties/"+facultyID, schoolB, nil)
		c.expect(200, "GET", "/users/"+studentID, schoolA, nil)
		out := c.expect(200, "GET", "/ediplomas/search?university_id="+uniA, schoolB, nil)
		if n := out["total"].(float64); n != 0 {
			t.Fatalf("school B saw %v diplomas of A", n)
		}
	})

	var studentTok string
	t.Run("student OTP registration", func(t *testing.T) {
		out := c.expect(200, "POST", "/auth/request-otp", "", gin.H{"student_email": "nobody@actvn.edu.vn"})
		generic := out["message"]
		out = c.expect(200, "POST", "/auth/request-otp", "", gin.H{"student_email": "ct060346@actvn.edu.vn"})
		if out["message"] != generic {
			t.Fatalf("OTP response leaks existence: %v vs %v", out["message"], generic)
		}
		m := mailer.last("ct060346@actvn.edu.vn", t)
		code := otpRe.FindStringSubmatch(m.body)
		if code == nil {
			t.Fatalf("no numeric OTP in %q", m.body)
		}
		wrong := "000000"
		if code[1] == wrong {
			wrong = "111111"
		}
		c.expect(400, "POST", "/auth/verify-otp", "", gin.H{"student_email": "ct060346@actvn.edu.vn", "otp": wrong})
		c.expect(400, "POST", "/auth/verify-otp", "", gin.H{"student_email": "ct060346@actvn.edu.vn", "otp": "12ab56"})
		v := c.expect(200, "POST", "/auth/verify-otp", "", gin.H{"student_email": "ct060346@actvn.edu.vn", "otp": code[1]})
		c.expect(400, "POST", "/auth/verify-otp", "", gin.H{"student_email": "ct060346@actvn.edu.vn", "otp": code[1]})
		c.expect(400, "POST", "/auth/register", "", gin.H{"user_id": v["user_id"], "personal_email": "a@gmail.com", "password": "short", "otp_token": v["verification_token"]})
		c.expect(409, "POST", "/auth/register", "", gin.H{"user_id": v["user_id"], "personal_email": "admin@kma.test", "password": "Student#123", "otp_token": v["verification_token"]})
		c.expect(201, "POST", "/auth/register", "", gin.H{"user_id": v["user_id"], "personal_email": "Vana@Gmail.com", "password": "Student#123", "otp_token": v["verification_token"]})
		c.expect(400, "POST", "/auth/register", "", gin.H{"user_id": v["user_id"], "personal_email": "other@gmail.com", "password": "Student#123", "otp_token": v["verification_token"]})
		studentTok = c.login("vana@gmail.com", "Student#123")
		c.login("CT060346@actvn.edu.vn", "Student#123")
		before := mailer.count("ct060346@actvn.edu.vn")
		c.expect(200, "POST", "/auth/request-otp", "", gin.H{"student_email": "ct060346@actvn.edu.vn"})
		time.Sleep(100 * time.Millisecond)
		if mailer.count("ct060346@actvn.edu.vn") != before {
			t.Fatal("OTP sent for an already active account")
		}
	})

	t.Run("students are read-only and scoped to themselves", func(t *testing.T) {
		me := c.expect(200, "GET", "/users/me", studentTok, nil)
		if me["student_code"] != "CT060346" {
			t.Fatalf("unexpected profile %v", me)
		}
		c.expect(200, "GET", "/ediplomas/simple", studentTok, nil)
		c.expect(403, "GET", "/users/search", studentTok, nil)
		c.expect(403, "GET", "/users/"+studentID, studentTok, nil)
		c.expect(403, "PUT", "/users/"+studentID, studentTok, gin.H{"full_name": "X"})
		c.expect(403, "POST", "/users", studentTok, student)
		c.expect(403, "GET", "/faculties", studentTok, nil)
		c.expect(403, "GET", "/ediplomas/search", studentTok, nil)
		c.expect(403, "POST", "/pqc/keys", studentTok, gin.H{"name": "k"})
		c.expect(404, "GET", "/ediplomas/"+primitive.NewObjectID().Hex(), studentTok, nil)
		c.expect(403, "GET", "/users/me", schoolA, nil)
	})

	t.Run("legacy product routes are gone", func(t *testing.T) {
		for _, p := range []string{"/certificates", "/reward-disciplines", "/verification/my-codes"} {
			if code, _ := c.do("GET", p, adminTok, nil); code != 404 {
				t.Fatalf("%s still routed: %d", p, code)
			}
		}
		if code, _ := c.do("POST", "/universities/approve-or-reject", adminTok, gin.H{}); code != 404 {
			t.Fatalf("approve-or-reject still routed: %d", code)
		}
		if code, _ := c.do("POST", "/upload", adminTok, nil); code != 404 {
			t.Fatalf("/upload still routed: %d", code)
		}
	})

	t.Run("password change revokes old sessions", func(t *testing.T) {
		time.Sleep(1100 * time.Millisecond)
		c.expect(400, "POST", "/auth/change-password", studentTok, gin.H{"old_password": "Student#123", "new_password": "short"})
		c.expect(200, "POST", "/auth/change-password", studentTok, gin.H{"old_password": "Student#123", "new_password": "Student#456"})
		c.expect(401, "GET", "/users/me", studentTok, nil)
		studentTok = c.login("vana@gmail.com", "Student#456")
	})

	t.Run("forgot password is generic and single use", func(t *testing.T) {
		a := c.expect(200, "POST", "/auth/forgot-password", "", gin.H{"email": "ghost@nowhere.test"})
		b := c.expect(200, "POST", "/auth/forgot-password", "", gin.H{"email": "VANA@gmail.com"})
		if a["message"] != b["message"] {
			t.Fatal("forgot-password leaks existence")
		}
		m := mailer.last("vana@gmail.com", t)
		tok := tokenRe.FindStringSubmatch(m.body)
		if tok == nil || !strings.Contains(m.body, "/auth/reset-password?token=") {
			t.Fatalf("bad reset email %q", m.body)
		}
		time.Sleep(1100 * time.Millisecond)
		c.expect(200, "POST", "/auth/reset-password", "", gin.H{"token": tok[1], "password": "Student#789"})
		c.expect(400, "POST", "/auth/reset-password", "", gin.H{"token": tok[1], "password": "Student#789"})
		c.expect(401, "GET", "/users/me", studentTok, nil)
		studentTok = c.login("vana@gmail.com", "Student#789")
	})

	t.Run("locking a university blocks its admin but not its students", func(t *testing.T) {
		time.Sleep(1100 * time.Millisecond)
		c.expect(200, "POST", "/universities/"+uniA+"/lock", adminTok, nil)
		c.expect(401, "GET", "/faculties", schoolA, nil)
		out := c.expect(401, "POST", "/auth/login", "", gin.H{"email": "admin@kma.test", "password": "SchoolA#123"})
		if !strings.Contains(out["error"].(string), "khóa") {
			t.Fatalf("locked login message: %v", out)
		}
		c.expect(200, "GET", "/users/me", studentTok, nil)
		c.expect(200, "POST", "/auth/forgot-password", "", gin.H{"email": "admin@kma.test"})
		time.Sleep(100 * time.Millisecond)
		for _, m := range mailer.sent {
			if m.to == "admin@kma.test" && strings.Contains(m.body, "reset-password") {
				t.Fatal("reset link sent to a locked account")
			}
		}
		c.expect(200, "POST", "/universities/"+uniA+"/unlock", adminTok, nil)
		schoolA = c.login("admin@kma.test", "SchoolA#123")
		c.expect(200, "GET", "/faculties", schoolA, nil)
		list := c.expect(200, "GET", "/universities", adminTok, nil)["data"].([]interface{})
		if len(list) != 2 {
			t.Fatalf("want 2 universities, got %d", len(list))
		}
	})

	t.Run("changing the school admin email hands over the account", func(t *testing.T) {
		time.Sleep(1100 * time.Millisecond)
		out := c.expect(200, "PUT", "/universities/"+uniA, adminTok, gin.H{
			"university_name": "Học viện Kỹ thuật Mật mã", "address": "141 Chiến Thắng", "email_domain": "actvn.edu.vn",
			"admin_email": "newadmin@kma.test", "admin_name": "QTV mới", "admin_phone": "0987654321",
		})
		if out["activation_sent"] != true {
			t.Fatalf("activation not sent on handover: %v", out)
		}
		c.expect(401, "GET", "/faculties", schoolA, nil)
		c.expect(401, "POST", "/auth/login", "", gin.H{"email": "admin@kma.test", "password": "SchoolA#123"})
		activate("newadmin@kma.test", "NewAdmin#123")
		schoolA = c.login("newadmin@kma.test", "NewAdmin#123")
		c.expect(409, "PUT", "/universities/"+uniB, adminTok, gin.H{
			"university_name": "Trường HUST", "address": "HN", "email_domain": "actvn.edu.vn",
			"admin_email": "admin@hust.test", "admin_name": "QTV", "admin_phone": "0912345678",
		})
	})

	t.Run("deleting a student removes the account", func(t *testing.T) {
		c.expect(200, "DELETE", "/users/"+studentID, schoolA, nil)
		c.expect(401, "GET", "/users/me", studentTok, nil)
		c.expect(401, "POST", "/auth/login", "", gin.H{"email": "vana@gmail.com", "password": "Student#789"})
	})

	t.Run("rate limit on login", func(t *testing.T) {
		limited := false
		for i := 0; i < 12; i++ {
			req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"email":"x@y.z","password":"p"}`))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = "192.0.2.1:5555"
			req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code == 429 {
				limited = true
			}
		}
		if !limited {
			t.Fatal("spoofed X-Forwarded-For bypassed the login rate limit")
		}
	})

	t.Run("activation link cannot be resent right after it was sent", func(t *testing.T) {
		pending := createUni("TLU", "tlu.edu.vn", "admin@tlu.test")
		before := mailer.count("admin@tlu.test")
		c.expect(429, "POST", "/universities/"+pending+"/resend-activation", adminTok, nil)
		if mailer.count("admin@tlu.test") != before {
			t.Fatal("a refused resend must not send another email")
		}
		list := c.expect(200, "GET", "/universities?q=TLU", adminTok, nil)
		row := list["data"].([]interface{})[0].(map[string]interface{})
		if row["activation_resend_at"] == nil || row["activation_resend_at"] == "" {
			t.Fatalf("list must tell the UI when a resend is allowed: %v", row)
		}
	})

	t.Run("issuance rounds scope import, batch actions and revocation", func(t *testing.T) {
		uni := createUni("PTIT", "ptit.edu.vn", "admin@ptit.test")
		activate("admin@ptit.test", "SchoolP#123")
		school := c.login("admin@ptit.test", "SchoolP#123")
		c.expect(201, "POST", "/faculties", school, gin.H{"faculty_code": "ATTT", "faculty_name": "An toàn thông tin"})
		faculty := c.expect(200, "GET", "/faculties", school, nil)["data"].([]interface{})[0].(map[string]interface{})["id"].(string)
		c.expect(201, "POST", "/users", school, gin.H{
			"student_code": "AT180101", "full_name": "Trần Thị Bình", "email": "at180101@ptit.edu.vn", "faculty_code": "ATTT",
			"course": "2021", "citizen_id_number": "001203000099", "date_of_birth": "02/02/2003",
		})
		diploma := func(round, serial string) gin.H {
			return gin.H{"student_code": "AT180101", "name": "Bằng kỹ sư", "certificate_type": "Kỹ sư", "course": "2021",
				"issue_date": "2026-07-15", "serial_number": serial, "registration_number": "SO-" + serial, "round_id": round}
		}

		// Rounds: required, unique per university (case and spaces ignored).
		c.expect(400, "POST", "/ediplomas", school, diploma("", "S-0"))
		c.expect(400, "POST", "/issuance-rounds", school, gin.H{"name": "ab"})
		first := c.expect(201, "POST", "/issuance-rounds", school, gin.H{"name": "Tốt nghiệp đợt 1 (K2021)", "decision_number": "123/QĐ"})["data"].(map[string]interface{})["id"].(string)
		c.expect(409, "POST", "/issuance-rounds", school, gin.H{"name": "  tốt nghiệp   ĐỢT 1 (k2021) "})
		var newest string
		for i := 2; i <= 7; i++ {
			newest = c.expect(201, "POST", "/issuance-rounds", school, gin.H{"name": fmt.Sprintf("Đợt bổ sung %d", i)})["data"].(map[string]interface{})["id"].(string)
			time.Sleep(5 * time.Millisecond)
		}
		recent := c.expect(200, "GET", "/issuance-rounds", school, nil)["data"].([]interface{})
		if len(recent) != 5 || recent[0].(map[string]interface{})["id"] != newest {
			t.Fatalf("the picker must suggest the 5 newest rounds first: %v", recent)
		}
		found := c.expect(200, "GET", "/issuance-rounds?q=(K2021)", school, nil)["data"].([]interface{})
		if len(found) != 1 || found[0].(map[string]interface{})["id"] != first {
			t.Fatalf("round search must match the typed text literally: %v", found)
		}
		if other := c.expect(200, "GET", "/issuance-rounds", schoolB, nil)["data"].([]interface{}); len(other) != 0 {
			t.Fatalf("another university sees these rounds: %v", other)
		}

		c.expect(404, "POST", "/ediplomas", schoolB, diploma(first, "S-1"))
		created := c.expect(201, "POST", "/ediplomas", school, diploma(first, "S-1"))["data"].(map[string]interface{})
		if created["round_id"] != first {
			t.Fatalf("diploma not attached to its round: %v", created)
		}
		byFaculty := c.expect(200, "GET", "/issuance-rounds?faculty_id="+faculty, school, nil)["data"].([]interface{})
		if len(byFaculty) != 1 || byFaculty[0].(map[string]interface{})["id"] != first {
			t.Fatalf("faculty suggestions must list only rounds holding its diplomas: %v", byFaculty)
		}

		// Search by round and by student code or name.
		for query, want := range map[string]float64{
			"round_id=" + first + "&keyword=tr%E1%BA%A7n%20th%E1%BB%8B": 1,
			"keyword=at1801":     1,
			"keyword=.*":         0,
			"round_id=" + newest: 0,
			"round_id=none":      0,
		} {
			out := c.expect(200, "GET", "/ediplomas/search?"+query, school, nil)
			if out["total"].(float64) != want {
				t.Fatalf("search %s: want %v, got %v", query, want, out["total"])
			}
			if want == 1 && out["data"].([]interface{})[0].(map[string]interface{})["round_name"] != "Tốt nghiệp đợt 1 (K2021)" {
				t.Fatalf("search result must carry the round name: %v", out["data"])
			}
		}
		c.expect(400, "GET", "/ediplomas/search?round_id=zzz", school, nil)

		// Records created before rounds existed can be attached afterwards.
		uniID, _ := primitive.ObjectIDFromHex(uni)
		facultyOID, _ := primitive.ObjectIDFromHex(faculty)
		_, _ = db.Collection("ediplomas").InsertOne(ctx, models.EDiploma{ID: primitive.NewObjectID(), UniversityID: uniID, FacultyID: facultyOID, StudentCode: "AT180101", Name: "Bằng cũ"})
		if n := c.expect(200, "GET", "/ediplomas/search?round_id=none", school, nil)["total"].(float64); n != 1 {
			t.Fatalf("legacy diploma not listed as unassigned: %v", n)
		}
		assigned := c.expect(200, "POST", "/ediplomas/assign-round", school, gin.H{"round_id": first, "faculty_id": faculty})
		if assigned["updated"].(float64) != 1 {
			t.Fatalf("assign-round: %v", assigned)
		}

		// Batch actions need both a faculty and a round.
		c.expect(400, "POST", "/ediplomas/issue-pqc-batch", school, gin.H{"faculty_id": faculty, "template_id": primitive.NewObjectID().Hex()})
		c.expect(400, "POST", "/pqc/ediplomas/sign-batch", school, gin.H{"faculty_id": faculty})
		c.expect(400, "POST", "/blockchain/push-ediploma", school, gin.H{"faculty_id": faculty})
		c.expect(400, "POST", "/blockchain/push-revocations", school, gin.H{"faculty_id": faculty})
		// Issuing needs one faculty (its template); the other batch actions may cover the whole round.
		c.expect(400, "POST", "/ediplomas/issue-pqc-batch", school, gin.H{"round_id": first, "template_id": primitive.NewObjectID().Hex()})
		c.expect(404, "POST", "/ediplomas/issue-pqc-batch", school, gin.H{"faculty_id": faculty, "round_id": primitive.NewObjectID().Hex(), "template_id": primitive.NewObjectID().Hex()})

		// Issue & sign also picks up issued records that were never signed, and only signs them.
		pendingID := primitive.NewObjectID()
		firstOID, _ := primitive.ObjectIDFromHex(first)
		_, _ = db.Collection("ediplomas").InsertOne(ctx, models.EDiploma{ID: pendingID, UniversityID: uniID, FacultyID: facultyOID, RoundID: firstOID, StudentCode: "AT180199", Name: "Đã cấp chưa ký", Issued: true})
		// Dry runs list the targets of each stage without touching them.
		signTargets := c.expect(200, "POST", "/ediplomas/issue-pqc-batch", school, gin.H{"round_id": first, "stage": "sign", "dry_run": true})
		if signTargets["total"].(float64) != 1 || signTargets["data"].([]any)[0].(map[string]any)["id"] != pendingID.Hex() {
			t.Fatalf("sign stage should list only the issued, unsigned diploma: %v", signTargets)
		}
		issueTargets := c.expect(200, "POST", "/ediplomas/issue-pqc-batch", school, gin.H{"faculty_id": faculty, "round_id": first, "stage": "issue", "dry_run": true})
		for _, row := range issueTargets["data"].([]any) {
			if row.(map[string]any)["id"] == pendingID.Hex() || row.(map[string]any)["issued"] == true {
				t.Fatalf("issue stage listed an issued diploma: %v", issueTargets)
			}
		}
		if issueTargets["total"].(float64) == 0 {
			t.Fatalf("issue stage found nothing to issue: %v", issueTargets)
		}
		c.expect(400, "POST", "/ediplomas/issue-pqc-batch", school, gin.H{"round_id": first, "stage": "other", "dry_run": true})
		// Ids only narrow the scope: a diploma of the sign stage is not picked up by the issue stage.
		narrowed := c.expect(200, "POST", "/ediplomas/issue-pqc-batch", school, gin.H{"faculty_id": faculty, "round_id": first, "stage": "issue", "dry_run": true, "ids": []string{pendingID.Hex()}})
		if narrowed["total"].(float64) != 0 {
			t.Fatalf("ids widened the issue stage: %v", narrowed)
		}
		batch := c.expect(200, "POST", "/ediplomas/issue-pqc-batch", school, gin.H{"round_id": first, "stage": "sign", "ids": []string{pendingID.Hex()}})
		var pendingRow map[string]any
		for _, row := range batch["data"].([]any) {
			if r := row.(map[string]any); r["id"] == pendingID.Hex() {
				pendingRow = r
			}
		}
		if pendingRow == nil || pendingRow["issued"] != true || pendingRow["signed"] != false || pendingRow["error"] == nil {
			t.Fatalf("issued, unsigned diploma not handed to signing: %v", batch)
		}
		_, _ = db.Collection("ediplomas").DeleteOne(ctx, bson.M{"_id": pendingID})

		// Revoking a round: typed confirmation, only issued diplomas, only this university.
		revoke := gin.H{"faculty_id": faculty, "round_id": first, "reason": "Cấp sai danh sách tốt nghiệp", "confirm": "Tốt nghiệp đợt 1 (K2021)"}
		c.expect(400, "POST", "/ediplomas/revoke-batch", school, gin.H{"faculty_id": faculty, "round_id": first, "reason": "Cấp sai danh sách", "confirm": "đợt khác"})
		c.expect(404, "POST", "/ediplomas/revoke-batch", school, revoke)
		roundOID, _ := primitive.ObjectIDFromHex(first)
		_, _ = db.Collection("ediplomas").UpdateMany(ctx, bson.M{"round_id": roundOID}, bson.M{"$set": bson.M{"issued": true}})
		c.expect(404, "POST", "/ediplomas/revoke-batch", schoolB, revoke)
		out := c.expect(200, "POST", "/ediplomas/revoke-batch", school, revoke)
		if out["revoked"].(float64) != 2 {
			t.Fatalf("revoke-batch: %v", out)
		}
		// The student is told that the diploma was revoked, and why.
		if notice := mailer.last("at180101@ptit.edu.vn", t); !strings.Contains(notice.subject, "thu hồi") || !strings.Contains(notice.body, "Cấp sai danh sách tốt nghiệp") {
			t.Fatalf("revocation notice missing the reason: %+v", notice)
		}
		if n := c.expect(200, "GET", "/ediplomas/search?round_id="+first+"&revoked=true", school, nil)["total"].(float64); n != 2 {
			t.Fatalf("revoked diplomas: %v", n)
		}
		c.expect(404, "POST", "/ediplomas/revoke-batch", school, revoke)
		// A replacement stays in the round of the revoked original.
		replacement := c.expect(201, "POST", "/ediplomas/"+created["id"].(string)+"/replace", school, gin.H{
			"serial_number": "S-9", "registration_number": "SO-S-9", "issue_date": "2026-08-01T00:00:00Z"})["data"].(map[string]interface{})
		if replacement["round_id"] != first {
			t.Fatalf("replacement left the round: %v", replacement)
		}
		// "All faculties" covers the whole round; nothing is left to revoke here.
		c.expect(404, "POST", "/ediplomas/revoke-batch", school, gin.H{"faculty_id": "all", "round_id": first, "reason": "Cấp sai danh sách tốt nghiệp", "confirm": "Tốt nghiệp đợt 1 (K2021)"})

		// Writing revocations to the ledger needs a Fabric connection, for one faculty or the whole round.
		c.expect(503, "POST", "/blockchain/push-revocations", school, gin.H{"faculty_id": faculty, "round_id": first})
		c.expect(503, "POST", "/blockchain/push-revocations", school, gin.H{"round_id": first})
	})
}
