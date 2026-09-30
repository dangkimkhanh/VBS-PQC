package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	RoleAdmin           = "admin"
	RoleUniversityAdmin = "university_admin"
	RoleStudent         = "student"
)

// AccountGuard is consulted on every authenticated request so that locking an
// account or changing its password invalidates tokens issued before that.
type AccountGuard func(ctx context.Context, accountID string, issuedAt time.Time) error

var accountGuard AccountGuard

func SetAccountGuard(guard AccountGuard) {
	accountGuard = guard
}

type rateWindow struct {
	start time.Time
	count int
}

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]*rateWindow
	sweptAt time.Time
}

func (l *rateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.sweptAt) > l.window {
		for k, e := range l.entries {
			if now.Sub(e.start) > l.window {
				delete(l.entries, k)
			}
		}
		l.sweptAt = now
	}
	e, ok := l.entries[key]
	if !ok || now.Sub(e.start) > l.window {
		l.entries[key] = &rateWindow{start: now, count: 1}
		return true
	}
	if e.count >= l.limit {
		return false
	}
	e.count++
	return true
}

// RateLimit allows `limit` requests per client IP within `window`. Each call
// creates an independent bucket, so different endpoints do not block each other.
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	limiter := &rateLimiter{limit: limit, window: window, entries: map[string]*rateWindow{}}
	return func(c *gin.Context) {
		if !limiter.allow(c.ClientIP(), time.Now()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Bạn thao tác quá nhanh, vui lòng thử lại sau ít phút"})
			return
		}
		c.Next()
	}
}

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Bạn chưa đăng nhập"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := utils.ParseToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
			return
		}
		if accountGuard != nil {
			var issuedAt time.Time
			if claims.IssuedAt != nil {
				issuedAt = claims.IssuedAt.Time
			}
			if err := accountGuard(c.Request.Context(), claims.AccountID, issuedAt); err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập đã hết hiệu lực, vui lòng đăng nhập lại"})
				return
			}
		}

		ctx := context.WithValue(c.Request.Context(), utils.ClaimsContextKey, claims)
		c.Request = c.Request.WithContext(ctx)
		c.Set("claims", claims)
		c.Next()
	}
}

func claimsFrom(c *gin.Context) (*utils.CustomClaims, bool) {
	raw, exists := c.Get("claims")
	if !exists {
		return nil, false
	}
	claims, ok := raw.(*utils.CustomClaims)
	return claims, ok && claims != nil
}

// RequireRoles must run after JWTAuthMiddleware.
func RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := claimsFrom(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
			return
		}
		for _, role := range roles {
			if claims.Role == role {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Bạn không có quyền thực hiện thao tác này"})
	}
}

func AdminOnlyMiddleware() gin.HandlerFunc {
	return RequireRoles(RoleAdmin)
}

// Ownership describes how a document identified by a URL parameter is tied to
// the caller. University admins match on university_id; students match on
// StudentField (when set) against their own user id.
type Ownership struct {
	DB         *mongo.Database
	Collection string
	Param      string
	// StudentField is the document field holding the student's user id. Empty
	// means students can never access the resource.
	StudentField string
	// Shared allows reading documents with a nil university_id (system samples).
	Shared bool
}

// Owned rejects requests for documents that do not belong to the caller. It
// answers 404 rather than 403 so other universities' ids cannot be probed.
func Owned(o Ownership) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := claimsFrom(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
			return
		}
		id, err := primitive.ObjectIDFromHex(c.Param(o.Param))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
			return
		}
		filter := bson.M{"_id": id}
		switch claims.Role {
		case RoleUniversityAdmin:
			universityID, err := primitive.ObjectIDFromHex(claims.UniversityID)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
				return
			}
			if o.Shared {
				filter["university_id"] = bson.M{"$in": bson.A{universityID, primitive.NilObjectID}}
			} else {
				filter["university_id"] = universityID
			}
		case RoleStudent:
			userID, err := primitive.ObjectIDFromHex(claims.UserID)
			if o.StudentField == "" || err != nil {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Bạn không có quyền truy cập dữ liệu này"})
				return
			}
			filter[o.StudentField] = userID
		default:
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Bạn không có quyền truy cập dữ liệu này"})
			return
		}
		count, err := o.DB.Collection(o.Collection).CountDocuments(c.Request.Context(), filter)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Không thể kiểm tra quyền truy cập"})
			return
		}
		if count == 0 {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy dữ liệu"})
			return
		}
		c.Next()
	}
}
