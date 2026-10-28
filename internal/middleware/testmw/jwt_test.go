package testmw

import (
	"mygin-api/internal/middleware"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestJWTAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := []byte("test-secret-that-is-at-least-32-bytes-long")
	issuer := "test-issuer"
	token, err := middleware.IssueToken(42, secret, issuer, time.Minute)
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	engine := gin.New()
	engine.GET("/protected", middleware.JWTAuth(secret, issuer), func(c *gin.Context) {
		userID, ok := middleware.AuthenticatedUserID(c)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	tests := []struct {
		name       string
		authHeader string
		status     int
	}{
		{name: "missing token", status: http.StatusUnauthorized},
		{name: "valid token", authHeader: "Bearer " + token, status: http.StatusOK},
		{name: "malformed scheme", authHeader: "Basic " + token, status: http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if test.authHeader != "" {
				request.Header.Set("Authorization", test.authHeader)
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}
