#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""Cognos E2E QA test: comprehensive multi-round agent testing."""
import sys, io, requests, json, time
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')
sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8', errors='replace')

BASE = "http://localhost:8080/api/v1"

def login():
    r = requests.post(f"{BASE}/auth/login", json={"username":"admin","password":"Admin@123"})
    return r.json()["data"]["access_token"]

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
    {"q":"How to use pandas DataFrame?","expect":"pandas"},
    {"q":"What is Git Flow branching strategy?","expect":"Flow"},
    {"q":"How to use sync.WaitGroup in Go?","expect":"WaitGroup"},
    {"q":"How to configure PgBouncer?","expect":"PgBouncer"},
    {"q":"What is VACUUM in PostgreSQL?","expect":"VACUUM"},
    {"q":"How to use React Context?","expect":"Context"},
    {"q":"What is k-means clustering?","expect":"k-means"},
    {"q":"How to do layer caching in Docker?","expect":"cache"},
    {"q":"How to use git cherry-pick?","expect":"cherry-pick"},
    {"q":"What is REST API versioning?","expect":"version"},
]

def main():
    print("=== Login ===")
    token = login()
    print("OK")

    print("\n=== Create Thread ===")
    tid = requests.post(f"{BASE}/portal/threads",
        headers={"Authorization":f"Bearer {token}","Content-Type":"application/json"},
        json={"title":"Comprehensive QA"}).json()["data"]["id"]
    print(f"Thread: {tid}")

    print(f"\n=== Running {len(QA)} QA Tests ===")
    pass_count = 0
    for i, qa in enumerate(QA):
        print(f"\n--- QA {i+1}/{len(QA)} ---")
        print(f"Q: {qa['q']}")
        print(f"Expect: '{qa['expect']}'")
        try:
            result = chat(token, tid, qa["q"])
            tc = [t for t in result["tool_calls"] if "label" in t]
            print(f"Tool calls: {len(tc)}")
            for t in tc:
                safe = str(t).encode('ascii','replace').decode()
                print(f"  {safe[:80]}")
            answer = result["answer"].strip()
            print(f"Answer: {len(answer)} chars")
            if qa["expect"].lower() in answer.lower():
                print("[PASS]")
                pass_count += 1
            else:
                print("[FAIL]")
        except Exception as e:
            print(f"[ERROR] {str(e)[:100]}")

    print(f"\n=== Results: {pass_count}/{len(QA)} passed ===")

if __name__ == "__main__":
    main()
