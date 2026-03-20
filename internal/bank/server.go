package bank

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	bankpb "github.com/RAF-SI-2025/Banka-3-Backend/gen/bank"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type Server struct {
	bankpb.UnimplementedBankServiceServer
	database *sql.DB
	db_gorm  *gorm.DB
}

func NewServer(database *sql.DB, gorm_db *gorm.DB) *Server {
	return &Server{
		database: database,
		db_gorm:  gorm_db,
	}
}

func mapCompanyToProto(company *Company) *bankpb.Company {
	if company == nil {
		return nil
	}

	return &bankpb.Company{
		Id:             company.Id,
		RegisteredId:   company.Registered_id,
		Name:           company.Name,
		TaxCode:        company.Tax_code,
		ActivityCodeId: company.Activity_code_id,
		Address:        company.Address,
		OwnerId:        company.Owner_id,
	}
}

func validateCreateCompanyInput(registeredID int64, name string, taxCode int64, address string, ownerID int64) error {
	if registeredID <= 0 {
		return status.Error(codes.InvalidArgument, "registered id must be greater than zero")
	}
	if strings.TrimSpace(name) == "" {
		return status.Error(codes.InvalidArgument, "name is required")
	}
	if taxCode <= 0 {
		return status.Error(codes.InvalidArgument, "tax code must be greater than zero")
	}
	if strings.TrimSpace(address) == "" {
		return status.Error(codes.InvalidArgument, "address is required")
	}
	if ownerID <= 0 {
		return status.Error(codes.InvalidArgument, "owner id must be greater than zero")
	}
	return nil
}

func validateUpdateCompanyInput(id int64, name string, address string, ownerID int64) error {
	if id <= 0 {
		return status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	if strings.TrimSpace(name) == "" {
		return status.Error(codes.InvalidArgument, "name is required")
	}
	if strings.TrimSpace(address) == "" {
		return status.Error(codes.InvalidArgument, "address is required")
	}
	if ownerID <= 0 {
		return status.Error(codes.InvalidArgument, "owner id must be greater than zero")
	}
	return nil
}

func (s *Server) CreateCompany(_ context.Context, req *bankpb.CreateCompanyRequest) (*bankpb.CreateCompanyResponse, error) {
	if err := validateCreateCompanyInput(req.RegisteredId, req.Name, req.TaxCode, req.Address, req.OwnerId); err != nil {
		return nil, err
	}

	company, err := s.CreateCompanyRecord(Company{
		Registered_id:    req.RegisteredId,
		Name:             strings.TrimSpace(req.Name),
		Tax_code:         req.TaxCode,
		Activity_code_id: req.ActivityCodeId,
		Address:          strings.TrimSpace(req.Address),
		Owner_id:         req.OwnerId,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrCompanyRegisteredIDExists):
			return nil, status.Error(codes.AlreadyExists, "company with that registered id already exists")
		case errors.Is(err, ErrCompanyOwnerNotFound):
			return nil, status.Error(codes.InvalidArgument, "owner does not exist")
		case errors.Is(err, ErrCompanyActivityCodeNotFound):
			return nil, status.Error(codes.InvalidArgument, "activity code does not exist")
		default:
			return nil, status.Error(codes.Internal, "company creation failed")
		}
	}

	return &bankpb.CreateCompanyResponse{Company: mapCompanyToProto(company)}, nil
}

func (s *Server) GetCompanyById(_ context.Context, req *bankpb.GetCompanyByIdRequest) (*bankpb.GetCompanyByIdResponse, error) {
	if req.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}

	company, err := s.GetCompanyByIDRecord(req.Id)
	if err != nil {
		switch {
		case errors.Is(err, ErrCompanyNotFound):
			return nil, status.Error(codes.NotFound, "company not found")
		default:
			return nil, status.Error(codes.Internal, "company lookup failed")
		}
	}

	return &bankpb.GetCompanyByIdResponse{Company: mapCompanyToProto(company)}, nil
}

func (s *Server) GetCompanies(_ context.Context, _ *bankpb.GetCompaniesRequest) (*bankpb.GetCompaniesResponse, error) {
	companies, err := s.GetCompaniesRecords()
	if err != nil {
		return nil, status.Error(codes.Internal, "company listing failed")
	}

	var responseCompanies []*bankpb.Company
	for _, company := range companies {
		responseCompanies = append(responseCompanies, mapCompanyToProto(company))
	}

	return &bankpb.GetCompaniesResponse{Companies: responseCompanies}, nil
}

func (s *Server) UpdateCompany(_ context.Context, req *bankpb.UpdateCompanyRequest) (*bankpb.UpdateCompanyResponse, error) {
	if err := validateUpdateCompanyInput(req.Id, req.Name, req.Address, req.OwnerId); err != nil {
		return nil, err
	}

	company, err := s.UpdateCompanyRecord(Company{
		Id:               req.Id,
		Name:             strings.TrimSpace(req.Name),
		Activity_code_id: req.ActivityCodeId,
		Address:          strings.TrimSpace(req.Address),
		Owner_id:         req.OwnerId,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrCompanyNotFound):
			return nil, status.Error(codes.NotFound, "company not found")
		case errors.Is(err, ErrCompanyOwnerNotFound):
			return nil, status.Error(codes.InvalidArgument, "owner does not exist")
		case errors.Is(err, ErrCompanyActivityCodeNotFound):
			return nil, status.Error(codes.InvalidArgument, "activity code does not exist")
		default:
			return nil, status.Error(codes.Internal, "company update failed")
		}
	}

	return &bankpb.UpdateCompanyResponse{Company: mapCompanyToProto(company)}, nil
}

func mapCardToProto(card *Card) *bankpb.CardResponse {
	if card == nil {
		return nil
	}
	return &bankpb.CardResponse{
		CardId:         fmt.Sprintf("%d", card.Id),
		CardNumber:     card.Number,
		CardType:       string(card.Type),
		CardBrand:      string(card.Brand),
		CreationDate:   card.Creation_date.Format(time.RFC3339),
		ExpirationDate: card.Valid_until.Format(time.RFC3339),
		AccountNumber:  card.Account_number,
		Cvv:            card.Cvv,
		Limit:          card.Card_limit,
		Status:         string(card.Status),
	}
}

func (s *Server) CreateCard(_ context.Context, req *bankpb.CreateCardRequest) (*bankpb.CardResponse, error) {
	brand := card_brand(strings.ToLower(req.CardBrand))
	number, err := GenerateCardNumber(brand, req.AccountNumber)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	card, err := s.CreateCardRecord(Card{
		Number:         number,
		Type:           card_type(strings.ToLower(req.CardType)),
		Brand:          brand,
		Valid_until:    time.Now().AddDate(5, 0, 0),
		Account_number: req.AccountNumber,
		Cvv:            GenerateCVV(),
		Status:         Active,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create card")
	}

	return mapCardToProto(card), nil
}

func (s *Server) RequestCard(ctx context.Context, req *bankpb.RequestCardRequest) (*bankpb.RequestCardResponse, error) {
	log.Printf("[RequestCard] Received request for account: %s", req.AccountNumber)

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		log.Println("[RequestCard] Error: metadata missing")
		return nil, status.Error(codes.Unauthenticated, "metadata missing")
	}

	emails := md.Get("user-email")
	if len(emails) == 0 {
		log.Println("[RequestCard] Error: user-email missing in metadata")
		return nil, status.Error(codes.Unauthenticated, "email missing in metadata")
	}
	userEmail := emails[0]
	log.Printf("[RequestCard] User identified: %s", userEmail)

	// Provera naloga
	acc, err := s.GetAccountByNumberRecord(req.AccountNumber)
	if err != nil {
		log.Printf("[RequestCard] Error: Account %s not found in database: %v", req.AccountNumber, err)
		return nil, status.Error(codes.NotFound, "account not found")
	}

	// Provera autorizacije i limita
	isAuth, _ := s.IsAuthorizedParty(userEmail, req.AccountNumber)
	limit := 2
	if isAuth {
		limit = 1
		log.Printf("[RequestCard] User %s is authorized party. Limit set to %d", userEmail, limit)
	} else {
		log.Printf("[RequestCard] User %s is account owner. Limit set to %d", userEmail, limit)
	}

	count, err := s.CountActiveCardsByAccountNumber(req.AccountNumber)
	if err != nil {
		log.Printf("[RequestCard] Error: Failed to count active cards for %s: %v", req.AccountNumber, err)
		return nil, status.Error(codes.Internal, "failed to check limits")
	}

	if count >= limit {
		log.Printf("[RequestCard] Rejected: Account %s has %d active cards (Limit: %d)", req.AccountNumber, count, limit)
		return nil, status.Error(codes.FailedPrecondition, "card limit reached for this user type")
	}

	// Kreiranje zahteva
	token := fmt.Sprintf("tkn-%d-%d", time.Now().UnixNano(), acc.Id)
	cardReq := CardRequest{
		Account_number: req.AccountNumber,
		Type:           card_type(strings.ToLower(req.CardType)),
		Brand:          card_brand(strings.ToLower(req.CardBrand)),
		Token:          token,
		ExpirationDate: time.Now().Add(24 * time.Hour),
		Complete:       false,
		Email:          userEmail,
	}

	_, err = s.CreateCardRequestRecord(cardReq)
	if err != nil {
		log.Printf("[RequestCard] Error: Database failure creating request record: %v", err)
		return nil, status.Error(codes.Internal, "failed to create request")
	}
	log.Printf("[RequestCard] Success: Card request created for %s. Token: %s", userEmail, token)

	baseUrl := "http://localhost:8080/api/cards/confirm/?token="
	url := baseUrl + token

	err = s.sendCardConfirmationEmail(ctx, userEmail, url)
	if err != nil {
		log.Printf("[RequestCard] Warning: Failed to send confirmation email to %s: %v", userEmail, err)
		return nil, err
	}
	log.Printf("[RequestCard] Confirmation email triggered for %s", userEmail)

	return &bankpb.RequestCardResponse{Accepted: true}, nil
}

func (s *Server) ConfirmCard(ctx context.Context, req *bankpb.ConfirmCardRequest) (*bankpb.ConfirmCardResponse, error) {
	request, err := s.GetCardRequestByToken(req.Token)
	if err != nil {
		return nil, status.Error(codes.NotFound, "invalid or expired token")
	}

	if time.Now().After(request.ExpirationDate) {
		return nil, status.Error(codes.DeadlineExceeded, "token expired")
	}

	cardNumber, _ := GenerateCardNumber(request.Brand, request.Account_number)
	_, err = s.CreateCardRecord(Card{
		Number:         cardNumber,
		Type:           request.Type,
		Brand:          request.Brand,
		Valid_until:    time.Now().AddDate(5, 0, 0),
		Account_number: request.Account_number,
		Cvv:            GenerateCVV(),
		Status:         Active,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create card from request")
	}

	err = s.MarkCardRequestFulfilled(request.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to close request")
	}

	err = s.sendCardCreatedEmail(ctx, request.Email)
	if err != nil {
		return nil, err
	}

	return &bankpb.ConfirmCardResponse{}, nil
}

func (s *Server) GetCards(_ context.Context, _ *bankpb.GetCardsRequest) (*bankpb.GetCardsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

func (s *Server) BlockCard(_ context.Context, req *bankpb.BlockCardRequest) (*bankpb.BlockCardResponse, error) {
	if req.CardId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid card id")
	}

	err := s.BlockCardRecord(req.CardId)
	if err != nil {
		return &bankpb.BlockCardResponse{Success: false}, status.Error(codes.NotFound, "card not found")
	}

	return &bankpb.BlockCardResponse{Success: true}, nil
}
