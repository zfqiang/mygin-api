package middleware

import (
	"net/http"
	"strings"
	"time"

	"mygin-api/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const userIDContextKey = "authenticated_user_id"

type Claims struct {
	UserID uint64 `json:"uid"`
	jwt.RegisteredClaims
}

func IssueToken(userID uint64, secret []byte, issuer string, ttl time.Duration) (string, error) {
	if userID == 0 {
		return "", jwt.ErrInvalidKey
	}
	if len(secret) < 32 {
		return "", jwt.ErrInvalidKey
	}
	if ttl <= 0 {
		return "", jwt.ErrTokenInvalidClaims
	}
	now := time.Now()
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "user",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func JWTAuth(secret []byte, issuer string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.Fields(c.GetHeader("Authorization"))
		if len(header) != 2 || !strings.EqualFold(header[0], "Bearer") {
			unauthorized(c)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(header[1], claims, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrSignatureInvalid
			}
			return secret, nil
		},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer(issuer),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
		)
		if err != nil || token == nil || !token.Valid || claims.UserID == 0 {
			unauthorized(c)
			return
		}

		c.Set(userIDContextKey, claims.UserID)
		c.Next()
	}
}

func AuthenticatedUserID(c *gin.Context) (uint64, bool) {
	userID, ok := c.Get(userIDContextKey)
	if !ok {
		return 0, false
	}
	id, ok := userID.(uint64)
	return id, ok
}

func unauthorized(c *gin.Context) {
	utils.AbortFailure(c, http.StatusUnauthorized, "unauthorized")
}
