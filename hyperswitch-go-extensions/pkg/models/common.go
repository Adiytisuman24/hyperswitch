package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Currency represents ISO 4217 currency codes
type Currency string

const (
	USD Currency = "USD"
	EUR Currency = "EUR"
	GBP Currency = "GBP"
	JPY Currency = "JPY"
	INR Currency = "INR"
	CNY Currency = "CNY"
	AUD Currency = "AUD"
	CAD Currency = "CAD"
	SGD Currency = "SGD"
	HKD Currency = "HKD"
)

// PaymentStatus represents the status of a payment
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusSucceeded PaymentStatus = "succeeded"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusCanceled  PaymentStatus = "canceled"
	PaymentStatusRefunded  PaymentStatus = "refunded"
)

// Money represents an amount with currency
type Money struct {
	Amount   decimal.Decimal `json:"amount"`
	Currency Currency        `json:"currency"`
}

// NewMoney creates a new Money instance
func NewMoney(amount float64, currency Currency) Money {
	return Money{
		Amount:   decimal.NewFromFloat(amount),
		Currency: currency,
	}
}

// Add adds two Money values (must be same currency)
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}
	return Money{
		Amount:   m.Amount.Add(other.Amount),
		Currency: m.Currency,
	}, nil
}

// Subtract subtracts two Money values (must be same currency)
func (m Money) Subtract(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}
	return Money{
		Amount:   m.Amount.Sub(other.Amount),
		Currency: m.Currency,
	}, nil
}

// Multiply multiplies Money by a decimal value
func (m Money) Multiply(multiplier decimal.Decimal) Money {
	return Money{
		Amount:   m.Amount.Mul(multiplier),
		Currency: m.Currency,
	}
}

// MultiplyFloat multiplies Money by a float value
func (m Money) MultiplyFloat(multiplier float64) Money {
	return m.Multiply(decimal.NewFromFloat(multiplier))
}

// IsPositive checks if amount is positive
func (m Money) IsPositive() bool {
	return m.Amount.GreaterThan(decimal.Zero)
}

// IsZero checks if amount is zero
func (m Money) IsZero() bool {
	return m.Amount.Equal(decimal.Zero)
}

// BaseEntity contains common fields for all entities
type BaseEntity struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// NewBaseEntity creates a new BaseEntity
func NewBaseEntity() BaseEntity {
	now := time.Now()
	return BaseEntity{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Metadata is a flexible key-value store for additional data
type Metadata map[string]interface{}

// Address represents a physical address
type Address struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state,omitempty"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"` // ISO 3166-1 alpha-2
}

// Customer represents a customer entity
type Customer struct {
	BaseEntity
	MerchantID uuid.UUID `json:"merchant_id" db:"merchant_id"`
	Email      string    `json:"email" db:"email"`
	Name       string    `json:"name" db:"name"`
	Phone      string    `json:"phone,omitempty" db:"phone"`
	Address    *Address  `json:"address,omitempty" db:"address"`
	Metadata   Metadata  `json:"metadata,omitempty" db:"metadata"`
	Locale     string    `json:"locale" db:"locale"`
	TimeZone   string    `json:"timezone" db:"timezone"`
}

// Merchant represents a merchant/platform entity
type Merchant struct {
	BaseEntity
	Name                string     `json:"name" db:"name"`
	Email               string     `json:"email" db:"email"`
	Country             string     `json:"country" db:"country"`
	SettlementCurrency  Currency   `json:"settlement_currency" db:"settlement_currency"`
	SupportedCurrencies []Currency `json:"supported_currencies" db:"supported_currencies"`
	Metadata            Metadata   `json:"metadata,omitempty" db:"metadata"`
}

// Error types
var (
	ErrCurrencyMismatch = &AppError{Code: "CURRENCY_MISMATCH", Message: "Currency mismatch in operation"}
	ErrInvalidAmount    = &AppError{Code: "INVALID_AMOUNT", Message: "Invalid amount"}
	ErrNotFound         = &AppError{Code: "NOT_FOUND", Message: "Resource not found"}
	ErrUnauthorized     = &AppError{Code: "UNAUTHORIZED", Message: "Unauthorized"}
	ErrInvalidInput     = &AppError{Code: "INVALID_INPUT", Message: "Invalid input"}
)

// AppError represents an application error
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	if e.Details != "" {
		return e.Code + ": " + e.Message + " (" + e.Details + ")"
	}
	return e.Code + ": " + e.Message
}
