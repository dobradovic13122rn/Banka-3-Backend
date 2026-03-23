package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	exchangepb "github.com/RAF-SI-2025/Banka-3-Backend/gen/exchange"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type Server struct {
	exchangepb.UnsafeExchangeServiceServer
	db_gorm *gorm.DB
}

func NewServer(gorm_db *gorm.DB) *Server {
	return &Server{
		db_gorm: gorm_db,
	}
}

func (s *Server) fetchAndStoreRates() error {
	log.Println("[ExchangeService] Starting fetchAndStoreRates...")

	apiKey := os.Getenv("EXCHANGE_RATE_API_KEY")
	if apiKey == "" || apiKey == "YOUR_KEY" {
		err := fmt.Errorf("missing EXCHANGE_RATE_API_KEY environment variable")
		log.Printf("[ExchangeService] Configuration error: %v", err)
		return err
	}

	url := fmt.Sprintf("https://v6.exchangerate-api.com/v6/%s/latest/RSD", apiKey)
	log.Printf("[ExchangeService] Calling external API: %s", url)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("[ExchangeService] HTTP Request failed: %v", err)
		return err
	}
	defer func(Body io.ReadCloser) { _ = Body.Close() }(resp.Body)

	log.Printf("[ExchangeService] API Response Status: %s", resp.Status)

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		log.Printf("[ExchangeService] JSON Decode error: %v", err)
		return err
	}

	if apiResp.Result != "success" {
		err := fmt.Errorf("api error: %s", apiResp.Result)
		log.Printf("[ExchangeService] External API returned failure: %v", err)
		return err
	}

	supported := []string{"EUR", "CHF", "USD", "GBP", "JPY", "CAD", "AUD"}
	var ratesToUpdate []Rate
	for _, code := range supported {
		if val, ok := apiResp.ConversionRates[code]; ok {
			ratesToUpdate = append(ratesToUpdate, Rate{
				CurrencyCode: code,
				RateToRSD:    1.0 / val,
			})
		}
	}

	log.Printf("[ExchangeService] Successfully fetched %d rates. Proceeding to database update...", len(ratesToUpdate))
	if err := s.UpdateRatesRecord(ratesToUpdate); err != nil {
		log.Printf("[ExchangeService] Failed to update rates in DB: %v", err)
		return err
	}

	log.Println("[ExchangeService] fetchAndStoreRates completed successfully.")
	return nil
}

func (s *Server) GetExchangeRates(_ context.Context, _ *exchangepb.ExchangeRateListRequest) (*exchangepb.ExchangeRateListResponse, error) {
	log.Println("[ExchangeService] GetExchangeRates called")

	rates, err := s.GetRatesRecord()
	if err != nil {
		log.Printf("[ExchangeService] Error retrieving rates from record: %v", err)
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	// Trigger refresh if no rates or if they are older than 24h
	if len(rates) == 0 || time.Since(rates[0].UpdatedAt) > 24*time.Hour {
		log.Printf("[ExchangeService] Rates stale or missing (count: %d). Fetching new rates...", len(rates))
		if err := s.fetchAndStoreRates(); err != nil {
			log.Printf("[ExchangeService] Background fetch failed: %v", err)
			// Proceed with old rates if available, even if refresh failed
		}
		// Fetch again after update
		rates, _ = s.GetRatesRecord()
	}

	var pbRates []*exchangepb.CurrencyRate
	var latestUpdate time.Time
	for _, r := range rates {
		pbRates = append(pbRates, &exchangepb.CurrencyRate{
			Code: r.CurrencyCode,
			Rate: r.RateToRSD,
		})
		if r.UpdatedAt.After(latestUpdate) {
			latestUpdate = r.UpdatedAt
		}
	}
	pbRates = append(pbRates, &exchangepb.CurrencyRate{Code: "RSD", Rate: 1.0})

	log.Printf("[ExchangeService] GetExchangeRates success: returning %d rates", len(pbRates))
	return &exchangepb.ExchangeRateListResponse{
		Rates:       pbRates,
		LastUpdated: latestUpdate.Unix(),
	}, nil
}

func (s *Server) ConvertMoney(_ context.Context, req *exchangepb.ConversionRequest) (*exchangepb.ConversionResponse, error) {
	log.Printf("[ExchangeService] ConvertMoney called: %f %s -> %s", req.Amount, req.FromCurrency, req.ToCurrency)

	if req.Amount <= 0 {
		log.Printf("[ExchangeService] ConvertMoney failed: Invalid amount %f", req.Amount)
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}

	from, err := s.GetRateByCodeRecord(req.FromCurrency)
	if err != nil {
		log.Printf("[ExchangeService] ConvertMoney error: Source currency %s not found", req.FromCurrency)
		return nil, status.Errorf(codes.NotFound, "source %s not supported", req.FromCurrency)
	}

	to, err := s.GetRateByCodeRecord(req.ToCurrency)
	if err != nil {
		log.Printf("[ExchangeService] ConvertMoney error: Target currency %s not found", req.ToCurrency)
		return nil, status.Errorf(codes.NotFound, "target %s not supported", req.ToCurrency)
	}

	effectiveRate := from.RateToRSD / to.RateToRSD
	converted := req.Amount * effectiveRate

	log.Printf("[ExchangeService] ConvertMoney success: %f %s is %f %s (Rate: %f)",
		req.Amount, req.FromCurrency, converted, req.ToCurrency, effectiveRate)

	return &exchangepb.ConversionResponse{
		ConvertedAmount: converted,
		ExchangeRate:    effectiveRate,
	}, nil
}
