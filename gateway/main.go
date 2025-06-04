package main

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"time"

	pb "gateway/api/posts"
	pbStats "gateway/api/stats"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	BASE_PATH    = "/api/v1"
	AUTH_SERVER  = "http://auth:8081"
	POSTS_SERVER = "posts:8082"
	STATS_SERVER = "statistics:8084"
)

var AUTH_PATHS = []string{
	"/login",
	"/register",
	"/whoami",
}

const (
	PUBLIC_KEY_PATH = "./public_key.pem"
)

type grpcProxy struct {
	postsConn   *grpc.ClientConn
	statsConn   *grpc.ClientConn
	jwtSecret   *rsa.PublicKey
	postsClient pb.PostsServiceClient
	statsClient pbStats.StatsServiceClient
}

func debugUnaryClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	log.Printf("gRPC call: %s, req: %+v", method, req)
	err := invoker(ctx, method, req, reply, cc, opts...)
	log.Printf("gRPC response: %+v, error: %v", reply, err)
	return err
}

func (p *grpcProxy) Close() {
	p.postsConn.Close()
	p.statsConn.Close()
}

func NewGRPCProxy(postsTarget, statsTarget string, jwtSecret *rsa.PublicKey) (*grpcProxy, error) {
	postsConn, err := grpc.Dial(
		postsTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithUnaryInterceptor(debugUnaryClientInterceptor),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to posts service: %v", err)
	}

	statsConn, err := grpc.Dial(
		statsTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		postsConn.Close()
		return nil, fmt.Errorf("failed to connect to stats service: %v", err)
	}

	return &grpcProxy{
		postsConn:   postsConn,
		statsConn:   statsConn,
		jwtSecret:   jwtSecret,
		postsClient: pb.NewPostsServiceClient(postsConn),
		statsClient: pbStats.NewStatsServiceClient(statsConn),
	}, nil
}

func (p *grpcProxy) extractUserIDFromToken(r *http.Request) (int, error) {
	cookie, err := r.Cookie("jwt")
	if err != nil {
		return 0, err
	}

	token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("invalid signing method")
		}

		return p.jwtSecret, nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to parse token: %v", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if userID, ok := claims["id"].(float64); ok {
			return int(userID), nil
		}
	}

	return 0, fmt.Errorf("invalid token claims")
}

func (p *grpcProxy) createPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := p.extractUserIDFromToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var request struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := p.postsClient.CreatePost(r.Context(), &pb.CreatePostRequest{
		UserId: int32(userID),
		Text:   request.Text,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Добавляем новые обработчики в grpcProxy
func (p *grpcProxy) likePost(w http.ResponseWriter, r *http.Request) {
	p.handlePostAction(w, r, "like")
}

func (p *grpcProxy) viewPost(w http.ResponseWriter, r *http.Request) {
	p.handlePostAction(w, r, "view")
}

func (p *grpcProxy) commentPost(w http.ResponseWriter, r *http.Request) {
	userID, err := p.extractUserIDFromToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var request struct {
		PostID int32  `json:"post_id"`
		Text   string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := p.postsClient.CommentPost(r.Context(), &pb.CommentRequest{
		UserId: int32(userID),
		PostId: request.PostID,
		Text:   request.Text,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (p *grpcProxy) handlePostAction(w http.ResponseWriter, r *http.Request, actionType string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := p.extractUserIDFromToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var request struct {
		PostID int32 `json:"post_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var resp *pb.PostActionResponse
	var actionErr error

	switch actionType {
	case "like":
		log.Println("like 1")
		resp, actionErr = p.postsClient.LikePost(context.Background(), &pb.PostActionRequest{
			UserId: int32(userID),
			PostId: request.PostID,
		})
		log.Println("like 2")
		log.Println(request.PostID, userID, resp, actionErr)
	case "view":
		resp, actionErr = p.postsClient.ViewPost(context.Background(), &pb.PostActionRequest{
			UserId: int32(userID),
			PostId: request.PostID,
		})
	default:
		http.Error(w, "Invalid action type", http.StatusBadRequest)
		return
	}

	if actionErr != nil {
		http.Error(w, actionErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (p *grpcProxy) getPostStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	postID, err := strconv.Atoi(r.URL.Path[len(BASE_PATH+"/stats/post/"):])
	if err != nil {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	resp, err := p.statsClient.GetPostStats(ctx, &pbStats.PostRequest{
		PostId: int32(postID),
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (p *grpcProxy) getPostTrends(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	postID, err := strconv.Atoi(r.URL.Path[len(BASE_PATH+"/stats/trends/"):])
	if err != nil {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if days, err = strconv.Atoi(d); err != nil {
			http.Error(w, "Invalid days parameter", http.StatusBadRequest)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	resp, err := p.statsClient.GetPostTrends(ctx, &pbStats.PostTrendsRequest{
		PostId: int32(postID),
		Days:   uint32(days),
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (p *grpcProxy) getTopPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "likes"
	}

	limit := 10

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	resp, err := p.statsClient.GetTopPosts(ctx, &pbStats.TopRequest{
		Metric: metric,
		Limit:  uint32(limit),
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (p *grpcProxy) getTopUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "likes"
	}

	limit := 10

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	resp, err := p.statsClient.GetTopUsers(ctx, &pbStats.TopRequest{
		Metric: metric,
		Limit:  uint32(limit),
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
func handleGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch st.Code() {
	case codes.NotFound:
		http.Error(w, st.Message(), http.StatusNotFound)
	case codes.InvalidArgument:
		http.Error(w, st.Message(), http.StatusBadRequest)
	case codes.Unauthenticated:
		http.Error(w, st.Message(), http.StatusUnauthorized)
	case codes.PermissionDenied:
		http.Error(w, st.Message(), http.StatusForbidden)
	case codes.DeadlineExceeded:
		http.Error(w, "Request timeout", http.StatusGatewayTimeout)
	case codes.Unavailable:
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	default:
		http.Error(w, st.Message(), http.StatusInternalServerError)
	}
}

type homeHandler struct{}

func NewHomeHandler() *homeHandler {
	return &homeHandler{}
}

func (h homeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)
	// w.Write([]byte("Hello"))
}

func main() {
	authURL, err := url.Parse(AUTH_SERVER)
	if err != nil {
		log.Fatal("Failed to parse auth URL:", err)
	}
	authProxy := httputil.NewSingleHostReverseProxy(authURL)

	handler := NewHomeHandler()
	mux := http.NewServeMux()
	mux.Handle(BASE_PATH, handler)

	for _, path := range AUTH_PATHS {
		mux.Handle(BASE_PATH+path, authProxy)
	}

	publicKey, err := os.ReadFile(PUBLIC_KEY_PATH)
	if err != nil {
		log.Fatal("Failed to parse public key:", err)
	}
	jwtPublic, err := jwt.ParseRSAPublicKeyFromPEM(publicKey)
	if err != nil {
		log.Fatal("Failed to parse public key:", err)
	}

	postsProxy, err := NewGRPCProxy(POSTS_SERVER, STATS_SERVER, jwtPublic)
	if err != nil {
		log.Fatal("Failed to create posts proxy:", err)
	}
	defer postsProxy.Close()

	mux.HandleFunc(BASE_PATH+"/posts/create", postsProxy.createPost)
	mux.HandleFunc(BASE_PATH+"/posts/like", postsProxy.likePost)
	mux.HandleFunc(BASE_PATH+"/posts/view", postsProxy.viewPost)
	mux.HandleFunc(BASE_PATH+"/posts/comment", postsProxy.commentPost)
	
	mux.HandleFunc(BASE_PATH+"/stats/post/", postsProxy.getPostStats)
	mux.HandleFunc(BASE_PATH+"/stats/trends/", postsProxy.getPostTrends)
	mux.HandleFunc(BASE_PATH+"/stats/top/posts", postsProxy.getTopPosts)
	mux.HandleFunc(BASE_PATH+"/stats/top/users", postsProxy.getTopUsers)

	log.Println("Starting server on :8080")
	http.ListenAndServe(":8080", mux)
}
