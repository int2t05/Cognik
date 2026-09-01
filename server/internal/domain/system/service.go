// Package system 聚合系统管理领域的业务逻辑层。
//
// service.go 合并原 audit / config / dashboard / message 四个 Service。
// AuditWriter 接口定义在此——各业务 Service 通过它写入审计日志，
// 不直接依赖 AuditRepo。ConfigService 持有 confidenceScoreQuerier 接口，
// 由 main 装配时注入 chat 仓库以计算置信度分位数。
package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"opsmind/internal/domain/chat"
	"opsmind/internal/shared/dto/request"
	"opsmind/internal/shared/dto/response"
	"opsmind/internal/shared/model"
	"opsmind/internal/shared/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// =============================================================================
// 审计日志 AuditService
// =============================================================================

// AuditWriter 定义审计日志写入接口。
//
// 各 Service 通过此接口写入审计日志，而非直接依赖 AuditRepo。
// 这样做的好处：
//   - locality：审计格式（操作人、来源、IP）变更只需改 AuditService.Write，不影响 5 个调用方
//   - leverage：一个接口，5 个 Service 调用方
//   - testability：Service 测试可注入假 AuditWriter，无需构造完整 AuditRepo
type AuditWriter interface {
	// Write 写入一条审计日志（使用 AuditService 持有的默认 DB 连接）。
	Write(ctx context.Context, operatorID int64, action, targetType string, targetID int64, detail string) error
	// WriteWithTx 在事务中写入审计日志（用于 Service 层事务内审计）。
	// tx 为 GORM 事务句柄，审计写入与业务操作在同一事务中提交或回滚。
	WriteWithTx(ctx context.Context, tx *gorm.DB, operatorID int64, action, targetType string, targetID int64, detail string) error
}

// AuditService 审计日志读写服务——唯一的审计接缝。
type AuditService struct {
	auditRepo *AuditRepo
}

// NewAuditService 创建 AuditService 实例。
func NewAuditService(auditRepo *AuditRepo) *AuditService {
	return &AuditService{auditRepo: auditRepo}
}

// buildAuditLog 构造 model.AuditLog 并处理 detail 字段的类型转换。
// detail 为空字符串时写入 NULL（PostgreSQL jsonb 接受），
// detail 为非空字符串时预期为已编码的 JSON 字节序列。
func (s *AuditService) buildAuditLog(operatorID int64, action, targetType string, targetID int64, detail string) *model.AuditLog {
	var jsonDetail datatypes.JSON
	if detail != "" {
		jsonDetail = datatypes.JSON(detail)
	}
	return &model.AuditLog{
		OperatorID: operatorID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     jsonDetail,
	}
}

// Write 实现 AuditWriter 接口——写入一条审计日志（非事务）。
// 调用方无需知道 model.AuditLog 的结构，只需传入业务字段。
func (s *AuditService) Write(ctx context.Context, operatorID int64, action, targetType string, targetID int64, detail string) error {
	return s.auditRepo.Create(ctx, s.buildAuditLog(operatorID, action, targetType, targetID, detail))
}

// WriteWithTx 在事务中写入审计日志——用于 Service 层事务内审计。
// tx 为 GORM 事务句柄，审计写入与业务操作在同一事务中提交或回滚。
func (s *AuditService) WriteWithTx(ctx context.Context, tx *gorm.DB, operatorID int64, action, targetType string, targetID int64, detail string) error {
	txRepo := NewAuditRepo(tx)
	return txRepo.Create(ctx, s.buildAuditLog(operatorID, action, targetType, targetID, detail))
}

// Create 直接写入审计日志记录（用于 ChatService 等已有完整 AuditLog 的调用方）。
// 实现 consumer-defined auditLogWriter 接口。
func (s *AuditService) Create(ctx context.Context, log any) error {
	return s.auditRepo.Create(ctx, log)
}

// List 分页查询审计日志（含操作人姓名，operatorID=0 映射为"系统"）。
func (s *AuditService) List(ctx context.Context, f AuditFilter) ([]response.AuditLogItem, int64, error) {
	rows, total, err := s.auditRepo.List(ctx, f)
	if err != nil {
		return nil, 0, err
	}

	items := make([]response.AuditLogItem, len(rows))
	for i, row := range rows {
		name := row.OperatorName
		if row.OperatorID == 0 {
			name = "系统"
		}
		items[i] = response.AuditLogItem{
			ID:           row.ID,
			OperatorID:   row.OperatorID,
			OperatorName: name,
			Action:       row.Action,
			TargetType:   row.TargetType,
			TargetID:     row.TargetID,
			Detail:       row.Detail,
			IPAddress:    row.IPAddress,
			CreatedAt:    row.CreatedAt,
		}
	}

	return items, total, nil
}

// BatchDelete 批量删除审计日志。
func (s *AuditService) BatchDelete(ctx context.Context, ids []int64) (int64, error) {
	return s.auditRepo.BatchDelete(ctx, ids)
}

// =============================================================================
// 系统配置 ConfigService
// =============================================================================

// configKeyMeta 定义配置键的元信息：期望类型和用途说明。
type configKeyMeta struct {
	ValueType   string // "string" | "number" | "bool"
	Description string // 配置项说明，写入 system_configs.description
}

// validConfigKeys 配置键白名单。
//
// 为什么用白名单而非自由 key-value：
// 自由 key-value 允许调用方写入任意键名，拼写错误导致静默创建无用配置项，
// 且前端无法区分「配置不存在」和「配置类型不符」。
var validConfigKeys = map[string]configKeyMeta{
	"app_name":                     {ValueType: "string", Description: "应用名称，显示在页面标题和系统通知中"},
	"ai.rag_enabled":               {ValueType: "bool", Description: "全局 RAG 检索开关（关闭后为纯 LLM 对话模式）"},
	"ai.top_k":                     {ValueType: "number", Description: "RAG 默认检索 Top K"},
	"ai.confidence_threshold_low":  {ValueType: "number", Description: "低置信阈值——Conf_raw 低于此值为低置信"},
	"ai.confidence_threshold_high": {ValueType: "number", Description: "高置信阈值——Conf_raw 达到此值为高置信"},
	"ai.max_history_messages":      {ValueType: "number", Description: "多轮对话历史消息数上限"},
	"ai.rag_query_rewrite":         {ValueType: "bool", Description: "RAG 查询改写开关"},
	"ai.rag_multi_route":           {ValueType: "bool", Description: "RAG 多路检索开关"},
	"ai.rag_hybrid":                {ValueType: "bool", Description: "RAG BM25 混合检索开关"},
	"ai.rag_rerank":                {ValueType: "bool", Description: "RAG 重排序开关"},
	"ai.enable_thinking":           {ValueType: "bool", Description: "流式回答启用思考模式（推理链提升质量但延迟 5-10x）"},
}

// ConfigService 系统配置管理服务。
type ConfigService struct {
	repo        *ConfigRepo
	auditWriter AuditWriter
	chatRepo    confidenceScoreQuerier
}

// confidenceScoreQuerier 分位数计算所需的置信度分数查询接口。
type confidenceScoreQuerier interface {
	QueryRawScores(ctx context.Context, days int) ([]float64, error)
}

// SetChatRepo 注入 chat 仓库依赖（避免构造循环依赖）。
func (s *ConfigService) SetChatRepo(r confidenceScoreQuerier) {
	s.chatRepo = r
}

// NewConfigService 创建 ConfigService 实例。
func NewConfigService(repo *ConfigRepo, auditWriter AuditWriter) *ConfigService {
	return &ConfigService{repo: repo, auditWriter: auditWriter}
}

// GetInt 读取整数配置，不存在或类型不匹配返回 (0, false)。
func (s *ConfigService) GetInt(ctx context.Context, key string) (int, bool) {
	v, err := s.GetConfig(ctx, key)
	if err != nil || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

// GetFloat 读取浮点配置，不存在或类型不匹配返回 (0, false)。
func (s *ConfigService) GetFloat(ctx context.Context, key string) (float64, bool) {
	v, err := s.GetConfig(ctx, key)
	if err != nil || v == nil {
		return 0, false
	}
	if n, ok := v.(float64); ok {
		return n, true
	}
	return 0, false
}

// GetBool 读取布尔配置，不存在或类型不匹配返回 (false, false)。
func (s *ConfigService) GetBool(ctx context.Context, key string) (bool, bool) {
	v, err := s.GetConfig(ctx, key)
	if err != nil || v == nil {
		return false, false
	}
	if b, ok := v.(bool); ok {
		return b, true
	}
	return false, false
}

// GetConfig 获取指定 key 的配置值。
func (s *ConfigService) GetConfig(ctx context.Context, key string) (interface{}, error) {
	if _, ok := validConfigKeys[key]; !ok {
		return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: fmt.Sprintf("配置项 %s 不存在", key)}
	}

	cfg, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 有效 key 但尚未初始化：返回 null 而非报错
		}
		return nil, err
	}

	var value interface{}
	if err := json.Unmarshal(cfg.Value, &value); err != nil {
		return nil, fmt.Errorf("解析配置值失败: %w", err)
	}

	return value, nil
}

// UpdateConfig 更新或创建系统配置。
//
// value 会被序列化为 JSONB 存储，nil 被拒绝。
// 仅允许白名单内的 key 写入，同时写入白名单中对应的 description。
func (s *ConfigService) UpdateConfig(ctx context.Context, key string, value interface{}, updatedBy int64) error {
	meta, ok := validConfigKeys[key]
	if !ok {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: fmt.Sprintf("配置项 %s 不存在", key)}
	}
	if value == nil {
		return errcode.AppError{Code: errcode.ErrParam, Message: "配置值不能为 nil"}
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化配置值失败: %w", err)
	}

	if err := s.repo.Upsert(ctx, key, meta.Description, datatypes.JSON(jsonBytes), updatedBy); err != nil {
		return err
	}
	s.auditWriter.Write(ctx, updatedBy, "config.update", "config", 0, string(jsonBytes))
	return nil
}

// ComputeThresholdsResult 分位数计算结果。
type ComputeThresholdsResult struct {
	P30         float64 `json:"p30"`
	P70         float64 `json:"p70"`
	SampleCount int     `json:"sample_count"`
	DateFrom    string  `json:"date_from"`
	DateTo      string  `json:"date_to"`
	Warning     string  `json:"warning,omitempty"`
}

// ComputeThresholds 从近 N 天 chat_messages 的 confidence_raw 中计算 P30/P70 分位数。
func (s *ConfigService) ComputeThresholds(ctx context.Context, days int) (*ComputeThresholdsResult, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	if s.chatRepo == nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}

	scores, err := s.chatRepo.QueryRawScores(ctx, days)
	if err != nil {
		return nil, fmt.Errorf("查询原始置信度分数失败: %w", err)
	}

	n := len(scores)
	if n == 0 {
		return &ComputeThresholdsResult{
			P30: 0.40, P70: 0.70, SampleCount: 0, Warning: "无可用数据，返回默认值",
		}, nil
	}

	p30 := percentile(scores, 0.30)
	p70 := percentile(scores, 0.70)

	if p30 < 0.10 {
		p30 = 0.10
	}
	if p70 > 0.95 {
		p70 = 0.95
	}
	if p70-p30 < 0.10 {
		p70 = p30 + 0.10
		if p70 > 0.95 {
			p70 = 0.95
		}
	}

	var warning string
	if n < 50 {
		warning = fmt.Sprintf("样本数量不足（%d < 50），建议积累更多数据后重新计算", n)
	}

	return &ComputeThresholdsResult{
		P30:         round2(p30),
		P70:         round2(p70),
		SampleCount: n,
		Warning:     warning,
	}, nil
}

// percentile 线性插值法计算第 p 百分位数（scores 须已升序）。
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	idx := p * float64(n-1)
	lo := int(idx)
	hi := lo + 1
	if hi >= n {
		return sorted[n-1]
	}
	return sorted[lo]*(1-(idx-float64(lo))) + sorted[hi]*(idx-float64(lo))
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// IsPublicKey 判断指定 key 是否为无需认证的公开配置项。
// Handler 层通过此方法判断是否允许公开访问，而非自行维护白名单。
func (s *ConfigService) IsPublicKey(key string) bool {
	return key == "app_name"
}

// =============================================================================
// 数据看板 DashboardService
// =============================================================================

const maxTrendDays = 90

// DashboardService 数据看板服务。
type DashboardService struct {
	repo dashboardRepo
}

type dashboardRepo interface {
	CountTodayTickets(ctx context.Context) (int64, error)
	CountByStatus(ctx context.Context, status int16) (int64, error)
	CountTodayChats(ctx context.Context) (int64, error)
	AvgTodayConfidence(ctx context.Context) (float64, error)
	CountKnowledgeArticles(ctx context.Context) (int64, error)
	CountFeedbackByType(ctx context.Context, feedbackType int16) (int64, error)
	GetTicketTrends(ctx context.Context, startDate, endDate, granularity string) ([]TrendPoint, error)
	GetChatTrends(ctx context.Context, startDate, endDate string, granularity string) ([]TrendPoint, error)
}

// NewDashboardService 创建 DashboardService 实例。
func NewDashboardService(repo dashboardRepo) *DashboardService {
	return &DashboardService{repo: repo}
}

// GetStats 获取看板统计数据（7 项查询并行执行）。
func (s *DashboardService) GetStats(ctx context.Context) (*response.StatsResponse, error) {
	queryCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		resp     response.StatsResponse
		mu       sync.Mutex
		wg       sync.WaitGroup
		firstErr error
		once     sync.Once
	)

	setErr := func(err error) {
		once.Do(func() {
			firstErr = err
			cancel()
		})
	}

	wg.Add(9)
	go func() {
		defer wg.Done()
		count, err := s.repo.CountTodayTickets(queryCtx)
		mu.Lock()
		resp.TodayTickets = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountByStatus(queryCtx, 1)
		mu.Lock()
		resp.PendingTickets = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountByStatus(queryCtx, 2)
		mu.Lock()
		resp.ProcessingTickets = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountByStatus(queryCtx, 4)
		mu.Lock()
		resp.ResolvedTickets = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountTodayChats(queryCtx)
		mu.Lock()
		resp.TodayChats = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		avg, err := s.repo.AvgTodayConfidence(queryCtx)
		mu.Lock()
		resp.AvgConfidence = avg
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountKnowledgeArticles(queryCtx)
		mu.Lock()
		resp.KnowledgeCount = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountFeedbackByType(queryCtx, 1)
		mu.Lock()
		resp.HelpfulFeedback = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountFeedbackByType(queryCtx, 2)
		mu.Lock()
		resp.UnhelpfulFeedback = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return &resp, nil
}

// GetTrends 获取趋势数据（支持 day/week 粒度，上限 90 天）。
func (s *DashboardService) GetTrends(ctx context.Context, req request.TrendRequest) (*response.TrendResponse, error) {
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "开始日期格式错误，格式应为 YYYY-MM-DD"}
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "结束日期格式错误，格式应为 YYYY-MM-DD"}
	}
	if endDate.Before(startDate) {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "结束日期不能早于开始日期"}
	}
	if endDate.Sub(startDate) > maxTrendDays*24*time.Hour {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: fmt.Sprintf("日期范围不能超过 %d 天", maxTrendDays)}
	}

	// 生成日期序列（按 day 或 week 粒度）
	granularity := req.Granularity
	if granularity != "week" {
		granularity = "day"
	}

	var labels []string
	if granularity == "week" {
		// 对齐到周一
		cur := startDate
		for cur.Weekday() != time.Monday {
			cur = cur.AddDate(0, 0, -1)
		}
		for !cur.After(endDate) {
			labels = append(labels, cur.Format("2006-01-02"))
			cur = cur.AddDate(0, 0, 7)
		}
	} else {
		cur := startDate
		for !cur.After(endDate) {
			labels = append(labels, cur.Format("2006-01-02"))
			cur = cur.AddDate(0, 0, 1)
		}
	}

	dataPoints := make([]response.DataPoint, len(labels))
	for i, d := range labels {
		dataPoints[i] = response.DataPoint{Date: d, TicketCount: 0, ChatCount: 0}
	}

	// 查询趋势（已支持 granularity 参数）
	ticketCounts, err := s.repo.GetTicketTrends(ctx, req.StartDate, req.EndDate, granularity)
	if err != nil {
		return nil, fmt.Errorf("查询每日申告数失败: %w", err)
	}
	ticketMap := make(map[string]int64, len(ticketCounts))
	for _, tc := range ticketCounts {
		ticketMap[tc.Date] = tc.Count
	}

	chatCounts, err := s.repo.GetChatTrends(ctx, req.StartDate, req.EndDate, granularity)
	if err != nil {
		return nil, fmt.Errorf("查询每日问答数失败: %w", err)
	}
	chatMap := make(map[string]int64, len(chatCounts))
	for _, cc := range chatCounts {
		chatMap[cc.Date] = cc.Count
	}

	// O(n) 填充（替代 O(n²) 双重循环）
	for i, dp := range dataPoints {
		dataPoints[i].TicketCount = ticketMap[dp.Date]
		dataPoints[i].ChatCount = chatMap[dp.Date]
	}

	return &response.TrendResponse{DataPoints: dataPoints}, nil
}

// =============================================================================
// 站内消息 MessageService
// =============================================================================

// MessageService 站内消息服务。
type MessageService struct {
	repo        *chat.MessageRepo
	cacheTTL    time.Duration
	unreadMu    sync.RWMutex
	unreadCache map[int64]unreadCountCacheEntry
}

type unreadCountCacheEntry struct {
	count     int64
	expiresAt time.Time
}

const defaultUnreadCountCacheTTL = 15 * time.Second

// NewMessageService 创建 MessageService 实例。
func NewMessageService(repo *chat.MessageRepo) *MessageService {
	return NewMessageServiceWithCacheTTL(repo, defaultUnreadCountCacheTTL)
}

// NewMessageServiceWithCacheTTL 创建 MessageService 实例，并允许测试覆盖未读数缓存 TTL。
func NewMessageServiceWithCacheTTL(repo *chat.MessageRepo, ttl time.Duration) *MessageService {
	return &MessageService{
		repo:        repo,
		cacheTTL:    ttl,
		unreadCache: make(map[int64]unreadCountCacheEntry),
	}
}

// =============================================================================
// 通知方法（被各业务 Service 调用）
// =============================================================================

func (s *MessageService) notify(ctx context.Context, userID int64, title, content, msgType, relatedType string, relatedID int64) error {
	msg := &model.Message{
		UserID: userID, Title: title, Content: content,
		Type: msgType, RelatedType: relatedType, RelatedID: relatedID, IsRead: false,
	}
	if err := s.repo.Create(ctx, msg); err != nil {
		return err
	}
	s.invalidateUnread(userID)
	return nil
}

// NotifySupplement 通知申告人补充信息（TicketService.request_info 调用）。
func (s *MessageService) NotifySupplement(ctx context.Context, ticketID int64, userID int64, ticketTitle string) error {
	content := "您的申告需要补充更多信息，请尽快登录系统查看并补充相关材料。"
	if ticketTitle != "" {
		content = fmt.Sprintf("您的申告「%s」需要补充更多信息，请尽快登录系统查看并补充相关材料。", ticketTitle)
	}
	return s.notify(ctx, userID, "申告需补充信息", content, model.MessageTypeTicketSupplement, "ticket", ticketID)
}

// NotifyTicketResolved 通知申告人申告已解决（TicketService 状态变更为已解决时调用）。
func (s *MessageService) NotifyTicketResolved(ctx context.Context, ticketID int64, userID int64, ticketTitle string) error {
	content := fmt.Sprintf("您的申告「%s」已被标记为已解决，如有疑问请联系运维人员。", ticketTitle)
	return s.notify(ctx, userID, "申告已解决", content, model.MessageTypeTicketResolved, "ticket", ticketID)
}

// NotifyTicketClosed 通知申告人申告已关闭（TicketService 状态变更为已关闭时调用）。
func (s *MessageService) NotifyTicketClosed(ctx context.Context, ticketID int64, userID int64, ticketTitle string) error {
	content := fmt.Sprintf("您的申告「%s」已被关闭，如有需要请重新提交申告。", ticketTitle)
	return s.notify(ctx, userID, "申告已关闭", content, model.MessageTypeTicketClosed, "ticket", ticketID)
}

// NotifyKnowledgeReviewed 通知文章作者审核结果（KnowledgeService.Review 调用）。
func (s *MessageService) NotifyKnowledgeReviewed(ctx context.Context, articleID int64, articleTitle string, userID int64, approved bool, comment string) error {
	if approved {
		content := fmt.Sprintf("您的文章「%s」已通过审核，可前往发布。", articleTitle)
		return s.notify(ctx, userID, "文章审核通过", content, model.MessageTypeKnowledgeApproved, "knowledge_article", articleID)
	}
	content := fmt.Sprintf("您的文章「%s」已被驳回", articleTitle)
	if comment != "" {
		content += "，原因：" + comment
	}
	return s.notify(ctx, userID, "文章被驳回", content, model.MessageTypeKnowledgeRejected, "knowledge_article", articleID)
}

// =============================================================================
// 查询和操作
// =============================================================================

// MessageFilter 消息列表过滤条件。
//
// 在 Service 层定义而非直接暴露 chat.MessageFilter，
// 避免上层（Handler）依赖 Chat 域的数据访问层类型。
type MessageFilter struct {
	IsRead *bool
	Type   string
}

// ListMessages 分页查询用户消息列表，支持按 is_read/type 过滤。
func (s *MessageService) ListMessages(ctx context.Context, userID int64, page, pageSize int, filter MessageFilter) ([]model.Message, int64, error) {
	if userID <= 0 {
		return nil, 0, errcode.AppError{Code: errcode.ErrParam, Message: "无效的用户 ID"}
	}
	return s.repo.ListByUser(ctx, userID, page, pageSize, chat.MessageFilter{IsRead: filter.IsRead, Type: filter.Type})
}

// MarkAsRead 将指定用户的消息标记为已读。
//
// 校验消息归属（userID），防止水平越权：用户 A 不能标记用户 B 的消息已读。
// 消息不存在或不属于该用户时返回 AppError{Code: ErrNotFound}。
func (s *MessageService) MarkAsRead(ctx context.Context, id int64, userID int64) error {
	if userID <= 0 {
		return errcode.AppError{Code: errcode.ErrParam, Message: "无效的用户 ID"}
	}
	if err := s.repo.MarkAsRead(ctx, id, userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "消息不存在"}
		}
		return err
	}
	s.invalidateUnread(userID)
	return nil
}

// MarkAsReadAndCount 标记消息已读并返回最新未读计数。
//
// 合并两次操作减少前端请求数：标记已读后直接返回 unread_count，
// 前端无需额外调用 CountUnread 即可更新未读角标。
func (s *MessageService) MarkAsReadAndCount(ctx context.Context, id int64, userID int64) (int64, error) {
	if err := s.MarkAsRead(ctx, id, userID); err != nil {
		return 0, err
	}
	count, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return 0, err
	}
	s.setCachedUnread(userID, count)
	return count, nil
}

// MarkAllRead 将用户所有未读消息标记为已读，返回操作影响的条数。
func (s *MessageService) MarkAllRead(ctx context.Context, userID int64) (int64, error) {
	if userID <= 0 {
		return 0, errcode.AppError{Code: errcode.ErrParam, Message: "无效的用户 ID"}
	}
	affected, err := s.repo.MarkAllRead(ctx, userID)
	if err != nil {
		return 0, err
	}
	s.invalidateUnread(userID)
	return affected, nil
}

// CountUnread 查询指定用户的未读消息数。
func (s *MessageService) CountUnread(ctx context.Context, userID int64) (int64, error) {
	if userID <= 0 {
		return 0, errcode.AppError{Code: errcode.ErrParam, Message: "无效的用户 ID"}
	}
	if count, ok := s.getCachedUnread(userID); ok {
		return count, nil
	}
	count, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return 0, err
	}
	s.setCachedUnread(userID, count)
	return count, nil
}

func (s *MessageService) getCachedUnread(userID int64) (int64, bool) {
	if s.cacheTTL <= 0 {
		return 0, false
	}
	s.unreadMu.RLock()
	entry, ok := s.unreadCache[userID]
	s.unreadMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			s.invalidateUnread(userID)
		}
		return 0, false
	}
	return entry.count, true
}

func (s *MessageService) setCachedUnread(userID int64, count int64) {
	if s.cacheTTL <= 0 {
		return
	}
	s.unreadMu.Lock()
	s.unreadCache[userID] = unreadCountCacheEntry{
		count:     count,
		expiresAt: time.Now().Add(s.cacheTTL),
	}
	s.unreadMu.Unlock()
}

func (s *MessageService) invalidateUnread(userID int64) {
	if s.cacheTTL <= 0 {
		return
	}
	s.unreadMu.Lock()
	delete(s.unreadCache, userID)
	s.unreadMu.Unlock()
}
