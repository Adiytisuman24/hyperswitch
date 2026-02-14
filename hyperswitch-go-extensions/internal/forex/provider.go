package forex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/juspay/hyperswitch-go-extensions/pkg/models"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// OpenExchangeRatesProvider implements Provider using OpenExchangeRates API
type OpenExchangeRatesProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// OpenExchangeRatesResponse represents the API response
type OpenExchangeRatesResponse struct {
	Base      string             `json:"base"`
	Rates     map[string]float64 `json:"rates"`
	Timestamp int64              `json:"timestamp"`
}

// NewOpenExchangeRatesProvider creates a new OpenExchangeRates provider
func NewOpenExchangeRatesProvider(apiKey string, logger *zap.Logger) *OpenExchangeRatesProvider {
	return &OpenExchangeRatesProvider{
		apiKey:  apiKey,
		baseURL: "https://openexchangerates.org/api",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// GetRate fetches the exchange rate between two currencies
func (p *OpenExchangeRatesProvider) GetRate(ctx context.Context, from, to models.Currency) (*ExchangeRate, error) {
	rates, err := p.GetRates(ctx, from)
	if err != nil {
		return nil, err
	}

	rate, exists := rates[to]
	if !exists {
		return nil, fmt.Errorf("rate not found for currency pair %s-%s", from, to)
	}

	return &ExchangeRate{
		From:      from,
		To:        to,
		Rate:      rate,
		Timestamp: time.Now(),
		Provider:  "openexchangerates",
	}, nil
}

// GetRates fetches all exchange rates for a base currency
func (p *OpenExchangeRatesProvider) GetRates(ctx context.Context, base models.Currency) (map[models.Currency]decimal.Decimal, error) {
	url := fmt.Sprintf("%s/latest.json?app_id=%s&base=%s", p.baseURL, p.apiKey, base)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result OpenExchangeRatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to map[Currency]decimal.Decimal
	rates := make(map[models.Currency]decimal.Decimal)
	for currencyStr, rateFloat := range result.Rates {
		currency := models.Currency(currencyStr)
		rates[currency] = decimal.NewFromFloat(rateFloat)
	}

	p.logger.Debug("Fetched exchange rates",
		zap.String("base", string(base)),
		zap.Int("count", len(rates)),
	)

	return rates, nil
}

// Mock implementation for testing

// MockProvider is a mock forex provider for testing
type MockProvider struct {
	rates map[string]decimal.Decimal
}

// NewMockProvider creates a new mock provider
func NewMockProvider() *MockProvider {
	return &MockProvider{
		rates: map[string]decimal.Decimal{
			"USD-EUR": decimal.NewFromFloat(0.85),
			"USD-GBP": decimal.NewFromFloat(0.73),
			"USD-JPY": decimal.NewFromFloat(110.50),
			"USD-INR": decimal.NewFromFloat(74.50),
			"EUR-USD": decimal.NewFromFloat(1.18),
			"GBP-USD": decimal.NewFromFloat(1.37),
		},
	}
}

// GetRate returns a mock exchange rate
func (m *MockProvider) GetRate(ctx context.Context, from, to models.Currency) (*ExchangeRate, error) {
	key := fmt.Sprintf("%s-%s", from, to)
	rate, exists := m.rates[key]
	if !exists {
		// Try reverse
		reverseKey := fmt.Sprintf("%s-%s", to, from)
		if reverseRate, ok := m.rates[reverseKey]; ok {
			rate = decimal.NewFromInt(1).Div(reverseRate)
		} else {
			rate = decimal.NewFromInt(1) // Default to 1:1
		}
	}

	return &ExchangeRate{
		From:      from,
		To:        to,
		Rate:      rate,
		Timestamp: time.Now(),
		Provider:  "mock",
	}, nil
}

// GetRates returns all mock rates for a base currency
func (m *MockProvider) GetRates(ctx context.Context, base models.Currency) (map[models.Currency]decimal.Decimal, error) {
	result := make(map[models.Currency]decimal.Decimal)

	for key, rate := range m.rates {
		if len(key) > 3 && key[:3] == string(base) {
			toCurrency := models.Currency(key[4:])
			result[toCurrency] = rate
		}
	}

	return result, nil
}
