package gateway

import (
	"net/http"

	exchangepb "github.com/RAF-SI-2025/Banka-3-Backend/gen/exchange"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/status"
)

func (s *Server) GetExchangeRates(c *gin.Context) {
	resp, err := s.ExchangeClient.GetExchangeRates(c.Request.Context(), &exchangepb.ExchangeRateListRequest{})
	if err != nil {
		st, _ := status.FromError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": st.Message()})
		return
	}

	rates := make([]gin.H, 0, len(resp.Rates))
	for _, r := range resp.Rates {
		// If proto doesn't have buy/sell/middle yet, derive at gateway
		middleRate := r.MiddleRate
		buyRate := r.BuyRate
		sellRate := r.SellRate
		if middleRate == 0 {
			middleRate = r.Rate
		}
		if buyRate == 0 {
			buyRate = r.Rate * 0.995
		}
		if sellRate == 0 {
			sellRate = r.Rate * 1.005
		}

		rates = append(rates, gin.H{
			"currencyCode": r.Code,
			"buyRate":      buyRate,
			"sellRate":     sellRate,
			"middleRate":   middleRate,
		})
	}

	c.JSON(http.StatusOK, rates)
}

func (s *Server) ConvertMoney(c *gin.Context) {
	var req conversionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.ExchangeClient.ConvertMoney(c.Request.Context(), &exchangepb.ConversionRequest{
		FromCurrency: req.FromCurrency,
		ToCurrency:   req.ToCurrency,
		Amount:       req.Amount,
	})
	if err != nil {
		st, _ := status.FromError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": st.Message()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
