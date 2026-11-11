package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInactiveAccount    = errors.New("account is inactive")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
)

type User struct {
	ID           string      `json:"id"`
	FullName     string      `json:"full_name"`
	Email        string      `json:"email"`
	DepartmentID *string     `json:"-"`
	Department   *Department `json:"department,omitempty"`
	PasswordHash string      `json:"-"`
	IsActive     bool        `json:"-"`
}

type Department struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Principal struct {
	User        User     `json:"-"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	RefreshUntil time.Time
}

type Store interface {
	UserByEmail(context.Context, string) (User, error)
	Principal(context.Context, string) (Principal, error)
	CreateRefresh(context.Context, string, string, time.Time) error
	RotateRefresh(context.Context, string, string, time.Time) (string, error)
	RevokeRefresh(context.Context, string, string) error
}

type Service struct {
	store      Store
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

type accessClaims struct {
	jwt.RegisteredClaims
}

func NewService(store Store, secret string, accessTTL, refreshTTL time.Duration) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("authentication store is required")
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT signing secret must be at least 32 bytes")
	}
	if accessTTL <= 0 || refreshTTL <= accessTTL {
		return nil, fmt.Errorf("refresh token lifetime must exceed a positive access token lifetime")
	}
	return &Service{
		store:      store,
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        time.Now,
	}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (Principal, TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.store.UserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return Principal{}, TokenPair{}, ErrInvalidCredentials
		}
		return Principal{}, TokenPair{}, err
	}
	if !user.IsActive {
		return Principal{}, TokenPair{}, ErrInactiveAccount
	}
	if VerifyPassword(user.PasswordHash, password) != nil {
		return Principal{}, TokenPair{}, ErrInvalidCredentials
	}

	principal, err := s.store.Principal(ctx, user.ID)
	if err != nil {
		return Principal{}, TokenPair{}, err
	}
	pair, err := s.issuePair(ctx, user.ID)
	if err != nil {
		return Principal{}, TokenPair{}, err
	}
	return principal, pair, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	if refreshToken == "" {
		return TokenPair{}, ErrUnauthorized
	}
	nextToken, err := newRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	now := s.now()
	userID, err := s.store.RotateRefresh(ctx, hashRefreshToken(refreshToken), hashRefreshToken(nextToken), now.Add(s.refreshTTL))
	if err != nil {
		return TokenPair{}, err
	}
	accessToken, err := s.accessToken(userID, now)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: nextToken,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
		RefreshUntil: now.Add(s.refreshTTL),
	}, nil
}

func (s *Service) Logout(ctx context.Context, userID, refreshToken string) error {
	if refreshToken == "" {
		return ErrUnauthorized
	}
	return s.store.RevokeRefresh(ctx, userID, hashRefreshToken(refreshToken))
}

func (s *Service) Authenticate(ctx context.Context, rawToken string) (Principal, error) {
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrUnauthorized
		}
		return s.secret, nil
	}, jwt.WithIssuer("projectflow"), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithTimeFunc(s.now))
	if err != nil || !token.Valid || claims.Subject == "" {
		return Principal{}, ErrUnauthorized
	}
	principal, err := s.store.Principal(ctx, claims.Subject)
	if err != nil {
		return Principal{}, err
	}
	return principal, nil
}

func (s *Service) issuePair(ctx context.Context, userID string) (TokenPair, error) {
	now := s.now()
	refreshToken, err := newRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	if err := s.store.CreateRefresh(ctx, userID, hashRefreshToken(refreshToken), now.Add(s.refreshTTL)); err != nil {
		return TokenPair{}, err
	}
	accessToken, err := s.accessToken(userID, now)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
		RefreshUntil: now.Add(s.refreshTTL),
	}, nil
}

func (s *Service) accessToken(userID string, now time.Time) (string, error) {
	claims := accessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "projectflow",
			Subject:   userID,
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

func newRefreshToken() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (p Principal) HasPermission(permission string) bool {
	for _, current := range p.Permissions {
		if current == permission {
			return true
		}
	}
	return false
}

func (p Principal) HasRole(role string) bool {
	for _, current := range p.Roles {
		if current == role {
			return true
		}
	}
	return false
}

func ValidUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}
