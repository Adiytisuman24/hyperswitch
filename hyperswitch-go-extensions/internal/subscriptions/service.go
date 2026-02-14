package subscriptions

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/juspay/hyperswitch-go-extensions/pkg/models"
	"github.com/robfig/cron/v3"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// Service handles subscription business logic
type Service struct {
	repo          Repository
	paymentSvc    PaymentService
	logger        *zap.Logger
	dunningConfig DunningConfig
	cron          *cron.Cron
}

// Repository defines the subscription data access interface
type Repository interface {
	CreateSubscription(ctx context.Context, sub *Subscription) error
	GetSubscription(ctx context.Context, id uuid.UUID) (*Subscription, error)
	UpdateSubscription(ctx context.Context, sub *Subscription) error
	ListSubscriptions(ctx context.Context, customerID uuid.UUID, status *SubscriptionStatus) ([]*Subscription, error)
	GetPlan(ctx context.Context, id uuid.UUID) (*Plan, error)
	CreateInvoice(ctx context.Context, invoice *Invoice) error
	GetDueSubscriptions(ctx context.Context, dueDate time.Time) ([]*Subscription, error)
	RecordEvent(ctx context.Context, event *SubscriptionEvent) error
}

// PaymentService defines the payment processing interface
type PaymentService interface {
	ChargeCustomer(ctx context.Context, customerID uuid.UUID, amount models.Money, description string) (string, error)
}

// NewService creates a new subscription service
func NewService(repo Repository, paymentSvc PaymentService, logger *zap.Logger, dunningConfig DunningConfig) *Service {
	svc := &Service{
		repo:          repo,
		paymentSvc:    paymentSvc,
		logger:        logger,
		dunningConfig: dunningConfig,
		cron:          cron.New(),
	}

	// Schedule daily billing job
	svc.cron.AddFunc("0 0 * * *", func() {
		ctx := context.Background()
		if err := svc.ProcessDueSubscriptions(ctx); err != nil {
			logger.Error("Failed to process due subscriptions", zap.Error(err))
		}
	})

	svc.cron.Start()
	return svc
}

// CreateSubscription creates a new subscription
func (s *Service) CreateSubscription(ctx context.Context, req *CreateSubscriptionRequest) (*Subscription, error) {
	plan, err := s.repo.GetPlan(ctx, req.PlanID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	now := time.Now()
	sub := &Subscription{
		BaseEntity:         models.NewBaseEntity(),
		CustomerID:         req.CustomerID,
		PlanID:             req.PlanID,
		Status:             SubscriptionStatusActive,
		CurrentPeriodStart: now,
		Quantity:           req.Quantity,
		Metadata:           req.Metadata,
	}

	if sub.Quantity == 0 {
		sub.Quantity = 1
	}

	// Handle trial period
	if req.TrialDays != nil && *req.TrialDays > 0 {
		trialStart := now
		trialEnd := now.AddDate(0, 0, *req.TrialDays)
		sub.TrialStart = &trialStart
		sub.TrialEnd = &trialEnd
		sub.Status = SubscriptionStatusTrialing
	}

	// Calculate period end
	sub.CurrentPeriodEnd = s.calculatePeriodEnd(now, plan.BillingCycle, plan.BillingInterval)

	// Apply coupon if provided
	if req.CouponCode != "" {
		sub.CouponCode = req.CouponCode
		// TODO: Validate and apply coupon discount
		sub.DiscountPercent = decimal.NewFromFloat(10) // Example: 10% discount
	}

	if err := s.repo.CreateSubscription(ctx, sub); err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	// Record event
	event := &SubscriptionEvent{
		BaseEntity:     models.NewBaseEntity(),
		SubscriptionID: sub.ID,
		EventType:      "subscription.created",
		NewStatus:      string(sub.Status),
		Metadata:       req.Metadata,
	}
	_ = s.repo.RecordEvent(ctx, event)

	s.logger.Info("Subscription created",
		zap.String("subscription_id", sub.ID.String()),
		zap.String("customer_id", sub.CustomerID.String()),
		zap.String("plan_id", sub.PlanID.String()),
	)

	return sub, nil
}

// UpdateSubscription updates an existing subscription
func (s *Service) UpdateSubscription(ctx context.Context, id uuid.UUID, req *UpdateSubscriptionRequest) (*Subscription, error) {
	sub, err := s.repo.GetSubscription(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	oldStatus := sub.Status

	// Handle plan change
	if req.PlanID != nil && *req.PlanID != sub.PlanID {
		if err := s.handlePlanChange(ctx, sub, *req.PlanID, req.ProrationBehavior); err != nil {
			return nil, fmt.Errorf("failed to change plan: %w", err)
		}
	}

	// Handle quantity change
	if req.Quantity != nil && *req.Quantity != sub.Quantity {
		if err := s.handleQuantityChange(ctx, sub, *req.Quantity, req.ProrationBehavior); err != nil {
			return nil, fmt.Errorf("failed to change quantity: %w", err)
		}
	}

	// Handle cancellation
	if req.CancelAtPeriodEnd != nil {
		sub.CancelAtPeriodEnd = *req.CancelAtPeriodEnd
		if *req.CancelAtPeriodEnd {
			now := time.Now()
			sub.CanceledAt = &now
		}
	}

	// Update metadata
	if req.Metadata != nil {
		sub.Metadata = req.Metadata
	}

	sub.UpdatedAt = time.Now()

	if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		return nil, fmt.Errorf("failed to update subscription: %w", err)
	}

	// Record event
	oldStatusStr := string(oldStatus)
	event := &SubscriptionEvent{
		BaseEntity:     models.NewBaseEntity(),
		SubscriptionID: sub.ID,
		EventType:      "subscription.updated",
		OldStatus:      &oldStatusStr,
		NewStatus:      string(sub.Status),
	}
	_ = s.repo.RecordEvent(ctx, event)

	return sub, nil
}

// CancelSubscription cancels a subscription
func (s *Service) CancelSubscription(ctx context.Context, id uuid.UUID, immediately bool) (*Subscription, error) {
	sub, err := s.repo.GetSubscription(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	now := time.Now()
	sub.CanceledAt = &now

	if immediately {
		sub.Status = SubscriptionStatusCanceled
		sub.CurrentPeriodEnd = now
	} else {
		sub.CancelAtPeriodEnd = true
	}

	sub.UpdatedAt = now

	if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		return nil, fmt.Errorf("failed to cancel subscription: %w", err)
	}

	// Record event
	event := &SubscriptionEvent{
		BaseEntity:     models.NewBaseEntity(),
		SubscriptionID: sub.ID,
		EventType:      "subscription.canceled",
		NewStatus:      string(sub.Status),
	}
	_ = s.repo.RecordEvent(ctx, event)

	s.logger.Info("Subscription canceled",
		zap.String("subscription_id", sub.ID.String()),
		zap.Bool("immediately", immediately),
	)

	return sub, nil
}

// ProcessDueSubscriptions processes all subscriptions due for billing
func (s *Service) ProcessDueSubscriptions(ctx context.Context) error {
	dueDate := time.Now().Add(24 * time.Hour)
	subscriptions, err := s.repo.GetDueSubscriptions(ctx, dueDate)
	if err != nil {
		return fmt.Errorf("failed to get due subscriptions: %w", err)
	}

	s.logger.Info("Processing due subscriptions", zap.Int("count", len(subscriptions)))

	for _, sub := range subscriptions {
		if err := s.billSubscription(ctx, sub); err != nil {
			s.logger.Error("Failed to bill subscription",
				zap.String("subscription_id", sub.ID.String()),
				zap.Error(err),
			)
		}
	}

	return nil
}

// billSubscription bills a single subscription
func (s *Service) billSubscription(ctx context.Context, sub *Subscription) error {
	plan, err := s.repo.GetPlan(ctx, sub.PlanID)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}

	// Calculate invoice amount
	amount := plan.Amount.MultiplyFloat(float64(sub.Quantity))

	// Apply discount
	if sub.DiscountPercent.GreaterThan(decimal.Zero) {
		discount := amount.Multiply(sub.DiscountPercent.Div(decimal.NewFromInt(100)))
		amount, _ = amount.Subtract(discount)
	}

	// Create invoice
	invoice := &Invoice{
		BaseEntity:     models.NewBaseEntity(),
		SubscriptionID: sub.ID,
		CustomerID:     sub.CustomerID,
		InvoiceNumber:  s.generateInvoiceNumber(),
		Status:         models.PaymentStatusPending,
		Subtotal:       amount,
		Tax:            models.NewMoney(0, amount.Currency),
		Total:          amount,
		AmountDue:      amount,
		PeriodStart:    sub.CurrentPeriodStart,
		PeriodEnd:      sub.CurrentPeriodEnd,
		DueDate:        time.Now(),
	}

	if err := s.repo.CreateInvoice(ctx, invoice); err != nil {
		return fmt.Errorf("failed to create invoice: %w", err)
	}

	// Attempt payment
	paymentID, err := s.paymentSvc.ChargeCustomer(
		ctx,
		sub.CustomerID,
		amount,
		fmt.Sprintf("Subscription payment for %s", plan.Name),
	)

	if err != nil {
		return s.handlePaymentFailure(ctx, sub, invoice, err)
	}

	// Payment succeeded
	return s.handlePaymentSuccess(ctx, sub, invoice, paymentID)
}

// handlePaymentSuccess handles successful payment
func (s *Service) handlePaymentSuccess(ctx context.Context, sub *Subscription, invoice *Invoice, paymentID string) error {
	now := time.Now()
	invoice.Status = models.PaymentStatusSucceeded
	invoice.PaidAt = &now
	invoice.AmountPaid = invoice.Total
	invoice.AmountDue = models.NewMoney(0, invoice.Total.Currency)

	// Reset failed payment counter
	sub.FailedPaymentCount = 0
	sub.Status = SubscriptionStatusActive

	// Advance to next billing period
	plan, _ := s.repo.GetPlan(ctx, sub.PlanID)
	sub.CurrentPeriodStart = sub.CurrentPeriodEnd
	sub.CurrentPeriodEnd = s.calculatePeriodEnd(sub.CurrentPeriodStart, plan.BillingCycle, plan.BillingInterval)

	return s.repo.UpdateSubscription(ctx, sub)
}

// handlePaymentFailure handles failed payment with dunning
func (s *Service) handlePaymentFailure(ctx context.Context, sub *Subscription, invoice *Invoice, paymentErr error) error {
	invoice.Status = models.PaymentStatusFailed
	sub.FailedPaymentCount++
	now := time.Now()
	sub.LastPaymentAttempt = &now

	// Calculate next retry
	if sub.FailedPaymentCount < s.dunningConfig.MaxRetries {
		retryDays := s.dunningConfig.RetryIntervalDays[min(sub.FailedPaymentCount-1, len(s.dunningConfig.RetryIntervalDays)-1)]
		nextRetry := now.AddDate(0, 0, retryDays)
		sub.NextRetryAt = &nextRetry
		sub.Status = SubscriptionStatusPastDue
	} else {
		// Max retries reached, cancel subscription
		sub.Status = SubscriptionStatusCanceled
		canceledAt := now
		sub.CanceledAt = &canceledAt
	}

	s.logger.Warn("Payment failed for subscription",
		zap.String("subscription_id", sub.ID.String()),
		zap.Int("attempt", sub.FailedPaymentCount),
		zap.Error(paymentErr),
	)

	return s.repo.UpdateSubscription(ctx, sub)
}

// Helper functions

func (s *Service) calculatePeriodEnd(start time.Time, cycle BillingCycle, interval int) time.Time {
	if interval == 0 {
		interval = 1
	}

	switch cycle {
	case BillingCycleDaily:
		return start.AddDate(0, 0, interval)
	case BillingCycleWeekly:
		return start.AddDate(0, 0, interval*7)
	case BillingCycleMonthly:
		return start.AddDate(0, interval, 0)
	case BillingCycleYearly:
		return start.AddDate(interval, 0, 0)
	default:
		return start.AddDate(0, interval, 0)
	}
}

func (s *Service) generateInvoiceNumber() string {
	return fmt.Sprintf("INV-%d-%s", time.Now().Unix(), uuid.New().String()[:8])
}

func (s *Service) handlePlanChange(ctx context.Context, sub *Subscription, newPlanID uuid.UUID, behavior ProrationBehavior) error {
	// TODO: Implement proration logic
	sub.PlanID = newPlanID
	return nil
}

func (s *Service) handleQuantityChange(ctx context.Context, sub *Subscription, newQuantity int, behavior ProrationBehavior) error {
	// TODO: Implement proration logic
	sub.Quantity = newQuantity
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Shutdown gracefully shuts down the service
func (s *Service) Shutdown() {
	s.cron.Stop()
}
