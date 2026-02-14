package subscriptions_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/juspay/hyperswitch-go-extensions/internal/subscriptions"
	"github.com/juspay/hyperswitch-go-extensions/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// Mock repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateSubscription(ctx context.Context, sub *subscriptions.Subscription) error {
	args := m.Called(ctx, sub)
	return args.Error(0)
}

func (m *MockRepository) GetSubscription(ctx context.Context, id uuid.UUID) (*subscriptions.Subscription, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*subscriptions.Subscription), args.Error(1)
}

func (m *MockRepository) UpdateSubscription(ctx context.Context, sub *subscriptions.Subscription) error {
	args := m.Called(ctx, sub)
	return args.Error(0)
}

func (m *MockRepository) ListSubscriptions(ctx context.Context, customerID uuid.UUID, status *subscriptions.SubscriptionStatus) ([]*subscriptions.Subscription, error) {
	args := m.Called(ctx, customerID, status)
	return args.Get(0).([]*subscriptions.Subscription), args.Error(1)
}

func (m *MockRepository) GetPlan(ctx context.Context, id uuid.UUID) (*subscriptions.Plan, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*subscriptions.Plan), args.Error(1)
}

func (m *MockRepository) CreateInvoice(ctx context.Context, invoice *subscriptions.Invoice) error {
	args := m.Called(ctx, invoice)
	return args.Error(0)
}

func (m *MockRepository) GetDueSubscriptions(ctx context.Context, dueDate time.Time) ([]*subscriptions.Subscription, error) {
	args := m.Called(ctx, dueDate)
	return args.Get(0).([]*subscriptions.Subscription), args.Error(1)
}

func (m *MockRepository) RecordEvent(ctx context.Context, event *subscriptions.SubscriptionEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// Mock payment service
type MockPaymentService struct {
	mock.Mock
}

func (m *MockPaymentService) ChargeCustomer(ctx context.Context, customerID uuid.UUID, amount models.Money, description string) (string, error) {
	args := m.Called(ctx, customerID, amount, description)
	return args.String(0), args.Error(1)
}

func TestCreateSubscription(t *testing.T) {
	// Setup
	mockRepo := new(MockRepository)
	mockPayment := new(MockPaymentService)
	logger, _ := zap.NewDevelopment()

	dunningConfig := subscriptions.DunningConfig{
		MaxRetries:        3,
		RetryIntervalDays: []int{1, 3, 7},
		GracePeriodDays:   5,
		CancelAfterDays:   30,
	}

	svc := subscriptions.NewService(mockRepo, mockPayment, logger, dunningConfig)

	// Test data
	customerID := uuid.New()
	planID := uuid.New()

	plan := &subscriptions.Plan{
		BaseEntity:      models.NewBaseEntity(),
		Name:            "Pro Plan",
		Amount:          models.NewMoney(29.99, models.USD),
		BillingCycle:    subscriptions.BillingCycleMonthly,
		BillingInterval: 1,
		TrialDays:       14,
	}
	plan.ID = planID

	// Setup mocks
	mockRepo.On("GetPlan", mock.Anything, planID).Return(plan, nil)
	mockRepo.On("CreateSubscription", mock.Anything, mock.AnythingOfType("*subscriptions.Subscription")).Return(nil)
	mockRepo.On("RecordEvent", mock.Anything, mock.AnythingOfType("*subscriptions.SubscriptionEvent")).Return(nil)

	// Execute
	req := &subscriptions.CreateSubscriptionRequest{
		CustomerID: customerID,
		PlanID:     planID,
		Quantity:   1,
	}

	sub, err := svc.CreateSubscription(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, sub)
	assert.Equal(t, customerID, sub.CustomerID)
	assert.Equal(t, planID, sub.PlanID)
	assert.Equal(t, subscriptions.SubscriptionStatusActive, sub.Status)
	assert.Equal(t, 1, sub.Quantity)

	mockRepo.AssertExpectations(t)
}

func TestCreateSubscriptionWithTrial(t *testing.T) {
	mockRepo := new(MockRepository)
	mockPayment := new(MockPaymentService)
	logger, _ := zap.NewDevelopment()

	dunningConfig := subscriptions.DunningConfig{
		MaxRetries:        3,
		RetryIntervalDays: []int{1, 3, 7},
	}

	svc := subscriptions.NewService(mockRepo, mockPayment, logger, dunningConfig)

	customerID := uuid.New()
	planID := uuid.New()
	trialDays := 14

	plan := &subscriptions.Plan{
		BaseEntity:      models.NewBaseEntity(),
		Name:            "Pro Plan",
		Amount:          models.NewMoney(29.99, models.USD),
		BillingCycle:    subscriptions.BillingCycleMonthly,
		BillingInterval: 1,
	}
	plan.ID = planID

	mockRepo.On("GetPlan", mock.Anything, planID).Return(plan, nil)
	mockRepo.On("CreateSubscription", mock.Anything, mock.AnythingOfType("*subscriptions.Subscription")).Return(nil)
	mockRepo.On("RecordEvent", mock.Anything, mock.AnythingOfType("*subscriptions.SubscriptionEvent")).Return(nil)

	req := &subscriptions.CreateSubscriptionRequest{
		CustomerID: customerID,
		PlanID:     planID,
		Quantity:   1,
		TrialDays:  &trialDays,
	}

	sub, err := svc.CreateSubscription(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, sub)
	assert.Equal(t, subscriptions.SubscriptionStatusTrialing, sub.Status)
	assert.NotNil(t, sub.TrialStart)
	assert.NotNil(t, sub.TrialEnd)

	expectedTrialEnd := sub.TrialStart.AddDate(0, 0, trialDays)
	assert.WithinDuration(t, expectedTrialEnd, *sub.TrialEnd, time.Second)
}

func TestCancelSubscription(t *testing.T) {
	mockRepo := new(MockRepository)
	mockPayment := new(MockPaymentService)
	logger, _ := zap.NewDevelopment()

	dunningConfig := subscriptions.DunningConfig{}
	svc := subscriptions.NewService(mockRepo, mockPayment, logger, dunningConfig)

	subID := uuid.New()
	sub := &subscriptions.Subscription{
		BaseEntity: models.NewBaseEntity(),
		Status:     subscriptions.SubscriptionStatusActive,
	}
	sub.ID = subID

	mockRepo.On("GetSubscription", mock.Anything, subID).Return(sub, nil)
	mockRepo.On("UpdateSubscription", mock.Anything, mock.AnythingOfType("*subscriptions.Subscription")).Return(nil)
	mockRepo.On("RecordEvent", mock.Anything, mock.AnythingOfType("*subscriptions.SubscriptionEvent")).Return(nil)

	// Cancel immediately
	result, err := svc.CancelSubscription(context.Background(), subID, true)

	assert.NoError(t, err)
	assert.Equal(t, subscriptions.SubscriptionStatusCanceled, result.Status)
	assert.NotNil(t, result.CanceledAt)

	mockRepo.AssertExpectations(t)
}
