package main

import (
	"context"
	"fmt"
	"github.com/twmb/franz-go/pkg/kmsg"
	"log"
	"net"
	"os"
	"posts/repository"
	"posts/storage"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	pb "posts/api/posts"
)

type postsServer struct {
	pb.UnimplementedPostsServiceServer
	postsRepo     *repository.PostsRepository
	kafkaClient   *kgo.Client
	likesTopic    string
	viewsTopic    string
	commentsTopic string
}

func NewKafkaClient(brokers []string) (*kgo.Client, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(brokers...),
		kgo.WithLogger(kgo.BasicLogger(os.Stderr, kgo.LogLevelInfo, nil)),
		kgo.RecordDeliveryTimeout(5 * time.Second),
		kgo.DisableIdempotentWrite(), // Для простоты
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	// Проверка подключения
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}

func (s *postsServer) produceEvent(ctx context.Context, topic string, key, value []byte) error {
	record := &kgo.Record{
		Topic: topic,
		Key:   key,
		Value: value,
	}

	log.Printf("Producing to topic '%s': key='%s', value='%s'",
		topic, string(key), string(value))

	result := s.kafkaClient.ProduceSync(ctx, record)
	if err := result.FirstErr(); err != nil {
		log.Printf("Delivery failed: topic=%s error=%v", topic, err)
		return err
	}

	delivered := result[0].Record
	log.Printf("Successfully delivered to %s [partition %d @ offset %d]",
		delivered.Topic, delivered.Partition, delivered.Offset)

	return nil
}

func (s *postsServer) LikePost(ctx context.Context, req *pb.PostActionRequest) (*pb.PostActionResponse, error) {
	log.Println("LIKE", req)
	key := []byte(fmt.Sprintf("%d:%d", req.UserId, req.PostId))
	err := s.produceEvent(ctx, s.likesTopic, key, nil)
	if err != nil {
		return &pb.PostActionResponse{Success: false}, grpc.Errorf(codes.Internal, "failed to record like")
	}
	return &pb.PostActionResponse{Success: true}, nil
}

func (s *postsServer) ViewPost(ctx context.Context, req *pb.PostActionRequest) (*pb.PostActionResponse, error) {
	log.Println("VIEW", req)
	key := []byte(fmt.Sprintf("%d:%d", req.UserId, req.PostId))
	err := s.produceEvent(ctx, s.viewsTopic, key, nil)
	if err != nil {
		return &pb.PostActionResponse{Success: false}, grpc.Errorf(codes.Internal, "failed to record view")
	}
	return &pb.PostActionResponse{Success: true}, nil
}

func (s *postsServer) CommentPost(ctx context.Context, req *pb.CommentRequest) (*pb.PostActionResponse, error) {
	key := []byte(fmt.Sprintf("%d:%d", req.UserId, req.PostId))
	err := s.produceEvent(ctx, s.commentsTopic, key, []byte(req.Text))
	if err != nil {
		return &pb.PostActionResponse{Success: false}, grpc.Errorf(codes.Internal, "failed to record comment")
	}
	return &pb.PostActionResponse{Success: true}, nil
}

func main() {
	// Инициализация PostgreSQL...
	pg, err := storage.New("postgres://admin:superpass@postgres:5432/posts?sslmode=disable")
	if err != nil {
		log.Fatalf("Postgres error: %v", err)
	}

	// Инициализация Kafka
	kafkaClient, err := NewKafkaClient([]string{"kafka:9092"})
	if err != nil {
		log.Fatalf("Kafka init error: %v", err)
	}
	defer kafkaClient.Close()

	// Создаем топики при необходимости
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ensureTopics(ctx, kafkaClient, []string{
		os.Getenv("KAFKA_LIKES_TOPIC"),
		os.Getenv("KAFKA_VIEWS_TOPIC"),
		os.Getenv("KAFKA_COMMENTS_TOPIC"),
	})

	// gRPC сервер
	lis, err := net.Listen("tcp", ":8082")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(errorInterceptor),
	)
	pb.RegisterPostsServiceServer(s, &postsServer{
		postsRepo:     repository.NewPostsRepository(*pg),
		kafkaClient:   kafkaClient,
		likesTopic:    os.Getenv("KAFKA_LIKES_TOPIC"),
		viewsTopic:    os.Getenv("KAFKA_VIEWS_TOPIC"),
		commentsTopic: os.Getenv("KAFKA_COMMENTS_TOPIC"),
	})

	log.Println("Starting posts service on :8082")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// Вспомогательные функции

func ensureTopics(ctx context.Context, client *kgo.Client, topics []string) {
	req := kmsg.NewCreateTopicsRequest()
	for _, topic := range topics {
		if topic == "" {
			continue
		}
		req.Topics = append(req.Topics, kmsg.CreateTopicsRequestTopic{
			Topic:             topic,
			NumPartitions:     3,
			ReplicationFactor: 1,
		})
	}

	resp, err := req.RequestWith(ctx, client)
	if err != nil {
		log.Printf("Warning: topic creation failed: %v", err)
		return
	}

	for _, topic := range resp.Topics {
		if topic.ErrorCode != 0 {
			log.Printf("Warning: topic %s creation error: %d", topic.Topic, topic.ErrorCode)
		}
	}
}

func errorInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: %v", r)
			err = grpc.Errorf(codes.Internal, "internal server error")
		}
	}()

	resp, err = handler(ctx, req)
	if err != nil {
		log.Printf("gRPC error in %s: %v", info.FullMethod, err)
	}
	return
}
