#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""Cognos E2E test: create articles, publish, agent QA."""
import sys, io, requests, json, time
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')
sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8', errors='replace')

BASE = "http://localhost:8080/api/v1"

def login():
    r = requests.post(f"{BASE}/auth/login", json={"username":"admin","password":"Admin@2024"})
    d = r.json()
    assert d["code"]==0, f"Login failed: {d}"
    return d["data"]["access_token"]

def create_kb(token, name, desc):
    requests.post(f"{BASE}/admin/knowledge-bases", headers={"Authorization":f"Bearer {token}"}, json={"name":name,"description":desc})
    r = requests.get(f"{BASE}/admin/knowledge-bases", headers={"Authorization":f"Bearer {token}"})
    kbs = r.json()["data"]
    return kbs[-1]["id"] if kbs else None

def create_article(token, kb_id, title, content, tags):
    r = requests.post(f"{BASE}/admin/knowledge-bases/{kb_id}/articles",
        headers={"Authorization":f"Bearer {token}"},
        json={"title":title,"content":content,"source_type":1,"tags":tags})
    d = r.json()
    if d["code"]!=0:
        print(f"  [SKIP] {title}: {d['message']}")
        return None
    return d["data"]["id"]

def publish(token, aid):
    h={"Authorization":f"Bearer {token}"}
    requests.post(f"{BASE}/admin/articles/{aid}/submit-review", headers=h)
    requests.post(f"{BASE}/admin/articles/{aid}/review", headers=h, json={"approved":True})
    r = requests.post(f"{BASE}/admin/articles/{aid}/publish", headers=h)
    return r.json()["code"]==0

def chat(token, tid, question, timeout=300):
    r = requests.post(f"{BASE}/portal/threads/{tid}/stream",
        headers={"Authorization":f"Bearer {token}","Content-Type":"application/json"},
        json={"question":question}, stream=True, timeout=timeout)
    answer='';tools=[]
    for line in r.iter_lines(decode_unicode=True):
        if not line or not line.startswith("data:"): continue
        try: d=json.loads(line[5:])
        except: continue
        t=d.get("type","");c=d.get("content","")
        if t=="token": answer+=c
        elif t=="tool_call": tools.append({"label":d.get("label",""),"args":c[:80]})
        elif t=="tool_result": tools.append({"result":c[:80]})
        elif t=="done": break
        elif t=="error": tools.append({"error":c[:100]})
    return {"answer":answer,"tool_calls":tools}

ARTICLES = [
    {"title":"Python Data Science Guide","tags":["python","data"],
     "content":"# Python Data Science Guide\n\n## NumPy Arrays\n\nNumPy is the foundation of scientific computing in Python.\n\n```python\nimport numpy as np\narr = np.array([1, 2, 3, 4, 5])\nprint(arr.mean())\n```\n\n## Pandas DataFrames\n\nPandas provides data structures for tabular data:\n\n```python\nimport pandas as pd\ndf = pd.read_csv('data.csv')\ndf.head()\ndf.describe()\n```\n\n## Matplotlib Visualization\n\n```python\nimport matplotlib.pyplot as plt\nplt.plot([1,2,3],[4,5,6])\nplt.show()\n```\n\n## Scikit-learn ML\n\n```python\nfrom sklearn.model_selection import train_test_split\nX_train, X_test = train_test_split(X, test_size=0.2)\n```"},
    {"title":"Kubernetes Deployment Guide","tags":["k8s","devops"],
     "content":"# Kubernetes Deployment Guide\n\n## Pod and Deployment\n\nA Pod is the smallest deployable unit in Kubernetes.\n\n```yaml\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: my-app\nspec:\n  replicas: 3\n  selector:\n    matchLabels:\n      app: my-app\n  template:\n    spec:\n      containers:\n      - name: my-app\n        image: my-app:latest\n        ports:\n        - containerPort: 8080\n```\n\n## Service\n\n```yaml\napiVersion: v1\nkind: Service\nmetadata:\n  name: my-app-svc\nspec:\n  selector:\n    app: my-app\n  ports:\n  - port: 80\n    targetPort: 8080\n```\n\n## ConfigMap and Secret\n\nConfigMaps store configuration, Secrets store sensitive data:\n```yaml\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app-config\ndata:\n  DB_HOST: postgres\n```\n\n## Rolling Update\n\n```bash\nkubectl rollout status deployment/my-app\nkubectl rollout undo deployment/my-app\n```"},
    {"title":"React Frontend Best Practices","tags":["react","frontend"],
     "content":"# React Frontend Best Practices\n\n## Component Design\n\nUse functional components with hooks:\n```jsx\nfunction UserProfile({ user }) {\n  const [count, setCount] = useState(0)\n  return <div>{user.name}: {count}</div>\n}\n```\n\n## State Management\n\nUse Context for global state, useState for local:\n```jsx\nconst AppContext = React.createContext()\nfunction App() {\n  return <AppContext.Provider value={{user}}>...</AppContext.Provider>\n}\n```\n\n## useEffect Cleanup\n\nAlways clean up side effects:\n```jsx\nuseEffect(() => {\n  const timer = setInterval(() => {}, 1000)\n  return () => clearInterval(timer)\n}, [])\n```\n\n## Performance\n\n- Use React.memo for expensive components\n- Use useCallback for stable function references\n- Use useMemo for expensive computations\n- Code-split with React.lazy and Suspense"},
    {"title":"PostgreSQL Performance Tuning","tags":["postgres","database"],
     "content":"# PostgreSQL Performance Tuning\n\n## EXPLAIN ANALYZE\n\n```sql\nEXPLAIN ANALYZE SELECT * FROM users WHERE email = 'test@test.com';\n```\n\n## Index Optimization\n\n```sql\nCREATE INDEX idx_users_email ON users(email);\nCREATE INDEX idx_orders_date ON orders(created_at DESC);\n```\n\n## Connection Pooling with PgBouncer\n\n```\n[pgbouncer]\npool_mode = transaction\nmax_client_conn = 1000\ndefault_pool_size = 25\n```\n\n## VACUUM and ANALYZE\n\n```sql\nVACUUM ANALYZE;\nALTER TABLE events SET (autovacuum_vacuum_scale_factor = 0.05);\n```\n\n## Memory Parameters\n\n- shared_buffers: 25% of total RAM\n- effective_cache_size: 50-75% of total RAM\n- work_mem: 4-16MB per connection\n- maintenance_work_mem: 256MB-1GB"},
    {"title":"Redis Cache Design Patterns","tags":["redis","cache"],
     "content":"# Redis Cache Design Patterns\n\n## Cache Penetration (Bloom Filter)\n\n```python\nbf.add(\"users\", user_id)\nif not bf.exists(\"users\", user_id):\n    return None\n```\n\n## Cache Breakdown (Mutex Lock)\n\n```python\nlock = redis.set(f\"lock:{key}\", \"1\", nx=True, ex=10)\nif lock:\n    value = db.query(key)\n    redis.set(key, value, ex=300)\n```\n\n## Cache Avalanche (Random TTL)\n\n```python\nexpire = 300 + random.randint(0, 60)\nredis.setex(key, expire, value)\n```\n\n## Data Consistency (Cache Aside)\n\n```python\nredis.delete(key)\ndb.update(data)\ntime.sleep(0.5)\nredis.delete(key)\n```"},
    {"title":"Git Workflow Best Practices","tags":["git","workflow"],
     "content":"# Git Workflow Best Practices\n\n## Branch Strategy\n\nGit Flow: main/develop/feature/release/hotfix\nGitHub Flow: main + short feature branches\n\n## Conventional Commits\n\n```\nfeat: add user authentication\nfix: resolve login redirect\ndocs: update API docs\nrefactor: extract validation\ntest: add integration tests\n```\n\n## Rebase vs Merge\n\n```bash\ngit rebase main\ngit merge --no-ff feature\n```\n\n## Stash and Cherry-pick\n\n```bash\ngit stash\ngit stash pop\ngit cherry-pick <hash>\ngit tag v1.0.0\n```"},
    {"title":"Go Concurrency Guide","tags":["go","concurrency"],
     "content":"# Go Concurrency Guide\n\n## Goroutine\n\n```go\ngo func() {\n    fmt.Println(\"hello\")\n}()\n```\n\n## Channel\n\n```go\nch := make(chan int)\ngo func() { ch <- 42 }()\nval := <-ch\n```\n\n## Buffered Channel\n\n```go\nch := make(chan int, 10)\nch <- 1\n```\n\n## Select Multiplexing\n\n```go\nselect {\ncase v := <-ch1:\n    fmt.Println(v)\ncase <-time.After(time.Second):\n    fmt.Println(\"timeout\")\n}\n```\n\n## sync.WaitGroup\n\n```go\nvar wg sync.WaitGroup\nfor i := 0; i < 5; i++ {\n    wg.Add(1)\n    go func(n int) { defer wg.Done(); fmt.Println(n) }(i)\n}\nwg.Wait()\n```"},
    {"title":"Docker Container Deployment","tags":["docker","devops"],
     "content":"# Docker Container Deployment\n\n## Multi-Stage Build\n\n```dockerfile\nFROM golang:1.21 AS builder\nWORKDIR /app\nCOPY . .\nRUN go build -o myapp ./cmd/main.go\n\nFROM alpine:3.18\nCOPY --from=builder /app/myapp /usr/local/bin/\nENTRYPOINT [\"myapp\"]\n```\n\n## Layer Caching\n\n```dockerfile\nCOPY go.mod go.sum ./\nRUN go mod download\nCOPY . .\n```\n\n## Health Check\n\n```yaml\nhealthcheck:\n  test: [\"CMD\", \"curl\", \"-f\", \"http://localhost:8080/health\"]\n  interval: 30s\n  timeout: 10s\n  retries: 3\n```\n\n## Security\n\n```dockerfile\nRUN adduser -D appuser\nUSER appuser\n```"},
    {"title":"REST API Design Principles","tags":["api","rest"],
     "content":"# REST API Design Principles\n\n## HTTP Methods\n\n- GET: retrieve resource\n- POST: create resource\n- PUT: full update\n- PATCH: partial update\n- DELETE: remove resource\n\n## Status Codes\n\n- 200: OK\n- 201: Created\n- 400: Bad Request\n- 401: Unauthorized\n- 404: Not Found\n- 500: Internal Error\n\n## URL Design\n\n```\nGET /api/v1/users\nPOST /api/v1/users\nGET /api/v1/users/:id\nPUT /api/v1/users/:id\nDELETE /api/v1/users/:id\n```\n\n## Pagination\n\n```\nGET /api/v1/users?page=1&limit=20\n```\n\nResponse:\n```json\n{\"data\": [], \"total\": 100, \"page\": 1, \"limit\": 20}\n```\n\n## Versioning\n\nUse URL path versioning: /api/v1/, /api/v2/"},
    {"title":"Machine Learning Fundamentals","tags":["ml","ai"],
     "content":"# Machine Learning Fundamentals\n\n## Supervised Learning\n\nTrain with labeled data:\n```python\nfrom sklearn.linear_model import LinearRegression\nmodel = LinearRegression()\nmodel.fit(X_train, y_train)\npredictions = model.predict(X_test)\n```\n\n## Unsupervised Learning\n\nK-means clustering:\n```python\nfrom sklearn.cluster import KMeans\nkmeans = KMeans(n_clusters=3)\nkmeans.fit(X)\n```\n\n## Overfitting Prevention\n\n- Cross-validation\n- Regularization (L1/L2)\n- Dropout in neural networks\n- Early stopping\n\n## Model Evaluation\n\n```python\nfrom sklearn.metrics import accuracy_score, f1_score\nacc = accuracy_score(y_true, y_pred)\nf1 = f1_score(y_true, y_pred, average='weighted')\n```"},
]

QA = [
    {"q":"How to use bloom filter for cache penetration in Redis?","expect":"bloom"},
    {"q":"How to do multi-stage build in Docker?","expect":"FROM"},
    {"q":"How to use channel in Go for goroutine communication?","expect":"channel"},
    {"q":"What is conventional commit format?","expect":"feat"},
    {"q":"How to create index in PostgreSQL?","expect":"INDEX"},
    {"q":"How to use useEffect cleanup in React?","expect":"cleanup"},
    {"q":"How to deploy to Kubernetes?","expect":"Deployment"},
    {"q":"What are the HTTP status codes?","expect":"200"},
    {"q":"How to prevent overfitting in ML?","expect":"overfitting"},
    {"q":"How to use numpy arrays?","expect":"numpy"},
]

def main():
    print("=== 1. Login ===")
    token = login()
    print("OK")

    print("\n=== 2. Create KB + Articles ===")
    kb_id = create_kb(token, "Tech Knowledge Base", "Team knowledge")
    print(f"KB ID: {kb_id}")

    for a in ARTICLES:
        aid = create_article(token, kb_id, a["title"], a["content"], a["tags"])
        if aid:
            print(f"  Created: {a['title']} (ID={aid})")
            if publish(token, aid):
                print(f"  Published")
                time.sleep(5)

    print("\n=== 3. Verify Index ===")
    r = requests.get(f"{BASE}/admin/knowledge-bases/{kb_id}/articles?status=4&page=1&pageSize=100",
        headers={"Authorization":f"Bearer {token}"})
    data = r.json()["data"]
    if isinstance(data, list):
        print(f"Published articles: {len(data)}")
    elif isinstance(data, dict):
        total = data.get('total', len(data.get('articles', [])))
        print(f"Published articles: {total}")

    print("\n=== 4. Agent QA Test ===")
    tid = requests.post(f"{BASE}/portal/threads",
        headers={"Authorization":f"Bearer {token}","Content-Type":"application/json"},
        json={"title":"E2E QA"}).json()["data"]["id"]
    print(f"Thread: {tid}")

    pass_count = 0
    for i, qa in enumerate(QA):
        print(f"\n--- QA {i+1}/{len(QA)} ---")
        print(f"Q: {qa['q']}")
        print(f"Expect: '{qa['expect']}'")
        result = chat(token, tid, qa["q"])
        print(f"Tool calls: {len([t for t in result['tool_calls'] if 'label' in t])}")
        for t in result["tool_calls"]:
            safe = str(t).encode('ascii','replace').decode()
            print(f"  {safe[:80]}")
        answer = result["answer"].strip()
        print(f"Answer ({len(answer)} chars)")
        if qa["expect"].lower() in answer.lower():
            print("[PASS]")
            pass_count += 1
        else:
            print("[FAIL]")

    print(f"\n=== Results: {pass_count}/{len(QA)} passed ===")

if __name__ == "__main__":
    main()
