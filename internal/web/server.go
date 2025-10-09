package web

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	domain "best_trade_logs/internal/domain/trade"
	tradesvc "best_trade_logs/internal/service/trade"
	"best_trade_logs/internal/storage"
	"best_trade_logs/internal/web/templates"
)

// Server wires the HTTP layer with the trade service.
type Server struct {
	svc       *tradesvc.Service
	templates *templates.Engine
}

// NewServer builds a Server with embedded templates parsed.
func NewServer(svc *tradesvc.Service) (*Server, error) {
	tmpl, err := templates.New()
	if err != nil {
		return nil, err
	}
	return &Server{svc: svc, templates: tmpl}, nil
}

// Handler exposes the configured HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/trades", s.handleTrades)
	mux.HandleFunc("/trades/new", s.handleNewTrade)
	mux.HandleFunc("/trades/", s.handleTradeRoutes)
	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	trades, err := s.svc.List(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filters := parseIndexFilters(r)
	filtered := applyIndexFilters(trades, filters)

	summaries := make([]tradeSummary, 0, len(filtered))
	now := time.Now().UTC()
	for _, tr := range filtered {
		summary := tradeSummary{
			Trade:         tr,
			NetResult:     tr.NetResult(),
			ResultPercent: tr.ResultPercent(),
			RMultiple:     tr.RMultiple(),
			Status:        tradeStatus(tr),
			IsOpen:        !tr.HasExited(),
		}
		if v, ok := tr.FollowUpChangePercent(7); ok {
			val := v
			summary.FollowUp7 = &val
		}
		if v, ok := tr.FollowUpChangePercent(30); ok {
			val := v
			summary.FollowUp30 = &val
		}
		if hold, ok := holdDays(tr, now); ok {
			summary.HoldDays = hold
			summary.HasHold = true
		}
		summaries = append(summaries, summary)
	}

	metrics := summarizeTrades(filtered, now)
	tags := collectTags(trades)
	data := struct {
		Title         string
		Trades        []tradeSummary
		Flash         string
		Metrics       dashboardMetrics
		Filters       indexFilters
		TotalTrades   int
		VisibleTrades int
		Tags          []string
	}{
		Title:         "交易日誌",
		Trades:        summaries,
		Flash:         r.URL.Query().Get("flash"),
		Metrics:       metrics,
		Filters:       filters,
		TotalTrades:   len(trades),
		VisibleTrades: len(filtered),
		Tags:          tags,
	}

	s.render(w, "index.gohtml", data)
}

func (s *Server) handleTrades(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleCreateTrade(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleNewTrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	tr := &domain.Trade{}
	tr.Direction = domain.DirectionLong
	data := map[string]interface{}{
		"Title":  "新增交易",
		"Trade":  tr,
		"Action": "/trades",
		"Form":   newTradeFormData(tr, true),
	}
	s.render(w, "trade_form.gohtml", data)
}

func (s *Server) handleTradeRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/trades/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		s.handleShowTrade(w, r, id)
	case len(parts) == 2 && parts[1] == "edit" && r.Method == http.MethodGet:
		s.handleEditTrade(w, r, id)
	case len(parts) == 2 && parts[1] == "update" && r.Method == http.MethodPost:
		s.handleUpdateTrade(w, r, id)
	case len(parts) == 2 && parts[1] == "delete" && r.Method == http.MethodPost:
		s.handleDeleteTrade(w, r, id)
	case len(parts) == 2 && parts[1] == "followups" && r.Method == http.MethodPost:
		s.handleAddFollowUp(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleCreateTrade(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "表單格式錯誤", http.StatusBadRequest)
		return
	}
	tr, errs := buildTradeFromForm(r)
	if len(errs) > 0 {
		http.Error(w, strings.Join(errs, "; "), http.StatusBadRequest)
		return
	}
	if err := s.svc.Create(r.Context(), tr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/trades/%s?flash=%s", tr.ID, url.QueryEscape("交易已建立")), http.StatusSeeOther)
}

func (s *Server) handleShowTrade(w http.ResponseWriter, r *http.Request, id string) {
	tr, err := s.svc.Get(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, storage.ErrNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	metrics := buildTradeMetrics(tr, r.URL.Query().Get("close_price"))

	data := struct {
		Title      string
		Trade      *domain.Trade
		Metrics    tradeMetrics
		QueryClose *float64
		Flash      string
	}{
		Title:      fmt.Sprintf("交易 - %s", tr.Instrument),
		Trade:      tr,
		Metrics:    metrics,
		QueryClose: metrics.QueryClose,
		Flash:      r.URL.Query().Get("flash"),
	}
	s.render(w, "trade_detail.gohtml", data)
}

func (s *Server) handleEditTrade(w http.ResponseWriter, r *http.Request, id string) {
	tr, err := s.svc.Get(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, storage.ErrNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	data := map[string]interface{}{
		"Title":  "編輯交易",
		"Trade":  tr,
		"Action": fmt.Sprintf("/trades/%s/update", tr.ID),
		"Form":   newTradeFormData(tr, false),
	}
	s.render(w, "trade_form.gohtml", data)
}

func (s *Server) handleUpdateTrade(w http.ResponseWriter, r *http.Request, id string) {
	existing, err := s.svc.Get(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, storage.ErrNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "表單格式錯誤", http.StatusBadRequest)
		return
	}
	tr, errs := buildTradeFromForm(r)
	if len(errs) > 0 {
		http.Error(w, strings.Join(errs, "; "), http.StatusBadRequest)
		return
	}
	tr.ID = existing.ID
	tr.CreatedAt = existing.CreatedAt
	tr.FollowUps = existing.FollowUps
	if err := s.svc.Update(r.Context(), tr); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, storage.ErrNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/trades/%s?flash=%s", tr.ID, url.QueryEscape("交易已更新")), http.StatusSeeOther)
}

func (s *Server) handleDeleteTrade(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.svc.Delete(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, storage.ErrNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/?flash=%s", url.QueryEscape("交易已刪除")), http.StatusSeeOther)
}

func (s *Server) handleAddFollowUp(w http.ResponseWriter, r *http.Request, id string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "表單格式錯誤", http.StatusBadRequest)
		return
	}
	daysStr := normalizeIntegerInput(r.FormValue("days_after"))
	if daysStr == "" {
		http.Error(w, "天數格式錯誤", http.StatusBadRequest)
		return
	}
	days, err := strconv.Atoi(daysStr)
	if err != nil {
		http.Error(w, "天數格式錯誤", http.StatusBadRequest)
		return
	}
	priceStr := normalizeNumericInput(r.FormValue("price"))
	if priceStr == "" {
		http.Error(w, "價格格式錯誤", http.StatusBadRequest)
		return
	}
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		http.Error(w, "價格格式錯誤", http.StatusBadRequest)
		return
	}
	follow := domain.FollowUp{DaysAfter: days, Price: price, Notes: strings.TrimSpace(r.FormValue("notes"))}
	if err := s.svc.AddFollowUp(r.Context(), id, follow); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, storage.ErrNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/trades/%s?flash=%s", id, url.QueryEscape("已新增後續追蹤")), http.StatusSeeOther)
}

func (s *Server) render(w http.ResponseWriter, name string, data interface{}) {
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, fmt.Sprintf("template render error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := buf.WriteTo(w); err != nil {
		log.Printf("template write error for %s: %v", name, err)
	}
}

type tradeSummary struct {
	*domain.Trade
	NetResult     float64
	ResultPercent float64
	RMultiple     float64
	FollowUp7     *float64
	FollowUp30    *float64
	Status        string
	HoldDays      float64
	HasHold       bool
	IsOpen        bool
}

type tradeMetrics struct {
	Net           float64
	NetPercent    float64
	RMultiple     float64
	TotalRisk     float64
	TargetR       float64
	FollowUp7     *float64
	FollowUp30    *float64
	Unrealized    float64
	UnrealizedPct float64
	QueryClose    *float64
}

func buildTradeMetrics(tr *domain.Trade, closePrice string) tradeMetrics {
	metrics := tradeMetrics{
		Net:        tr.NetResult(),
		NetPercent: tr.ResultPercent(),
		RMultiple:  tr.RMultiple(),
		TotalRisk:  tr.TotalRiskAmount(),
		TargetR:    tr.EffectiveRewardTarget(),
	}
	if v, ok := tr.FollowUpChangePercent(7); ok {
		val := v
		metrics.FollowUp7 = &val
	}
	if v, ok := tr.FollowUpChangePercent(30); ok {
		val := v
		metrics.FollowUp30 = &val
	}
	if strings.TrimSpace(closePrice) != "" {
		if v, err := strconv.ParseFloat(strings.TrimSpace(closePrice), 64); err == nil {
			metrics.Unrealized = tr.UnrealizedResult(v)
			metrics.UnrealizedPct = tr.UnrealizedPercent(v)
			metrics.QueryClose = &v
		}
	}
	return metrics
}

type indexFilters struct {
	Instrument string
	Direction  string
	Status     string
	Tag        string
}

func (f indexFilters) Active() bool {
	return f.Instrument != "" || f.Direction != "" || f.Status != "" || f.Tag != ""
}

type dashboardMetrics struct {
	Total        int
	Closed       int
	Open         int
	WinRate      float64
	AvgR         float64
	AvgHoldDays  float64
	AvgReturnPct float64
	TotalNet     float64
	OpenRisk     float64
}

func parseIndexFilters(r *http.Request) indexFilters {
	q := r.URL.Query()
	filters := indexFilters{
		Instrument: strings.TrimSpace(q.Get("instrument")),
		Direction:  strings.ToUpper(strings.TrimSpace(q.Get("direction"))),
		Status:     strings.ToLower(strings.TrimSpace(q.Get("status"))),
		Tag:        strings.ToLower(strings.TrimSpace(q.Get("tag"))),
	}
	if filters.Direction != string(domain.DirectionLong) && filters.Direction != string(domain.DirectionShort) {
		filters.Direction = ""
	}
	switch filters.Status {
	case "open", "closed", "wins", "losses":
	default:
		filters.Status = ""
	}
	if filters.Tag != "" {
		filters.Tag = normalizeTag(filters.Tag)
	}
	return filters
}

func applyIndexFilters(trades []*domain.Trade, filters indexFilters) []*domain.Trade {
	if !filters.Active() {
		return trades
	}

	filtered := make([]*domain.Trade, 0, len(trades))
	needle := strings.ToLower(filters.Instrument)
	for _, tr := range trades {
		if needle != "" {
			instrument := strings.ToLower(tr.Instrument)
			market := strings.ToLower(tr.Market)
			setup := strings.ToLower(tr.Setup)
			if !strings.Contains(instrument, needle) && !strings.Contains(market, needle) && !strings.Contains(setup, needle) {
				continue
			}
		}
		if filters.Direction != "" && string(tr.Direction) != filters.Direction {
			continue
		}
		switch filters.Status {
		case "open":
			if tr.HasExited() {
				continue
			}
		case "closed":
			if !tr.HasExited() {
				continue
			}
		case "wins":
			if !tr.HasExited() || tr.NetResult() <= 0 {
				continue
			}
		case "losses":
			if !tr.HasExited() || tr.NetResult() >= 0 {
				continue
			}
		}
		if filters.Tag != "" {
			match := false
			for _, tag := range tr.Review.Tags {
				if normalizeTag(tag) == filters.Tag {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		filtered = append(filtered, tr)
	}
	return filtered
}

func summarizeTrades(trades []*domain.Trade, now time.Time) dashboardMetrics {
	metrics := dashboardMetrics{}
	metrics.Total = len(trades)
	if len(trades) == 0 {
		return metrics
	}

	var winCount int
	var rTotal float64
	var rSamples int
	var holdTotal float64
	var holdSamples int
	var returnTotal float64
	var returnSamples int

	for _, tr := range trades {
		net := tr.NetResult()
		metrics.TotalNet += net
		if tr.HasExited() {
			metrics.Closed++
			if net > 0 {
				winCount++
			}
			if tr.TotalRiskAmount() > 0 {
				rTotal += tr.RMultiple()
				rSamples++
			}
			if hold, ok := holdDays(tr, now); ok {
				holdTotal += hold
				holdSamples++
			}
			returnTotal += tr.ResultPercent()
			returnSamples++
		} else {
			metrics.Open++
			metrics.OpenRisk += tr.TotalRiskAmount()
		}
	}

	if metrics.Closed > 0 {
		metrics.WinRate = (float64(winCount) / float64(metrics.Closed)) * 100
	}
	if rSamples > 0 {
		metrics.AvgR = rTotal / float64(rSamples)
	}
	if holdSamples > 0 {
		metrics.AvgHoldDays = holdTotal / float64(holdSamples)
	}
	if returnSamples > 0 {
		metrics.AvgReturnPct = returnTotal / float64(returnSamples)
	}
	return metrics
}

func collectTags(trades []*domain.Trade) []string {
	seen := make(map[string]struct{})
	for _, tr := range trades {
		for _, tag := range tr.Review.Tags {
			normalised := normalizeTag(tag)
			if normalised == "" {
				continue
			}
			seen[normalised] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	values := make([]string, 0, len(seen))
	for tag := range seen {
		values = append(values, tag)
	}
	sort.Strings(values)
	return values
}

func tradeStatus(tr *domain.Trade) string {
	if tr.HasExited() {
		return "已平倉"
	}
	return "未平倉"
}

func holdDays(tr *domain.Trade, now time.Time) (float64, bool) {
	if tr.Entry.Date.IsZero() {
		return 0, false
	}
	end := now
	if tr.HasExited() {
		if tr.Exit == nil || tr.Exit.Date.IsZero() {
			return 0, false
		}
		end = tr.Exit.Date
	}
	if end.Before(tr.Entry.Date) {
		return 0, false
	}
	duration := end.Sub(tr.Entry.Date).Hours() / 24
	return duration, true
}

func normalizeTag(tag string) string {
	trimmed := strings.TrimSpace(strings.ToLower(tag))
	if trimmed == "" {
		return ""
	}
	if !utf8.ValidString(trimmed) {
		return ""
	}
	return trimmed
}

func buildTradeFromForm(r *http.Request) (*domain.Trade, []string) {
	var errs []string
	get := func(name string) string { return strings.TrimSpace(r.FormValue(name)) }

	tr := &domain.Trade{}
	tr.Instrument = get("instrument")
	tr.Market = get("market")
	tr.Setup = get("setup")
	tr.Direction = domain.Direction(strings.ToUpper(get("direction")))
	if tr.Direction != domain.DirectionLong && tr.Direction != domain.DirectionShort {
		tr.Direction = domain.DirectionLong
	}

	entryDateStr := get("entry_date")
	if entryDateStr == "" {
		errs = append(errs, "必須填寫進場日期")
	} else {
		if dt, err := time.Parse("2006-01-02", entryDateStr); err == nil {
			tr.Entry.Date = dt
		} else {
			errs = append(errs, "進場日期格式錯誤")
		}
	}

	var err error
	if tr.Entry.Price, err = parseRequiredFloat(get("entry_price")); err != nil {
		errs = append(errs, "進場價格格式錯誤")
	}
	if tr.Entry.Quantity, err = parseRequiredFloat(get("entry_quantity")); err != nil {
		errs = append(errs, "數量格式錯誤")
	}
	if tr.Entry.Fees, err = parseOptionalFloat(get("entry_fees"), 0); err != nil {
		errs = append(errs, "進場手續費格式錯誤")
	}
	if tr.Entry.StopLoss, err = parseOptionalPtrFloat(get("entry_stop_loss")); err != nil {
		errs = append(errs, "停損價格格式錯誤")
	}
	if tr.Entry.Target, err = parseOptionalPtrFloat(get("entry_target")); err != nil {
		errs = append(errs, "目標價格式錯誤")
	}
	if tr.Entry.RiskPerShare, err = parseOptionalPtrFloat(get("entry_risk")); err != nil {
		errs = append(errs, "自訂每股風險格式錯誤")
	}
	tr.Entry.Notes = get("entry_notes")

	tr.RiskManagement = domain.RiskManagement{
		Thesis:          get("thesis"),
		Plan:            get("plan"),
		Checklist:       get("checklist"),
		PositionSizing:  get("position_sizing"),
		ContingencyPlan: get("contingency_plan"),
	}
	if tr.RiskManagement.MaxRiskAmount, err = parseOptionalFloat(get("max_risk"), 0); err != nil {
		errs = append(errs, "最大風險格式錯誤")
	}

	exitProvided := false
	if dateStr := get("exit_date"); dateStr != "" {
		if dt, err := time.Parse("2006-01-02", dateStr); err == nil {
			ensureExit(tr)
			tr.Exit.Date = dt
			exitProvided = true
		} else {
			errs = append(errs, "出場日期格式錯誤")
		}
	}
	if priceStr := get("exit_price"); priceStr != "" {
		if val, err := parseFloatValue(priceStr); err == nil {
			ensureExit(tr)
			tr.Exit.Price = val
			exitProvided = true
		} else {
			errs = append(errs, "出場價格格式錯誤")
		}
	}
	if qtyStr := get("exit_quantity"); qtyStr != "" {
		if val, err := parseFloatValue(qtyStr); err == nil {
			ensureExit(tr)
			tr.Exit.Quantity = val
			exitProvided = true
		} else {
			errs = append(errs, "出場數量格式錯誤")
		}
	}
	if feeStr := get("exit_fees"); feeStr != "" {
		if val, err := parseFloatValue(feeStr); err == nil {
			ensureExit(tr)
			tr.Exit.Fees = val
			exitProvided = true
		} else {
			errs = append(errs, "出場手續費格式錯誤")
		}
	}
	if reason := get("exit_reason"); reason != "" {
		ensureExit(tr)
		tr.Exit.Reason = reason
		exitProvided = true
	}
	if notes := get("exit_notes"); notes != "" {
		ensureExit(tr)
		tr.Exit.Notes = notes
		exitProvided = true
	}
	if tr.Exit != nil && !exitProvided {
		tr.Exit = nil
	}
	if tr.Exit != nil && tr.Exit.Quantity == 0 {
		tr.Exit.Quantity = tr.Entry.Quantity
	}

	tr.Review = domain.TradeReview{
		OutcomeSummary: get("outcome"),
		Psychology:     get("psychology"),
		Improvements:   get("improvements"),
	}
	if tags := get("tags"); tags != "" {
		parts := strings.Split(tags, ",")
		seen := make(map[string]struct{})
		var cleaned []string
		for _, tag := range parts {
			normalized := normalizeTag(tag)
			if normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			cleaned = append(cleaned, normalized)
		}
		tr.Review.Tags = cleaned
	}

	tr.MarketContext = get("market_context")
	tr.AdditionalNotes = get("additional_notes")

	if tr.ExecutionScore, err = parseOptionalPtrFloat(get("execution_score")); err != nil {
		errs = append(errs, "執行評分格式錯誤")
	}
	if tr.ConfidenceBefore, err = parseOptionalPtrFloat(get("confidence_before")); err != nil {
		errs = append(errs, "進場前信心格式錯誤")
	}
	if tr.ConfidenceAfter, err = parseOptionalPtrFloat(get("confidence_after")); err != nil {
		errs = append(errs, "出場後信心格式錯誤")
	}

	return tr, errs
}

func parseRequiredFloat(val string) (float64, error) {
	normalized := normalizeNumericInput(val)
	if normalized == "" {
		return 0, fmt.Errorf("empty")
	}
	return strconv.ParseFloat(normalized, 64)
}

func parseOptionalFloat(val string, def float64) (float64, error) {
	normalized := normalizeNumericInput(val)
	if normalized == "" {
		return def, nil
	}
	return strconv.ParseFloat(normalized, 64)
}

func parseOptionalPtrFloat(val string) (*float64, error) {
	normalized := normalizeNumericInput(val)
	if normalized == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func ensureExit(tr *domain.Trade) {
	if tr.Exit == nil {
		tr.Exit = &domain.ExitDetail{}
	}
}

type tradeFormData struct {
	Instrument       string
	Market           string
	Direction        string
	Setup            string
	EntryDate        string
	EntryPrice       string
	EntryQuantity    string
	EntryFees        string
	EntryStopLoss    string
	EntryTarget      string
	EntryRisk        string
	EntryNotes       string
	Thesis           string
	Plan             string
	Checklist        string
	MaxRisk          string
	PositionSizing   string
	ContingencyPlan  string
	ExitDate         string
	ExitPrice        string
	ExitQuantity     string
	ExitFees         string
	ExitReason       string
	ExitNotes        string
	Outcome          string
	Psychology       string
	Improvements     string
	Tags             string
	MarketContext    string
	AdditionalNotes  string
	ExecutionScore   string
	ConfidenceBefore string
	ConfidenceAfter  string
}

func newTradeFormData(tr *domain.Trade, isNew bool) tradeFormData {
	data := tradeFormData{
		Instrument:      tr.Instrument,
		Market:          tr.Market,
		Setup:           tr.Setup,
		Direction:       string(tr.Direction),
		EntryNotes:      tr.Entry.Notes,
		Thesis:          tr.RiskManagement.Thesis,
		Plan:            tr.RiskManagement.Plan,
		Checklist:       tr.RiskManagement.Checklist,
		PositionSizing:  tr.RiskManagement.PositionSizing,
		ContingencyPlan: tr.RiskManagement.ContingencyPlan,
		ExitReason:      "",
		ExitNotes:       "",
		Outcome:         tr.Review.OutcomeSummary,
		Psychology:      tr.Review.Psychology,
		Improvements:    tr.Review.Improvements,
		MarketContext:   tr.MarketContext,
		AdditionalNotes: tr.AdditionalNotes,
	}

	if data.Direction == "" {
		data.Direction = string(domain.DirectionLong)
	}

	if !tr.Entry.Date.IsZero() {
		data.EntryDate = tr.Entry.Date.Format("2006-01-02")
	} else if isNew {
		data.EntryDate = time.Now().Format("2006-01-02")
	}
	data.EntryPrice = formatRequiredFloat(tr.Entry.Price, 4, isNew)
	data.EntryQuantity = formatRequiredFloat(tr.Entry.Quantity, 4, isNew)
	data.EntryFees = formatOptionalFloat(tr.Entry.Fees, 2)
	data.EntryStopLoss = formatOptionalPtrFloat(tr.Entry.StopLoss, 4)
	data.EntryTarget = formatOptionalPtrFloat(tr.Entry.Target, 4)
	data.EntryRisk = formatOptionalPtrFloat(tr.Entry.RiskPerShare, 4)

	data.MaxRisk = formatOptionalFloat(tr.RiskManagement.MaxRiskAmount, 2)

	if tr.Exit != nil {
		if !tr.Exit.Date.IsZero() {
			data.ExitDate = tr.Exit.Date.Format("2006-01-02")
		}
		data.ExitPrice = formatOptionalFloat(tr.Exit.Price, 4)
		data.ExitQuantity = formatOptionalFloat(tr.Exit.Quantity, 4)
		data.ExitFees = formatOptionalFloat(tr.Exit.Fees, 2)
		data.ExitReason = tr.Exit.Reason
		data.ExitNotes = tr.Exit.Notes
	}

	if len(tr.Review.Tags) > 0 {
		formatted := make([]string, 0, len(tr.Review.Tags))
		for _, tag := range tr.Review.Tags {
			formatted = append(formatted, templates.FormatTag(tag))
		}
		data.Tags = strings.Join(formatted, ", ")
	}

	data.ExecutionScore = formatOptionalPtrFloat(tr.ExecutionScore, 1)
	data.ConfidenceBefore = formatOptionalPtrFloat(tr.ConfidenceBefore, 1)
	data.ConfidenceAfter = formatOptionalPtrFloat(tr.ConfidenceAfter, 1)

	return data
}

func formatRequiredFloat(val float64, precision int, isNew bool) string {
	if isNew && val == 0 {
		return ""
	}
	return strconv.FormatFloat(val, 'f', precision, 64)
}

func formatOptionalFloat(val float64, precision int) string {
	if val == 0 {
		return ""
	}
	return strconv.FormatFloat(val, 'f', precision, 64)
}

func formatOptionalPtrFloat(val *float64, precision int) string {
	if val == nil {
		return ""
	}
	return strconv.FormatFloat(*val, 'f', precision, 64)
}

func parseFloatValue(val string) (float64, error) {
	normalized := normalizeNumericInput(val)
	if normalized == "" {
		return 0, fmt.Errorf("empty")
	}
	return strconv.ParseFloat(normalized, 64)
}

func normalizeNumericInput(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range val {
		switch {
		case r >= '０' && r <= '９':
			b.WriteRune('0' + (r - '０'))
		case r == '．' || r == '。':
			b.WriteRune('.')
		case r == '－' || r == '﹣' || r == '—' || r == '–':
			b.WriteRune('-')
		case r == '＋' || r == '﹢':
			b.WriteRune('+')
		case r == ',' || r == '，':
			continue
		case unicode.IsSpace(r):
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeIntegerInput(val string) string {
	normalized := normalizeNumericInput(val)
	if normalized == "" {
		return ""
	}
	if idx := strings.IndexRune(normalized, '.'); idx >= 0 {
		normalized = normalized[:idx]
	}
	return normalized
}
