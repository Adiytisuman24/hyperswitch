# Hyperswitch Go Extensions - API Documentation

## Base URL
```
http://localhost:8081/api/v1
```

## Authentication
All endpoints require API key authentication via header:
```
Authorization: Bearer <your-api-key>
```

---

## 1. Forex / Multi-Currency

### Convert Currency
Convert amount from one currency to another with FX fees.

**Endpoint:** `POST /forex/convert`

**Request:**
```json
{
  "amount": {
    "amount": 100.00,
    "currency": "USD"
  },
  "to": "EUR"
}
```

**Response:**
```json
{
  "original": {
    "amount": 100.00,
    "currency": "USD"
  },
  "converted": {
    "amount": 85.00,
    "currency": "EUR"
  },
  "exchange_rate": 0.85,
  "fee": {
    "amount": 1.28,
    "currency": "EUR"
  },
  "total_converted": {
    "amount": 86.28,
    "currency": "EUR"
  },
  "timestamp": "2026-02-14T18:48:39Z"
}
```

### Get Supported Currencies
**Endpoint:** `GET /forex/supported-currencies`

**Response:**
```json
{
  "currencies": ["USD", "EUR", "GBP", "JPY", "INR", ...]
}
```

### Multi-Currency Pricing
Get pricing in multiple currencies simultaneously.

**Endpoint:** `GET /demo/multi-currency-pricing`

**Response:**
```json
{
  "USD": {"amount": 100.00, "currency": "USD"},
  "EUR": {"amount": 85.00, "currency": "EUR"},
  "GBP": {"amount": 73.00, "currency": "GBP"},
  ...
}
```

---

## 2. Subscriptions / Recurring Billing

### Create Subscription
**Endpoint:** `POST /subscriptions`

**Request:**
```json
{
  "customer_id": "cust_123abc",
  "plan_id": "plan_456def",
  "quantity": 1,
  "trial_days": 14,
  "coupon_code": "SAVE20",
  "metadata": {
    "source": "web"
  }
}
```

**Response:**
```json
{
  "id": "sub_789ghi",
  "customer_id": "cust_123abc",
  "plan_id": "plan_456def",
  "status": "trialing",
  "current_period_start": "2026-02-14T18:48:39Z",
  "current_period_end": "2026-03-14T18:48:39Z",
  "trial_start": "2026-02-14T18:48:39Z",
  "trial_end": "2026-02-28T18:48:39Z",
  "quantity": 1,
  "discount_percent": 20
}
```

### Update Subscription
**Endpoint:** `PATCH /subscriptions/:id`

**Request:**
```json
{
  "plan_id": "plan_new123",
  "quantity": 2,
  "proration_behavior": "create_prorations"
}
```

### Cancel Subscription
**Endpoint:** `DELETE /subscriptions/:id`

**Query Parameters:**
- `immediately` (boolean): Cancel immediately or at period end

**Response:**
```json
{
  "id": "sub_789ghi",
  "status": "canceled",
  "canceled_at": "2026-02-14T18:48:39Z"
}
```

### List Invoices
**Endpoint:** `GET /subscriptions/:id/invoices`

**Response:**
```json
{
  "invoices": [
    {
      "id": "inv_001",
      "invoice_number": "INV-1707936519-a1b2c3d4",
      "status": "paid",
      "total": {"amount": 29.99, "currency": "USD"},
      "paid_at": "2026-02-14T18:48:39Z"
    }
  ]
}
```

---

## 3. Marketplace / Split Payouts

### Split Payment
Split a payment among multiple vendors with platform fees.

**Endpoint:** `POST /marketplace/split-payment`

**Request:**
```json
{
  "payment_id": "pay_123",
  "total_amount": {
    "amount": 100.00,
    "currency": "USD"
  },
  "config": {
    "platform_fee": {
      "amount": 10.00,
      "currency": "USD"
    },
    "platform_fee_type": "percentage",
    "vendor_splits": [
      {
        "vendor_id": "vendor_001",
        "amount": {"amount": 70.00, "currency": "USD"},
        "description": "Product sale"
      },
      {
        "vendor_id": "vendor_002",
        "amount": {"amount": 20.00, "currency": "USD"},
        "description": "Shipping fee"
      }
    ],
    "escrow_enabled": true,
    "escrow_release_days": 7
  }
}
```

**Response:**
```json
{
  "id": "split_789",
  "payment_id": "pay_123",
  "total_amount": {"amount": 100.00, "currency": "USD"},
  "platform_fee": {"amount": 10.00, "currency": "USD"},
  "vendor_payouts": [
    {
      "vendor_id": "vendor_001",
      "amount": {"amount": 70.00, "currency": "USD"},
      "commission": {"amount": 7.00, "currency": "USD"},
      "net_amount": {"amount": 63.00, "currency": "USD"},
      "status": "pending"
    }
  ],
  "escrow_until": "2026-02-21T18:48:39Z",
  "released": false
}
```

### Release Escrow
**Endpoint:** `POST /marketplace/split-payment/:id/release`

### Process Scheduled Payouts
**Endpoint:** `POST /marketplace/payouts/process` (Admin only)

---

## 4. Locale / Multi-Locale Checkout

### Get Supported Locales
**Endpoint:** `GET /locale/supported`

**Response:**
```json
{
  "locales": [
    "en-US", "es-ES", "fr-FR", "de-DE", "ja-JP", 
    "zh-CN", "ar-SA", "hi-IN", "pt-BR", ...
  ]
}
```

### Get Checkout Content
Get localized checkout strings and configuration.

**Endpoint:** `GET /locale/checkout/:locale`

**Example:** `GET /locale/checkout/es-ES`

**Response:**
```json
{
  "locale": "es-ES",
  "title": "Pagar",
  "pay_button_text": "Pagar ahora",
  "cancel_button_text": "Cancelar",
  "security_note": "Tu información de pago está cifrada y segura",
  "terms_and_conditions": "Al continuar, aceptas nuestros Términos y Condiciones",
  "error_messages": {
    "error.invalid_card": "Número de tarjeta inválido",
    "error.card_declined": "Tarjeta rechazada..."
  },
  "direction": "ltr"
}
```

### Get Preferred Payment Methods
Get payment methods ordered by regional preference.

**Endpoint:** `GET /locale/payment-methods/:locale`

**Example:** `GET /locale/payment-methods/zh-CN`

**Response:**
```json
{
  "payment_methods": [
    "card",
    "alipay",
    "wechat_pay",
    "paynow"
  ]
}
```

---

## 5. Digital Wallets

### Process Wallet Payment
**Endpoint:** `POST /wallets/pay`

**Request (Apple Pay):**
```json
{
  "wallet_type": "apple_pay",
  "amount": {
    "amount": 49.99,
    "currency": "USD"
  },
  "customer_id": "cust_123",
  "return_url": "https://example.com/success",
  "cancel_url": "https://example.com/cancel",
  "apple_pay_token": {
    "payment_data": "encrypted_token_data",
    "transaction_id": "txn_abc123",
    "payment_method": "card",
    "display_name": "Visa 1234",
    "network": "Visa"
  }
}
```

**Request (Google Pay):**
```json
{
  "wallet_type": "google_pay",
  "amount": {
    "amount": 49.99,
    "currency": "USD"
  },
  "customer_id": "cust_123",
  "google_pay_token": {
    "signature": "signature_string",
    "protocol_version": "ECv2",
    "signed_message": "signed_message_data"
  }
}
```

**Response:**
```json
{
  "payment_id": "pay_wallet_789",
  "status": "succeeded",
  "amount": {
    "amount": 49.99,
    "currency": "USD"
  },
  "requires_action": false
}
```

### Get Supported Wallets
**Endpoint:** `GET /wallets/supported?region=APAC&currency=USD`

**Response:**
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

---

## Error Responses

All endpoints return errors in the following format:

```json
{
  "error": {
    "code": "INVALID_CURRENCY",
    "message": "Currency USD is not supported",
    "details": "Supported currencies: EUR, GBP, JPY"
  }
}
```

### Common Error Codes
- `INVALID_INPUT` - Invalid request parameters
- `NOT_FOUND` - Resource not found
- `UNAUTHORIZED` - Authentication failed
- `CURRENCY_MISMATCH` - Currency mismatch in operation
- `INSUFFICIENT_FUNDS` - Insufficient funds
- `PAYMENT_FAILED` - Payment processing failed

---

## Webhooks

### Subscription Events
- `subscription.created`
- `subscription.updated`
- `subscription.canceled`
- `subscription.past_due`

### Payment Events
- `payment.succeeded`
- `payment.failed`
- `invoice.paid`
- `invoice.payment_failed`

### Payout Events
- `payout.scheduled`
- `payout.paid`
- `payout.failed`

**Webhook Payload:**
```json
{
  "id": "evt_123",
  "type": "subscription.created",
  "created": 1707936519,
  "data": {
    "object": { /* subscription object */ }
  }
}
```

---

## Rate Limits
- **Standard:** 100 requests/minute
- **Burst:** 200 requests/minute

Rate limit headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1707936600
```

---

## SDK Examples

### JavaScript/TypeScript
```typescript
import { HyperswitchExtensions } from 'hyperswitch-go-extensions-sdk';

const client = new HyperswitchExtensions({
  apiKey: 'sk_test_...',
  baseURL: 'http://localhost:8081/api/v1'
});

// Create subscription
const subscription = await client.subscriptions.create({
  customerId: 'cust_123',
  planId: 'plan_456',
  quantity: 1,
  trialDays: 14
});

// Convert currency
const conversion = await client.forex.convert({
  amount: { amount: 100, currency: 'USD' },
  to: 'EUR'
});
```

### Python
```python
from hyperswitch_extensions import Client

client = Client(api_key='sk_test_...')

# Process wallet payment
payment = client.wallets.pay({
    'wallet_type': 'apple_pay',
    'amount': {'amount': 49.99, 'currency': 'USD'},
    'customer_id': 'cust_123',
    'apple_pay_token': {...}
})
```

---

## Support

For issues and questions:
- Email: support@hyperswitch.io
- Docs: https://docs.hyperswitch.io
- GitHub: https://github.com/juspay/hyperswitch
