package notification

import (
	"banka-raf/gen/notification"
	"context"
)

type Server struct {
	notification.UnimplementedNotificationServiceServer
}

func (s *Server) SendConfirmationEmail(ctx context.Context, req *notification.ConfirmationMailRequest) (*notification.SuccessResponse, error) {
	//todo implement logic for sending an email

	return &notification.SuccessResponse{
		Successful: true,
	}, nil

}

func (s *Server) SendActivationEmail(ctx context.Context, req *notification.ActivationMailRequest) (*notification.SuccessResponse, error) {
	//todo implement logic for sending an email with link

	return &notification.SuccessResponse{
		Successful: true,
	}, nil
}
