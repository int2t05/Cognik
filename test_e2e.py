#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""Cognos E2E test: create articles, publish, agent QA."""
import sys, io, requests, json, time, subprocess
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')
sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8', errors='replace')

BASE = "http://localhost:8080/api/v1"

def login():
    r = requests.post(f"{BASE}/auth/login", json={"username":"admin","password":"Admin@2024"})
    d = r.json()
    assert d["code"]==0, f"Login failed: {d}"
    return d["data"]["access_token"]

def create_kb(token, name, desc):
    requests.post(f"{BASE}/admin/knowledge-bases",
        headers={"Authorization":f"Bearer {token}"},
        json={"name":name,"description":desc})
    # KB create returns null, list to get ID
    r = requests.get(f"{BASE}/admin/knowledge-bases",
        headers={"Authorization":f"Bearer {token}"})
    kbs = r.json()["data"]
    if kbs:
        return kbs[-1]  # return last (just created)
    return None

def create_article(token, kb_id, title, content, tags):
    r = requests.post(f"{BASE}/admin/knowledge-bases/{kb_id}/articles",
        headers={"Authorization":f"Bearer {token}"},
        json={"title":title,"content":content,"source_type":1,"tags":tags})
    d = r.json()
    if d["code"]!=0:
        print(f"  [SKIP] Article exists: {title}")
        return None
    return d["data"]["id"]

def publish(token, aid):
    h={"Authorization":f"Bearer {token}"}
    requests.post(f"{BASE}/admin/articles/{aid}/submit-review", headers=h)
    requests.post(f"{BASE}/admin/articles/{aid}/review", headers=h, json={"approved":True})
    r = requests.post(f"{BASE}/admin/articles/{aid}/publish", headers=h)
    return r.json()["code"]==0

def check_idx(aid):
    r = subprocess.run(["docker","exec","cognos-postgres","psql","-U","cognos","-d","cognos","-t","-c",
        f"SELECT chunk_count, process_status FROM knowledge_articles WHERE id={aid}"],
        capture_output=True, text=True)
    return r.stdout.strip()

def chat(token, tid, question, timeout=120):
    r = requests.post(f"{BASE}/portal/threads/{tid}/stream",
        headers={"Authorization":f"Bearer {token}","Content-Type":"application/json"},
        json={"question":question}, stream=True, timeout=timeout)
    answer=""; tools=[]; results=[]; errors=[]
    for line in r.iter_lines(decode_unicode=True):
        if not line or not line.startswith("data:"): continue
        try: evt=json.loads(line[5:])
        except: continue
        t=evt.get("type",""); c=evt.get("content","")
        if t=="token": answer+=c
        elif t=="tool_call": tools.append({"label":evt.get("label",""),"args":c[:100]})
        elif t=="tool_result": results.append(c[:200])
        elif t=="done": break
        elif t=="error": errors.append(c[:200])
    return {"answer":answer,"tool_calls":tools,"tool_results":results,"errors":errors}

ARTICLES = [
    {"title":"PostgreSQL Performance Tuning","tags":["pg","perf"],
     "content":"# PostgreSQL Performance Tuning\n\n## EXPLAIN ANALYZE\n\nUse EXPLAIN ANALYZE to view query execution plans:\n\n```sql\nEXPLAIN ANALYZE SELECT * FROM users WHERE email = 'test@test.com';\n```\n\n## Index Optimization\n\nCreate indexes for common query conditions:\n\n```sql\nCREATE INDEX idx_users_email ON users(email);\nCREATE INDEX idx_orders_created ON orders(created_at DESC);\n```\n\nComposite index column order matters - put most filtered column first.\n\n## Connection Pooling\n\nPostgreSQL default max_connections=100. Use PgBouncer for high concurrency:\n\n```\n[pgbouncer]\npool_mode = transaction\nmax_client_conn = 1000\ndefault_pool_size = 25\n```\n\n## VACUUM and ANALYZE\n\nRun VACUUM to reclaim dead tuples, ANALYZE to update statistics:\n\n```sql\nVACUUM ANALYZE;\n```\n\nautovacuum is on by default, but high-write tables may need tuning:\n\n```sql\nALTER TABLE events SET (autovacuum_vacuum_scale_factor = 0.05);\n```\n\n## Memory Parameters\n\n- shared_buffers: 25% of total RAM\n- effective_cache_size: 50-75% of total RAM\n- work_mem: 4-16MB per connection\n- maintenance_work_mem: 256MB-1GB for VACUUM/CREATE INDEX\n"},
    {"title":"Docker Container Deployment","tags":["docker","devops"],
     "content":"# Docker Container Deployment Best Practices\n\n## Multi-Stage Build\n\n```dockerfile\nFROM golang:1.21 AS builder\nWORKDIR /app\nCOPY . .\nRUN go build -o myapp ./cmd/main.go\n\nFROM alpine:3.18\nCOPY --from=builder /app/myapp /usr/local/bin/\nENTRYPOINT [\"myapp\"]\n```\n\n## Layer Caching\n\nPut low-frequency-change operations first:\n\n```dockerfile\nCOPY go.mod go.sum ./\nRUN go mod download\nCOPY . .\nRUN go build -o myapp ./cmd/main.go\n```\n\n## Health Check\n\n```yaml\nservices:\n  api:\n    healthcheck:\n      test: [\"CMD\", \"curl\", \"-f\", \"http://localhost:8080/health\"]\n      interval: 30s\n      timeout: 10s\n      retries: 3\n```\n\n## Resource Limits\n\n```yaml\nservices:\n  api:\n    deploy:\n      resources:\n        limits:\n          cpus: '2'\n          memory: 512M\n```\n\n## Security\n\n- Use custom networks for inter-container communication\n- Don't expose unnecessary ports\n- Use .dockerignore to reduce image size\n- Run as non-root user\n\n```dockerfile\nRUN adduser -D appuser\nUSER appuser\n```\n"},
    {"title":"Redis Cache Design Patterns","tags":["redis","cache"],
     "content":"# Redis Cache Design Patterns\n\n## Cache Penetration (Cache Bypass)\n\nQuerying non-existent data bypasses cache and hits database directly.\n\nSolution - Bloom Filter:\n\n```python\n# Add to bloom filter when writing cache\nbf.add(\"users\", user_id)\n# Check bloom filter before querying\nif not bf.exists(\"users\", user_id):\n    return None  # non-existent data returns directly\n```\n\n## Cache Breakdown\n\nHot key expires, causing massive requests to hit database.\n\nSolution - Mutex Lock:\n\n```python\ndef get_with_lock(key):\n    value = redis.get(key)\n    if value:\n        return value\n    lock = redis.set(f\"lock:{key}\", \"1\", nx=True, ex=10)\n    if lock:\n        value = db.query(key)\n        redis.set(key, value, ex=300)\n        redis.delete(f\"lock:{key}\")\n        return value\n    else:\n        time.sleep(0.1)\n        return get_with_lock(key)\n```\n\n## Cache Avalanche\n\nMassive keys expire at the same time.\n\nSolution - Random expiration offset:\n\n```python\nexpire = 300 + random.randint(0, 60)\nredis.setex(key, expire, value)\n```\n\n## Data Consistency - Cache Aside\n\n```\n1. Update database\n2. Delete cache (not update cache)\n```\n\nDelayed double deletion:\n\n```python\nredis.delete(key)\ndb.update(data)\ntime.sleep(0.5)\nredis.delete(key)\n```\n"},
    {"title":"Go Concurrency Guide","tags":["go","concurrency"],
     "content":"# Go Concurrency Programming Guide\n\nGo language concurrency model is based on goroutines and channels.\n\n## Goroutine\n\nUse the go keyword to start:\n\n```go\ngo func() {\n    fmt.Println(\"hello\")\n}()\n```\n\n## Channel\n\nChannels are used for communication between goroutines:\n\n```go\nch := make(chan int)\ngo func() {\n    ch <- 42\n}()\nval := <-ch\n```\n\n### Buffered Channel\n\n```go\nch := make(chan int, 10)\nch <- 1  // non-blocking if buffer has space\n```\n\n### Channel Direction\n\n```go\nfunc producer(ch chan<- int) { ch <- 42 }      // send only\nfunc consumer(ch <-chan int) { val := <-ch }    // receive only\n```\n\n## Select Multiplexing\n\n```go\nselect {\ncase v := <-ch1:\n    fmt.Println(v)\ncase <-time.After(time.Second):\n    fmt.Println(\"timeout\")\ndefault:\n    fmt.Println(\"no data\")\n}\n```\n\n## sync.WaitGroup\n\n```go\nvar wg sync.WaitGroup\nfor i := 0; i < 5; i++ {\n    wg.Add(1)\n    go func(n int) {\n        defer wg.Done()\n        fmt.Println(n)\n    }(i)\n}\nwg.Wait()\n```\n\n## Context\n\n```go\nctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)\ndefer cancel()\nselect {\ncase <-ctx.Done():\n    fmt.Println(\"cancelled\")\n}\n```\n"},
    {"title":"Git Workflow Best Practices","tags":["git","workflow"],
     "content":"# Git Workflow Best Practices\n\n## Branch Strategy\n\n### Git Flow\n\n- main: production code\n- develop: integration branch\n- feature/*: feature branches\n- release/*: release preparation\n- hotfix/*: urgent fixes\n\n### GitHub Flow (simpler)\n\n- main: production\n- feature branches: short-lived, PR to main\n\n## Commit Messages\n\nUse conventional commits:\n\n```\nfeat: add user authentication\nfix: resolve login redirect loop\ndocs: update API documentation\nrefactor: extract validation logic\ntest: add integration tests\n```\n\n## Rebase vs Merge\n\n- Rebase: linear history, cleaner log\n- Merge: preserves branch context\n\n```bash\n# Rebase feature onto main\ngit checkout feature\ngit rebase main\n\n# Merge with --no-ff to preserve branch info\ngit merge --no-ff feature\n```\n\n## Stash\n\n```bash\ngit stash\ngit stash pop\ngit stash list\n```\n\n## Cherry Pick\n\n```bash\n# Apply a specific commit from another branch\ngit cherry-pick <commit-hash>\n```\n\n## Tags\n\n```bash\ngit tag v1.0.0\ngit push origin v1.0.0\n```\n"},
]

QA = [
    {"q":"How to view query execution plan in PostgreSQL?","expect":"EXPLAIN","article":"PostgreSQL"},
    {"q":"How to do multi-stage build in Docker?","expect":"FROM","article":"Docker"},
    {"q":"How to solve cache penetration in Redis?","expect":"bloom","article":"Redis"},
    {"q":"How to use channels in Go for goroutine communication?","expect":"channel","article":"Go"},
    {"q":"What is conventional commit format?","expect":"feat","article":"Git"},
]

def main():
    print("=== 1. Login ===")
    token = login()
    print("OK")

    print("\n=== 2. Create KB + Articles ===")
    kb = create_kb(token, "Tech Knowledge Base", "Team knowledge repository")
    kb_id = kb["id"]
    print(f"KB ID: {kb_id}")

    for a in ARTICLES:
        aid = create_article(token, kb_id, a["title"], a["content"], a["tags"])
        if aid:
            print(f"  Created: {a['title']} (ID={aid})")
            if publish(token, aid):
                print(f"  Published")
                time.sleep(5)
                status = check_idx(aid)
                print(f"  Index: {status}")

    print("\n=== 3. Create Thread ===")
    r = requests.post(f"{BASE}/portal/threads",
        headers={"Authorization":f"Bearer {token}"}, json={"title":"E2E QA Test"})
    tid = r.json()["data"]["id"]
    print(f"Thread ID: {tid}")

    print("\n=== 4. Agent QA Test ===")
    pass_count = 0
    for i, qa in enumerate(QA):
        print(f"\n--- QA {i+1}/{len(QA)} ---")
        print(f"Q: {qa['q']}")
        print(f"Expect: '{qa['expect']}'")
        result = chat(token, tid, qa["q"])
        print(f"Tool calls: {len(result['tool_calls'])}")
        for tc in result["tool_calls"]:
            safe = tc["args"].encode('ascii','replace').decode()
            print(f"  -> {tc['label']}: {safe[:60]}")
        if result["errors"]:
            print(f"Errors: {result['errors']}")
        answer = result["answer"].strip()
        print(f"Answer length: {len(answer)} chars")
        if answer:
            safe = answer[:200].encode('ascii','replace').decode()
            print(f"Preview: {safe}")
        if qa["expect"].lower() in answer.lower():
            print(f"[PASS]")
            pass_count += 1
        else:
            print(f"[FAIL] - keyword not found")

    print(f"\n=== Results: {pass_count}/{len(QA)} passed ===")

if __name__ == "__main__":
    main()
