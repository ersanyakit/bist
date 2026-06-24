package corporateactions

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/kapingest"
	servicekap "hissebot/internal/services/kap"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/util"
)

func Load(equitiesDir, symbol string) ActionSet {
	symbol = ohlcv.NormalizeSymbol(symbol)
	set := ActionSet{Symbol: symbol, Status: "missing"}
	if symbol == "" || strings.TrimSpace(equitiesDir) == "" {
		set.Warnings = append(set.Warnings, "corporate_actions_symbol_or_equities_dir_missing")
		return finalizeActionSet(set)
	}
	root := filepath.Join(equitiesDir, symbol)
	actions := []Action{}
	for _, path := range actionCandidateFiles(root) {
		loaded, warnings := readActionFile(path, symbol)
		if len(warnings) > 0 {
			set.Warnings = append(set.Warnings, warnings...)
		}
		if len(loaded) > 0 {
			set.SourceFiles = append(set.SourceFiles, path)
			actions = append(actions, loaded...)
		}
	}
	for _, path := range marketWSDatasetFiles(root, equitiesDir) {
		loaded, warnings := readMarketWSDataset(path, symbol)
		if len(warnings) > 0 {
			set.Warnings = append(set.Warnings, warnings...)
		}
		if len(loaded) > 0 {
			set.SourceFiles = append(set.SourceFiles, path)
			actions = append(actions, loaded...)
		}
	}
	for _, path := range mkkDatasetFiles(root) {
		loaded, warnings := readMKKDataset(path, symbol)
		if len(warnings) > 0 {
			set.Warnings = append(set.Warnings, warnings...)
		}
		if len(loaded) > 0 {
			set.SourceFiles = append(set.SourceFiles, path)
			actions = append(actions, loaded...)
		}
	}
	for _, path := range kapDisclosurePaths(root) {
		loaded, warnings := readKAPDisclosures(path, symbol)
		if len(warnings) > 0 {
			set.Warnings = append(set.Warnings, warnings...)
		}
		if len(loaded) > 0 {
			set.SourceFiles = append(set.SourceFiles, path)
			actions = append(actions, loaded...)
		}
	}
	for _, path := range kapCorporateEventPaths(equitiesDir, symbol) {
		loaded, warnings := readKAPCorporateEvents(path, symbol)
		if len(warnings) > 0 {
			set.Warnings = append(set.Warnings, warnings...)
		}
		if len(loaded) > 0 {
			set.SourceFiles = append(set.SourceFiles, path)
			actions = append(actions, loaded...)
		}
	}
	set.Actions = dedupeActions(actions)
	return finalizeActionSet(set)
}

func actionCandidateFiles(root string) []string {
	return []string{
		filepath.Join(root, "corporate_actions.json"),
		filepath.Join(root, "corporate_actions.jsonl"),
		filepath.Join(root, "kap", "corporate_actions.json"),
		filepath.Join(root, "kap", "corporate_actions.jsonl"),
	}
}

func marketWSDatasetFiles(root string, equitiesDir string) []string {
	files := []string{}
	candidates := []string{
		filepath.Join(root, "market_ws", "dividend_history_data.json"),
		filepath.Join(root, "market_ws", "upcoming_dividends_data.json"),
		filepath.Join(root, "market_ws", "capital_increases_data.json"),
		filepath.Join(filepath.Dir(equitiesDir), "market", "ws", "public", "dividend_history_data.json"),
		filepath.Join(filepath.Dir(equitiesDir), "market", "ws", "public", "upcoming_dividends_data.json"),
		filepath.Join(filepath.Dir(equitiesDir), "market", "ws", "public", "capital_increases_data.json"),
	}
	for _, path := range candidates {
		if fileExists(path) {
			files = append(files, path)
		}
	}
	dirs, err := os.ReadDir(filepath.Join(root, "market_ws"))
	if err != nil {
		return files
	}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		name := dir.Name()
		if _, err := time.Parse("2006-01-02", name); err != nil {
			continue
		}
		for _, base := range []string{"dividend_history_data.json", "upcoming_dividends_data.json", "capital_increases_data.json"} {
			path := filepath.Join(root, "market_ws", name, base)
			if fileExists(path) {
				files = append(files, path)
			}
		}
	}
	return files
}

func mkkDatasetFiles(root string) []string {
	return existingFiles([]string{
		filepath.Join(root, "mkk.json"),
		filepath.Join(root, "mkk", "corporate_actions.json"),
		filepath.Join(root, "mkk", "dividends.json"),
		filepath.Join(root, "mkk", "capital_increases.json"),
		filepath.Join(root, "mkk", "rights_issues.json"),
	})
}

func kapDisclosurePaths(root string) []string {
	return existingFiles([]string{
		filepath.Join(root, "kap_disclosures.json"),
		filepath.Join(root, "kap", "disclosures.json"),
	})
}

func kapCorporateEventPaths(equitiesDir, symbol string) []string {
	lower := strings.ToLower(symbol)
	return uniqueStrings([]string{
		filepath.Join(filepath.Dir(equitiesDir), "processed", "by_ticker", symbol, kapingest.CorporateFactsFile),
		filepath.Join(filepath.Dir(equitiesDir), "processed", "by_ticker", lower, kapingest.CorporateFactsFile),
		filepath.Join(filepath.Dir(equitiesDir), "processed", lower, "by_ticker", symbol, kapingest.CorporateFactsFile),
		filepath.Join(filepath.Dir(equitiesDir), "processed", lower, "by_ticker", lower, kapingest.CorporateFactsFile),
		filepath.Join(filepath.Dir(equitiesDir), "processed", lower, kapingest.CorporateFactsFile),
	})
}

func readActionFile(path, symbol string) ([]Action, []string) {
	if !fileExists(path) {
		return nil, nil
	}
	if strings.HasSuffix(path, ".jsonl") {
		return readActionJSONL(path, symbol)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{"corporate_actions_read_failed:" + path}
	}
	var actions []Action
	if err := json.Unmarshal(raw, &actions); err != nil {
		var wrapper struct {
			Symbol  string   `json:"symbol,omitempty"`
			Status  string   `json:"status,omitempty"`
			Actions []Action `json:"actions"`
		}
		if wrapErr := json.Unmarshal(raw, &wrapper); wrapErr != nil {
			return nil, []string{"corporate_actions_parse_failed:" + path}
		}
		if strings.EqualFold(filepath.Base(path), "corporate_actions.json") && wrapper.Symbol != "" && wrapper.Status != "" {
			return nil, nil
		}
		actions = wrapper.Actions
	}
	out := make([]Action, 0, len(actions))
	for _, action := range actions {
		action.Symbol = firstNonEmpty(action.Symbol, symbol)
		action.Source = firstNonEmpty(action.Source, "corporate_actions_file")
		action.SourceFile = firstNonEmpty(action.SourceFile, path)
		action = normalizeAction(action)
		if strings.EqualFold(action.Symbol, symbol) {
			out = append(out, action)
		}
	}
	return out, nil
}

func readActionJSONL(path, symbol string) ([]Action, []string) {
	file, err := os.Open(path)
	if err != nil {
		return nil, []string{"corporate_actions_jsonl_open_failed:" + path}
	}
	defer func() { _ = file.Close() }()
	out := []Action{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var action Action
		if err := json.Unmarshal([]byte(line), &action); err != nil {
			continue
		}
		action.Symbol = firstNonEmpty(action.Symbol, symbol)
		action.Source = firstNonEmpty(action.Source, "corporate_actions_file")
		action.SourceFile = firstNonEmpty(action.SourceFile, path)
		action = normalizeAction(action)
		if strings.EqualFold(action.Symbol, symbol) {
			out = append(out, action)
		}
	}
	if err := scanner.Err(); err != nil {
		return out, []string{"corporate_actions_jsonl_scan_failed:" + path}
	}
	return out, nil
}

func readMarketWSDataset(path, symbol string) ([]Action, []string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{"market_ws_corporate_actions_read_failed:" + path}
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, []string{"market_ws_corporate_actions_parse_failed:" + path}
	}
	dataset := strings.TrimSuffix(filepath.Base(path), ".json")
	records := flattenRecords(payload)
	out := []Action{}
	for _, record := range records {
		action, ok := actionFromMarketWSRecord(record, symbol, dataset, path)
		if ok {
			out = append(out, action)
		}
	}
	return out, nil
}

func readMKKDataset(path, symbol string) ([]Action, []string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{"mkk_corporate_actions_read_failed:" + path}
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, []string{"mkk_corporate_actions_parse_failed:" + path}
	}
	records := flattenRecords(payload)
	out := []Action{}
	for _, record := range records {
		action, ok := actionFromGenericDatasetRecord(record, symbol, "mkk", "mkk", path, StatusCandidate, 0.75)
		if ok {
			out = append(out, action)
		}
	}
	return out, nil
}

func readKAPDisclosures(path, symbol string) ([]Action, []string) {
	items, err := servicekap.LoadFinancialDisclosures(path)
	if err != nil {
		return nil, []string{"kap_disclosures_parse_failed:" + path}
	}
	out := []Action{}
	for _, disclosure := range items {
		if !strings.EqualFold(strings.TrimSpace(disclosure.Ticker), symbol) {
			continue
		}
		if action, ok := actionFromKAPDisclosure(disclosure, symbol, path); ok {
			out = append(out, action)
		}
	}
	return out, nil
}

func readKAPCorporateEvents(path, symbol string) ([]Action, []string) {
	if !fileExists(path) {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, []string{"kap_corporate_events_open_failed:" + path}
	}
	defer func() { _ = file.Close() }()
	out := []Action{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event kapingest.ExtractedCorporateEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(event.Ticker), symbol) {
			continue
		}
		if action, ok := actionFromKAPEvent(event, path); ok {
			out = append(out, action)
		}
	}
	if err := scanner.Err(); err != nil {
		return out, []string{"kap_corporate_events_scan_failed:" + path}
	}
	return out, nil
}

func actionFromKAPDisclosure(disclosure servicekap.FinancialDisclosure, symbol, path string) (Action, bool) {
	text := strings.Join([]string{disclosure.Title, disclosure.Subject, disclosure.Summary, disclosure.DisclosureType, disclosure.DisclosureCategory}, " ")
	if !issuerCorporateActionDisclosure(text) {
		return Action{}, false
	}
	actionType := inferActionType(text)
	if actionType == "" {
		return Action{}, false
	}
	effective := parseDateFromAny(disclosure.Summary, disclosure.Title)
	if effective == nil && executionDateDisclosure(text) {
		effective = dateOnlyPtr(disclosure.PublishDate)
	}
	ratio := ratioFromText(text)
	cash := cashAmountFromText(text)
	action := Action{
		ID:               firstNonEmpty(disclosure.ID, stableID(symbol, actionType, text, path)),
		Symbol:           symbol,
		Type:             actionType,
		Status:           StatusReview,
		AnnouncementDate: dateOnlyPtr(disclosure.PublishDate),
		EffectiveDate:    effective,
		Source:           "kap_disclosures",
		SourceDataset:    "kap_disclosures",
		SourceFile:       path,
		SourceURL:        disclosureURL(disclosure),
		Title:            strings.TrimSpace(strings.Join([]string{disclosure.Title, disclosure.Summary}, " | ")),
		Ratio:            ratio,
		CashAmount:       cash,
		Currency:         "TRY",
		Confidence:       kapDisclosureConfidence(disclosure, ratio, cash, effective),
		ReviewRequired:   true,
	}
	if action.EffectiveDate == nil {
		action.Warnings = append(action.Warnings, "effective_date_missing")
	}
	if action.SourceURL == "" {
		action.Warnings = append(action.Warnings, "source_url_missing")
	}
	if action.Ratio == nil && action.CashAmount == nil {
		action.Warnings = append(action.Warnings, "ratio_or_cash_amount_missing")
	}
	if action.EffectiveDate != nil && actionHasAdjustmentPayload(action) && concreteKAPActionDisclosure(text) {
		action.Status = StatusCandidate
	}
	return normalizeAction(action), true
}

func actionFromKAPEvent(event kapingest.ExtractedCorporateEvent, path string) (Action, bool) {
	actionType := mapKAPEventType(event.EventType, event.Title)
	if actionType == "" {
		return Action{}, false
	}
	title := strings.TrimSpace(event.Title)
	effective := parseDateFromAny(event.EffectiveDate, event.PaymentDate, title)
	announcement := parseDateFromAny(event.DocumentDate, nil, "")
	action := Action{
		ID:               firstNonEmpty(event.ID, stableID(event.Ticker, actionType, title, path)),
		Symbol:           strings.ToUpper(strings.TrimSpace(event.Ticker)),
		Type:             actionType,
		Status:           StatusReview,
		AnnouncementDate: announcement,
		EffectiveDate:    effective,
		Source:           "kap_pdf_document_intelligence",
		SourceDataset:    "corporate_events",
		SourceFile:       firstNonEmpty(event.SourceFile, path),
		Title:            title,
		Currency:         event.Currency,
		Confidence:       event.Confidence,
		ReviewRequired:   true,
		Warnings:         append([]string{}, event.Warnings...),
	}
	if event.Certification.Status != "" && event.Certification.Status != kapingest.EvidenceStatusCertified {
		action.Warnings = append(action.Warnings, event.Certification.Reasons...)
	}
	if event.ReviewRequired {
		action.Warnings = append(action.Warnings, "kap_event_review_required")
	}
	if event.Amount != nil && perShareAmountLikely(title) {
		action.CashAmount = event.Amount
	}
	if event.Ratio != nil {
		action.Ratio = event.Ratio
	} else if ratio := ratioFromText(title); ratio != nil {
		action.Ratio = ratio
	}
	if event.SubscriptionPrice != nil {
		action.SubscriptionPrice = event.SubscriptionPrice
	}
	if action.EffectiveDate == nil {
		action.Warnings = append(action.Warnings, "effective_date_missing")
	}
	if action.Ratio == nil && action.CashAmount == nil {
		action.Warnings = append(action.Warnings, "ratio_or_cash_amount_missing")
	}
	return normalizeAction(action), true
}

func actionFromMarketWSRecord(record map[string]any, symbol, dataset, path string) (Action, bool) {
	return actionFromGenericDatasetRecord(record, symbol, "market_ws", dataset, path, StatusVerified, 0.9)
}

func actionFromGenericDatasetRecord(record map[string]any, symbol, source, dataset, path, status string, confidence float64) (Action, bool) {
	recordSymbol := normalizeRecordSymbol(record)
	if recordSymbol != "" && !strings.EqualFold(recordSymbol, symbol) {
		return Action{}, false
	}
	text := strings.Join([]string{dataset, stringValue(record, "type", "event_type", "title", "summary", "description", "name")}, " ")
	actionType := inferActionType(text)
	if actionType == "" {
		return Action{}, false
	}
	effective := parseDateFromRecord(record, "effective_date", "ex_date", "exdate", "hak_kullanim_tarihi", "payment_date", "odeme_tarihi", "date", "tarih")
	announcement := parseDateFromRecord(record, "announcement_date", "kap_date", "publish_date", "created_at", "updated_at")
	ratio := numberPtrFromRecord(record, "ratio", "split_ratio", "bedelsiz_oran", "bedelli_oran", "bonus_ratio", "capital_increase_ratio")
	if ratio == nil {
		ratio = ratioFromText(text)
	}
	cash := numberPtrFromRecord(record, "cash_amount", "gross_cash_amount", "gross_dividend", "brut_temettu", "net_temettu", "dividend", "amount_per_share")
	subscription := numberPtrFromRecord(record, "subscription_price", "exercise_price", "ruchan_kullanim_fiyati", "rights_price")
	action := Action{
		ID:                stableID(symbol, actionType, dataset, fmt.Sprint(record)),
		Symbol:            symbol,
		Type:              actionType,
		Status:            status,
		AnnouncementDate:  announcement,
		EffectiveDate:     effective,
		Source:            source,
		SourceDataset:     dataset,
		SourceFile:        path,
		SourceURL:         stringValue(record, "source_url", "url", "kap_url"),
		Title:             stringValue(record, "title", "summary", "description", "name"),
		Ratio:             ratio,
		CashAmount:        cash,
		SubscriptionPrice: subscription,
		Currency:          firstNonEmpty(stringValue(record, "currency", "para_birimi"), "TRY"),
		Confidence:        confidence,
	}
	return normalizeAction(action), true
}

func finalizeActionSet(set ActionSet) ActionSet {
	sort.SliceStable(set.Actions, func(i, j int) bool {
		di := actionDateKey(set.Actions[i])
		dj := actionDateKey(set.Actions[j])
		if di == dj {
			return set.Actions[i].ID < set.Actions[j].ID
		}
		return di < dj
	})
	for i := range set.Actions {
		action := &set.Actions[i]
		switch action.Status {
		case StatusVerified:
			set.VerifiedActions++
		case StatusReview:
			set.ReviewRequiredActions++
		default:
			set.CandidateActions++
		}
		if actionHasAdjustmentPayload(*action) {
			if action.EffectiveDate != nil {
				set.AdjustmentReadyActions++
			} else {
				set.MissingEffectiveDateActions++
			}
		} else {
			set.MissingAdjustmentActions++
		}
	}
	set.SourceFiles = uniqueStrings(set.SourceFiles)
	switch {
	case len(set.Actions) == 0:
		set.Status = "missing"
		set.Warnings = appendUnique(set.Warnings, "corporate_actions_missing")
	case set.AdjustmentReadyActions == 0:
		set.Status = "events_only"
		set.Warnings = appendUnique(set.Warnings, "corporate_actions_not_adjustment_ready")
	case set.VerifiedActions == 0:
		set.Status = "candidate_adjustments_review_required"
	default:
		set.Status = "adjustment_ready"
	}
	return set
}

func normalizeAction(action Action) Action {
	action.Symbol = ohlcv.NormalizeSymbol(action.Symbol)
	action.Type = inferActionType(firstNonEmpty(action.Type, action.Title))
	if action.Type == "" {
		action.Type = "corporate_action"
	}
	if action.ID == "" {
		action.ID = stableID(action.Symbol, action.Type, action.Title, action.SourceFile, actionDateKey(action))
	}
	if action.Status == "" {
		action.Status = StatusCandidate
	}
	if action.Confidence == 0 {
		switch action.Status {
		case StatusVerified:
			action.Confidence = 0.9
		case StatusReview:
			action.Confidence = 0.55
		default:
			action.Confidence = 0.7
		}
	}
	if action.EffectiveDate == nil {
		action.EffectiveDate = parseDateFromAny(nil, nil, action.Title)
	}
	if action.Ratio != nil {
		v := normalizeRatio(*action.Ratio)
		action.Ratio = &v
	}
	if action.AdjustmentFactor != nil && (*action.AdjustmentFactor <= 0 || *action.AdjustmentFactor > 5) {
		action.Warnings = append(action.Warnings, "invalid_adjustment_factor")
		action.AdjustmentFactor = nil
	}
	if action.Status != StatusVerified {
		action.ReviewRequired = true
	}
	return action
}

func actionHasAdjustmentPayload(action Action) bool {
	if action.AdjustmentFactor != nil && *action.AdjustmentFactor > 0 {
		return true
	}
	switch action.Type {
	case TypeDividend:
		return action.CashAmount != nil && *action.CashAmount > 0
	case TypeBonusIssue, TypeSplit:
		return action.Ratio != nil && *action.Ratio > 0
	case TypeRightsIssue:
		return action.Ratio != nil && *action.Ratio > 0 && action.SubscriptionPrice != nil && *action.SubscriptionPrice > 0
	default:
		return false
	}
}

func mapKAPEventType(eventType, title string) string {
	switch strings.TrimSpace(eventType) {
	case "dividend":
		return TypeDividend
	case "capital_increase":
		return inferCapitalActionType(title, TypeCapitalIncrease)
	case "capital_action":
		return inferCapitalActionType(title, TypeCapitalIncrease)
	case "merger":
		return TypeMerger
	case "spin_off":
		return TypeSpinOff
	default:
		return inferActionType(title)
	}
}

func inferActionType(text string) string {
	slug := util.SlugTR(text)
	switch {
	case strings.Contains(slug, "temettu") || strings.Contains(slug, "karpayi") || strings.Contains(slug, "dividend"):
		return TypeDividend
	case strings.Contains(slug, "bedelsiz") || strings.Contains(slug, "bonus"):
		return TypeBonusIssue
	case strings.Contains(slug, "bedelli") || strings.Contains(slug, "ruchan") || strings.Contains(slug, "rights"):
		return TypeRightsIssue
	case strings.Contains(slug, "sermayeazalt"):
		return TypeCapitalReduction
	case strings.Contains(slug, "sermayeartir") || strings.Contains(slug, "capitalincrease"):
		return TypeCapitalIncrease
	case strings.Contains(slug, "split") || strings.Contains(slug, "bolunme") || strings.Contains(slug, "paybolun"):
		return TypeSplit
	case strings.Contains(slug, "birlesme") || strings.Contains(slug, "merger"):
		return TypeMerger
	default:
		return ""
	}
}

func inferCapitalActionType(text, fallback string) string {
	if typed := inferActionType(text); typed != "" {
		return typed
	}
	return fallback
}

func flattenRecords(value any) []map[string]any {
	out := []map[string]any{}
	var walk func(any, int)
	walk = func(value any, depth int) {
		if depth > 8 {
			return
		}
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				walk(item, depth+1)
			}
		case map[string]any:
			if looksLikeCorporateActionRecord(typed) {
				out = append(out, typed)
				return
			}
			for _, key := range []string{"results", "data", "items", "records", "list", "rows"} {
				if nested, ok := typed[key]; ok {
					walk(nested, depth+1)
				}
			}
		}
	}
	walk(value, 0)
	return out
}

func looksLikeCorporateActionRecord(record map[string]any) bool {
	text := strings.Join([]string{
		stringValue(record, "type", "event_type"),
		stringValue(record, "title", "summary", "description", "name"),
	}, " ")
	if inferActionType(text) != "" {
		return true
	}
	for _, key := range []string{"cash_amount", "gross_dividend", "brut_temettu", "bedelli_oran", "bedelsiz_oran", "split_ratio", "capital_increase_ratio"} {
		if _, ok := record[key]; ok {
			return true
		}
	}
	return false
}

func normalizeRecordSymbol(record map[string]any) string {
	return ohlcv.NormalizeSymbol(stringValue(record, "symbol", "ticker", "code", "company", "s"))
}

func parseDateFromRecord(record map[string]any, keys ...string) *time.Time {
	for _, key := range keys {
		if value := stringValue(record, key); value != "" {
			if t := parseDateString(value); t != nil {
				return t
			}
		}
	}
	return nil
}

func parseDateFromAny(values ...any) *time.Time {
	for _, value := range values {
		switch typed := value.(type) {
		case nil:
			continue
		case *string:
			if typed != nil {
				if t := parseDateString(*typed); t != nil {
					return t
				}
			}
		case string:
			if t := parseDateString(typed); t != nil {
				return t
			}
		}
	}
	return nil
}

func parseDateString(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02",
		"02.01.2006",
		"2.1.2006",
		"02/01/2006",
		"2/1/2006",
		"2006/01/02",
		"2006-01",
	} {
		if t, err := time.Parse(layout, value); err == nil {
			tt := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			return &tt
		}
	}
	if t := parseTurkishDate(value); t != nil {
		return t
	}
	if t := extractDateFromText(value); t != nil {
		return t
	}
	return nil
}

var numericDateInTextRE = regexp.MustCompile(`\b(\d{1,2})[./-](\d{1,2})[./-](\d{4})\b`)

func extractDateFromText(value string) *time.Time {
	match := numericDateInTextRE.FindString(value)
	if match == "" || strings.TrimSpace(match) == strings.TrimSpace(value) {
		return nil
	}
	return parseDateString(match)
}

func parseTurkishDate(value string) *time.Time {
	slug := util.SlugTR(value)
	months := map[string]time.Month{
		"ocak": time.January, "subat": time.February, "mart": time.March, "nisan": time.April,
		"mayis": time.May, "haziran": time.June, "temmuz": time.July, "agustos": time.August,
		"eylul": time.September, "ekim": time.October, "kasim": time.November, "aralik": time.December,
	}
	fields := strings.FieldsFunc(slug, func(r rune) bool {
		return r < '0' || (r > '9' && r < 'a') || r > 'z'
	})
	for i := 0; i+2 < len(fields); i++ {
		day, dayErr := strconv.Atoi(fields[i])
		year, yearErr := strconv.Atoi(fields[i+2])
		month, ok := months[fields[i+1]]
		if dayErr == nil && yearErr == nil && ok && day >= 1 && day <= 31 && year >= 1990 && year <= 2100 {
			t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
			return &t
		}
	}
	return nil
}

func cashAmountFromText(text string) *float64 {
	slug := util.SlugTR(text)
	if !strings.Contains(slug, "karpay") && !strings.Contains(slug, "temettu") && !strings.Contains(slug, "dividend") {
		return nil
	}
	if !strings.Contains(slug, "paybasina") && !strings.Contains(slug, "hissebasina") && !strings.Contains(slug, "brut") && !strings.Contains(slug, "net") {
		return nil
	}
	fields := strings.Fields(text)
	for i, field := range fields {
		n, ok := numberValue(strings.Trim(field, "():;,_"))
		if !ok || n <= 0 || n > 1000 {
			continue
		}
		start := i - 4
		if start < 0 {
			start = 0
		}
		end := i + 5
		if end > len(fields) {
			end = len(fields)
		}
		window := util.SlugTR(strings.Join(fields[start:end], " "))
		if strings.Contains(window, "nominal") {
			continue
		}
		if strings.Contains(window, "tl") && (strings.Contains(window, "paybasina") || strings.Contains(window, "hissebasina")) {
			return &n
		}
	}
	return nil
}

func numberPtrFromRecord(record map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		if value, ok := lookupCaseInsensitive(record, key); ok {
			if n, ok := numberValue(value); ok {
				return &n
			}
		}
	}
	return nil
}

func ratioFromText(text string) *float64 {
	for _, match := range percentNumberInTextRE.FindAllStringSubmatch(text, -1) {
		for _, value := range match[1:] {
			if n, ok := numberValue(value); ok {
				v := normalizeRatio(n)
				return &v
			}
		}
	}
	for _, match := range ratioLabelNumberInTextRE.FindAllStringSubmatch(text, -1) {
		for _, value := range match[1:] {
			if n, ok := numberValue(value); ok && n > 0 && n <= 1000 {
				v := normalizeRatio(n)
				return &v
			}
		}
	}
	return nil
}

var (
	percentNumberInTextRE    = regexp.MustCompile(`(?:%\s*(\d+(?:[.,]\d+)?)|(\d+(?:[.,]\d+)?)\s*%)`)
	ratioLabelNumberInTextRE = regexp.MustCompile(`(?i)(?:oran[ıi]?|yüzde|yuzde)\s*[:=]?\s*(\d+(?:[.,]\d+)?)`)
)

func normalizeRatio(value float64) float64 {
	if value > 1 {
		return value / 100.0
	}
	return value
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, typed != 0
	case int:
		return float64(typed), typed != 0
	case json.Number:
		n, err := typed.Float64()
		return n, err == nil && n != 0
	case string:
		clean := strings.TrimSpace(typed)
		if clean == "" || clean == "-" {
			return 0, false
		}
		clean = strings.ReplaceAll(clean, "%", "")
		clean = strings.ReplaceAll(clean, "TL", "")
		clean = strings.ReplaceAll(clean, "TRY", "")
		clean = strings.ReplaceAll(clean, " ", "")
		if strings.Contains(clean, ".") && strings.Contains(clean, ",") {
			clean = strings.ReplaceAll(clean, ".", "")
			clean = strings.ReplaceAll(clean, ",", ".")
		} else if strings.Contains(clean, ",") {
			clean = strings.ReplaceAll(clean, ",", ".")
		}
		n, err := strconv.ParseFloat(clean, 64)
		return n, err == nil && n != 0
	default:
		return 0, false
	}
}

func stringValue(record map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := lookupCaseInsensitive(record, key)
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case float64, int, json.Number:
			return fmt.Sprint(typed)
		}
	}
	return ""
}

func lookupCaseInsensitive(record map[string]any, key string) (any, bool) {
	if value, ok := record[key]; ok {
		return value, true
	}
	want := strings.ToLower(key)
	for k, value := range record {
		if strings.ToLower(k) == want {
			return value, true
		}
	}
	return nil, false
}

func perShareAmountLikely(text string) bool {
	slug := util.SlugTR(text)
	return strings.Contains(slug, "paybasina") || strings.Contains(slug, "hissebasina") || strings.Contains(slug, "1tlnominal")
}

func issuerCorporateActionDisclosure(text string) bool {
	slug := util.SlugTR(text)
	if inferActionType(text) == "" {
		return false
	}
	if strings.Contains(slug, "hakkullanimsureciptal") || strings.Contains(slug, "sureciptal") || strings.Contains(slug, "iptal") {
		return false
	}
	if strings.Contains(slug, "dagitimiyapilmamasi") || strings.Contains(slug, "dagitimiyapilmayacak") || strings.Contains(slug, "karpayidagitilmamasi") {
		return false
	}
	if strings.Contains(slug, "bagliortaklik") || strings.Contains(slug, "istirakimiz") || strings.Contains(slug, "isortakligimiz") {
		if !strings.Contains(slug, "sermayeartirimiazaltimiislemlerineiliskinbildirim") {
			return false
		}
	}
	if strings.Contains(slug, "sermayeartirimindaneldeedilecek") || strings.Contains(slug, "fonunkullanimina") && strings.Contains(slug, "rapor") {
		return false
	}
	return true
}

func concreteKAPActionDisclosure(text string) bool {
	slug := util.SlugTR(text)
	return strings.Contains(slug, "odemetarihi") ||
		strings.Contains(slug, "dagitimtarihi") ||
		strings.Contains(slug, "hakkikullanimtarihi") ||
		strings.Contains(slug, "hakkikullanimbaslangictarihi") ||
		strings.Contains(slug, "payalmahakkikullanim") ||
		strings.Contains(slug, "paydagitimtarihi") ||
		strings.Contains(slug, "ticaretsiciltescili")
}

func executionDateDisclosure(text string) bool {
	slug := util.SlugTR(text)
	return strings.Contains(slug, "tarih") && concreteKAPActionDisclosure(text)
}

func kapDisclosureConfidence(disclosure servicekap.FinancialDisclosure, ratio, cash *float64, effective *time.Time) float64 {
	confidence := 0.55
	if concreteKAPActionDisclosure(disclosure.Title + " " + disclosure.Summary) {
		confidence = 0.68
	}
	if effective != nil && (ratio != nil || cash != nil) {
		confidence = 0.78
	}
	if disclosure.AttachmentCount > 0 {
		confidence += 0.03
	}
	if confidence > 0.85 {
		return 0.85
	}
	return confidence
}

func disclosureURL(disclosure servicekap.FinancialDisclosure) string {
	if strings.TrimSpace(disclosure.URL) != "" {
		return strings.TrimSpace(disclosure.URL)
	}
	if disclosure.DisclosureIndex > 0 {
		return fmt.Sprintf("https://www.kap.org.tr/tr/Bildirim/%d", disclosure.DisclosureIndex)
	}
	return ""
}

func dateOnlyPtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	t := time.Date(value.UTC().Year(), value.UTC().Month(), value.UTC().Day(), 0, 0, 0, 0, time.UTC)
	return &t
}

func actionDateKey(action Action) string {
	if action.EffectiveDate != nil {
		return action.EffectiveDate.Format("2006-01-02")
	}
	if action.AnnouncementDate != nil {
		return action.AnnouncementDate.Format("2006-01-02")
	}
	return ""
}

func dedupeActions(actions []Action) []Action {
	out := []Action{}
	seen := map[string]int{}
	for _, action := range actions {
		key := strings.Join([]string{action.Symbol, action.Type, actionDateKey(action), fmtFloatPtr(action.Ratio), fmtFloatPtr(action.CashAmount), action.Title}, "|")
		if existing, ok := seen[key]; ok {
			if betterAction(action, out[existing]) {
				out[existing] = action
			}
			continue
		}
		seen[key] = len(out)
		out = append(out, action)
	}
	return out
}

func betterAction(candidate, current Action) bool {
	if candidate.Status == StatusVerified && current.Status != StatusVerified {
		return true
	}
	if candidate.Confidence != current.Confidence {
		return candidate.Confidence > current.Confidence
	}
	return len(candidate.Warnings) < len(current.Warnings)
}

func fmtFloatPtr(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', 6, 64)
}

func stableID(parts ...string) string {
	h := sha1.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:20]
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func existingFiles(candidates []string) []string {
	files := []string{}
	for _, path := range candidates {
		if fileExists(path) {
			files = append(files, path)
		}
	}
	return files
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
