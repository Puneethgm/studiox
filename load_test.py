import json
import time
import urllib.request
import urllib.error
import concurrent.futures
import random

# Target API settings
API_URL = "http://localhost:8080/api/v1/public/studios/test-studio/campaigns/testr-rxec/leads"
TOTAL_REQUESTS = 250
CONCURRENCY = 15

def send_request(idx):
    payload = {
        "firstName": f"LoadUser{idx}",
        "lastName": f"Test{random.randint(1000, 9999)}",
        "email": f"loadtest_{idx}_{random.randint(10000, 99999)}@example.com",
        "phone": f"+65{random.randint(80000000, 99999999)}",
        "fitnessPlan": "Yoga",
        "goals": "Testing API load balancing and concurrent write performance."
    }
    
    data = json.dumps(payload).encode('utf-8')
    req = urllib.request.Request(
        API_URL,
        data=data,
        headers={'Content-Type': 'application/json', 'User-Agent': 'StudioX-Load-Tester/1.0'}
    )
    
    start_time = time.time()
    try:
        with urllib.request.urlopen(req) as response:
            status = response.status
            response.read() # Consume response body
            latency = time.time() - start_time
            return {"status": status, "latency": latency, "success": True}
    except urllib.error.HTTPError as e:
        latency = time.time() - start_time
        return {"status": e.code, "latency": latency, "success": False, "error": str(e)}
    except Exception as e:
        latency = time.time() - start_time
        return {"status": 0, "latency": latency, "success": False, "error": str(e)}

def main():
    print(f"🚀 Starting StudioX Load Test...")
    print(f"🎯 Target Endpoint: {API_URL}")
    print(f"📊 Total Requests: {TOTAL_REQUESTS} | Concurrency Level: {CONCURRENCY}")
    print("-" * 60)
    
    start_test = time.time()
    results = []
    
    with concurrent.futures.ThreadPoolExecutor(max_workers=CONCURRENCY) as executor:
        futures = {executor.submit(send_request, i): i for i in range(TOTAL_REQUESTS)}
        for future in concurrent.futures.as_completed(futures):
            results.append(future.result())
            if len(results) % 25 == 0:
                print(f"Processed {len(results)}/{TOTAL_REQUESTS} requests...")

    total_time = time.time() - start_test
    
    # Calculate stats
    success_count = sum(1 for r in results if r["success"] and r["status"] == 201)
    failed_count = TOTAL_REQUESTS - success_count
    latencies = [r["latency"] for r in results]
    
    avg_latency = sum(latencies) / len(latencies) if latencies else 0
    min_latency = min(latencies) if latencies else 0
    max_latency = max(latencies) if latencies else 0
    rps = TOTAL_REQUESTS / total_time
    
    print("\n" + "=" * 60)
    print("📈 LOAD TEST SUMMARY REPORT")
    print("=" * 60)
    print(f"⏱️  Total Duration:     {total_time:.3f} seconds")
    print(f"⚡ Requests Per Second: {rps:.2f} RPS")
    print(f"✅ Success Rate:        {success_count}/{TOTAL_REQUESTS} ({success_count/TOTAL_REQUESTS*100:.1f}%)")
    print(f"❌ Failed Requests:     {failed_count}")
    print(f"🏎️  Min Latency:         {min_latency*1000:.2f} ms")
    print(f"🐌 Max Latency:         {max_latency*1000:.2f} ms")
    print(f"📊 Avg Latency:         {avg_latency*1000:.2f} ms")
    print("=" * 60)
    
    # Write a clean markdown file for modeling presentation
    report_md = f"""# 📊 StudioX API Load Testing Report

**Target Endpoint:** `{API_URL}`  
**Concurrency Level:** `{CONCURRENCY} concurrent threads`  
**Total Requests Transmitted:** `{TOTAL_REQUESTS}`

## 📈 Executive Metrics

| Metric | Value |
|--------|-------|
| **Total Duration** | `{total_time:.3f} s` |
| **Requests Per Second (RPS)** | `{rps:.2f} RPS` |
| **Average Latency** | `{avg_latency*1000:.2f} ms` |
| **Min Latency** | `{min_latency*1000:.2f} ms` |
| **Max Latency** | `{max_latency*1000:.2f} ms` |
| **Success Rate** | `{success_count}/{TOTAL_REQUESTS} ({success_count/TOTAL_REQUESTS*100:.1f}%)` |
| **Failed Requests** | `{failed_count}` |

## 🛡️ Response Code Breakdown
- **201 Created:** `{success_count}` requests
- **Errors/Other:** `{failed_count}` requests
"""
    with open("load_test_report.md", "w") as f:
        f.write(report_md)
    print("💾 Saved results to load_test_report.md")

if __name__ == "__main__":
    main()
