package helper

import (
	"fmt"
	"time"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTMaker struct {
	secretKey string
}

func NewJWTMaker(secretKey string) (*JWTMaker, error) {
	if len(secretKey) < 32 {
		return nil, errors.New("secret key must be at least 32 characters long")
	}
	return &JWTMaker{secretKey: secretKey}, nil
}

type UserClaims struct {
	jwt.RegisteredClaims
	ID uuid.UUID `json:"id"`
}

func (maker *JWTMaker) CreateToken(userID uuid.UUID, duration time.Duration) (string, error) {
	// Create JWT claims with user ID and expiration time
	if(userID == uuid.Nil) {
		return "", fmt.Errorf("invalid user ID: cannot be nil")
	}

	if duration <= 0 {
		return "", fmt.Errorf("invalid token duration: must be greater than zero")
	}

	now := time.Now()
	claims := UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID.String(),
			Issuer :  "chat-v2",
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

	// Assert that the token claims are of type UserClaims and return them
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
	issuer , err := token.Claims.GetIssuer()
	if err != nil {
		return nil, fmt.Errorf("failed to get issuer from token claims: %w", err)
	}
	if issuer != "chat-v2" {
		return nil, fmt.Errorf("invalid token issuer: expected 'chat-v2', got '%s': %w", issuer, jwt.ErrTokenInvalidClaims)
	}
	
	return claims, nil
}
