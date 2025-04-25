package main

import (
	"crypto/md5"
	"crypto/rsa"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	_ "github.com/lib/pq"
)

type SignupInfo struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (info *SignupInfo) hash() string {
	data := []byte(info.Id + "#" + info.Password)
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

type LoginInfo struct {
	Id       string `json:"id"`
	Password string `json:"password"`
}

func (info *LoginInfo) hash() string {
	data := []byte(info.Id + "#" + info.Password)
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

type UpdateInfo struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Description string `json:"description"`
}

type AuthHandler struct {
	db         *sql.DB
	jwtPrivate *rsa.PrivateKey
	jwtPublic  *rsa.PublicKey
}

func (h *AuthHandler) genToken(id string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"id":  id,
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	signedToken, _ := token.SignedString(h.jwtPrivate)
	return signedToken
}

func (h *AuthHandler) checkUserExists(id string) (bool, error) {
	var exists bool
	err := h.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM "users" WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

func (h *AuthHandler) createUser(creds SignupInfo) error {
	_, err := h.db.Exec(`
		INSERT INTO "users" (id, name, premium, description, rank, email) 
		VALUES ($1, $2, false, '', 0, $3)
	`, creds.Id, creds.Name, creds.Email)
	return err
}

func (h *AuthHandler) createPassword(userId, passwordHash string) error {
	_, err := h.db.Exec(`
		INSERT INTO "passwords" (user_id, password) 
		VALUES ($1, $2)
	`, userId, passwordHash)
	return err
}

func (h *AuthHandler) signup(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "signup can be done only with POST HTTP method")
		return
	}

	body := make([]byte, req.ContentLength)
	read, err := req.Body.Read(body)
	defer req.Body.Close()

	if read != int(req.ContentLength) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err != io.EOF {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error reading body: %v", err)
		return
	}

	creds := SignupInfo{}
	err = json.Unmarshal(body, &creds)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error unmarshalling body: %v", err)
		return
	}

	exists, err := h.checkUserExists(creds.Id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error connecting with db: %v", err)
		return
	}

	if exists {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "User already exists")
		return
	}

	if err := h.createUser(creds); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error connecting with db: %v", err)
		return
	}

	if err := h.createPassword(creds.Id, creds.hash()); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error connecting with db: %v", err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:  "jwt",
		Value: h.genToken(creds.Id),
	})
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Signup successful")
}

func (h *AuthHandler) getPasswordHash(userId string) (string, error) {
	var hash string
	err := h.db.QueryRow(`SELECT password FROM "passwords" WHERE user_id = $1`, userId).Scan(&hash)
	return hash, err
}

func (h *AuthHandler) login(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "login can be done only with POST HTTP method")
		return
	}

	body := make([]byte, req.ContentLength)
	read, err := req.Body.Read(body)
	defer req.Body.Close()

	if read != int(req.ContentLength) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err != io.EOF {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error reading body: %v", err)
		return
	}

	creds := LoginInfo{}
	err = json.Unmarshal(body, &creds)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error unmarshalling body: %v", err)
		return
	}

	hash, err := h.getPasswordHash(creds.Id)
	if err == sql.ErrNoRows {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "Invalid username or password")
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error connecting with db: %v", err)
		return
	}

	if hash != creds.hash() {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "Invalid username or password")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:  "jwt",
		Value: h.genToken(creds.Id),
	})
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Login successful")
}

func (h *AuthHandler) getUserName(id string) (string, error) {
	var name string
	err := h.db.QueryRow(`SELECT name FROM "users" WHERE id = $1`, id).Scan(&name)
	return name, err
}

func (h *AuthHandler) whoami(w http.ResponseWriter, req *http.Request) {
	cookie, err := req.Cookie("jwt")
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "Cookie is missing")
		return
	}

	token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("invalid signing method")
		}

		return h.jwtPublic, nil
	})

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid token")
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid token")
		return
	}

	id, ok := claims["id"].(string)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid token")
		return
	}

	name, err := h.getUserName(id)
	if err == sql.ErrNoRows {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid token")
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error connecting with db: %v", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Hello, %s\n", name)
}

func (h *AuthHandler) updateUser(id string, creds UpdateInfo) error {
	_, err := h.db.Exec(`
		UPDATE "users" 
		SET name = $1, email = $2, description = $3
		WHERE id = $4
	`, creds.Name, creds.Email, creds.Description, id)
	return err
}

func (h *AuthHandler) update(w http.ResponseWriter, req *http.Request) {
	cookie, err := req.Cookie("jwt")
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "Cookie is missing")
		return
	}

	token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("invalid signing method")
		}

		return h.jwtPublic, nil
	})

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid token")
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid token")
		return
	}

	id, ok := claims["id"].(string)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid token")
		return
	}

	body := make([]byte, req.ContentLength)
	read, err := req.Body.Read(body)
	defer req.Body.Close()

	if read != int(req.ContentLength) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err != io.EOF {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error reading body: %v", err)
		return
	}

	creds := UpdateInfo{}
	err = json.Unmarshal(body, &creds)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error unmarshalling body: %v", err)
		return
	}

	exists, err := h.checkUserExists(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error connecting with db: %v", err)
		return
	}

	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid token")
		return
	}

	if err := h.updateUser(id, creds); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid token")
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Upd successful")
}

func connectToDB() (*sql.DB, error) {
	fmt.Println("Connecting to database...")
	connStr := "host=users_db port=5432 user=auth password=password dbname=usersdb sslmode=disable"
	return sql.Open("postgres", connStr)
}

func initDB(db *sql.DB) {
	fmt.Println("Init tables...")

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS "users" (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			premium BOOLEAN DEFAULT FALSE,
			description TEXT,
			rank INTEGER DEFAULT 0,
			email TEXT NOT NULL
		);
	`)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to init table:", err)
		os.Exit(1)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS "passwords" (
			user_id TEXT PRIMARY KEY REFERENCES "users"(id) ON DELETE CASCADE,
			password TEXT
		);
	`)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to init table:", err)
		os.Exit(1)
	}
}

func main() {
	db, err := connectToDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to connect to the database:", err)
		os.Exit(1)
	}

	defer db.Close()

	initDB(db)

	privatePath := "signature.pem"
	private, err := os.ReadFile(privatePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	publicPath := "signature.pub"
	public, err := os.ReadFile(publicPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	jwtPrivate, err := jwt.ParseRSAPrivateKeyFromPEM(private)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	jwtPublic, err := jwt.ParseRSAPublicKeyFromPEM(public)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	handler := AuthHandler{db, jwtPrivate, jwtPublic}
	http.HandleFunc("/signup", handler.signup)
	http.HandleFunc("/login", handler.login)
	http.HandleFunc("/whoami", handler.whoami)
	http.HandleFunc("/update", handler.update)

	port := 8091
	fmt.Printf("Starting server at port %d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
