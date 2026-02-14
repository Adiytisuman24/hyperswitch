package wallets

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/juspay/hyperswitch-go-extensions/pkg/models"
	"go.uber.org/zap"
)

// WalletType represents the type of digital wallet
type WalletType string

const (
	WalletTypeApplePay    WalletType = "apple_pay"
	WalletTypeGooglePay   WalletType = "google_pay"
	WalletTypePayPal      WalletType = "paypal"
	WalletTypeWeChatPay   WalletType = "wechat_pay"
	WalletTypeAliPay      WalletType = "alipay"
	WalletTypeSamsungPay  WalletType = "samsung_pay"
	WalletTypeAmazonPay   WalletType = "amazon_pay"
	WalletTypeVenmo       WalletType = "venmo"
	WalletTypePayNow      WalletType = "paynow"
	WalletTypeMercadoPago WalletType = "mercado_pago"
	WalletTypePix         WalletType = "pix"
	WalletTypeGrabPay     WalletType = "grabpay"
	WalletTypeTouchNGo    WalletType = "touch_n_go"
	WalletTypeKakaoPay    WalletType = "kakao_pay"
	WalletTypeLinePay     WalletType = "line_pay"
)

// WalletPaymentRequest represents a wallet payment request
type WalletPaymentRequest struct {
	WalletType WalletType      `json:"wallet_type" binding:"required"`
	Amount     models.Money    `json:"amount" binding:"required"`
	CustomerID uuid.UUID       `json:"customer_id" binding:"required"`
	ReturnURL  string          `json:"return_url"`
	CancelURL  string          `json:"cancel_url"`
	Metadata   models.Metadata `json:"metadata,omitempty"`

	// Wallet-specific fields
	ApplePayToken  *ApplePayToken  `json:"apple_pay_token,omitempty"`
	GooglePayToken *GooglePayToken `json:"google_pay_token,omitempty"`
	PayPalData     *PayPalData     `json:"paypal_data,omitempty"`
}

// ApplePayToken represents Apple Pay payment data
type ApplePayToken struct {
	PaymentData   string `json:"payment_data"`
	TransactionID string `json:"transaction_id"`
	PaymentMethod string `json:"payment_method"`
	DisplayName   string `json:"display_name"`
	Network       string `json:"network"`
}

// GooglePayToken represents Google Pay payment data
type GooglePayToken struct {
	Signature              string `json:"signature"`
	ProtocolVersion        string `json:"protocol_version"`
	SignedMessage          string `json:"signed_message"`
	IntermediateSigningKey string `json:"intermediate_signing_key"`
}

// PayPalData represents PayPal payment data
type PayPalData struct {
	PayerID string `json:"payer_id"`
	OrderID string `json:"order_id"`
	Email   string `json:"email"`
}

// WalletPaymentResponse represents the payment response
type WalletPaymentResponse struct {
	PaymentID      uuid.UUID            `json:"payment_id"`
	Status         models.PaymentStatus `json:"status"`
	Amount         models.Money         `json:"amount"`
	RedirectURL    string               `json:"redirect_url,omitempty"`
	RequiresAction bool                 `json:"requires_action"`
	Metadata       models.Metadata      `json:"metadata,omitempty"`
}

// WalletConfig represents wallet configuration
type WalletConfig struct {
	WalletType          WalletType        `json:"wallet_type"`
	Enabled             bool              `json:"enabled"`
	MerchantID          string            `json:"merchant_id"`
	APIKey              string            `json:"api_key"`
	WebhookSecret       string            `json:"webhook_secret"`
	SupportedRegions    []string          `json:"supported_regions"`
	SupportedCurrencies []models.Currency `json:"supported_currencies"`
	Metadata            models.Metadata   `json:"metadata,omitempty"`
}

// Service handles digital wallet operations
type Service struct {
	providers map[WalletType]Provider
	logger    *zap.Logger
}

// Provider defines the interface for wallet providers
type Provider interface {
	ProcessPayment(ctx context.Context, req *WalletPaymentRequest) (*WalletPaymentResponse, error)
	ValidateWebhook(ctx context.Context, payload []byte, signature string) error
	RefundPayment(ctx context.Context, paymentID string, amount models.Money) error
}

// NewService creates a new wallet service
func NewService(logger *zap.Logger) *Service {
	return &Service{
		providers: make(map[WalletType]Provider),
		logger:    logger,
	}
}

// RegisterProvider registers a wallet provider
func (s *Service) RegisterProvider(walletType WalletType, provider Provider) {
	s.providers[walletType] = provider
	s.logger.Info("Wallet provider registered", zap.String("type", string(walletType)))
}

// ProcessPayment processes a wallet payment
func (s *Service) ProcessPayment(ctx context.Context, req *WalletPaymentRequest) (*WalletPaymentResponse, error) {
	provider, exists := s.providers[req.WalletType]
	if !exists {
		return nil, fmt.Errorf("wallet provider %s not found or not enabled", req.WalletType)
	}

	// Validate wallet-specific data
	if err := s.validateWalletData(req); err != nil {
		return nil, fmt.Errorf("invalid wallet data: %w", err)
	}

	response, err := provider.ProcessPayment(ctx, req)
	if err != nil {
		s.logger.Error("Wallet payment failed",
			zap.String("wallet_type", string(req.WalletType)),
			zap.Error(err),
		)
		return nil, err
	}

	s.logger.Info("Wallet payment processed",
		zap.String("wallet_type", string(req.WalletType)),
		zap.String("payment_id", response.PaymentID.String()),
		zap.String("status", string(response.Status)),
	)

	return response, nil
}

// GetSupportedWallets returns wallets supported for a region/currency
func (s *Service) GetSupportedWallets(region string, currency models.Currency) []WalletType {
	regions := map[string][]WalletType{
		"NA": {
			WalletTypeApplePay,
			WalletTypeGooglePay,
			WalletTypePayPal,
			WalletTypeSamsungPay,
			WalletTypeAmazonPay,
			WalletTypeVenmo,
		},
		"EU": {
			WalletTypeApplePay,
			WalletTypeGooglePay,
			WalletTypePayPal,
			WalletTypeSamsungPay,
		},
		"APAC": {
			WalletTypeApplePay,
			WalletTypeGooglePay,
			WalletTypeWeChatPay,
			WalletTypeAliPay,
			WalletTypePayNow,
			WalletTypeGrabPay,
			WalletTypeTouchNGo,
			WalletTypeKakaoPay,
			WalletTypeLinePay,
		},
		"LATAM": {
			WalletTypeGooglePay,
			WalletTypeMercadoPago,
			WalletTypePix,
		},
	}

	if wallets, exists := regions[region]; exists {
		return wallets
	}
	return []WalletType{WalletTypeApplePay, WalletTypeGooglePay, WalletTypePayPal}
}

// validateWalletData validates wallet-specific payment data
func (s *Service) validateWalletData(req *WalletPaymentRequest) error {
	switch req.WalletType {
	case WalletTypeApplePay:
		if req.ApplePayToken == nil || req.ApplePayToken.PaymentData == "" {
			return fmt.Errorf("apple pay token required")
		}
	case WalletTypeGooglePay:
		if req.GooglePayToken == nil || req.GooglePayToken.SignedMessage == "" {
			return fmt.Errorf("google pay token required")
		}
	case WalletTypePayPal:
		if req.PayPalData == nil || req.PayPalData.PayerID == "" {
			return fmt.Errorf("paypal data required")
		}
	}
	return nil
}

// Mock Provider for testing

// MockProvider is a mock wallet provider
type MockProvider struct {
	walletType WalletType
	logger     *zap.Logger
}

// NewMockProvider creates a new mock provider
func NewMockProvider(walletType WalletType, logger *zap.Logger) *MockProvider {
	return &MockProvider{
		walletType: walletType,
		logger:     logger,
	}
}

// ProcessPayment processes a mock payment
func (m *MockProvider) ProcessPayment(ctx context.Context, req *WalletPaymentRequest) (*WalletPaymentResponse, error) {
	// Simulate processing delay
	time.Sleep(500 * time.Millisecond)

	return &WalletPaymentResponse{
		PaymentID:      uuid.New(),
		Status:         models.PaymentStatusSucceeded,
		Amount:         req.Amount,
		RequiresAction: false,
		Metadata:       req.Metadata,
	}, nil
}

// ValidateWebhook validates a webhook signature
func (m *MockProvider) ValidateWebhook(ctx context.Context, payload []byte, signature string) error {
	return nil
}

// RefundPayment processes a refund
func (m *MockProvider) RefundPayment(ctx context.Context, paymentID string, amount models.Money) error {
	m.logger.Info("Processing refund",
		zap.String("payment_id", paymentID),
		zap.String("amount", amount.Amount.String()),
	)
	return nil
}
