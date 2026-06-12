package helper

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"time"
	passwordvalidator "github.com/wagslane/go-password-validator"
)

type JWTMaker struct {
	secretKey string
}

const (
	minSecretKeySize = 32
	maxSecretKeySize = 256
	minEntropyBits   = 60
)

func NewJWTMaker(secretKey string) (*JWTMaker, error) {

	if strings.TrimSpace(secretKey) == "" {
		return nil, errors.New("secret key cannot be empty")
	}

	if len(secretKey) < minSecretKeySize {
		return nil, errors.New("secret key must be at least 32 bytes long for security reasons")
	}

	if len(secretKey) > maxSecretKeySize {
		return nil, errors.New("secret key is too long: must be less than 256 bytes")
	}

	if strings.ContainsAny(secretKey, " \t\n\r") {
		return nil, errors.New("secret key cannot contain whitespace characters")
	}

	err := passwordvalidator.Validate(secretKey, minEntropyBits)
	if err != nil {
		return nil, fmt.Errorf("secret key does not meet complexity requirements: %w", err)
	}
	return &JWTMaker{secretKey: secretKey}, nil
}

type UserClaims struct {
	jwt.RegisteredClaims
	ID uuid.UUID `json:"id"`
}

func (maker *JWTMaker) CreateToken(userID uuid.UUID, duration time.Duration) (string, error) {
	// Create JWT claims with user ID and expiration time
	if userID == uuid.Nil {
		return "", fmt.Errorf("invalid user ID: cannot be nil")
	}

	if duration <= 0 {
		return "", fmt.Errorf("invalid token duration: must be greater than zero")
	}

	now := time.Now()
	claims := UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    "chat-v2",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
		},
		ID: userID,
	}

	// Create a new JWT token with the claims and sign it using the secret key
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(maker.secretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

func (maker *JWTMaker) VerifyToken(tokenStr string) (*UserClaims, error) {
	// Validate the token string
	if tokenStr == "" {
		return nil, fmt.Errorf("token string cannot be empty: %w", jwt.ErrTokenMalformed)
	}

	// Parse the JWT token and validate its signature and claims
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %w", jwt.ErrTokenSignatureInvalid)
		}

		return []byte(maker.secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Assert that the token claims are of type UserClaims
	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", jwt.ErrTokenInvalidClaims)
	}

	// Subject validation
	if claims.ID == uuid.Nil {
		return nil, fmt.Errorf("invalid token claims: user ID cannot be nil: %w", jwt.ErrTokenInvalidClaims)
	}

	subject, err := claims.GetSubject()
	if err != nil {
		return nil, fmt.Errorf("failed to get subject from token claims: %w", err)
	}
	if subject != claims.ID.String() {
		return nil, fmt.Errorf("token subject does not match user ID: expected '%s', got '%s': %w", claims.ID.String(), subject, jwt.ErrTokenInvalidClaims)
	}

	// Issuer validation
	issuer, err := token.Claims.GetIssuer()
	if err != nil {
		return nil, fmt.Errorf("failed to get issuer from token claims: %w", err)
	}
	if issuer != "chat-v2" {
		return nil, fmt.Errorf("invalid token issuer: expected 'chat-v2', got '%s': %w", issuer, jwt.ErrTokenInvalidClaims)
	}

	// Expiration validation is handled by jwt.ParseWithClaims,
	// so we don't need to check it manually here
	return claims, nil
}

func ExtractBearerToken(r *http.Request) (string, error) {
	// Extract the Authorization header from the HTTP request
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("authorization header is missing")
	}

	// Check if the Authorization header is in the correct format (Bearer token)
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", fmt.Errorf("authorization header format must be 'Bearer {token}'")
	}

	// check if token is empty
	if strings.TrimSpace(parts[1]) == "" {
		return "", fmt.Errorf("token cannot be empty")
	}

	return parts[1], nil
}

func JWTVerifier(r *http.Request, maker *JWTMaker) (uuid.UUID, error) {
	// Verify the JWT token from the HTTP request and extract the user ID
	if r == nil {
		return uuid.Nil, fmt.Errorf("request cannot be nil")
	}

	if maker == nil {
		return uuid.Nil, fmt.Errorf("jwt maker cannot be nil")
	}

	token, err := ExtractBearerToken(r)
	if err != nil {
		return uuid.Nil, err
	}

	claims, err := maker.VerifyToken(token)
	if err != nil {
		return uuid.Nil, err
	}

	return claims.ID, nil
}

type contextKey string

const userContextKey = contextKey("userID")

func SetUserContext(ctx context.Context, userID uuid.UUID) context.Context {
	// set user ID in context for downstream handlers
	return context.WithValue(ctx, userContextKey, userID)
}

func GetUserFromContext(ctx context.Context) (uuid.UUID, bool) {
	// retrieve user ID from context
	if ctx == nil {
		return uuid.Nil, false
	}

	userID, ok := ctx.Value(userContextKey).(uuid.UUID)
	if ok {
		return userID, true
	}

	return userID, ok
}
