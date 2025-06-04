package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/grpc"

	pb "statistics/api/stats"
	"statistics/repository"
)

type statsServer struct {
	pb.UnimplementedStatsServiceServer
	repo *repository.EventRepository
}

func main() {
	// Инициализация PostgreSQL
	db, err := sql.Open("postgres", os.Getenv("POSTGRES_URL"))
	if err != nil {
		log.Fatalf("PostgreSQL connection failed: %v", err)
	}
	defer db.Close()

	// Проверка соединения
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("PostgreSQL ping failed: %v", err)
	}

	// Инициализация репозитория
	repo := repository.NewEventRepository(db)
	if err := repo.InitSchema(ctx); err != nil {
		log.Fatalf("Failed to init schema: %v", err)
	}

	// Kafka consumer (остается без изменений)
	kClient, err := kgo.NewClient(
		kgo.SeedBrokers(strings.Split(os.Getenv("KAFKA_BROKERS"), ",")...),
		kgo.ConsumerGroup(os.Getenv("KAFKA_GROUP_ID")),
		kgo.ConsumeTopics("post_likes", "post_views", "post_comments"),
	)
	if err != nil {
		log.Fatalf("Kafka client error: %v", err)
	}
	defer kClient.Close()

	// Обработчик Kafka в фоне
	go consumeEvents(context.Background(), kClient, repo)

	// gRPC сервер
	lis, err := net.Listen("tcp", ":8084")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterStatsServiceServer(s, &statsServer{repo: repo})

	log.Println("Starting statistics service on :8084")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
func consumeEvents(ctx context.Context, client *kgo.Client, repo *repository.EventRepository) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			log.Println("wow")
			fetches := client.PollFetches(ctx)
			if fetches.IsClientClosed() {
				log.Println("Kafka client closed")
				return
			}

			if errs := fetches.Errors(); len(errs) > 0 {
				for _, err := range errs {
					log.Printf("Kafka error: %v", err)
				}
				continue
			}

			fetches.EachPartition(func(p kgo.FetchTopicPartition) {
				for _, record := range p.Records {
					if err := processEvent(record, repo); err != nil {
						log.Printf("Failed to process event: %v", err)
					} else {
						client.MarkCommitRecords(record)
					}
				}
			})
		}
	}
}

func processEvent(record *kgo.Record, repo *repository.EventRepository) error {
	// Парсинг ключа "user_id:post_id"
	log.Println("got msg: ", record)
	parts := strings.Split(string(record.Key), ":")
	if (len(parts) != 2) && !(len(parts) == 3 && record.Topic == "post_comments") {
		return errors.New("invalid event key format")
	}

	userID, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return err
	}

	postID, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return err
	}

	// Определение типа события
	var eventType string
	var comment *string

	switch record.Topic {
	case "post_likes":
		eventType = "like"
	case "post_views":
		eventType = "view"
	case "post_comments":
		eventType = "comment"
		commentText := parts[2]
		comment = &commentText
	default:
		return errors.New("unknown topic")
	}

	// Сохранение в ClickHouse
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return repo.SaveEvent(ctx, eventType, uint32(userID), uint32(postID), comment)
}

func loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	start := time.Now()
	resp, err = handler(ctx, req)
	log.Printf("Method: %s, Duration: %v, Error: %v", info.FullMethod, time.Since(start), err)
	return
}

// Реализация gRPC методов
func (s *statsServer) GetPostStats(ctx context.Context, req *pb.PostRequest) (*pb.PostStatsResponse, error) {
	stats, err := s.repo.GetPostStats(ctx, uint32(req.PostId))
	if err != nil {
		return nil, err
	}
	return &pb.PostStatsResponse{
		Likes:    stats.Likes,
		Views:    stats.Views,
		Comments: stats.Comments,
	}, nil
}

func (s *statsServer) GetPostTrends(ctx context.Context, req *pb.PostTrendsRequest) (*pb.PostTrendsResponse, error) {
	trends, err := s.repo.GetPostTrends(ctx, uint32(req.PostId), int(req.Days))
	if err != nil {
		return nil, err
	}

	pbTrends := make([]*pb.DayStats, 0, len(trends))
	for _, t := range trends {
		pbTrends = append(pbTrends, &pb.DayStats{
			Date:     t.Date.Format(time.RFC3339),
			Likes:    t.Likes,
			Views:    t.Views,
			Comments: t.Comments,
		})
	}

	return &pb.PostTrendsResponse{Trends: pbTrends}, nil
}

func (s *statsServer) GetTopPosts(ctx context.Context, req *pb.TopRequest) (*pb.TopPostsResponse, error) {
	posts, err := s.repo.GetTopPosts(ctx, req.Metric, int(req.Limit))
	if err != nil {
		return nil, err
	}

	pbPosts := make([]*pb.PostStats, 0, len(posts))
	for _, p := range posts {
		pbPosts = append(pbPosts, &pb.PostStats{
			PostId: int32(p.PostID),
			Count:  p.Count,
		})
	}

	return &pb.TopPostsResponse{Posts: pbPosts}, nil
}

func (s *statsServer) GetTopUsers(ctx context.Context, req *pb.TopRequest) (*pb.TopUsersResponse, error) {
	users, err := s.repo.GetTopUsers(ctx, req.Metric, int(req.Limit))
	if err != nil {
		return nil, err
	}

	pbUsers := make([]*pb.UserStats, 0, len(users))
	for _, u := range users {
		pbUsers = append(pbUsers, &pb.UserStats{
			UserId: int32(u.UserID),
			Count:  u.Count,
		})
	}

	return &pb.TopUsersResponse{Users: pbUsers}, nil
}
