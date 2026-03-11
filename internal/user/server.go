package user

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"time"

	"banka-raf/gen/user"
	userpb "banka-raf/gen/user"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	userpb.UnimplementedUserServiceServer
	accessJwtSecret  string
	refreshJwtSecret string
	database         *sql.DB
}

func NewServer(accessJwtSecret string, refreshJwtSecret string, database *sql.DB) *Server {
	return &Server{
		accessJwtSecret:  accessJwtSecret,
		refreshJwtSecret: refreshJwtSecret,
		database:         database,
	}
}

func (s *Server) GetEmployeeById(ctx context.Context, req *userpb.GetEmployeeByIdRequest) (*user.EmployeeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (s *Server) GenerateRefreshToken(email string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   email,
		ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour * 7)),
		IssuedAt:  jwt.NewNumericDate(now),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.refreshJwtSecret))
}

func (s *Server) GenerateAccessToken(email string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   email,
		ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour * 7)),
		IssuedAt:  jwt.NewNumericDate(now),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.accessJwtSecret))
}

func (s *Server) Login(ctx context.Context, req *userpb.LoginRequest) (*userpb.LoginResponse, error) {
	hasher := sha256.New()
	hasher.Write([]byte(req.Password))
	hashedPassword := hasher.Sum(nil)
	user, err := s.GetUserByEmail(req.Email)
	if err != nil {
		return nil, err
	}

	if user != nil && bytes.Equal(hashedPassword, user.hashedPassword) {
		accessToken, err := s.GenerateAccessToken(user.email)
		if err != nil {
			return nil, err
		}
		refreshToken, err := s.GenerateRefreshToken(user.email)
		if err != nil {
			return nil, err
		}
		err = s.InsertRefreshToken(refreshToken)
		if err != nil {
			return nil, err
		}

		return &userpb.LoginResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		}, nil
	}

	return &userpb.LoginResponse{
		AccessToken:  "",
		RefreshToken: "",
	}, errors.New("wrong creds")
}
