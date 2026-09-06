// Package scifact 提供 BEIR SciFact 全量数据集的加载与检索质量评估。
//
// 数据源：BEIR 官方 SciFact（5183 文档 + 1109 查询 + 339 条 test 相关性标注）。
// 首次运行自动下载并解压到 benchmark/.cache/scifact/（git-ignored）。
// 后续运行复用缓存,不重复下载。
package scifact

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SciFactURL BEIR 官方下载地址。
const SciFactURL = "https://public.ukp.informatik.tu-darmstadt.de/thakur/BEIR/datasets/scifact.zip"

// CacheDir 数据集缓存目录(benchmark/.cache/scifact/,git-ignored)。
const CacheDir = "benchmark/.cache/scifact"

// SciFactDoc 文档(corpus.jsonl 一行)。
type SciFactDoc struct {
	ID    string `json:"_id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// SciFactQuery 查询(queries.jsonl 一行)。
type SciFactQuery struct {
	ID   string `json:"_id"`
	Text string `json:"text"`
}

// QrelRel 一条相关性标注(qrels/test.tsv 一行)。
type QrelRel struct {
	QueryID string
	DocID   string
	Score   int
}

// LoadSciFact 加载 SciFact 全量数据集。
// 首次运行下载 zip 并解压到 CacheDir;后续复用缓存。
// 返回:文档列表、test 查询列表(仅有 qrels 标注的)、test 相关性映射。
func LoadSciFact() (docs []SciFactDoc, queries []SciFactQuery, qrels map[string]map[string]int, err error) {
	// 确保缓存目录存在
	if err := os.MkdirAll(CacheDir, 0755); err != nil {
		return nil, nil, nil, fmt.Errorf("创建缓存目录失败: %w", err)
	}

	corpusPath := filepath.Join(CacheDir, "scifact", "corpus.jsonl")
	queriesPath := filepath.Join(CacheDir, "scifact", "queries.jsonl")
	qrelsPath := filepath.Join(CacheDir, "scifact", "qrels", "test.tsv")

	// 缓存不存在则下载解压
	if _, e := os.Stat(corpusPath); os.IsNotExist(e) {
		if err := downloadAndExtract(SciFactURL, CacheDir); err != nil {
			return nil, nil, nil, fmt.Errorf("下载 SciFact 失败: %w", err)
		}
	}

	// 解析 corpus.jsonl
	docs, err = parseCorpus(corpusPath)
	if err != nil {
		return nil, nil, nil, err
	}

	// 解析 queries.jsonl
	allQueries, err := parseQueries(queriesPath)
	if err != nil {
		return nil, nil, nil, err
	}

	// 解析 qrels/test.tsv
	qrels, err = parseQrels(qrelsPath)
	if err != nil {
		return nil, nil, nil, err
	}

	// 仅保留有 qrels 标注的 test 查询
	querySet := make(map[string]bool, len(qrels))
	for qid := range qrels {
		querySet[qid] = true
	}
	for _, q := range allQueries {
		if querySet[q.ID] {
			queries = append(queries, q)
		}
	}

	return docs, queries, qrels, nil
}

// parseCorpus 解析 corpus.jsonl。
func parseCorpus(path string) ([]SciFactDoc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开 corpus.jsonl 失败: %w", err)
	}
	defer f.Close()

	var docs []SciFactDoc
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 大行缓冲(corpus 摘要可达数 KB)
	for scanner.Scan() {
		var doc SciFactDoc
		if err := json.Unmarshal(scanner.Bytes(), &doc); err != nil {
			continue // 跳过解析失败的行
		}
		docs = append(docs, doc)
	}
	return docs, scanner.Err()
}

// parseQueries 解析 queries.jsonl。
func parseQueries(path string) ([]SciFactQuery, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开 queries.jsonl 失败: %w", err)
	}
	defer f.Close()

	var queries []SciFactQuery
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var q SciFactQuery
		if err := json.Unmarshal(scanner.Bytes(), &q); err != nil {
			continue
		}
		queries = append(queries, q)
	}
	return queries, scanner.Err()
}

// parseQrels 解析 qrels/test.tsv。
// 格式：query-id\tcorpus-id\tscore(首行表头)。
// 返回 queryID → docID → score 映射。
func parseQrels(path string) (map[string]map[string]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开 qrels/test.tsv 失败: %w", err)
	}
	defer f.Close()

	qrels := make(map[string]map[string]int)
	scanner := bufio.NewScanner(f)
	firstLine := true
	for scanner.Scan() {
		line := scanner.Text()
		if firstLine {
			firstLine = false
			continue // 跳过表头
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		queryID, docID := parts[0], parts[1]
		var score int
		fmt.Sscanf(parts[2], "%d", &score)
		if qrels[queryID] == nil {
			qrels[queryID] = make(map[string]int)
		}
		qrels[queryID][docID] = score
	}
	return qrels, scanner.Err()
}

// downloadAndExtract 下载 zip 并解压到 destDir。
func downloadAndExtract(url, destDir string) error {
	// 下载 zip
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	// 读取 zip 到内存(SciFact ~2.8MB)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应体失败: %w", err)
	}

	// 在内存中解压 zip
	zipReader, err := zip.NewReader(strings.NewReader(string(body)), int64(len(body)))
	if err != nil {
		return fmt.Errorf("解析 zip 失败: %w", err)
	}

	for _, file := range zipReader.File {
		// 跳过目录
		if file.FileInfo().IsDir() {
			continue
		}
		// 构建目标路径(zip 内顶层是 scifact/，解压到 destDir/scifact/）
		outPath := filepath.Join(destDir, file.Name)
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return fmt.Errorf("创建子目录失败: %w", err)
		}
		// 解压文件
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("打开 zip 内文件失败: %w", err)
		}
		outFile, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("创建文件失败: %w", err)
		}
		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return fmt.Errorf("解压写入失败: %w", err)
		}
	}

	return nil
}
