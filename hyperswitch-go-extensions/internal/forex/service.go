package forex

import (
	"context"
	"fmt"
	"time"

	"github.com/juspay/hyperswitch-go-extensions/pkg/models"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// ExchangeRate represents a currency exchange rate
type ExchangeRate struct {
	From      models.Currency `json:"from"`
	To        models.Currency `json:"to"`
	Rate      decimal.Decimal `json:"rate"`
	Timestamp time.Time       `json:"timestamp"`
	Provider  string          `json:"provider"`
}

// ConversionRequest represents a currency conversion request
type ConversionRequest struct {
	Amount models.Money    `json:"amount"`
	To     models.Currency `json:"to"`
}

// ConversionResponse represents a currency conversion response
type ConversionResponse struct {
	Original       models.Money    `json:"original"`
	Converted      models.Money    `json:"converted"`
	ExchangeRate   decimal.Decimal `json:"exchange_rate"`
	Fee            models.Money    `json:"fee,omitempty"`
	TotalConverted models.Money    `json:"total_converted"`
	Timestamp      time.Time       `json:"timestamp"`
}

// Provider defines the interface for FX rate providers
type Provider interface {
	GetRate(ctx context.Context, from, to models.Currency) (*ExchangeRate, error)
	GetRates(ctx context.Context, base models.Currency) (map[models.Currency]decimal.Decimal, error)
}

// Cache defines the interface for rate caching
type Cache interface {
	Get(ctx context.Context, from, to models.Currency) (*ExchangeRate, error)
	Set(ctx context.Context, rate *ExchangeRate, ttl time.Duration) error
}

// Service handles currency conversion operations
type Service struct {
	provider      Provider
	cache         Cache
	logger        *zap.Logger
	cacheTTL      time.Duration
	feePercentage decimal.Decimal

	// Fallback rates for when provider is unavailable
	fallbackRates map[string]decimal.Decimal
}

// Config represents forex service configuration
type Config struct {
	CacheTTL      time.Duration
	FeePercentage float64
	FallbackRates map[string]float64
}

// NewService creates a new forex service
func NewService(provider Provider, cache Cache, logger *zap.Logger, config Config) *Service {
	fallbackRates := make(map[string]decimal.Decimal)
	for pair, rate := range config.FallbackRates {
		fallbackRates[pair] = decimal.NewFromFloat(rate)
	}

	return &Service{
		provider:      provider,
		cache:         cache,
		logger:        logger,
		cacheTTL:      config.CacheTTL,
		feePercentage: decimal.NewFromFloat(config.FeePercentage),
		fallbackRates: fallbackRates,
	}
}

// Convert converts an amount from one currency to another
func (s *Service) Convert(ctx context.Context, req ConversionRequest) (*ConversionResponse, error) {
	// If same currency, no conversion needed
	if req.Amount.Currency == req.To {
		return &ConversionResponse{
			Original:       req.Amount,
			Converted:      req.Amount,
			ExchangeRate:   decimal.NewFromInt(1),
			TotalConverted: req.Amount,
			Timestamp:      time.Now(),
		}, nil
	}

	// Get exchange rate
	rate, err := s.getExchangeRate(ctx, req.Amount.Currency, req.To)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange rate: %w", err)
	}

	// Convert amount
	convertedAmount := req.Amount.Amount.Mul(rate.Rate)

	// Apply rounding rules based on currency
	convertedAmount = s.roundByCurrency(convertedAmount, req.To)

	converted := models.Money{
		Amount:   convertedAmount,
		Currency: req.To,
	}

	// Calculate fee
	feeAmount := convertedAmount.Mul(s.feePercentage).Div(decimal.NewFromInt(100))
	feeAmount = s.roundByCurrency(feeAmount, req.To)

	fee := models.Money{
		Amount:   feeAmount,
		Currency: req.To,
	}

	// Total with fee
	totalAmount := convertedAmount.Add(feeAmount)
	total := models.Money{
		Amount:   totalAmount,
		Currency: req.To,
	}

	response := &ConversionResponse{
		Original:       req.Amount,
		Converted:      converted,
		ExchangeRate:   rate.Rate,
		Fee:            fee,
		TotalConverted: total,
		Timestamp:      time.Now(),
	}

	s.logger.Info("Currency conversion completed",
		zap.String("from", string(req.Amount.Currency)),
		zap.String("to", string(req.To)),
		zap.String("original_amount", req.Amount.Amount.String()),
		zap.String("converted_amount", converted.Amount.String()),
		zap.String("rate", rate.Rate.String()),
	)

	return response, nil
}

// ConvertForSettlement converts payment amount to merchant's settlement currency
func (s *Service) ConvertForSettlement(ctx context.Context, paymentAmount models.Money, settlementCurrency models.Currency) (*ConversionResponse, error) {
	return s.Convert(ctx, ConversionRequest{
		Amount: paymentAmount,
		To:     settlementCurrency,
	})
}

// GetMultiCurrencyPricing gets pricing in multiple currencies
func (s *Service) GetMultiCurrencyPricing(ctx context.Context, baseAmount models.Money, targetCurrencies []models.Currency) (map[models.Currency]models.Money, error) {
	result := make(map[models.Currency]models.Money)

	// Add base currency
	result[baseAmount.Currency] = baseAmount

	for _, targetCurrency := range targetCurrencies {
		if targetCurrency == baseAmount.Currency {
			continue
		}

		conversion, err := s.Convert(ctx, ConversionRequest{
			Amount: baseAmount,
			To:     targetCurrency,
		})
		if err != nil {
			s.logger.Warn("Failed to convert to currency",
				zap.String("currency", string(targetCurrency)),
				zap.Error(err),
			)
			continue
		}

		result[targetCurrency] = conversion.Converted
	}

	return result, nil
}

// getExchangeRate gets the exchange rate from cache or provider
func (s *Service) getExchangeRate(ctx context.Context, from, to models.Currency) (*ExchangeRate, error) {
	// Try cache first
	if s.cache != nil {
		rate, err := s.cache.Get(ctx, from, to)
		if err == nil && rate != nil {
			s.logger.Debug("Exchange rate found in cache",
				zap.String("from", string(from)),
				zap.String("to", string(to)),
			)
			return rate, nil
		}
	}

	// Fetch from provider
	rate, err := s.provider.GetRate(ctx, from, to)
	if err != nil {
		// Try fallback rates
		return s.getFallbackRate(from, to)
	}

	// Cache the rate
	if s.cache != nil {
		if err := s.cache.Set(ctx, rate, s.cacheTTL); err != nil {
			s.logger.Warn("Failed to cache exchange rate", zap.Error(err))
		}
	}

	return rate, nil
}

// getFallbackRate gets a fallback rate when provider is unavailable
func (s *Service) getFallbackRate(from, to models.Currency) (*ExchangeRate, error) {
	key := fmt.Sprintf("%s-%s", from, to)
	rate, exists := s.fallbackRates[key]
	if !exists {
		// Try reverse rate
		reverseKey := fmt.Sprintf("%s-%s", to, from)
		reverseRate, reverseExists := s.fallbackRates[reverseKey]
		if reverseExists {
			rate = decimal.NewFromInt(1).Div(reverseRate)
		} else {
			return nil, fmt.Errorf("no fallback rate available for %s to %s", from, to)
		}
	}

	return &ExchangeRate{
		From:      from,
		To:        to,
		Rate:      rate,
		Timestamp: time.Now(),
		Provider:  "fallback",
	}, nil
}

// roundByCurrency applies currency-specific rounding rules
func (s *Service) roundByCurrency(amount decimal.Decimal, currency models.Currency) decimal.Decimal {
	switch currency {
	case models.JPY:
		// Japanese Yen has no decimal places
		return amount.Round(0)
	case models.USD, models.EUR, models.GBP, models.AUD, models.CAD, models.SGD, models.HKD:
		// Most major currencies use 2 decimal places
		return amount.Round(2)
	case models.INR:
		// Indian Rupee uses 2 decimal places
		return amount.Round(2)
	case models.CNY:
		// Chinese Yuan uses 2 decimal places
		return amount.Round(2)
	default:
		// Default to 2 decimal places
		return amount.Round(2)
	}
}

// GetSupportedCurrencies returns list of supported currencies
func (s *Service) GetSupportedCurrencies() []models.Currency {
	return []models.Currency{
		models.USD, models.EUR, models.GBP, models.JPY, models.INR,
		models.CNY, models.AUD, models.CAD, models.SGD, models.HKD,
	}
}

// ValidateCurrency checks if a currency is supported
func (s *Service) ValidateCurrency(currency models.Currency) bool {
	supported := s.GetSupportedCurrencies()
	for _, c := range supported {
		if c == currency {
			return true
		}
	}
	return false
}
