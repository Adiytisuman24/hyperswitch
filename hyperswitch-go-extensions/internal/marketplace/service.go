package marketplace

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/juspay/hyperswitch-go-extensions/pkg/models"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// Vendor represents a marketplace vendor/seller
type Vendor struct {
	models.BaseEntity
	MerchantID     uuid.UUID       `json:"merchant_id" db:"merchant_id"`
	Name           string          `json:"name" db:"name"`
	Email          string          `json:"email" db:"email"`
	PayoutAccount  string          `json:"payout_account" db:"payout_account"`
	PayoutMethod   PayoutMethod    `json:"payout_method" db:"payout_method"`
	CommissionRate decimal.Decimal `json:"commission_rate" db:"commission_rate"` // Platform commission %
	Currency       models.Currency `json:"currency" db:"currency"`
	Verified       bool            `json:"verified" db:"verified"`
	Metadata       models.Metadata `json:"metadata,omitempty" db:"metadata"`
}

// PayoutMethod represents how vendors receive payments
type PayoutMethod string

const (
	PayoutMethodBankTransfer  PayoutMethod = "bank_transfer"
	PayoutMethodWallet        PayoutMethod = "wallet"
	PayoutMethodPayPal        PayoutMethod = "paypal"
	PayoutMethodStripeConnect PayoutMethod = "stripe_connect"
)

// SplitConfig defines how to split a payment
type SplitConfig struct {
	PlatformFee       models.Money  `json:"platform_fee"`
	PlatformFeeType   FeeType       `json:"platform_fee_type"` // fixed or percentage
	VendorSplits      []VendorSplit `json:"vendor_splits"`
	EscrowEnabled     bool          `json:"escrow_enabled"`
	EscrowReleaseDays int           `json:"escrow_release_days"`
}

// FeeType represents how fees are calculated
type FeeType string

const (
	FeeTypeFixed      FeeType = "fixed"
	FeeTypePercentage FeeType = "percentage"
)

// VendorSplit represents a vendor's share of the payment
type VendorSplit struct {
	VendorID    uuid.UUID       `json:"vendor_id"`
	Amount      models.Money    `json:"amount"`
	Percentage  decimal.Decimal `json:"percentage,omitempty"` // If percentage-based
	Description string          `json:"description"`
	Metadata    models.Metadata `json:"metadata,omitempty"`
}

// Payout represents a payout to a vendor
type Payout struct {
	models.BaseEntity
	VendorID         uuid.UUID       `json:"vendor_id" db:"vendor_id"`
	Amount           models.Money    `json:"amount"`
	Status           PayoutStatus    `json:"status" db:"status"`
	Method           PayoutMethod    `json:"method" db:"method"`
	ScheduledAt      time.Time       `json:"scheduled_at" db:"scheduled_at"`
	ProcessedAt      *time.Time      `json:"processed_at,omitempty" db:"processed_at"`
	FailureReason    string          `json:"failure_reason,omitempty" db:"failure_reason"`
	ExternalPayoutID string          `json:"external_payout_id,omitempty" db:"external_payout_id"`
	Metadata         models.Metadata `json:"metadata,omitempty" db:"metadata"`
}

// PayoutStatus represents the status of a payout
type PayoutStatus string

const (
	PayoutStatusPending   PayoutStatus = "pending"
	PayoutStatusScheduled PayoutStatus = "scheduled"
	PayoutStatusInTransit PayoutStatus = "in_transit"
	PayoutStatusPaid      PayoutStatus = "paid"
	PayoutStatusFailed    PayoutStatus = "failed"
	PayoutStatusCanceled  PayoutStatus = "canceled"
)

// SplitPayment represents a payment that's been split among vendors
type SplitPayment struct {
	models.BaseEntity
	PaymentID     uuid.UUID       `json:"payment_id" db:"payment_id"`
	TotalAmount   models.Money    `json:"total_amount"`
	PlatformFee   models.Money    `json:"platform_fee"`
	VendorPayouts []VendorPayout  `json:"vendor_payouts"`
	EscrowUntil   *time.Time      `json:"escrow_until,omitempty" db:"escrow_until"`
	Released      bool            `json:"released" db:"released"`
	Metadata      models.Metadata `json:"metadata,omitempty" db:"metadata"`
}

// VendorPayout represents a vendor's payout from a split payment
type VendorPayout struct {
	VendorID   uuid.UUID    `json:"vendor_id"`
	Amount     models.Money `json:"amount"`
	Commission models.Money `json:"commission"`
	NetAmount  models.Money `json:"net_amount"`
	PayoutID   *uuid.UUID   `json:"payout_id,omitempty"`
	Status     PayoutStatus `json:"status"`
}

// Service handles marketplace operations
type Service struct {
	repo      Repository
	payoutSvc PayoutProvider
	logger    *zap.Logger
}

// Repository defines marketplace data access
type Repository interface {
	CreateVendor(ctx context.Context, vendor *Vendor) error
	GetVendor(ctx context.Context, id uuid.UUID) (*Vendor, error)
	CreateSplitPayment(ctx context.Context, split *SplitPayment) error
	GetSplitPayment(ctx context.Context, id uuid.UUID) (*SplitPayment, error)
	CreatePayout(ctx context.Context, payout *Payout) error
	GetPendingPayouts(ctx context.Context, vendorID uuid.UUID) ([]*Payout, error)
	GetScheduledPayouts(ctx context.Context, before time.Time) ([]*Payout, error)
}

// PayoutProvider defines the payout processing interface
type PayoutProvider interface {
	ProcessPayout(ctx context.Context, payout *Payout) (string, error)
}

// NewService creates a new marketplace service
func NewService(repo Repository, payoutSvc PayoutProvider, logger *zap.Logger) *Service {
	return &Service{
		repo:      repo,
		payoutSvc: payoutSvc,
		logger:    logger,
	}
}

// SplitPaymentRequest represents a request to split a payment
type SplitPaymentRequest struct {
	PaymentID   uuid.UUID    `json:"payment_id" binding:"required"`
	TotalAmount models.Money `json:"total_amount" binding:"required"`
	Config      SplitConfig  `json:"config" binding:"required"`
}

// SplitPayment splits a payment according to the configuration
func (s *Service) SplitPayment(ctx context.Context, req *SplitPaymentRequest) (*SplitPayment, error) {
	// Validate total splits don't exceed payment amount
	if err := s.validateSplitConfig(req.TotalAmount, req.Config); err != nil {
		return nil, fmt.Errorf("invalid split configuration: %w", err)
	}

	// Calculate platform fee
	platformFee, err := s.calculatePlatformFee(req.TotalAmount, req.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate platform fee: %w", err)
	}

	// Calculate vendor payouts
	vendorPayouts, err := s.calculateVendorPayouts(req.TotalAmount, platformFee, req.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate vendor payouts: %w", err)
	}

	// Create split payment record
	splitPayment := &SplitPayment{
		BaseEntity:    models.NewBaseEntity(),
		PaymentID:     req.PaymentID,
		TotalAmount:   req.TotalAmount,
		PlatformFee:   platformFee,
		VendorPayouts: vendorPayouts,
		Released:      !req.Config.EscrowEnabled,
	}

	// Set escrow release date if enabled
	if req.Config.EscrowEnabled && req.Config.EscrowReleaseDays > 0 {
		releaseDate := time.Now().AddDate(0, 0, req.Config.EscrowReleaseDays)
		splitPayment.EscrowUntil = &releaseDate
	}

	if err := s.repo.CreateSplitPayment(ctx, splitPayment); err != nil {
		return nil, fmt.Errorf("failed to create split payment: %w", err)
	}

	// Schedule payouts if not in escrow
	if !req.Config.EscrowEnabled {
		if err := s.schedulePayouts(ctx, splitPayment); err != nil {
			s.logger.Error("Failed to schedule payouts", zap.Error(err))
		}
	}

	s.logger.Info("Payment split created",
		zap.String("split_id", splitPayment.ID.String()),
		zap.String("payment_id", req.PaymentID.String()),
		zap.Int("vendor_count", len(vendorPayouts)),
	)

	return splitPayment, nil
}

// ReleaseEscrow releases escrowed funds and schedules payouts
func (s *Service) ReleaseEscrow(ctx context.Context, splitPaymentID uuid.UUID) error {
	split, err := s.repo.GetSplitPayment(ctx, splitPaymentID)
	if err != nil {
		return fmt.Errorf("failed to get split payment: %w", err)
	}

	if split.Released {
		return fmt.Errorf("escrow already released")
	}

	split.Released = true
	split.UpdatedAt = time.Now()

	if err := s.schedulePayouts(ctx, split); err != nil {
		return fmt.Errorf("failed to schedule payouts: %w", err)
	}

	s.logger.Info("Escrow released",
		zap.String("split_id", splitPaymentID.String()),
	)

	return nil
}

// ProcessScheduledPayouts processes all scheduled payouts
func (s *Service) ProcessScheduledPayouts(ctx context.Context) error {
	payouts, err := s.repo.GetScheduledPayouts(ctx, time.Now())
	if err != nil {
		return fmt.Errorf("failed to get scheduled payouts: %w", err)
	}

	s.logger.Info("Processing scheduled payouts", zap.Int("count", len(payouts)))

	for _, payout := range payouts {
		if err := s.processPayout(ctx, payout); err != nil {
			s.logger.Error("Failed to process payout",
				zap.String("payout_id", payout.ID.String()),
				zap.Error(err),
			)
		}
	}

	return nil
}

// Helper functions

func (s *Service) validateSplitConfig(total models.Money, config SplitConfig) error {
	platformFee, err := s.calculatePlatformFee(total, config)
	if err != nil {
		return err
	}

	vendorTotal := models.NewMoney(0, total.Currency)
	for _, split := range config.VendorSplits {
		vendorTotal, _ = vendorTotal.Add(split.Amount)
	}

	combined, _ := platformFee.Add(vendorTotal)
	if combined.Amount.GreaterThan(total.Amount) {
		return fmt.Errorf("total splits exceed payment amount")
	}

	return nil
}

func (s *Service) calculatePlatformFee(total models.Money, config SplitConfig) (models.Money, error) {
	if config.PlatformFeeType == FeeTypeFixed {
		return config.PlatformFee, nil
	}

	// Percentage-based fee
	feeAmount := total.MultiplyFloat(config.PlatformFee.Amount.InexactFloat64() / 100)
	return feeAmount, nil
}

func (s *Service) calculateVendorPayouts(total, platformFee models.Money, config SplitConfig) ([]VendorPayout, error) {
	var payouts []VendorPayout

	for _, split := range config.VendorSplits {
		// Get vendor to check commission rate
		vendor, err := s.repo.GetVendor(context.Background(), split.VendorID)
		if err != nil {
			return nil, fmt.Errorf("failed to get vendor %s: %w", split.VendorID, err)
		}

		// Calculate commission
		commission := split.Amount.MultiplyFloat(vendor.CommissionRate.InexactFloat64() / 100)

		// Net amount after commission
		netAmount, _ := split.Amount.Subtract(commission)

		payout := VendorPayout{
			VendorID:   split.VendorID,
			Amount:     split.Amount,
			Commission: commission,
			NetAmount:  netAmount,
			Status:     PayoutStatusPending,
		}

		payouts = append(payouts, payout)
	}

	return payouts, nil
}

func (s *Service) schedulePayouts(ctx context.Context, split *SplitPayment) error {
	for _, vendorPayout := range split.VendorPayouts {
		vendor, err := s.repo.GetVendor(ctx, vendorPayout.VendorID)
		if err != nil {
			return err
		}

		payout := &Payout{
			BaseEntity:  models.NewBaseEntity(),
			VendorID:    vendorPayout.VendorID,
			Amount:      vendorPayout.NetAmount,
			Status:      PayoutStatusScheduled,
			Method:      vendor.PayoutMethod,
			ScheduledAt: time.Now().Add(24 * time.Hour), // Schedule for next day
		}

		if err := s.repo.CreatePayout(ctx, payout); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) processPayout(ctx context.Context, payout *Payout) error {
	payout.Status = PayoutStatusInTransit

	externalID, err := s.payoutSvc.ProcessPayout(ctx, payout)
	if err != nil {
		payout.Status = PayoutStatusFailed
		payout.FailureReason = err.Error()
		return err
	}

	payout.Status = PayoutStatusPaid
	payout.ExternalPayoutID = externalID
	now := time.Now()
	payout.ProcessedAt = &now

	s.logger.Info("Payout processed",
		zap.String("payout_id", payout.ID.String()),
		zap.String("vendor_id", payout.VendorID.String()),
		zap.String("amount", payout.Amount.Amount.String()),
	)

	return nil
}
