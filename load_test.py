import json
import time
import urllib.request
import urllib.error
import concurrent.futures
import random

# Target Base URL
BASE_URL = "http://localhost:8080"
ENDPOINTS = {
    "health": f"{BASE_URL}/health",
    "public_campaign": f"{BASE_URL}/api/v1/public/studios/test-studio/campaigns/testr-rxec",
    "submit_lead": f"{BASE_URL}/api/v1/public/studios/test-studio/campaigns/testr-rxec/leads"
}

TOTAL_REQUESTS = 300
CONCURRENCY = 20

def test_endpoint(endpoint_name, url, method="GET", payload=None):
    data = None
    if payload:
        data = json.dumps(payload).encode('utf-8')
        
    req = urllib.request.Request(
        url,
        data=data,
        method=method,
        headers={'Content-Type': 'application/json', 'User-Agent': 'StudioX-Load-Tester/2.0'}
    )
    
    start_time = time.time()
    try:
        with urllib.request.urlopen(req) as response:
            status = response.status
            response.read() # Consume response body
            latency = time.time() - start_time
            return {"endpoint": endpoint_name, "status": status, "latency": latency, "success": True}
    except urllib.error.HTTPError as e:
        latency = time.time() - start_time
        return {"endpoint": endpoint_name, "status": e.code, "latency": latency, "success": False, "error": str(e)}
    except Exception as e:
        latency = time.time() - start_time
        return {"endpoint": endpoint_name, "status": 0, "latency": latency, "success": False, "error": str(e)}

def worker(idx):
    # Distribute requests across endpoints to simulate realistic multi-endpoint API traffic
    # 40% reads, 40% writes, 20% health checks
    rand = random.random()
    if rand < 0.20:
        return test_endpoint("health", ENDPOINTS["health"], "GET")
    elif rand < 0.60:
        return test_endpoint("public_campaign", ENDPOINTS["public_campaign"], "GET")
    else:
        payload = {
            "firstName": f"LoadUser{idx}",
            "lastName": f"Test{random.randint(1000, 9999)}",
            "email": f"loadtest_{idx}_{random.randint(10000, 99999)}@example.com",
            "phone": f"+65{random.randint(80000000, 99999999)}",
            "fitnessPlan": "Yoga",
            "goals": f"Rigorous multi-endpoint testing run #{idx}"
        }
        return test_endpoint("submit_lead", ENDPOINTS["submit_lead"], "POST", payload)

def main():
    print("🚀 Starting StudioX Comprehensive Multi-Endpoint Load Test...")
    print(f"📊 Running {TOTAL_REQUESTS} total requests across all endpoints | Concurrency: {CONCURRENCY}")
    print("-" * 75)
    
    start_test = time.time()
    results = []
    
    with concurrent.futures.ThreadPoolExecutor(max_workers=CONCURRENCY) as executor:
        futures = {executor.submit(worker, i): i for i in range(TOTAL_REQUESTS)}
        for future in concurrent.futures.as_completed(futures):
            results.append(future.result())
            if len(results) % 50 == 0:
                print(f"Completed {len(results)}/{TOTAL_REQUESTS} requests...")

    total_time = time.time() - start_test
    
    # Calculate statistics by endpoint
    stats = {}
    for r in results:
        ep = r["endpoint"]
        if ep not in stats:
            stats[ep] = {"latencies": [], "success": 0, "failed": 0, "status_codes": {}}
        
        stats[ep]["latencies"].append(r["latency"])
        if r["success"] and r["status"] in (200, 201):
            stats[ep]["success"] += 1
        else:
            stats[ep]["failed"] += 1
            
        stats[ep]["status_codes"][r["status"]] = stats[ep]["status_codes"].get(r["status"], 0) + 1

    print("\n" + "=" * 75)
    print("📈 COMPREHENSIVE MULTI-ENDPOINT LOAD TEST REPORT")
    print("=" * 75)
    print(f"⏱️  Total Test Duration:  {total_time:.3f} seconds")
    print(f"⚡ Aggregate Throughput:  {TOTAL_REQUESTS / total_time:.2f} RPS")
    print("-" * 75)
    
    for ep, data in stats.items():
        latencies = data["latencies"]
        avg_l = sum(latencies) / len(latencies) if latencies else 0
        min_l = min(latencies) if latencies else 0
        max_l = max(latencies) if latencies else 0
        total_ep_reqs = len(latencies)
        
        print(f"🟢 Endpoint: [{ep.upper()}]")
        print(f"   📥 Total Requests:  {total_ep_reqs}")
        print(f"   ✅ Success Rate:     {data['success']}/{total_ep_reqs} ({data['success']/total_ep_reqs*100:.1f}%)")
        print(f"   🏎️  Latency (Min/Avg/Max): {min_l*1000:.1f}ms / {avg_l*1000:.1f}ms / {max_l*1000:.1f}ms")
        print(f"   🔢 Status Codes:    {data['status_codes']}")
        print("-" * 75)
        
    print("=" * 75)

if __name__ == "__main__":
    main()
