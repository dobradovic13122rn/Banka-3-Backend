package exchange

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	exchangepb "github.com/RAF-SI-2025/Banka-3-Backend/gen/exchange"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newTestServer(t *testing.T) (*Server, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	return NewServer(gormDB), mock, db
}

func TestConvertMoney(t *testing.T) {
	s, mock, db := newTestServer(t)
	defer func(db *sql.DB) {
		_ = db.Close()
	}(db)

	ctx := context.Background()
	now := time.Now()

	t.Run("Success_EUR_to_USD", func(t *testing.T) {
		mock.ExpectQuery(`SELECT \* FROM "exchange_rates" WHERE currency_code = \$1 ORDER BY "exchange_rates"."currency_code" LIMIT \$2`).
			WithArgs("EUR", 1).
			WillReturnRows(sqlmock.NewRows([]string{"currency_code", "rate_to_rsd", "updated_at"}).
				AddRow("EUR", 117.0, now))

		mock.ExpectQuery(`SELECT \* FROM "exchange_rates" WHERE currency_code = \$1 ORDER BY "exchange_rates"."currency_code" LIMIT \$2`).
			WithArgs("USD", 1).
			WillReturnRows(sqlmock.NewRows([]string{"currency_code", "rate_to_rsd", "updated_at"}).
				AddRow("USD", 108.0, now))

		resp, err := s.ConvertMoney(ctx, &exchangepb.ConversionRequest{
			FromCurrency: "EUR",
			ToCurrency:   "USD",
			Amount:       100,
		})

		assert.NoError(t, err)
		require.NotNil(t, resp)
		assert.InDelta(t, 108.333333, resp.ConvertedAmount, 0.0001)
		assert.InDelta(t, 1.083333, resp.ExchangeRate, 0.0001)
	})

	t.Run("Success_RSD_Base", func(t *testing.T) {
		mock.ExpectQuery(`SELECT \* FROM "exchange_rates" WHERE currency_code = \$1 ORDER BY "exchange_rates"."currency_code" LIMIT \$2`).
			WithArgs("EUR", 1).
			WillReturnRows(sqlmock.NewRows([]string{"currency_code", "rate_to_rsd", "updated_at"}).
				AddRow("EUR", 117.0, now))

		resp, err := s.ConvertMoney(ctx, &exchangepb.ConversionRequest{
			FromCurrency: "RSD",
			ToCurrency:   "EUR",
			Amount:       1170,
		})
		assert.NoError(t, err)
		require.NotNil(t, resp)
		assert.InDelta(t, 10.0, resp.ConvertedAmount, 0.0000001)
	})
}

func TestGetExchangeRates(t *testing.T) {
	s, mock, db := newTestServer(t)
	defer func(db *sql.DB) {
		_ = db.Close()
	}(db)

	now := time.Now()

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery(`SELECT \* FROM "exchange_rates"`).
			WillReturnRows(sqlmock.NewRows([]string{"currency_code", "rate_to_rsd", "updated_at"}).
				AddRow("EUR", 117.0, now))

		resp, err := s.GetExchangeRates(context.Background(), nil)
		assert.NoError(t, err)
		assert.NotEmpty(t, resp.Rates)
	})
}

func TestUpdateRatesRecord(t *testing.T) {
	s, mock, db := newTestServer(t)
	defer func(db *sql.DB) {
		_ = db.Close()
	}(db)

	t.Run("Success", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO "exchange_rates"`).
			WithArgs("EUR", 117.0, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		err := s.UpdateRatesRecord([]Rate{{CurrencyCode: "EUR", RateToRSD: 117.0}})
		assert.NoError(t, err)
	})

	t.Run("RollbackOnFailure", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO "exchange_rates"`).
			WillReturnError(fmt.Errorf("db error"))
		mock.ExpectRollback()

		err := s.UpdateRatesRecord([]Rate{{CurrencyCode: "EUR", RateToRSD: 117.0}})
		assert.Error(t, err)
	})
}
