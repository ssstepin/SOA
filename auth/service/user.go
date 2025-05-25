package service

import (
	"auth/model"
	"crypto/md5"
	"crypto/rsa"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type repo interface {
	CreateUser(user *model.User) error
	GetUserByUsername(username string) (*model.User, error)
	GetUserById(id int) (*model.User, error)
}

type UserService struct {
	repo       repo
	jwtPrivate *rsa.PrivateKey
	jwtPublic  *rsa.PublicKey
}

type UserServiceError error

var (
	ErrValidation     UserServiceError = errors.New("validation error")
	ErrAuthentication UserServiceError = errors.New("authentication error")
)

func New(repo repo, privateKeyPath, publicKeyPath string) (*UserService, error) {
	publicKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, err
	}
	jwtPublic, err := jwt.ParseRSAPublicKeyFromPEM(publicKey)
	if err != nil {
		return nil, err
	}
	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}
	jwtPrivate, err := jwt.ParseRSAPrivateKeyFromPEM(privateKey)
	if err != nil {
		return nil, err
	}
	return &UserService{
		repo:       repo,
		jwtPublic:  jwtPublic,
		jwtPrivate: jwtPrivate,
	}, nil
}

func (s *UserService) RegisterUser(user *model.User) error {
	if !s.validateUser(user) {
		return ErrValidation
	}

	user.Password = s.hashPass(user.Password)

	return s.repo.CreateUser(user)
}

func (s *UserService) LoginUser(login string, password string) (*string, error) {
	user, err := s.repo.GetUserByUsername(login)
	if err != nil {
		return nil, err
	}
	hashedPasssword := s.hashPass(password)
	if hashedPasssword != user.Password {
		return nil, ErrAuthentication
	}

	token := s.generateToken(user)
	return &token, nil
}

func (s *UserService) GetUserById(id int) (*model.User, error) {
	user, err := s.repo.GetUserById(id)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) hashPass(pass string) string {
	hasher := md5.New()
	hasher.Write([]byte(pass))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (s *UserService) validateUser(user *model.User) bool {
	// TODO: https://trello.com/c/117zVU80
	return true
}

func (s *UserService) generateToken(user *model.User) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"id":  user.ID,
		"exp": time.Now().Add(3 * time.Hour).Unix(),
	})

	signedToken, _ := token.SignedString(s.jwtPrivate)
	log.Printf("signed token: %s", signedToken)
	return signedToken
}

func (s *UserService) GetPublicToken() interface{} {
	return s.jwtPublic
}
