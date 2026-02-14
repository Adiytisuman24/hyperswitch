# Hyperswitch Go Extensions

Advanced payment features implemented in Go to extend Hyperswitch functionality.

## Features

### 1. Recurring Billing & Subscriptions

- Flexible billing cycles (daily, weekly, monthly, yearly)
- Trial periods and grace periods
- Dunning management for failed payments
- Subscription lifecycle management
- Proration support

### 2. Multi-Currency Conversion

- Real-time exchange rate fetching
- Settlement currency conversion
- Multi-currency pricing
- FX rate caching and fallback
- Currency-specific rounding rules

### 3. Multi-Locale Checkout

- i18n support for 50+ locales
- RTL language support
- Locale-specific formatting (dates, numbers, currency)
- Dynamic payment method ordering by region
- Localized error messages

### 4. Marketplace Flows

- Platform fee configuration
- Multi-vendor split payouts
- Escrow support
- Delayed transfer scheduling
- Commission calculations

### 5. Digital Wallets

- Apple Pay
- Google Pay
- PayPal
- WeChat Pay
- Alipay
- Samsung Pay
- And more...

## Architecture

```
hyperswitch-go-extensions/
├── cmd/
│   └── server/           # Main application entry
├── internal/
│   ├── subscriptions/    # Recurring billing
│   ├── forex/            # Multi-currency
│   ├── locale/           # i18n & localization
│   ├── marketplace/      # Split payouts
│   └── wallets/          # Digital wallets
├── pkg/
│   ├── models/           # Shared data models
│   └── utils/            # Common utilities
├── api/
│   └── proto/            # gRPC definitions
└── config/               # Configuration files
```

## Quick Start

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build -o bin/hyperswitch-go-ext cmd/server/main.go

# Run
./bin/hyperswitch-go-ext
```

## Environment Variables

```env
DATABASE_URL=postgresql://user:pass@localhost:5432/hyperswitch
REDIS_URL=redis://localhost:6379
FX_API_KEY=your_forex_api_key
PORT=8081
```

## API Documentation

See [API.md](./docs/API.md) for detailed API documentation.

## Contributing

Please read [CONTRIBUTING.md](./CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## License

Apache 2.0
