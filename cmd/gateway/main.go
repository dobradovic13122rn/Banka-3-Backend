package main

import (
	"log"
	"net/http"
	"os"
	"time"

	notificationpb "banka-raf/gen/notification"
	userpb "banka-raf/gen/user"
	"banka-raf/internal/gateway"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	userAddr := os.Getenv("USER_GRPC_ADDR")
	if userAddr == "" {
		userAddr = "user:50051"
	}

	notificationAddr := os.Getenv("NOTIFICATION_GRPC_ADDR")
	if notificationAddr == "" {
		notificationAddr = "notification:50051"
	}

	userConn, err := grpc.Dial(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to user service: %v", err)
	}
	defer userConn.Close()

	notificationConn, err := grpc.Dial(notificationAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to notification service: %v", err)
	}
	defer notificationConn.Close()

	userClient := userpb.NewUserServiceClient(userConn)
	notificationClient := notificationpb.NewNotificationServiceClient(notificationConn)

	srv := gateway.NewServer(userClient, notificationClient)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	httpServer := &http.Server{
		Addr:              ":" + httpPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("gateway listening on :%s", httpPort)
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatalf("gateway stopped: %v", err)
	}
}
