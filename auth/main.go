package main

import (
	"auth/model"
	"auth/repository"
	"auth/service"
	"auth/storage"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"log"
	"net/http"
)

const (
	BASE_PATH = "/api/v1"
)

const (
	RegisterPath = "register"
	LoginPath    = "login"
	WhoAmIPath   = "whoami"
)

const (
	PRIVATE_KEY_PATH = "./private_key.pem"
	PUBLIC_KEY_PATH  = "./public_key.pem"
)

type LoginInfo struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type homeHandler struct {
	service *service.UserService
}

func NewHandler(service *service.UserService) *homeHandler {
	return &homeHandler{
		service: service,
	}
}

func BadRequestHandler(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(fmt.Sprintf("400: %s", err.Error())))
}

func (h *homeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)
	//body, err := json.Marshal(&userModel{
	//	Username: "test_user",
	//	Password: "test_pass",
	//})
	//if err != nil {
	//	http.Error(w, err.Error(), http.StatusInternalServerError)
	//} else {
	//	w.Write(body)
	//}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("%s/%s", BASE_PATH, RegisterPath):
		user := &model.User{}
		if err := json.NewDecoder(r.Body).Decode(user); err != nil {
			BadRequestHandler(w, r, err)
			return
		}

		log.Println(user)

		err := h.service.RegisterUser(user)
		if err != nil {
			BadRequestHandler(w, r, err)
			return
		}

		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("%s/%s", BASE_PATH, LoginPath):
		loginInfo := &LoginInfo{}
		if err := json.NewDecoder(r.Body).Decode(loginInfo); err != nil {
			BadRequestHandler(w, r, err)
			return
		}

		token, err := h.service.LoginUser(loginInfo.Username, loginInfo.Password)
		if err != nil {
			BadRequestHandler(w, r, err)
			return
		}
		log.Printf("%s %s", r.Method, r.URL.Path)
		http.SetCookie(w, &http.Cookie{
			Name:  "jwt",
			Value: *token,
		})
		w.WriteHeader(http.StatusOK)
		return
	case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("%s/%s", BASE_PATH, WhoAmIPath):
		cookie, err := r.Cookie("jwt")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "Cookie is missing")
			return
		}

		token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, errors.New("invalid signing method")
			}

			return h.service.GetPublicToken(), nil
		})

		log.Println("checkpoint 1")

		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Invalid token")
			log.Fatalf("aboba %v", err)
			return
		}

		log.Println("checkpoint 2")

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Invalid token")
			log.Fatalf("aboba ya gnomik")
			return
		}

		log.Println("checkpoint 3")

		idFloat, ok := claims["id"].(float64)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Invalid token")
			log.Printf("aboba ya gnomik %v", claims)
			return
		}
		id := int(idFloat)

		log.Println("checkpoint 4")

		user, err := h.service.GetUserById(id)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Invalid token")
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "You are: %s", user.Username)
	default:
		return
	}
}

func main() {
	pg, err := storage.New("postgres://admin:superpass@postgres:5432/auth?sslmode=disable")
	if err != nil {
		log.Fatalf("Postgres start error: %v", err)
		return
	}
	userRepo := repository.NewUserRepository(*pg)
	mux := http.NewServeMux()
	userService, err := service.New(userRepo, PRIVATE_KEY_PATH, PUBLIC_KEY_PATH)
	if err != nil {
		log.Fatalf("UserService start error: %v", err)
		return
	}
	mux.Handle(BASE_PATH+"/", NewHandler(userService))
	log.Println("Starting server on :8081")
	http.ListenAndServe(":8081", mux)
}
