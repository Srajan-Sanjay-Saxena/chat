package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTMaker struct {
	secretKey string
}

type Claims struct {
	jwt.RegisteredClaims
}

func NewJWTMaker(secretKey string) (*JWTMaker, error) {
	if secretKey == "" {
		return nil, errors.New("JWT secret key cannot be empty")
	}
	return &JWTMaker{secretKey: secretKey}, nil
}

func (m *JWTMaker) CreateToken(userID uuid.UUID, duration time.Duration) (string, error) {
	if userID == uuid.Nil {
		return "", fmt.Errorf("invalid user ID")
	}
	if duration <= 0 {
		return "", fmt.Errorf("invalid token duration")
	}

	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    "relay",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.secretKey))
}

func (m *JWTMaker) VerifyToken(tokenStr string) (*Claims, error) {
	if tokenStr == "" {
		return nil, fmt.Errorf("token is empty")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(m.secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	if claims.Subject == "" {
		return nil, fmt.Errorf("invalid user ID in token")
	}

	return claims, nil
}

// UserID extracts the user ID from the claims Subject field.
func (c *Claims) UserID() (uuid.UUID, error) {
	return uuid.Parse(c.Subject)
}

func ExtractTokenFromCookie(r *http.Request) (string, error) {
	cookie, err := r.Cookie("access_token")
	if err != nil {
		return "", fmt.Errorf("access_token cookie not found: %w", err)
	}
	if cookie.Value == "" {
		return "", fmt.Errorf("access_token cookie is empty")
	}
	return cookie.Value, nil
}

// Context key for user ID
type contextKey string

const userContextKey = contextKey("userID")

func SetUserInContext(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userContextKey, userID)
}

func GetUserFromContext(ctx context.Context) (uuid.UUID, bool) {
	if ctx == nil {
		return uuid.Nil, false
	}
	userID, ok := ctx.Value(userContextKey).(uuid.UUID)
	return userID, ok
}
