# Hyperswitch Go Extensions - Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                       CLIENT APPLICATIONS                             │
│  (Web, Mobile, Backend Services)                                    │
└────────────────────────┬────────────────────────────────────────────┘
                         │
                         │ HTTPS / REST API
                         ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    API GATEWAY / ROUTER (Gin)                        │
│                    Port: 8081                                        │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  Authentication & Rate Limiting Middleware                   │  │
│  └─────────────────────────────────────────────────────────────┘  │
└────────┬──────────┬──────────┬──────────┬──────────┬──────────────┘
         │          │          │          │          │
         ▼          ▼          ▼          ▼          ▼
┌────────────────────────────────────────────────────────────────────┐
│                      SERVICE LAYER                                  │
├─────────────┬─────────────┬─────────────┬─────────────┬───────────┤
│             │             │             │             │           │
│ Subscription│   Forex     │ Marketplace │   Locale    │  Wallets  │
│  Service    │  Service    │  Service    │  Service    │  Service  │
│             │             │             │             │           │
│ • Plans     │ • Convert   │ • Split Pay │ • i18n      │ • Apple   │
│ • Billing   │ • Rates     │ • Escrow    │ • L10n      │ • Google  │
│ • Invoices  │ • Cache     │ • Payouts   │ • RTL       │ • PayPal  │
│ • Dunning   │ • Fallback  │ • Platform  │ • Formats   │ • WeChat  │
│             │             │   Fees      │             │ • 15+ more│
└─────────────┴─────────────┴─────────────┴─────────────┴───────────┘
         │          │          │          │          │
         ▼          ▼          ▼          ▼          ▼
┌────────────────────────────────────────────────────────────────────┐
│                    REPOSITORY LAYER                                 │
│  ┌────────────┬────────────┬────────────┬────────────┐            │
│  │Subscription│ Marketplace│   Vendor   │   Invoice  │            │
│  │    Repo    │    Repo    │    Repo    │    Repo    │            │
│  └────────────┴────────────┴────────────┴────────────┘            │
└────────┬──────────┬──────────┬──────────┬──────────┬──────────────┘
         │          │          │          │          │
         ▼          ▼          ▼          ▼          ▼
┌────────────────────────────────────────────────────────────────────┐
│                    DATA STORAGE LAYER                               │
├─────────────────┬──────────────────┬──────────────────────────────┤
│   PostgreSQL    │      Redis       │      External APIs           │
│                 │                  │                              │
│ • Subscriptions │ • FX Rate Cache  │ • OpenExchangeRates         │
│ • Invoices      │ • Session Data   │ • Stripe                    │
│ • Vendors       │ • Rate Limiting  │ • PayPal                    │
│ • Payouts       │                  │ • Payment Processors        │
│ • Customers     │                  │                              │
└─────────────────┴──────────────────┴──────────────────────────────┘


┌─────────────────────────────────────────────────────────────────────┐
│                    BACKGROUND JOBS (Cron)                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────────────┐  ┌──────────────────────┐               │
│  │  Daily Billing Job   │  │  Payout Processing   │               │
│  │  ├─ Due Subs Check   │  │  ├─ Scheduled Check  │               │
│  │  ├─ Invoice Create   │  │  ├─ Batch Process    │               │
│  │  ├─ Charge Customer  │  │  ├─ Notify Vendors   │               │
│  │  └─ Dunning Handle   │  │  └─ Update Status    │               │
│  └──────────────────────┘  └───────────────────────┘               │
│                                                                      │
│  ┌──────────────────────┐  ┌──────────────────────┐               │
│  │  FX Rate Refresh     │  │  Escrow Release      │               │
│  │  ├─ Fetch Rates      │  │  ├─ Check Release    │               │
│  │  ├─ Update Cache     │  │  ├─ Schedule Payout  │               │
│  │  └─ Log Changes      │  │  └─ Notify Platform  │               │
│  └──────────────────────┘  └──────────────────────┘               │
└─────────────────────────────────────────────────────────────────────┘


┌─────────────────────────────────────────────────────────────────────┐
│                       EVENT/WEBHOOK SYSTEM                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Events:                          Webhooks:                         │
│  • subscription.created       →   POST merchant_webhook_url         │
│  • subscription.canceled      →   Signature: HMAC-SHA256           │
│  • payment.succeeded          →   Retry: 3 attempts                │
│  • invoice.paid               →   Timeout: 30s                     │
│  • payout.completed           →                                     │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘


DATA FLOW EXAMPLE - Subscription Creation:

Client → API Gateway → Subscription Service → Repository → PostgreSQL
   ↓            ↓              ↓                  ↓             ↓
   1         Validate      Create Plan       Save Sub       Return
             Auth Key      Calculate          Record         Success
                          NextBilling        Event

                                    ↓
                               Cron Job (Daily)
                                    ↓
                            Check Due Subscriptions
                                    ↓
                           Create Invoice + Charge
                                    ↓
                        Success? → Active : Past Due (Dunning)


FOREX CONVERSION FLOW:

Client → Convert API → Forex Service → Cache Check
                            ↓              ↓
                         Found?       Return Rate
                            ↓              
                         Not Found
                            ↓
                    OpenExchangeRates API
                            ↓
                      Cache + Return


MARKETPLACE SPLIT FLOW:

Payment → Split API → Marketplace Service → Calculate Splits
                            ↓                      ↓
                      Platform Fee          Vendor Shares
                            ↓                      ↓
                      Escrow Enabled?      Store Split Record
                            ↓                      
                        Yes / No
                            ↓
                Release After N Days / Immediate
                            ↓
                    Schedule Vendor Payouts
                            ↓
                    Process via Payout Provider


SECURITY LAYERS:

┌─────────────────────┐
│  API Key Auth       │  ← Bearer Token
├─────────────────────┤
│  Rate Limiting      │  ← 100 req/min
├─────────────────────┤
│  Input Validation   │  ← Strict types
├─────────────────────┤
│  SQL Injection Prev │  ← Prepared statements
├─────────────────────┤
│  HTTPS Only         │  ← TLS 1.2+
└─────────────────────┘
```

## Key Design Patterns Used

1. **Repository Pattern** - Data access abstraction
2. **Service Layer** - Business logic separation
3. **Provider Pattern** - External service abstraction
4. **Dependency Injection** - Loose coupling
5. **Factory Pattern** - Object creation
6. **Observer Pattern** - Event handling
7. **Strategy Pattern** - Payment method selection

## Scalability Considerations

- **Horizontal Scaling**: Stateless services allow multiple instances
- **Database Connection Pooling**: Efficient resource usage
- **Caching Layer**: Redis for frequently accessed data
- **Async Processing**: Background jobs for heavy operations
- **Rate Limiting**: Prevents abuse and ensures fair usage

## High Availability Features

- **Graceful Shutdown**: Proper cleanup on termination
- **Health Checks**: `/health` endpoint for monitoring
- **Circuit Breaker**: Failure isolation (planned)
- **Retry Logic**: Dunning for failed payments
- **Fallback Rates**: Forex continues during API outage
