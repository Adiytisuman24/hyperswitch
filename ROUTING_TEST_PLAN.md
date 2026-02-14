# Verification Test Plan for Routing Eligibility Fixes

## Test #1: Disabled Connector Filtering

### Setup
1. Configure merchant profile with 3 connectors:
   - Connector A: Stripe (enabled)
   - Connector B: Adyen (disabled)
   - Connector C: Braintree (enabled)

2. Set fallback list to include all three: [A, B, C]

### Test Steps
```bash
# 1. Disable Connector B in database
UPDATE merchant_connector_account 
SET disabled = true 
WHERE connector_name = 'adyen' AND profile_id = 'test_profile_id';

# 2. Remove or deactivate routing algorithm
UPDATE business_profile 
SET routing_algorithm_id = NULL 
WHERE profile_id = 'test_profile_id';

# 3. Initiate payment
curl -X POST http://localhost:8080/payments \
  -H "Content-Type: application/json" \
  -H "api-key: YOUR_API_KEY" \
  -d '{
    "amount": 1000,
    "currency": "USD",
    "customer_id": "test_customer",
    "profile_id": "test_profile_id"
  }'
```

### Expected Result
```rust
// Logs should show:
[INFO] Fetching eligible fallback connectors
[INFO] Retrieved 3 fallback connectors before eligibility analysis
[WARN] Filtering out disabled connector from routing
       connector: adyen
       reason: disabled
[INFO] Eligibility analysis complete
       original_count: 3
       eligible_count: 2
       filtered_count: 1

// Payment should route to ONLY Stripe or Braintree, NEVER Adyen
```

---

## Test #2: Payment Method Compatibility

### Setup
1. Configure connectors with specific payment methods:
   - Stripe: Supports cards, wallets
   - PayPal: Supports only wallets
   - Klarna: Supports only BNPL

2. Fallback list: [Stripe, PayPal, Klarna]

### Test Steps
```bash
# Test CARD payment
curl -X POST http://localhost:8080/payments \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 1000,
    "currency": "USD",
    "payment_method": "card",
    "payment_method_type": "credit",
    "profile_id": "test_profile_id"
  }'
```

### Expected Result
```rust
// Only Stripe should remain (supports cards)
[INFO] Eligibility analysis complete
       original_count: 3
       eligible_count: 1  // Only Stripe
       filtered_count: 2  // PayPal and Klarna filtered
```

---

## Test #3: Algorithm Cache Failure Fallback

### Setup
1. Configure valid routing algorithm
2. Temporarily corrupt the algorithm cache

### Test Steps
```bash
# Corrupt cache
redis-cli DEL "routing_cache_merchant_123_profile_abc"

# Or modify algorithm to cause parse error
UPDATE routing_algorithm 
SET algorithm_data = '{"invalid": "json"}' 
WHERE algorithm_id = 'test_algo_id';

# Attempt payment
curl -X POST http://localhost:8080/payments \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 1000,
    "currency": "USD",
    "profile_id": "test_profile_id"
  }'
```

### Expected Result
```rust
[ERROR] euclid_routing: ensure_algorithm_cached failed, 
        falling back to eligible merchant default connectors
        error: <cache error details>

[INFO] Fetching eligible fallback connectors
[INFO] Retrieved X fallback connectors before eligibility analysis
[INFO] Eligibility analysis complete
       eligible_count: Y  // Y <= X (filtered)

// Payment proceeds with eligible fallback connectors
```

---

## Test #4: Empty Eligible Connectors

### Setup
1. All connectors in fallback list are disabled

### Test Steps
```bash
# Disable all fallback connectors
UPDATE merchant_connector_account 
SET disabled = true 
WHERE profile_id = 'test_profile_id';

# Attempt payment
curl -X POST http://localhost:8080/payments \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 1000,
    "currency": "USD",
    "profile_id": "test_profile_id"
  }'
```

### Expected Result
```rust
[INFO] Retrieved 3 fallback connectors before eligibility analysis
[WARN] Filtering out disabled connector from routing (repeated 3x)
[INFO] Eligibility analysis complete
       original_count: 3
       eligible_count: 0
       filtered_count: 3
[ERROR] No eligible fallback connectors found after eligibility analysis

// Payment should fail with appropriate error:
// "No eligible connectors available for routing"
```

---

## Test #5: MCA Not Found for Profile

### Setup
1. Fallback list includes connector from different profile

### Test Steps
```bash
# Add connector to fallback for wrong profile
INSERT INTO config (key, config) VALUES (
  'routing_default_test_profile_id',
  '[{"connector": "stripe", "merchant_connector_id": "mca_from_other_profile"}]'
);

# Attempt payment
curl -X POST http://localhost:8080/payments \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 1000,
    "currency": "USD",
    "profile_id": "test_profile_id"
  }'
```

### Expected Result
```rust
[WARN] Filtering out connector: MCA not found for profile
       connector: stripe
       mca_id: mca_from_other_profile

[INFO] Eligibility analysis complete
       filtered_count: 1
```

---

## Automated Test Suite

```rust
#[cfg(test)]
mod routing_eligibility_tests {
    use super::*;

    #[tokio::test]
    async fn test_disabled_connector_filtered() {
        // Arrange
        let state = setup_test_state().await;
        let profile = create_test_profile();
        let connectors = vec![
            create_connector("stripe", Some("mca_1"), false),   // enabled
            create_connector("adyen", Some("mca_2"), true),     // disabled
            create_connector("braintree", Some("mca_3"), false), // enabled
        ];
        
        let transaction_data = create_test_payment();
        
        // Act
        let eligible = perform_connector_eligibility_analysis(
            &state,
            connectors,
            &transaction_data,
            &profile,
        ).await.unwrap();
        
        // Assert
        assert_eq!(eligible.len(), 2, "Should filter out 1 disabled connector");
        assert!(!eligible.iter().any(|c| c.connector == "adyen"));
    }

    #[tokio::test]
    async fn test_fallback_when_no_algorithm() {
        // Arrange
        let state = setup_test_state().await;
        let profile = create_test_profile();
        let transaction_data = create_test_payment();
        
        // Act
        let (connectors, approach) = perform_static_routing_v1(
            &state,
            &merchant_id,
            None,  // No algorithm
            &profile,
            &transaction_data,
        ).await.unwrap();
        
        // Assert
        assert!(approach.is_none());
        assert!(!connectors.is_empty(), "Should return eligible fallback connectors");
        assert!(connectors.iter().all(|c| !is_connector_disabled(&state, c).await));
    }

    #[tokio::test]
    async fn test_payment_method_filtering() {
        // Arrange: Setup connectors with specific PM support
        let connectors = vec![
            connector_supporting(&["card", "wallet"]),
            connector_supporting(&["wallet"]),
            connector_supporting(&["bnpl"]),
        ];
        
        let payment_data = payment_with_method("card");
        
        // Act
        let eligible = perform_connector_eligibility_analysis(
            &state,
            connectors,
            &payment_data,
            &profile,
        ).await.unwrap();
        
        // Assert
        assert_eq!(eligible.len(), 1, "Only connector supporting cards should remain");
    }
}
```

---

## Manual Verification Checklist

- [ ] Disabled connector never selected in routing
- [ ] Log messages clearly indicate filtering reasons
- [ ] Empty eligible list returns appropriate error
- [ ] Algorithm failure falls back to eligible connectors
- [ ] Payment method incompatibility filtered
- [ ] Currency mismatch handled (if implemented)
- [ ] Performance: Eligibility check completes in <100ms
- [ ] No false positives (valid connectors incorrectly filtered)

---

## Metrics to Monitor

After deployment, monitor these metrics:

```
1. routing_fallback_eligibility_filter_count
   - How many connectors filtered per request
   
2. routing_fallback_empty_eligible_count  
   - Frequency of zero eligible connectors
   
3. routing_disabled_connector_filtered_count
   - Specific count for disabled connectors
   
4. routing_eligibility_analysis_duration_ms
   - Performance of eligibility check
```

---

## Rollback Plan

If issues arise:

```bash
# Option 1: Feature flag disable
UPDATE config 
SET config = '{"eligibility_analysis_enabled": false}' 
WHERE key = 'routing_feature_flags';

# Option 2: Git revert
git revert <commit_hash>
cargo build --release
systemctl restart hyperswitch

# Option 3: Conditional compilation
# Wrap new code in:
#[cfg(feature = "eligibility_analysis")]
```

---

## Success Criteria

✅ All tests pass  
✅ No disabled connectors selected in production  
✅ Payment success rate maintained or improved  
✅ Latency increase <10ms for routing calls  
✅ No increase in payment failures due to routing  
