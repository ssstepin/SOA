package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func extractJWTToken(r *http.Request) string {
	cookie, err := r.Cookie("token")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func customMetadataAnnotator(ctx context.Context, r *http.Request) metadata.MD {
	token := extractJWTToken(r)
	if token == "" {
		return metadata.Pairs()
	}
	return metadata.Pairs("token", token)
}

func runProxy() error {
	port := 8094
	endpoint := "posts:8093"

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux(
		runtime.WithMetadata(customMetadataAnnotator),
	)
	opts := []grpc.DialOption{grpc.WithInsecure()}

	err := RegisterPostsHandlerFromEndpoint(ctx, mux, endpoint, opts)
	if err != nil {
		return err
	}

	log.Printf("Starting HTTP proxy on :%d\n", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}

func main() {
	if err := runProxy(); err != nil {
		log.Fatal("Failed to start HTTP proxy")
	}
}
