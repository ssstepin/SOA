package main

import (
	"context"
	"crypto/rsa"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Database layer
type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(connStr string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Init() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS "posts" (
			id          TEXT PRIMARY KEY,
			created     TIMESTAMP NOT NULL,
			author      TEXT NOT NULL,
			title       TEXT,
			text        TEXT
		);`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS "comments" (
			id          TEXT PRIMARY KEY,
			post_id		TEXT,
			created     TIMESTAMP NOT NULL,
			author      TEXT NOT NULL,
			text        TEXT
		);`)
	return err
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// type KafkaEventProducer struct {
// 	producer sarama.SyncProducer
// 	topic    string
// }

// func NewKafkaEventProducer(brokers []string, topic string) (*KafkaEventProducer, error) {
// 	config := sarama.NewConfig()
// 	config.Producer.RequiredAcks = sarama.WaitForAll
// 	config.Producer.Retry.Max = 5
// 	config.Producer.Return.Successes = true

// 	producer, err := sarama.NewSyncProducer(brokers, config)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &KafkaEventProducer{
// 		producer: producer,
// 		topic:    topic,
// 	}, nil
// }

// func (k *KafkaEventProducer) SendCommentCreatedEvent(commentID, postID, author, text string) error {
// 	event := map[string]interface{}{
// 		"event_type": "comment_created",
// 		"comment_id": commentID,
// 		"post_id":    postID,
// 		"author":     author,
// 		"text":       text,
// 		"created_at": time.Now().UTC(),
// 	}

// 	jsonEvent, err := json.Marshal(event)
// 	if err != nil {
// 		return err
// 	}

// 	msg := &sarama.ProducerMessage{
// 		Topic: k.topic,
// 		Value: sarama.ByteEncoder(jsonEvent),
// 	}

// 	_, _, err = k.producer.SendMessage(msg)
// 	return err
// }

// func (k *KafkaEventProducer) Close() error {
// 	return k.producer.Close()
// }

// // Auth service
type AuthService struct {
	jwtPublic *rsa.PublicKey
}

func NewAuthService(publicKeyPath string) (*AuthService, error) {
	public, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, err
	}

	jwtPublic, err := jwt.ParseRSAPublicKeyFromPEM(public)
	if err != nil {
		return nil, err
	}

	return &AuthService{jwtPublic: jwtPublic}, nil
}

func (a *AuthService) Authenticate(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("no metadata in context")
	}

	token, err := jwt.Parse(md.Get("token")[0], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("invalid signing method")
		}
		return a.jwtPublic, nil
	})

	if err != nil {
		return "", fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	userId, ok := claims["id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid token")
	}

	return userId, nil
}

// Posts service
type PostsService struct {
	UnimplementedPostsServer
	store *PostgresStore
	auth  *AuthService
	//producer *KafkaEventProducer
}

func NewPostsService(store *PostgresStore, auth *AuthService) *PostsService {
	return &PostsService{
		store: store,
		auth:  auth,
		// producer: producer,
	}
}

func (s *PostsService) CreatePost(ctx context.Context, req *CreatePostRequest) (*emptypb.Empty, error) {
	userId, err := s.auth.Authenticate(ctx)
	if err != nil {
		return nil, err
	}

	postId := uuid.New().String()

	_, err = s.store.db.Exec(`
		INSERT INTO "posts" (id, created, author, title, text) 
		VALUES ($1, $2, $3, $4, $5);
	`, postId, time.Now().UTC(), userId, req.GetTitle(), req.GetText())

	if err != nil {
		return nil, fmt.Errorf("error connecting with db: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *PostsService) CreateComment(ctx context.Context, req *CreateCommentRequest) (*emptypb.Empty, error) {
	userId, err := s.auth.Authenticate(ctx)
	if err != nil {
		return nil, err
	}

	commId := uuid.New().String()

	_, err = s.store.db.Exec(`
		INSERT INTO "comments" (id, post_id, created, author, text) 
		VALUES ($1, $2, $3, $4, $5);
	`, commId, req.GetPostId(), time.Now().UTC(), userId, req.GetText())

	if err != nil {
		return nil, fmt.Errorf("error connecting with db: %v", err)
	}

	// // Send event to Kafka
	// if err := s.producer.SendCommentCreatedEvent(commId, req.GetPostId(), userId, req.GetText()); err != nil {
	// 	log.Printf("Failed to send comment created event: %v", err)
	// }

	return &emptypb.Empty{}, nil
}

// ... (остальные методы остаются аналогичными, но используют s.store.db вместо s.db)

func main() {
	// Initialize database
	store, err := NewPostgresStore("host=posts_db port=5433 user=posts password=password dbname=postsdb sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer store.Close()

	if err := store.Init(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize auth service
	auth, err := NewAuthService("signature.pub")
	if err != nil {
		log.Fatalf("Failed to initialize auth service: %v", err)
	}

	// // Initialize Kafka producer
	// producer, err := NewKafkaEventProducer([]string{"kafka:9092"}, "comments")
	// if err != nil {
	// 	log.Fatalf("Failed to initialize Kafka producer: %v", err)
	// }
	// defer producer.Close()

	// Create gRPC server
	port := 8093
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	postsService := NewPostsService(store, auth)
	RegisterPostsServer(grpcServer, postsService)

	fmt.Printf("Server is running on port :%d\n", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
