package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrMissingAuth   = errors.New("missing authorization")
	ErrInvalidAuth   = errors.New("invalid authorization")
	ErrInvalidJobJWT = errors.New("invalid job token")
)

type Mode string

const (
	ModeService Mode = "service"
	ModeJob     Mode = "job"
)

type JobClaims struct {
	JobID    string `json:"jobId"`
	Kind     string `json:"kind"`
	EntityID string `json:"entityId"`
	MovieID  string `json:"movieId,omitempty"`
	jwt.RegisteredClaims
}

type Result struct {
	Mode   Mode
	Claims *JobClaims
}

type Verifier struct {
	ProcessorToken string
	JobJWTSecret   []byte
	Issuer         string
	Audience       string
}

func NewVerifier(processorToken, jobJWTSecret, issuer, audience string) *Verifier {
	var secret []byte
	if jobJWTSecret != "" {
		secret = []byte(jobJWTSecret)
	}
	return &Verifier{
		ProcessorToken: processorToken,
		JobJWTSecret:   secret,
		Issuer:         issuer,
		Audience:       audience,
	}
}

func BearerToken(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", ErrMissingAuth
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", ErrInvalidAuth
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", ErrInvalidAuth
	}
	return token, nil
}

func (v *Verifier) Authenticate(authHeader string) (Result, error) {
	token, err := BearerToken(authHeader)
	if err != nil {
		return Result{}, err
	}

	if v.ProcessorToken != "" && subtleConstEq(token, v.ProcessorToken) {
		return Result{Mode: ModeService}, nil
	}

	if len(v.JobJWTSecret) == 0 {
		return Result{}, ErrInvalidAuth
	}

	claims, err := v.parseJobJWT(token)
	if err != nil {
		return Result{}, err
	}
	return Result{Mode: ModeJob, Claims: claims}, nil
}

func (v *Verifier) parseJobJWT(tokenString string) (*JobClaims, error) {
	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithLeeway(30 * time.Second),
	}
	if v.Issuer != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(v.Issuer))
	}
	if v.Audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(v.Audience))
	}

	parsed, err := jwt.ParseWithClaims(tokenString, &JobClaims{}, func(t *jwt.Token) (any, error) {
		return v.JobJWTSecret, nil
	}, parserOpts...)
	if err != nil || !parsed.Valid {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJobJWT, err)
	}
	claims, ok := parsed.Claims.(*JobClaims)
	if !ok {
		return nil, ErrInvalidJobJWT
	}
	if claims.Kind == "" || claims.EntityID == "" {
		return nil, fmt.Errorf("%w: kind and entityId required", ErrInvalidJobJWT)
	}
	return claims, nil
}

func subtleConstEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
