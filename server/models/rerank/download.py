"""
下载 cross-encoder 模型文件。逐文件 HTTP 下载，无需 SDK，支持镜像。

用法：
    python models/rerank/download.py

默认：MiniLM-L-4-v2（~50MB），优先 hf-mirror.com → huggingface.co
"""

import os
import sys
import time

MODEL = "cross-encoder/ms-marco-MiniLM-L-4-v2"
MODEL_DIR = os.path.dirname(os.path.abspath(__file__))

# 镜像源（按优先级）
MIRRORS = [
    "https://hf-mirror.com",
    "https://huggingface.co",
]

# 模型需要的文件
FILES = [
    "config.json",
    "model.safetensors",
    "tokenizer_config.json",
    "vocab.txt",
    "special_tokens_map.json",
    "sentence_bert_config.json",
    "modules.json",
]

# 已全部下载则跳过
if all(os.path.exists(os.path.join(MODEL_DIR, f)) for f in ["config.json", "model.safetensors", "vocab.txt"]):
    total = sum(os.path.getsize(os.path.join(MODEL_DIR, f))
                for f in os.listdir(MODEL_DIR) if os.path.isfile(os.path.join(MODEL_DIR, f))) / (1024 * 1024)
    print(f"模型已存在 ({total:.1f} MB)")
    sys.exit(0)

import requests

def try_download(url, dst, retries=3):
    """逐文件下载，带进度条和重试。"""
    for attempt in range(retries):
        try:
            resp = requests.get(url, stream=True, timeout=30)
            if resp.status_code == 200:
                total_size = int(resp.headers.get("content-length", 0))
                downloaded = 0
                with open(dst, "wb") as f:
                    for chunk in resp.iter_content(chunk_size=8192):
                        f.write(chunk)
                        downloaded += len(chunk)
                # 校验
                actual = os.path.getsize(dst)
                if total_size > 0 and actual != total_size:
                    os.remove(dst)
                    print(f"  文件不完整 ({actual}/{total_size})，重试...")
                    continue
                return True
            elif resp.status_code == 404:
                return False  # 文件不存在，跳过
            else:
                print(f"  HTTP {resp.status_code}，重试...")
        except Exception as e:
            print(f"  错误: {e}，重试...")
        time.sleep(2 ** attempt)
    return False

# 逐个文件尝试所有镜像
print(f"下载 {MODEL}...")
for fname in FILES:
    dst = os.path.join(MODEL_DIR, fname)
    if os.path.exists(dst):
        print(f"  {fname} — 已跳过")
        continue

    downloaded = False
    for mirror in MIRRORS:
        url = f"{mirror}/{MODEL}/resolve/main/{fname}"
        print(f"  {fname} ← {mirror}", end="", flush=True)
        if try_download(url, dst):
            size_mb = os.path.getsize(dst) / (1024 * 1024)
            print(f" ({size_mb:.1f} MB)")
            downloaded = True
            break
        print(" 失败")

    if not downloaded:
        print(f"\n错误: 所有镜像均无法下载 {fname}")
        print("请手动将以上文件放入 " + MODEL_DIR)
        sys.exit(1)

total = sum(os.path.getsize(os.path.join(MODEL_DIR, f))
            for f in os.listdir(MODEL_DIR) if os.path.isfile(os.path.join(MODEL_DIR, f))) / (1024 * 1024)
print(f"\n完成 ({total:.1f} MB)")
