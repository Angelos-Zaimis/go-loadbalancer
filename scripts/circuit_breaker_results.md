# Circuit Breaker Comparison Test Results

**Test Date:** 2026-01-13 22:46:15  
**Configuration:**
- Total Requests: 60
- Backend Killed After: Request #25
- Backend Port: 8081

---

## 📊 Summary Table

| Metric | With Circuit Breaker | Without Circuit Breaker | Difference |
|--------|:--------------------:|:----------------------:|:----------:|
| **Success Rate** | 100.0% | 88.3% | +11.7% |
| **Successful** | 60 | 53 | +7 |
| **Failed** | 0 | 7 | -7 |
| **Avg Latency** | 1.549ms | 1.555ms | - |
| **Max Latency** | 4ms | 3ms | - |
| **Throughput** | 36.5 req/s | 36.5 req/s | - |

---

## 🎯 Conclusion

✅ **Circuit Breaker improved reliability by 7 requests (88.3% → 100.0%)**

The retry logic successfully recovered from backend failures.

---

## 📈 Detailed Results

### With Circuit Breaker
- Success Rate: **100.0%** (60/60)
- Failed Requests: 0
- Avg Latency: 1.549ms
- Unique Errors: 0

### Without Circuit Breaker  
- Success Rate: **88.3%** (53/60)
- Failed Requests: 7
- Avg Latency: 1.555ms
- Unique Errors: 1

---

## 🔧 How Circuit Breaker Helps

| Feature | Benefit |
|---------|---------|
| **Fail-Fast** | Opens circuit after 5 failures, avoiding timeouts |
| **Retry Logic** | Retries GET/PUT/DELETE on different backend |
| **Health Tracking** | Records success/failure per backend |
| **Recovery** | Half-open state probes backend recovery |

---

## ❌ Errors Observed

### With Circuit Breaker
_No errors_

### Without Circuit Breaker
- `HTTP 503` ×7

