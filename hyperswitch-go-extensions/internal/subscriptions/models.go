package subscriptions

import (
	"time"

	"github.com/google/uuid"
	"github.com/juspay/hyperswitch-go-extensions/pkg/models"
	"github.com/shopspring/decimal"
)

// BillingCycle represents the frequency of billing
type BillingCycle string

const (
	BillingCycleDaily   BillingCycle = "daily"
	BillingCycleWeekly  BillingCycle = "weekly"
	BillingCycleMonthly BillingCycle = "monthly"
	BillingCycleYearly  BillingCycle = "yearly"
	BillingCycleCustom  BillingCycle = "custom"
)

// SubscriptionStatus represents the status of a subscription
type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusTrialing SubscriptionStatus = "trialing"
	SubscriptionStatusPastDue  SubscriptionStatus = "past_due"
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
	SubscriptionStatusPaused   SubscriptionStatus = "paused"
	SubscriptionStatusExpired  SubscriptionStatus = "expired"
)

// ProrationBehavior defines how to handle proration
type ProrationBehavior string

const (
	ProrationCreateProrations ProrationBehavior = "create_prorations"
	ProrationNone             ProrationBehavior = "none"
	ProrationAlwaysInvoice    ProrationBehavior = "always_invoice"
)

// Plan represents a subscription plan
type Plan struct {
	models.BaseEntity
	MerchantID      uuid.UUID       `json:"merchant_id" db:"merchant_id"`
	Name            string          `json:"name" db:"name"`
	Description     string          `json:"description" db:"description"`
	Amount          models.Money    `json:"amount"`
	BillingCycle    BillingCycle    `json:"billing_cycle" db:"billing_cycle"`
	BillingInterval int             `json:"billing_interval" db:"billing_interval"` // e.g., 3 for "every 3 months"
	TrialDays       int             `json:"trial_days" db:"trial_days"`
	Features        []string        `json:"features" db:"features"`
	Metadata        models.Metadata `json:"metadata,omitempty" db:"metadata"`
	Active          bool            `json:"active" db:"active"`
}

// Subscription represents a customer subscription
type Subscription struct {
	models.BaseEntity
	CustomerID         uuid.UUID          `json:"customer_id" db:"customer_id"`
	PlanID             uuid.UUID          `json:"plan_id" db:"plan_id"`
	Status             SubscriptionStatus `json:"status" db:"status"`
	CurrentPeriodStart time.Time          `json:"current_period_start" db:"current_period_start"`
	CurrentPeriodEnd   time.Time          `json:"current_period_end" db:"current_period_end"`
	TrialStart         *time.Time         `json:"trial_start,omitempty" db:"trial_start"`
	TrialEnd           *time.Time         `json:"trial_end,omitempty" db:"trial_end"`
	CanceledAt         *time.Time         `json:"canceled_at,omitempty" db:"canceled_at"`
	CancelAtPeriodEnd  bool               `json:"cancel_at_period_end" db:"cancel_at_period_end"`
	Quantity           int                `json:"quantity" db:"quantity"`
	Metadata           models.Metadata    `json:"metadata,omitempty" db:"metadata"`

	// Dunning management
	FailedPaymentCount int        `json:"failed_payment_count" db:"failed_payment_count"`
	LastPaymentAttempt *time.Time `json:"last_payment_attempt,omitempty" db:"last_payment_attempt"`
	NextRetryAt        *time.Time `json:"next_retry_at,omitempty" db:"next_retry_at"`

	// Discount/coupon
	DiscountPercent decimal.Decimal `json:"discount_percent" db:"discount_percent"`
	CouponCode      string          `json:"coupon_code,omitempty" db:"coupon_code"`
}

// Invoice represents a billing invoice
type Invoice struct {
	models.BaseEntity
	SubscriptionID uuid.UUID            `json:"subscription_id" db:"subscription_id"`
	CustomerID     uuid.UUID            `json:"customer_id" db:"customer_id"`
	InvoiceNumber  string               `json:"invoice_number" db:"invoice_number"`
	Status         models.PaymentStatus `json:"status" db:"status"`
	Subtotal       models.Money         `json:"subtotal"`
	Tax            models.Money         `json:"tax"`
	Total          models.Money         `json:"total"`
	AmountPaid     models.Money         `json:"amount_paid"`
	AmountDue      models.Money         `json:"amount_due"`
	PeriodStart    time.Time            `json:"period_start" db:"period_start"`
	PeriodEnd      time.Time            `json:"period_end" db:"period_end"`
	DueDate        time.Time            `json:"due_date" db:"due_date"`
	PaidAt         *time.Time           `json:"paid_at,omitempty" db:"paid_at"`
	LineItems      []InvoiceLineItem    `json:"line_items"`
	Metadata       models.Metadata      `json:"metadata,omitempty" db:"metadata"`
}

// InvoiceLineItem represents a line item in an invoice
type InvoiceLineItem struct {
	ID          uuid.UUID    `json:"id"`
	InvoiceID   uuid.UUID    `json:"invoice_id"`
	Description string       `json:"description"`
	Quantity    int          `json:"quantity"`
	UnitAmount  models.Money `json:"unit_amount"`
	Amount      models.Money `json:"amount"`
	IsPr

	oration     bool            `json:"is_proration"`
	PeriodStart time.Time       `json:"period_start"`
	PeriodEnd   time.Time       `json:"period_end"`
	Metadata    models.Metadata `json:"metadata,omitempty"`
}

// SubscriptionEvent represents events in subscription lifecycle
type SubscriptionEvent struct {
	models.BaseEntity
	SubscriptionID uuid.UUID       `json:"subscription_id" db:"subscription_id"`
	EventType      string          `json:"event_type" db:"event_type"`
	OldStatus      *string         `json:"old_status,omitempty" db:"old_status"`
	NewStatus      string          `json:"new_status" db:"new_status"`
	Metadata       models.Metadata `json:"metadata,omitempty" db:"metadata"`
}

// DunningConfig represents dunning management configuration
type DunningConfig struct {
	MaxRetries         int   `json:"max_retries"`
	RetryIntervalDays  []int `json:"retry_interval_days"` // e.g., [1, 3, 7, 14]
	GracePeriodDays    int   `json:"grace_period_days"`
	CancelAfterDays    int   `json:"cancel_after_days"`
	EmailNotifications bool  `json:"email_notifications"`
}

// CreateSubscriptionRequest represents the request to create a subscription
type CreateSubscriptionRequest struct {
	CustomerID        uuid.UUID         `json:"customer_id" binding:"required"`
	PlanID            uuid.UUID         `json:"plan_id" binding:"required"`
	Quantity          int               `json:"quantity"`
	TrialDays         *int              `json:"trial_days,omitempty"`
	CouponCode        string            `json:"coupon_code,omitempty"`
	Metadata          models.Metadata   `json:"metadata,omitempty"`
	StartDate         *time.Time        `json:"start_date,omitempty"`
	ProrationBehavior ProrationBehavior `json:"proration_behavior"`
}

// UpdateSubscriptionRequest represents the request to update a subscription
type UpdateSubscriptionRequest struct {
	PlanID            *uuid.UUID        `json:"plan_id,omitempty"`
	Quantity          *int              `json:"quantity,omitempty"`
	CouponCode        *string           `json:"coupon_code,omitempty"`
	Metadata          models.Metadata   `json:"metadata,omitempty"`
	CancelAtPeriodEnd *bool             `json:"cancel_at_period_end,omitempty"`
	ProrationBehavior ProrationBehavior `json:"proration_behavior"`
}

// SubscriptionMetrics represents metrics for a subscription
type SubscriptionMetrics struct {
	TotalSubscriptions      int             `json:"total_subscriptions"`
	ActiveSubscriptions     int             `json:"active_subscriptions"`
	TrialingSubscriptions   int             `json:"trialing_subscriptions"`
	ChurnedSubscriptions    int             `json:"churned_subscriptions"`
	MonthlyRecurringRevenue models.Money    `json:"monthly_recurring_revenue"`
	AverageRevenuePerUser   models.Money    `json:"average_revenue_per_user"`
	ChurnRate               decimal.Decimal `json:"churn_rate"`
}
