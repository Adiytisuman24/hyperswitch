# 🚀 Quick Start Guide - Hyperswitch Go Extensions

## Prerequisites

✅ Go 1.21 or higher  
✅ PostgreSQL 14+ (optional, for persistence)  
✅ Redis 6+ (optional, for caching)  
✅ Git

---

## Installation

### 1. Clone & Setup

```bash
# Navigate to the project
cd c:\Users\suman\Downloads\hyperswitch-main\hyperswitch-go-extensions

# Download dependencies
go mod download
go mod tidy
```

### 2. Configure Environment

```bash
# Copy example env file
cp .env.example .env

# Edit .env with your settings (optional for quick start)
```

---

## Running the Application

### Option 1: Development Mode (Recommended for testing)

```bash
# Run directly with Go
go run cmd/server/main.go
```

Server starts at `http://localhost:8081`

### Option 2: Build & Run

```bash
# Using Makefile
make build
./bin/hyperswitch-go-ext

# Or manually
go build -o bin/hyperswitch-go-ext cmd/server/main.go
./bin/hyperswitch-go-ext
```

### Option 3: Using Make

```bash
# See all available commands
make help

# Run in development
make run

# Run tests
make test

# Build production binary
make build
```

---

## Testing the API

### 1. Health Check

```bash
curl http://localhost:8081/health
```

**Expected Response:**
```json
{"status": "healthy"}
```

### 2. Convert Currency

```bash
curl -X POST http://localhost:8081/api/v1/forex/convert \
  -H "Content-Type: application/json" \
  -d '{
    "amount": {
      "amount": 100,
      "currency": "USD"
    },
    "to": "EUR"
  }'
```

**Expected Response:**
```json
{
  "original": {"amount": 100, "currency": "USD"},
  "converted": {"amount": 85, "currency": "EUR"},
  "exchange_rate": 0.85,
  "fee": {"amount": 1.28, "currency": "EUR"},
  "total_converted": {"amount": 86.28, "currency": "EUR"}
}
```

### 3. Get Multi-Currency Pricing

```bash
curl http://localhost:8081/api/v1/demo/multi-currency-pricing
```

### 4. Get Checkout Content (Spanish)

```bash
curl http://localhost:8081/api/v1/locale/checkout/es-ES
```

### 5. Get Supported Wallets for APAC

```bash
curl "http://localhost:8081/api/v1/wallets/supported?region=APAC&currency=USD"
```

**Expected Response:**
```json
{
  "wallets": [
    "apple_pay",
    "google_pay",
    "wechat_pay",
    "alipay",
    "paynow",
    "grabpay",
    "touch_n_go",
    "kakao_pay"
  ]
}
```

### 6. Process Wallet Payment (Mock)

```bash
curl -X POST http://localhost:8081/api/v1/wallets/pay \
  -H "Content-Type: application/json" \
  -d '{
    "wallet_type": "apple_pay",
    "amount": {"amount": 49.99, "currency": "USD"},
    "customer_id": "cust_123abc",
    "return_url": "https://example.com/success",
    "apple_pay_token": {
      "payment_data": "test_token",
      "transaction_id": "txn_123",
      "payment_method": "card",
      "display_name": "Visa 1234",
      "network": "Visa"
    }
  }'
```

---

## Testing with Postman

### Import Collection

1. Open Postman
2. Import → Upload Files
3. Select: `docs/postman_collection.json` (if created)
4. All endpoints ready to test!

---

## Running Tests

### Unit Tests

```bash
# Run all tests
make test

# Or with Go
go test ./...

# Verbose output
go test -v ./...

# Specific package
go test -v ./internal/subscriptions/
```

### Test Coverage

```bash
# Generate coverage report
make test-coverage

# Opens coverage.html in browser
```

---

## Example Use Cases

### Use Case 1: Create a Subscription

```go
import (
    "github.com/juspay/hyperswitch-go-extensions/internal/subscriptions"
)

// Create subscription service
svc := subscriptions.NewService(repo, paymentSvc, logger, dunningConfig)

// Create subscription request
req := &subscriptions.CreateSubscriptionRequest{
    CustomerID: uuid.MustParse("customer-id"),
    PlanID:     uuid.MustParse("plan-id"),
    Quantity:   1,
    TrialDays:  intPtr(14),
}

// Create subscription
sub, err := svc.CreateSubscription(ctx, req)
```

### Use Case 2: Split Marketplace Payment

```go
import (
    "github.com/juspay/hyperswitch-go-extensions/internal/marketplace"
)

// Split payment request
req := &marketplace.SplitPaymentRequest{
    PaymentID: uuid.New(),
    TotalAmount: models.NewMoney(100, models.USD),
    Config: marketplace.SplitConfig{
        PlatformFee:     models.NewMoney(10, models.USD),
        PlatformFeeType: marketplace.FeeTypePercentage,
        VendorSplits: []marketplace.VendorSplit{
            {
                VendorID: vendor1ID,
                Amount:   models.NewMoney(70, models.USD),
            },
            {
                VendorID: vendor2ID,
                Amount:   models.NewMoney(20, models.USD),
            },
        },
        EscrowEnabled:     true,
        EscrowReleaseDays: 7,
    },
}

split, err := svc.SplitPayment(ctx, req)
```

### Use Case 3: Multi-Currency Conversion

```go
import (
    "github.com/juspay/hyperswitch-go-extensions/internal/forex"
)

// Convert USD to multiple currencies
baseAmount := models.NewMoney(100, models.USD)
targetCurrencies := []models.Currency{
    models.EUR, models.GBP, models.JPY,
}

pricing, err := forexSvc.GetMultiCurrencyPricing(
    ctx,
    baseAmount,
    targetCurrencies,
)
```

---

## Common Issues & Solutions

### Issue: Port 8081 already in use

**Solution:**
```bash
# Change port in main.go or use environment variable
export PORT=8082
go run cmd/server/main.go
```

### Issue: Module errors

**Solution:**
```bash
# Reinitialize modules
go mod tidy
go clean -modcache
go mod download
```

### Issue: Test failures

**Solution:**
```bash
# Ensure you're in the project root
cd c:\Users\suman\Downloads\hyperswitch-main\hyperswitch-go-extensions

# Run specific test
go test -v ./internal/subscriptions/ -run TestCreateSubscription
```

---

## Development Workflow

### 1. Make Changes
Edit files in `internal/`, `pkg/`, or `cmd/`

### 2. Format Code
```bash
make fmt
```

### 3. Run Tests
```bash
make test
```

### 4. Build
```bash
make build
```

### 5. Test Locally
```bash
./bin/hyperswitch-go-ext
```

---

## Production Deployment

### Build for Production

```bash
# Build optimized binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s" \
  -o bin/hyperswitch-go-ext-linux \
  cmd/server/main.go
```

### Docker Deployment

```bash
# Build image
docker build -t hyperswitch-go-ext:v1.0.0 .

# Run container
docker run -d \
  -p 8081:8081 \
  --env-file .env \
  --name hyperswitch-ext \
  hyperswitch-go-ext:v1.0.0
```

### Environment Variables for Production

```bash
# Required
DATABASE_URL=postgresql://...
REDIS_URL=redis://...
FX_API_KEY=your_key

# Recommended
ENV=production
LOG_LEVEL=warn
LOG_FORMAT=json
```

---

## Monitoring & Observability

### Health Checks

```bash
# Application health
curl http://localhost:8081/health

# Add to monitoring tools (Prometheus, Datadog, etc.)
```

### Logs

```bash
# View logs (structured JSON in production)
tail -f /var/log/hyperswitch-ext.log

# Or with Docker
docker logs -f hyperswitch-ext
```

### Metrics (Planned)

- Request count and latency
- Active subscriptions
- Conversion rates
- Payout volumes

---

## Next Steps

1. ✅ **Explore API Documentation**: `docs/API.md`
2. ✅ **Review Architecture**: `docs/ARCHITECTURE.md`
3. ✅ **Read Implementation Summary**: `IMPLEMENTATION_SUMMARY.md`
4. 🔨 **Integrate with Hyperswitch core**
5. 🔨 **Add database persistence**
6. 🔨 **Set up webhook handling**
7. 🔨 **Deploy to staging/production**

---

## Getting Help

📖 **Documentation**: See `docs/` folder  
💬 **Issues**: GitHub Issues  
📧 **Email**: dev@hyperswitch.io  
💬 **Slack**: hyperswitch-io.slack.com

---

## Quick Reference

```bash
# Development
make run          # Start server
make test         # Run tests
make fmt          # Format code

# Production
make build        # Build binary
make docker-build # Build Docker image

# Utilities
make clean        # Clean artifacts
make help         # Show all commands
```

---

🎉 **You're all set!** Start building advanced payment features with Hyperswitch Go Extensions.
