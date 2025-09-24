package web

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	domain "best_trade_logs/internal/domain/trade"
	tradesvc "best_trade_logs/internal/service/trade"
	"best_trade_logs/internal/storage"
	"best_trade_logs/internal/web/templates"
)

// Server wires the HTTP layer with the trade service.
type Server struct {
	svc       *tradesvc.Service
	templates *template.Template
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

	summaries := make([]tradeSummary, 0, len(trades))
	for _, tr := range trades {
		summary := tradeSummary{Trade: tr, NetResult: tr.NetResult(), ResultPercent: tr.ResultPercent(), RMultiple: tr.RMultiple()}
		if v, ok := tr.FollowUpChangePercent(7); ok {
			val := v
			summary.FollowUp7 = &val
		}
		if v, ok := tr.FollowUpChangePercent(30); ok {
			val := v
			summary.FollowUp30 = &val
		}
		summaries = append(summaries, summary)
	}

	data := struct {
		Title  string
		Trades []tradeSummary
		Flash  string
	}{
		Title:  "Trade Journal",

		Trades: summaries,
		Flash:  r.URL.Query().Get("flash"),
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
		"Title":  "Record new trade",
		"Trade":  tr,
		"Action": "/trades",
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
		http.Error(w, "invalid form", http.StatusBadRequest)
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
	http.Redirect(w, r, fmt.Sprintf("/trades/%s?flash=Trade%%20recorded", tr.ID), http.StatusSeeOther)
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
		Title:      fmt.Sprintf("Trade - %s", tr.Instrument),
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
		"Title":  "Edit trade",
		"Trade":  tr,
		"Action": fmt.Sprintf("/trades/%s/update", tr.ID),
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
		http.Error(w, "invalid form", http.StatusBadRequest)
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
	http.Redirect(w, r, fmt.Sprintf("/trades/%s?flash=Trade%%20updated", tr.ID), http.StatusSeeOther)
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
	http.Redirect(w, r, "/?flash=Trade%%20deleted", http.StatusSeeOther)
}

func (s *Server) handleAddFollowUp(w http.ResponseWriter, r *http.Request, id string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	days, err := strconv.Atoi(strings.TrimSpace(r.FormValue("days_after")))
	if err != nil {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}
	price, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("price")), 64)
	if err != nil {
		http.Error(w, "invalid price", http.StatusBadRequest)
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
	http.Redirect(w, r, fmt.Sprintf("/trades/%s?flash=Follow-up%%20added", id), http.StatusSeeOther)
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
		errs = append(errs, "entry date is required")
	} else {
		if dt, err := time.Parse("2006-01-02", entryDateStr); err == nil {
			tr.Entry.Date = dt
		} else {
			errs = append(errs, "invalid entry date")
		}
	}

	var err error
	if tr.Entry.Price, err = parseRequiredFloat(get("entry_price")); err != nil {
		errs = append(errs, "invalid entry price")
	}
	if tr.Entry.Quantity, err = parseRequiredFloat(get("entry_quantity")); err != nil {
		errs = append(errs, "invalid quantity")
	}
	if tr.Entry.Fees, err = parseOptionalFloat(get("entry_fees"), 0); err != nil {
		errs = append(errs, "invalid entry fees")
	}
	if tr.Entry.StopLoss, err = parseOptionalPtrFloat(get("entry_stop_loss")); err != nil {
		errs = append(errs, "invalid stop loss")
	}
	if tr.Entry.Target, err = parseOptionalPtrFloat(get("entry_target")); err != nil {
		errs = append(errs, "invalid target")
	}
	if tr.Entry.RiskPerShare, err = parseOptionalPtrFloat(get("entry_risk")); err != nil {
		errs = append(errs, "invalid manual risk per share")
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
		errs = append(errs, "invalid max risk")
	}

	exitProvided := false
	if dateStr := get("exit_date"); dateStr != "" {
		if dt, err := time.Parse("2006-01-02", dateStr); err == nil {
			ensureExit(tr)
			tr.Exit.Date = dt
			exitProvided = true
		} else {
			errs = append(errs, "invalid exit date")
		}
	}
	if priceStr := get("exit_price"); priceStr != "" {
		if val, err := strconv.ParseFloat(priceStr, 64); err == nil {
			ensureExit(tr)
			tr.Exit.Price = val
			exitProvided = true
		} else {
			errs = append(errs, "invalid exit price")
		}
	}
	if qtyStr := get("exit_quantity"); qtyStr != "" {
		if val, err := strconv.ParseFloat(qtyStr, 64); err == nil {
			ensureExit(tr)
			tr.Exit.Quantity = val
			exitProvided = true
		} else {
			errs = append(errs, "invalid exit quantity")
		}
	}
	if feeStr := get("exit_fees"); feeStr != "" {
		if val, err := strconv.ParseFloat(feeStr, 64); err == nil {
			ensureExit(tr)
			tr.Exit.Fees = val
			exitProvided = true
		} else {
			errs = append(errs, "invalid exit fees")
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
		tr.Review.Tags = parts
	}

	tr.MarketContext = get("market_context")
	tr.AdditionalNotes = get("additional_notes")

	if tr.ExecutionScore, err = parseOptionalPtrFloat(get("execution_score")); err != nil {
		errs = append(errs, "invalid execution score")
	}
	if tr.ConfidenceBefore, err = parseOptionalPtrFloat(get("confidence_before")); err != nil {
		errs = append(errs, "invalid confidence before")
	}
	if tr.ConfidenceAfter, err = parseOptionalPtrFloat(get("confidence_after")); err != nil {
		errs = append(errs, "invalid confidence after")
	}

	return tr, errs
}

func parseRequiredFloat(val string) (float64, error) {
	return strconv.ParseFloat(val, 64)
}

func parseOptionalFloat(val string, def float64) (float64, error) {
	if strings.TrimSpace(val) == "" {
		return def, nil
	}
	return strconv.ParseFloat(val, 64)
}

func parseOptionalPtrFloat(val string) (*float64, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(val, 64)
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
