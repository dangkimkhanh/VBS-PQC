package integration

// Temporary harness for the platform-admin screens; deleted after the run.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"regexp"
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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mailer struct{ last map[string]string }

func (m *mailer) SendEmail(to, _, body string) error { m.last[to] = body; return nil }

func TestAdminScreens(t *testing.T) {
	uri := os.Getenv("TEST_MONGODB_URI")
	if uri == "" {
		t.Skip("TEST_MONGODB_URI not set")
	}
	os.Setenv("JWT_SECRET", "smoke")
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
		t.Skip(err)
	}
	db := mc.Database(fmt.Sprintf("vbs_pqc_admin_smoke_%d", time.Now().UnixNano()))
	defer func() { _ = db.Drop(ctx); _ = mc.Disconnect(ctx) }()

	mail := &mailer{last: map[string]string{}}
	userRepo := repository.NewUserRepository(db)
	authRepo := repository.NewAuthRepository(db)
	universityRepo := repository.NewUniversityRepository(db)
	facultyRepo := repository.NewFacultyRepository(db)
	ediplomaRepo := repository.NewEDiplomaRepository(db, facultyRepo)
	templateRepo := repository.NewTemplateRepository(db)
	majorRepo := repository.NewMajorRepository(db)
	pqcKeyRepo := repository.NewPQCKeyRepository(db)
	sampleRepo := repository.NewTemplateSampleRepo(db)
	authService := service.NewAuthService(authRepo, userRepo, mail)
	middleware.SetAccountGuard(authService.ValidateSession)
	uniService := service.NewUniversityService(universityRepo, authRepo, repository.NewAdminRepository(db), mail)
	facultyService := service.NewFacultyService(universityRepo, facultyRepo)
	userService := service.NewUserService(userRepo, universityRepo, facultyRepo, authRepo, ediplomaRepo)
	edService := service.NewEDiplomaService(*sampleRepo, universityRepo, majorRepo, facultyRepo, ediplomaRepo, templateRepo, userRepo, nil, models.NewTemplateEngine(), utils.NewPDFGenerator(), repository.NewPQCKeyRepository(db), repository.NewIssuanceRoundRepository(db), nil)
	bc := service.NewBlockchainService(templateRepo, ediplomaRepo, userRepo, authRepo, facultyRepo, universityRepo, nil, nil, service.NewPQCTransactionSigner(pqcKeyRepo), mail)
	pqc := service.NewPQCService(pqcKeyRepo, ediplomaRepo, edService, bc)
	sampleService := service.NewTemplateSampleService(sampleRepo, templateRepo)
	r := routes.SetupRouter(db,
		handlers.NewUserHandler(userService),
		handlers.NewAuthHandler(authService, uniService, userService, facultyService),
		handlers.NewUniversityHandler(uniService),
		handlers.NewFacultyHandler(facultyService),
		handlers.NewBlockchainHandler(bc, edService),
		handlers.NewMajorHandler(service.NewMajorService(majorRepo, facultyRepo)),
		handlers.NewTemplateHandler(service.NewTemplateService(templateRepo, facultyRepo, universityRepo, facultyService, *sampleService, nil), nil, facultyService),
		handlers.NewEDiplomaHandler(edService),
		handlers.NewTemplateSampleHandler(sampleService),
		handlers.NewPQCHandler(pqc),
		&handlers.DegreePortal{DB: db, PQC: pqc, Degrees: edService},
	)
	n := 0
	call := func(method, path, token string, body interface{}) (int, map[string]interface{}) {
		n++
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, "/api/v1"+path, &buf)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = fmt.Sprintf("10.9.%d.%d:1", n/250, n%250)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		out := map[string]interface{}{}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	expect := func(want int, method, path, token string, body interface{}) map[string]interface{} {
		t.Helper()
		code, out := call(method, path, token, body)
		if code != want {
			t.Fatalf("%s %s: want %d got %d %v", method, path, want, code, out)
		}
		return out
	}

	hash, _ := utils.HashPassword("SuperAdmin#2026")
	_ = authRepo.CreateAccount(ctx, &models.Account{ID: primitive.NewObjectID(), PersonalEmail: "root@vbs.test", PasswordHash: hash, Role: "admin", Status: "active", CreatedAt: time.Now()})
	admin := expect(200, "POST", "/auth/login", "", gin.H{"email": "root@vbs.test", "password": "SuperAdmin#2026"})["token"].(string)

	ids := map[string]string{}
	for i := 0; i < 25; i++ {
		code := fmt.Sprintf("U%02d", i)
		out := expect(201, "POST", "/universities", admin, gin.H{
			"university_name": "Trường Đại học số " + code, "university_code": code, "address": "HN",
			"email_domain": fmt.Sprintf("u%02d.edu.vn", i), "admin_email": fmt.Sprintf("admin%02d@u.test", i),
			"admin_name": "QTV", "admin_phone": "0912345678",
		})
		ids[code] = out["id"].(string)
	}
	// Activate U00..U09, lock U20.
	tokenRe := regexp.MustCompile(`token=([A-Z0-9]+)`)
	for i := 0; i < 10; i++ {
		email := fmt.Sprintf("admin%02d@u.test", i)
		tok := tokenRe.FindStringSubmatch(mail.last[email])[1]
		expect(200, "POST", "/auth/activate-account", "", gin.H{"token": tok, "password": "School#12345"})
	}
	expect(200, "POST", "/universities/"+ids["U20"]+"/lock", admin, nil)

	// A school with a student and a diploma, for statistics.
	school := expect(200, "POST", "/auth/login", "", gin.H{"email": "admin00@u.test", "password": "School#12345"})["token"].(string)
	expect(201, "POST", "/faculties", school, gin.H{"faculty_code": "CNTT", "faculty_name": "CNTT"})
	expect(201, "POST", "/users", school, gin.H{"student_code": "S1", "full_name": "A", "email": "s1@u00.edu.vn", "faculty_code": "CNTT", "course": "2021", "citizen_id_number": "001203000001", "date_of_birth": "01/01/2003"})
	u00, _ := primitive.ObjectIDFromHex(ids["U00"])
	_, _ = db.Collection("ediplomas").InsertOne(ctx, models.EDiploma{ID: primitive.NewObjectID(), UniversityID: u00, Issued: true, Signed: true})

	t.Run("pagination", func(t *testing.T) {
		p1 := expect(200, "GET", "/universities?page=1&page_size=10", admin, nil)
		p3 := expect(200, "GET", "/universities?page=3&page_size=10", admin, nil)
		if p1["total"].(float64) != 25 || len(p1["data"].([]interface{})) != 10 || len(p3["data"].([]interface{})) != 5 || p1["total_page"].(float64) != 3 {
			t.Fatalf("bad pagination: total=%v p1=%d p3=%d", p1["total"], len(p1["data"].([]interface{})), len(p3["data"].([]interface{})))
		}
	})
	t.Run("search", func(t *testing.T) {
		for q, want := range map[string]float64{"U07": 1, "u12.edu": 1, "ADMIN03@": 1, "số U1": 10, "nothing": 0, "(": 0} {
			out := expect(200, "GET", "/universities?q="+urlq(q), admin, nil)
			if out["total"].(float64) != want {
				t.Fatalf("q=%q total=%v want %v", q, out["total"], want)
			}
		}
	})
	t.Run("status filter", func(t *testing.T) {
		for status, want := range map[string]float64{"active": 10, "pending": 14, "locked": 1} {
			out := expect(200, "GET", "/universities?status="+status, admin, nil)
			if out["total"].(float64) != want {
				t.Fatalf("status=%s total=%v want %v", status, out["total"], want)
			}
		}
		expect(400, "GET", "/universities?status=bogus", admin, nil)
	})
	t.Run("row stats", func(t *testing.T) {
		out := expect(200, "GET", "/universities?q=U00", admin, nil)
		stats := out["data"].([]interface{})[0].(map[string]interface{})["stats"].(map[string]interface{})
		if stats["students"].(float64) != 1 || stats["issued"].(float64) != 1 || stats["signed"].(float64) != 1 {
			t.Fatalf("bad stats %v", stats)
		}
	})
	t.Run("overview", func(t *testing.T) {
		o := expect(200, "GET", "/admin/overview", admin, nil)["data"].(map[string]interface{})
		u := o["universities"].(map[string]interface{})
		if u["total"].(float64) != 25 || u["active"].(float64) != 10 || u["locked"].(float64) != 1 || u["admin_pending"].(float64) != 14 {
			t.Fatalf("bad university counts %v", u)
		}
		if o["students"].(map[string]interface{})["total"].(float64) != 1 || o["diplomas"].(map[string]interface{})["issued"].(float64) != 1 {
			t.Fatalf("bad counts %v", o)
		}
		if top := o["top_universities"].([]interface{}); len(top) != 1 || top[0].(map[string]interface{})["university_code"] != "U00" {
			t.Fatalf("bad top %v", top)
		}
		if pend := o["pending_activations"].([]interface{}); len(pend) != 10 {
			t.Fatalf("pending list len %d", len(pend))
		}
	})
	t.Run("admin audit", func(t *testing.T) {
		time.Sleep(200 * time.Millisecond)
		events := expect(200, "GET", "/admin/audit", admin, nil)["data"].([]interface{})
		if len(events) != 26 {
			t.Fatalf("want 26 admin events (25 create + 1 lock), got %d", len(events))
		}
		first := events[0].(map[string]interface{})
		if first["university_code"] != "U20" || first["action"] != "/api/v1/universities/:id/lock" {
			t.Fatalf("latest event %v", first)
		}
		// Schools do not see platform-admin actions in their own history.
		for _, e := range expect(200, "GET", "/degree-audit", school, nil)["data"].([]interface{}) {
			if e.(map[string]interface{})["role"] == "admin" {
				t.Fatal("school sees admin events")
			}
		}
	})
	t.Run("only admin", func(t *testing.T) {
		expect(403, "GET", "/admin/overview", school, nil)
		expect(403, "GET", "/admin/audit", school, nil)
	})
}

func urlq(s string) string {
	b := &bytes.Buffer{}
	for _, r := range []byte(s) {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' {
			b.WriteByte(r)
		} else {
			fmt.Fprintf(b, "%%%02X", r)
		}
	}
	return b.String()
}
