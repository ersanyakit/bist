package professional

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hissebot/internal/domain/marketdata"
	"hissebot/internal/storage"
	"hissebot/pkg/mathutil"
)

const (
	marketMicrostructureMaxSpreadBps = 50.0
)

type MarketMicrostructureContext struct {
	Computed              bool                          `json:"computed"`
	Symbol                string                        `json:"symbol"`
	Source                string                        `json:"source,omitempty"`
	UpdatedAt             time.Time                     `json:"updated_at,omitempty"`
	Status                string                        `json:"status"`
	Score                 float64                       `json:"score"`
	Quote                 MarketMicrostructureQuote     `json:"quote,omitempty"`
	OrderBook             MarketOrderBookContext        `json:"order_book"`
	Depth                 MarketDepthContext            `json:"depth"`
	BrokerageDistribution MarketBrokerageDistribution   `json:"brokerage_distribution_akd"`
	Custody               MarketCustodyDistribution     `json:"custody_takas"`
	Equilibrium           MarketEquilibriumContext      `json:"equilibrium"`
	Liquidity             MarketMicrostructureLiquidity `json:"liquidity"`
	SourceFiles           []string                      `json:"source_files,omitempty"`
	Warnings              []string                      `json:"warnings,omitempty"`
}

type MarketMicrostructureQuote struct {
	Available bool      `json:"available"`
	Last      float64   `json:"last,omitempty"`
	Bid       float64   `json:"bid,omitempty"`
	Ask       float64   `json:"ask,omitempty"`
	High      float64   `json:"high,omitempty"`
	Low       float64   `json:"low,omitempty"`
	Volume    float64   `json:"volume,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type MarketOrderBookContext struct {
	Available     bool                   `json:"available"`
	BidLevels     int                    `json:"bid_levels,omitempty"`
	AskLevels     int                    `json:"ask_levels,omitempty"`
	BestBid       float64                `json:"best_bid,omitempty"`
	BestAsk       float64                `json:"best_ask,omitempty"`
	Spread        float64                `json:"spread,omitempty"`
	SpreadBps     float64                `json:"spread_bps,omitempty"`
	TopBidVolume  float64                `json:"top_bid_volume,omitempty"`
	TopAskVolume  float64                `json:"top_ask_volume,omitempty"`
	ImbalanceTop5 float64                `json:"imbalance_top5,omitempty"`
	TopBids       []MarketOrderBookLevel `json:"top_bids,omitempty"`
	TopAsks       []MarketOrderBookLevel `json:"top_asks,omitempty"`
	UpdatedAt     time.Time              `json:"updated_at,omitempty"`
}

type MarketOrderBookLevel struct {
	Level  int     `json:"level,omitempty"`
	Side   string  `json:"side,omitempty"`
	Price  float64 `json:"price,omitempty"`
	Volume float64 `json:"volume,omitempty"`
	Lot    float64 `json:"lot,omitempty"`
	Cost   float64 `json:"cost,omitempty"`
}

type MarketDepthContext struct {
	Available             bool               `json:"available"`
	Levels                int                `json:"levels,omitempty"`
	BuyVolumeWeightedPct  float64            `json:"buy_volume_weighted_pct,omitempty"`
	SellVolumeWeightedPct float64            `json:"sell_volume_weighted_pct,omitempty"`
	NetBuySellPct         float64            `json:"net_buy_sell_pct,omitempty"`
	TopDemandLevels       []MarketDepthLevel `json:"top_demand_levels,omitempty"`
	TopSupplyLevels       []MarketDepthLevel `json:"top_supply_levels,omitempty"`
	UpdatedAt             time.Time          `json:"updated_at,omitempty"`
}

type MarketDepthLevel struct {
	Price         float64 `json:"price,omitempty"`
	Lots          float64 `json:"lots,omitempty"`
	VolumePercent float64 `json:"volume_percent,omitempty"`
	BuyPercent    float64 `json:"buy_percent,omitempty"`
	SellPercent   float64 `json:"sell_percent,omitempty"`
}

type MarketBrokerageDistribution struct {
	Available        bool                    `json:"available"`
	ResultCount      int                     `json:"result_count,omitempty"`
	NetBuyTotal      float64                 `json:"net_buy_total,omitempty"`
	NetSellTotal     float64                 `json:"net_sell_total,omitempty"`
	TopNetBuyers     []MarketBrokerageRecord `json:"top_net_buyers,omitempty"`
	TopNetSellers    []MarketBrokerageRecord `json:"top_net_sellers,omitempty"`
	TopVolumeBrokers []MarketBrokerageRecord `json:"top_volume_brokers,omitempty"`
	UpdatedAt        time.Time               `json:"updated_at,omitempty"`
}

type MarketBrokerageRecord struct {
	Brokerage       string  `json:"brokerage"`
	NetSize         float64 `json:"net_size,omitempty"`
	NetPercentage   float64 `json:"net_percentage,omitempty"`
	NetCost         float64 `json:"net_cost,omitempty"`
	TotalSize       float64 `json:"total_size,omitempty"`
	TotalPercentage float64 `json:"total_percentage,omitempty"`
	TotalCost       float64 `json:"total_cost,omitempty"`
	TotalVolume     float64 `json:"total_volume,omitempty"`
}

type MarketCustodyDistribution struct {
	Available          bool                  `json:"available"`
	Date               string                `json:"date,omitempty"`
	ResultCount        int                   `json:"result_count,omitempty"`
	TopHolders         []MarketCustodyRecord `json:"top_holders,omitempty"`
	Top10Share         float64               `json:"top10_share,omitempty"`
	ForeignShare       float64               `json:"foreign_share,omitempty"`
	InstitutionalShare float64               `json:"institutional_share,omitempty"`
	AnomalyBroker      string                `json:"anomaly_broker,omitempty"`
	AnomalyLotChange   float64               `json:"anomaly_lot_change,omitempty"`
	UpdatedAt          time.Time             `json:"updated_at,omitempty"`
}

type MarketEquilibriumContext struct {
	Available    bool      `json:"available"`
	Price        float64   `json:"price,omitempty"`
	MatchedLots  float64   `json:"matched_lots,omitempty"`
	BidRemainder float64   `json:"bid_remainder,omitempty"`
	AskRemainder float64   `json:"ask_remainder,omitempty"`
	Imbalance    float64   `json:"imbalance,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

type MarketCustodyRecord struct {
	Custodian  string  `json:"custodian"`
	Value      float64 `json:"value,omitempty"`
	Percentage float64 `json:"percentage,omitempty"`
}

type MarketMicrostructureLiquidity struct {
	TopOfBookAvailable     bool     `json:"top_of_book_available"`
	SpreadBps              float64  `json:"spread_bps,omitempty"`
	ValueTradedTRY         float64  `json:"value_traded_try,omitempty"`
	AutomaticOrderReady    bool     `json:"automatic_order_ready"`
	AutomaticOrderBlockers []string `json:"automatic_order_blockers,omitempty"`
	MicrostructureComplete bool     `json:"microstructure_complete"`
	DecisionUsable         bool     `json:"decision_usable"`
}

func loadMarketMicrostructure(equitiesDir, symbol string) *MarketMicrostructureContext {
	symbol = storage.NormalizeTicker(symbol)
	ctx := &MarketMicrostructureContext{
		Symbol: symbol,
		Source: "market_ws_symbol_files",
		Status: "missing",
	}
	if strings.TrimSpace(equitiesDir) == "" || symbol == "" {
		ctx.Warnings = append(ctx.Warnings, "market_ws_symbol_context_missing_input")
		return ctx
	}
	root := filepath.Join(equitiesDir, symbol, "market_ws")
	if _, err := os.Stat(root); err != nil {
		ctx.Warnings = append(ctx.Warnings, "market_ws_symbol_dir_missing")
		finalizeMarketMicrostructure(ctx)
		return ctx
	}
	loadMicrostructureQuote(ctx, latestMarketWSFile(root, "live_symbol_snapshot.json"), symbol)
	loadOrderBook(ctx, latestMarketWSFile(root, "order_book.json"))
	loadDepth(ctx, latestMarketWSFile(root, "kdm2_data.json"))
	akdPath := latestMarketWSFile(root, "akd_data.json")
	if akdPath == "" {
		akdPath = latestMarketWSFile(root, "akd_scan_data.json")
	}
	loadBrokerageDistribution(ctx, akdPath)
	loadCustodyDistribution(ctx, latestMarketWSFile(root, "custodian_data.json"))
	if !ctx.Custody.Available {
		loadCustodyScans(ctx,
			latestMarketWSFile(root, "custody_scan_foreign_data.json"),
			latestMarketWSFile(root, "custody_scan_institution_data.json"),
			latestMarketWSFile(root, "custody_scan_anomaly_data.json"),
		)
	}
	loadEquilibrium(ctx, latestMarketWSFile(root, "equilibrium_data.json"))
	finalizeMarketMicrostructure(ctx)
	return ctx
}

func latestMarketWSFile(root, name string) string {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(name) == "" {
		return ""
	}
	direct := filepath.Join(root, name)
	if info, err := os.Stat(direct); err == nil && !info.IsDir() {
		return direct
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	dirs := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, dir := range dirs {
		candidate := filepath.Join(root, dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func liveSnapshotFromMarketMicrostructure(symbol string, micro *MarketMicrostructureContext) *marketdata.LiveMarketSnapshot {
	if micro == nil || !micro.Quote.Available {
		return nil
	}
	updatedAt := micro.Quote.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = micro.UpdatedAt
	}
	return &marketdata.LiveMarketSnapshot{
		Source:    micro.Source,
		UpdatedAt: updatedAt,
		Symbols: map[string]marketdata.LiveSymbolSnapshot{
			storage.NormalizeTicker(symbol): {
				Symbol:    storage.NormalizeTicker(symbol),
				Last:      micro.Quote.Last,
				Bid:       micro.Quote.Bid,
				Ask:       micro.Quote.Ask,
				High:      micro.Quote.High,
				Low:       micro.Quote.Low,
				Volume:    micro.Quote.Volume,
				UpdatedAt: updatedAt,
			},
		},
		Datasets: map[string][]json.RawMessage{},
	}
}

func loadMicrostructureQuote(ctx *MarketMicrostructureContext, path string, symbol string) {
	var snapshots []marketdata.LiveSymbolSnapshot
	if !readJSONFile(path, &snapshots) {
		return
	}
	ctx.SourceFiles = append(ctx.SourceFiles, filepath.Clean(path))
	for _, snapshot := range snapshots {
		if storage.NormalizeTicker(snapshot.Symbol) != symbol {
			continue
		}
		ctx.Quote = MarketMicrostructureQuote{
			Available: true,
			Last:      snapshot.Last,
			Bid:       snapshot.Bid,
			Ask:       snapshot.Ask,
			High:      snapshot.High,
			Low:       snapshot.Low,
			Volume:    snapshot.Volume,
			UpdatedAt: snapshot.UpdatedAt,
		}
		updateMicrostructureTime(ctx, snapshot.UpdatedAt)
		return
	}
}

func loadOrderBook(ctx *MarketMicrostructureContext, path string) {
	payload, ok := readAnyJSONFile(path)
	if !ok {
		return
	}
	ctx.SourceFiles = append(ctx.SourceFiles, filepath.Clean(path))
	obj, updatedAt, ok := firstObjectWithArray(payload, "bids", func(item map[string]any) bool {
		return numberFromAny(item["price"]) > 0
	})
	if !ok {
		return
	}
	asks := arrayFromAny(obj["asks"])
	bids := arrayFromAny(obj["bids"])
	parsedBids := parseOrderBookLevels(bids, "bid")
	parsedAsks := parseOrderBookLevels(asks, "ask")
	if len(parsedBids) == 0 || len(parsedAsks) == 0 {
		return
	}
	sort.SliceStable(parsedBids, func(i, j int) bool { return parsedBids[i].Price > parsedBids[j].Price })
	sort.SliceStable(parsedAsks, func(i, j int) bool { return parsedAsks[i].Price < parsedAsks[j].Price })
	bestBid := parsedBids[0].Price
	bestAsk := parsedAsks[0].Price
	spread := bestAsk - bestBid
	last := ctx.Quote.Last
	if last <= 0 {
		last = (bestBid + bestAsk) / 2
	}
	topBidVolume := sumOrderBookVolume(parsedBids, 5)
	topAskVolume := sumOrderBookVolume(parsedAsks, 5)
	ctx.OrderBook = MarketOrderBookContext{
		Available:     true,
		BidLevels:     len(parsedBids),
		AskLevels:     len(parsedAsks),
		BestBid:       bestBid,
		BestAsk:       bestAsk,
		Spread:        spread,
		SpreadBps:     mathutil.SafeDiv(spread, last) * 10000,
		TopBidVolume:  topBidVolume,
		TopAskVolume:  topAskVolume,
		ImbalanceTop5: mathutil.SafeDiv(topBidVolume-topAskVolume, topBidVolume+topAskVolume),
		TopBids:       limitOrderBookLevels(parsedBids, 5),
		TopAsks:       limitOrderBookLevels(parsedAsks, 5),
	}
	if updatedAt != nil {
		ctx.OrderBook.UpdatedAt = *updatedAt
		updateMicrostructureTime(ctx, *updatedAt)
	}
}

func loadDepth(ctx *MarketMicrostructureContext, path string) {
	payload, ok := readAnyJSONFile(path)
	if !ok {
		return
	}
	ctx.SourceFiles = append(ctx.SourceFiles, filepath.Clean(path))
	obj, updatedAt, ok := firstObjectWithArray(payload, "depth_levels", func(item map[string]any) bool {
		return numberFromAny(item["price"]) > 0
	})
	if !ok {
		return
	}
	levels := parseDepthLevels(arrayFromAny(obj["depth_levels"]))
	if len(levels) == 0 {
		return
	}
	var weightedBuy, weightedSell, totalWeight float64
	for _, level := range levels {
		weight := level.VolumePercent
		if weight <= 0 {
			weight = level.Lots
		}
		weightedBuy += level.BuyPercent * weight
		weightedSell += level.SellPercent * weight
		totalWeight += weight
	}
	sort.SliceStable(levels, func(i, j int) bool {
		if levels[i].VolumePercent == levels[j].VolumePercent {
			return levels[i].Lots > levels[j].Lots
		}
		return levels[i].VolumePercent > levels[j].VolumePercent
	})
	demand := append([]MarketDepthLevel(nil), levels...)
	sort.SliceStable(demand, func(i, j int) bool {
		if demand[i].BuyPercent == demand[j].BuyPercent {
			return demand[i].VolumePercent > demand[j].VolumePercent
		}
		return demand[i].BuyPercent > demand[j].BuyPercent
	})
	supply := append([]MarketDepthLevel(nil), levels...)
	sort.SliceStable(supply, func(i, j int) bool {
		if supply[i].SellPercent == supply[j].SellPercent {
			return supply[i].VolumePercent > supply[j].VolumePercent
		}
		return supply[i].SellPercent > supply[j].SellPercent
	})
	buyPct := mathutil.SafeDiv(weightedBuy, totalWeight)
	sellPct := mathutil.SafeDiv(weightedSell, totalWeight)
	ctx.Depth = MarketDepthContext{
		Available:             true,
		Levels:                len(levels),
		BuyVolumeWeightedPct:  buyPct,
		SellVolumeWeightedPct: sellPct,
		NetBuySellPct:         buyPct - sellPct,
		TopDemandLevels:       limitDepthLevels(demand, 5),
		TopSupplyLevels:       limitDepthLevels(supply, 5),
	}
	if updatedAt != nil {
		ctx.Depth.UpdatedAt = *updatedAt
		updateMicrostructureTime(ctx, *updatedAt)
	}
}

func loadBrokerageDistribution(ctx *MarketMicrostructureContext, path string) {
	payload, ok := readAnyJSONFile(path)
	if !ok {
		return
	}
	ctx.SourceFiles = append(ctx.SourceFiles, filepath.Clean(path))
	arrayKey := "results"
	obj, updatedAt, ok := firstObjectWithArray(payload, arrayKey, func(item map[string]any) bool {
		return strings.TrimSpace(stringFromAny(item["brokerage"])) != ""
	})
	if !ok {
		arrayKey = "brokers"
		obj, updatedAt, ok = firstObjectWithArray(payload, arrayKey, func(item map[string]any) bool {
			return strings.TrimSpace(stringFromAny(firstPresentAny(item, "broker_code", "brokerage"))) != ""
		})
	}
	if !ok {
		return
	}
	records := parseBrokerageRecords(arrayFromAny(obj[arrayKey]))
	if len(records) == 0 {
		return
	}
	buyers := append([]MarketBrokerageRecord(nil), records...)
	sort.SliceStable(buyers, func(i, j int) bool { return buyers[i].NetSize > buyers[j].NetSize })
	sellers := append([]MarketBrokerageRecord(nil), records...)
	sort.SliceStable(sellers, func(i, j int) bool { return sellers[i].NetSize < sellers[j].NetSize })
	volume := append([]MarketBrokerageRecord(nil), records...)
	sort.SliceStable(volume, func(i, j int) bool { return volume[i].TotalVolume > volume[j].TotalVolume })
	var netBuyTotal, netSellTotal float64
	for _, record := range records {
		if record.NetSize > 0 {
			netBuyTotal += record.NetSize
		} else if record.NetSize < 0 {
			netSellTotal += -record.NetSize
		}
	}
	ctx.BrokerageDistribution = MarketBrokerageDistribution{
		Available:        true,
		ResultCount:      len(records),
		NetBuyTotal:      netBuyTotal,
		NetSellTotal:     netSellTotal,
		TopNetBuyers:     limitBrokerageRecords(filterBrokerageRecords(buyers, func(r MarketBrokerageRecord) bool { return r.NetSize > 0 }), 5),
		TopNetSellers:    limitBrokerageRecords(filterBrokerageRecords(sellers, func(r MarketBrokerageRecord) bool { return r.NetSize < 0 }), 5),
		TopVolumeBrokers: limitBrokerageRecords(volume, 5),
	}
	if updatedAt != nil {
		ctx.BrokerageDistribution.UpdatedAt = *updatedAt
		updateMicrostructureTime(ctx, *updatedAt)
	}
}

func loadCustodyScans(ctx *MarketMicrostructureContext, foreignPath, institutionPath, anomalyPath string) {
	records := []MarketCustodyRecord{}
	date := ""
	var updatedAt time.Time
	if payload, ok := readAnyJSONFile(foreignPath); ok {
		if obj, at, found := firstObjectWithArray(payload, "foreign_brokers", nil); found {
			for _, item := range arrayFromAny(obj["foreign_brokers"]) {
				row := mapFromAny(item)
				broker := stringFromAny(firstPresentAny(row, "broker_code", "custodian"))
				if broker == "" {
					continue
				}
				records = append(records, MarketCustodyRecord{
					Custodian: broker, Value: numberFromAny(row["lot_count"]), Percentage: numberFromAny(row["percentage"]),
				})
			}
			ctx.Custody.ForeignShare = numberFromAny(obj["foreign_pct"])
			date = stringFromAny(obj["trade_date"])
			if at != nil {
				updatedAt = *at
			}
			ctx.SourceFiles = append(ctx.SourceFiles, filepath.Clean(foreignPath))
		}
	}
	if payload, ok := readAnyJSONFile(institutionPath); ok {
		obj, at, found := firstObjectWithKey(payload, "institutional_pct")
		if !found {
			// Some scan responses expose a single institution row and only carry
			// its percentage, without a separate aggregate institutional_pct.
			obj, at, found = firstObjectWithKey(payload, "percentage")
		}
		if found {
			pct := numberFromAny(obj["institutional_pct"])
			if pct == 0 {
				pct = numberFromAny(obj["percentage"])
			}
			ctx.Custody.InstitutionalShare = pct
			broker := stringFromAny(firstPresentAny(obj, "broker_code", "custodian"))
			if broker != "" {
				records = append(records, MarketCustodyRecord{
					Custodian: broker, Value: numberFromAny(obj["lot_count"]), Percentage: numberFromAny(obj["percentage"]),
				})
			}
			if date == "" {
				date = stringFromAny(obj["trade_date"])
			}
			if at != nil && (updatedAt.IsZero() || at.After(updatedAt)) {
				updatedAt = *at
			}
			ctx.SourceFiles = append(ctx.SourceFiles, filepath.Clean(institutionPath))
		}
	}
	if payload, ok := readAnyJSONFile(anomalyPath); ok {
		if obj, at, found := firstObjectWithKey(payload, "lot_change"); found {
			ctx.Custody.AnomalyBroker = stringFromAny(firstPresentAny(obj, "broker_code", "custodian"))
			ctx.Custody.AnomalyLotChange = numberFromAny(obj["lot_change"])
			if at != nil && (updatedAt.IsZero() || at.After(updatedAt)) {
				updatedAt = *at
			}
			ctx.SourceFiles = append(ctx.SourceFiles, filepath.Clean(anomalyPath))
		}
	}
	if len(records) == 0 && ctx.Custody.ForeignShare == 0 && ctx.Custody.InstitutionalShare == 0 && ctx.Custody.AnomalyBroker == "" {
		return
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].Percentage > records[j].Percentage })
	topShare := 0.0
	for i, record := range records {
		if i >= 10 {
			break
		}
		topShare += record.Percentage
	}
	ctx.Custody.Available = true
	ctx.Custody.Date = date
	ctx.Custody.ResultCount = len(records)
	ctx.Custody.TopHolders = limitCustodyRecords(records, 10)
	ctx.Custody.Top10Share = topShare
	ctx.Custody.UpdatedAt = updatedAt
	updateMicrostructureTime(ctx, updatedAt)
}

func loadEquilibrium(ctx *MarketMicrostructureContext, path string) {
	payload, ok := readAnyJSONFile(path)
	if !ok {
		return
	}
	obj, updatedAt, ok := firstObjectWithKey(payload, "equilibrium_price_or_last_lot")
	if !ok {
		return
	}
	bid := numberFromAny(obj["equilibrium_bid_remainder"])
	ask := numberFromAny(obj["equilibrium_ask_remainder"])
	price := numberFromAny(obj["equilibrium_price_or_last_lot"])
	matched := numberFromAny(obj["equilibrium_match_quantity"])
	ctx.Equilibrium = MarketEquilibriumContext{
		Available: true, Price: price, MatchedLots: matched,
		BidRemainder: bid, AskRemainder: ask,
		Imbalance: mathutil.SafeDiv(bid-ask, bid+ask),
	}
	if updatedAt != nil {
		ctx.Equilibrium.UpdatedAt = *updatedAt
		updateMicrostructureTime(ctx, *updatedAt)
	}
	ctx.SourceFiles = append(ctx.SourceFiles, filepath.Clean(path))
}

func loadCustodyDistribution(ctx *MarketMicrostructureContext, path string) {
	payload, ok := readAnyJSONFile(path)
	if !ok {
		return
	}
	ctx.SourceFiles = append(ctx.SourceFiles, filepath.Clean(path))
	obj, updatedAt, ok := firstObjectWithArray(payload, "results", func(item map[string]any) bool {
		return strings.TrimSpace(stringFromAny(item["custodian"])) != ""
	})
	if !ok {
		return
	}
	records := parseCustodyRecords(arrayFromAny(obj["results"]))
	if len(records) == 0 {
		return
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].Percentage > records[j].Percentage })
	var top10 float64
	for i, record := range records {
		if i >= 10 {
			break
		}
		top10 += record.Percentage
	}
	ctx.Custody = MarketCustodyDistribution{
		Available:   true,
		Date:        stringFromAny(obj["date"]),
		ResultCount: len(records),
		TopHolders:  limitCustodyRecords(records, 10),
		Top10Share:  top10,
	}
	if updatedAt != nil {
		ctx.Custody.UpdatedAt = *updatedAt
		updateMicrostructureTime(ctx, *updatedAt)
	}
}

func finalizeMarketMicrostructure(ctx *MarketMicrostructureContext) {
	if ctx == nil {
		return
	}
	score := 0.0
	if ctx.Quote.Available {
		score += 20
	} else {
		ctx.Warnings = append(ctx.Warnings, "market_ws_live_symbol_snapshot_missing")
	}
	if ctx.OrderBook.Available {
		score += 25
	} else {
		ctx.Warnings = append(ctx.Warnings, "market_ws_order_book_missing")
	}
	if ctx.Depth.Available {
		score += 20
	} else {
		ctx.Warnings = append(ctx.Warnings, "market_ws_kdm2_depth_missing")
	}
	if ctx.BrokerageDistribution.Available {
		score += 20
	} else {
		ctx.Warnings = append(ctx.Warnings, "market_ws_akd_missing")
	}
	if ctx.Custody.Available {
		score += 15
	} else {
		ctx.Warnings = append(ctx.Warnings, "market_ws_takas_missing")
	}
	if ctx.Equilibrium.Available {
		score += 10
	}
	ctx.Score = mathutil.Clamp(score, 0, 100)
	ctx.Computed = ctx.Score > 0
	ctx.Liquidity.TopOfBookAvailable = ctx.OrderBook.Available && ctx.OrderBook.BestBid > 0 && ctx.OrderBook.BestAsk >= ctx.OrderBook.BestBid
	ctx.Liquidity.SpreadBps = ctx.OrderBook.SpreadBps
	ctx.Liquidity.ValueTradedTRY = ctx.Quote.Volume
	ctx.Liquidity.MicrostructureComplete = ctx.Quote.Available && ctx.OrderBook.Available && ctx.Depth.Available && ctx.BrokerageDistribution.Available && ctx.Custody.Available
	ctx.Liquidity.DecisionUsable = ctx.Score >= 50 && ctx.Quote.Available &&
		(ctx.OrderBook.Available || ctx.Depth.Available || ctx.BrokerageDistribution.Available || ctx.Custody.Available || ctx.Equilibrium.Available)
	ctx.Liquidity.AutomaticOrderReady = ctx.Liquidity.MicrostructureComplete && ctx.Liquidity.TopOfBookAvailable && (ctx.OrderBook.SpreadBps == 0 || ctx.OrderBook.SpreadBps <= marketMicrostructureMaxSpreadBps)
	if !ctx.Liquidity.TopOfBookAvailable {
		ctx.Liquidity.AutomaticOrderBlockers = append(ctx.Liquidity.AutomaticOrderBlockers, "top_of_book_missing")
	}
	if ctx.OrderBook.Available && ctx.OrderBook.SpreadBps > marketMicrostructureMaxSpreadBps {
		ctx.Liquidity.AutomaticOrderBlockers = append(ctx.Liquidity.AutomaticOrderBlockers, fmt.Sprintf("spread_too_wide_%.1fbps", ctx.OrderBook.SpreadBps))
	}
	if !ctx.Depth.Available {
		ctx.Liquidity.AutomaticOrderBlockers = append(ctx.Liquidity.AutomaticOrderBlockers, "depth_levels_missing")
	}
	if !ctx.BrokerageDistribution.Available {
		ctx.Liquidity.AutomaticOrderBlockers = append(ctx.Liquidity.AutomaticOrderBlockers, "akd_missing")
	}
	if !ctx.Custody.Available {
		ctx.Liquidity.AutomaticOrderBlockers = append(ctx.Liquidity.AutomaticOrderBlockers, "takas_missing")
	}
	switch {
	case ctx.Score >= 80 && ctx.Liquidity.AutomaticOrderReady:
		ctx.Status = "pass"
	case ctx.Score >= 60 && ctx.Liquidity.DecisionUsable:
		ctx.Status = "limited"
	default:
		ctx.Status = "missing"
	}
	ctx.SourceFiles = uniqueSortedStrings(ctx.SourceFiles)
	ctx.Warnings = uniqueSortedStrings(ctx.Warnings)
}

func updateMicrostructureTime(ctx *MarketMicrostructureContext, value time.Time) {
	if ctx == nil || value.IsZero() {
		return
	}
	if ctx.UpdatedAt.IsZero() || value.After(ctx.UpdatedAt) {
		ctx.UpdatedAt = value
	}
}

func readJSONFile(path string, out any) bool {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(bytes, out) == nil
}

func readAnyJSONFile(path string) (any, bool) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var payload any
	if err := json.Unmarshal(bytes, &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func firstObjectWithArray(value any, arrayKey string, predicate func(map[string]any) bool) (map[string]any, *time.Time, bool) {
	var found map[string]any
	var foundAt *time.Time
	var walk func(any, *time.Time)
	walk = func(node any, inheritedAt *time.Time) {
		if found != nil {
			return
		}
		switch typed := node.(type) {
		case []any:
			for _, item := range typed {
				walk(item, inheritedAt)
			}
		case map[string]any:
			updatedAt := inheritedAt
			if parsed, ok := parseAnyTime(firstPresentAny(typed, "updated_at", "timestamp", "fetched_at", "received_at", "time")); ok {
				updatedAt = &parsed
			}
			if arr := arrayFromAny(typed[arrayKey]); len(arr) > 0 {
				if predicate == nil {
					found = typed
					foundAt = updatedAt
					return
				}
				for _, item := range arr {
					if obj, ok := item.(map[string]any); ok && predicate(obj) {
						found = typed
						foundAt = updatedAt
						return
					}
				}
			}
			for _, child := range typed {
				walk(child, updatedAt)
			}
		}
	}
	walk(value, nil)
	return found, foundAt, found != nil
}

func firstObjectWithKey(value any, key string) (map[string]any, *time.Time, bool) {
	var found map[string]any
	var foundAt *time.Time
	var walk func(any, *time.Time)
	walk = func(node any, inheritedAt *time.Time) {
		if found != nil {
			return
		}
		switch typed := node.(type) {
		case []any:
			for _, item := range typed {
				walk(item, inheritedAt)
			}
		case map[string]any:
			updatedAt := inheritedAt
			if parsed, ok := parseAnyTime(firstPresentAny(typed, "updated_at", "timestamp", "fetched_at", "received_at", "time")); ok {
				updatedAt = &parsed
			}
			if _, ok := typed[key]; ok {
				found = typed
				foundAt = updatedAt
				return
			}
			for _, child := range typed {
				walk(child, updatedAt)
			}
		}
	}
	walk(value, nil)
	return found, foundAt, found != nil
}

func parseOrderBookLevels(values []any, side string) []MarketOrderBookLevel {
	out := []MarketOrderBookLevel{}
	for _, item := range values {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		level := MarketOrderBookLevel{
			Level:  int(numberFromAny(obj["level"])),
			Side:   strings.TrimSpace(stringFromAny(obj["side"])),
			Price:  numberFromAny(obj["price"]),
			Volume: numberFromAny(obj["volume"]),
			Lot:    numberFromAny(obj["lot"]),
			Cost:   numberFromAny(obj["cost"]),
		}
		if level.Side == "" {
			level.Side = side
		}
		if level.Price <= 0 {
			continue
		}
		out = append(out, level)
	}
	return out
}

func parseDepthLevels(values []any) []MarketDepthLevel {
	out := []MarketDepthLevel{}
	for _, item := range values {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		level := MarketDepthLevel{
			Price:         numberFromAny(obj["price"]),
			Lots:          numberFromAny(obj["lots"]),
			VolumePercent: numberFromAny(obj["volume_percent"]),
			BuyPercent:    numberFromAny(obj["buy_percent"]),
			SellPercent:   numberFromAny(obj["sell_percent"]),
		}
		if level.Price <= 0 {
			continue
		}
		out = append(out, level)
	}
	return out
}

func parseBrokerageRecords(values []any) []MarketBrokerageRecord {
	out := []MarketBrokerageRecord{}
	for _, item := range values {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		net := mapFromAny(obj["net"])
		total := mapFromAny(obj["total"])
		record := MarketBrokerageRecord{
			Brokerage:       strings.TrimSpace(stringFromAny(firstPresentAny(obj, "brokerage", "broker_code"))),
			NetSize:         firstNonZeroNumber(numberFromAny(net["size"]), numberFromAny(obj["net_size"])),
			NetPercentage:   firstNonZeroNumber(numberFromAny(net["percentage"]), numberFromAny(obj["net_percentage"])),
			NetCost:         firstNonZeroNumber(numberFromAny(net["cost"]), numberFromAny(obj["net_cost"])),
			TotalSize:       numberFromAny(total["size"]),
			TotalPercentage: numberFromAny(total["percentage"]),
			TotalCost:       numberFromAny(total["cost"]),
			TotalVolume:     numberFromAny(total["volume"]),
		}
		if record.Brokerage == "" {
			continue
		}
		out = append(out, record)
	}
	return out
}

func firstNonZeroNumber(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func parseCustodyRecords(values []any) []MarketCustodyRecord {
	out := []MarketCustodyRecord{}
	for _, item := range values {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		record := MarketCustodyRecord{
			Custodian:  strings.TrimSpace(stringFromAny(obj["custodian"])),
			Value:      numberFromAny(obj["value"]),
			Percentage: numberFromAny(obj["percentage"]),
		}
		if record.Custodian == "" {
			continue
		}
		out = append(out, record)
	}
	return out
}

func sumOrderBookVolume(values []MarketOrderBookLevel, limit int) float64 {
	total := 0.0
	for i, value := range values {
		if i >= limit {
			break
		}
		total += value.Volume
	}
	return total
}

func limitOrderBookLevels(values []MarketOrderBookLevel, limit int) []MarketOrderBookLevel {
	if len(values) <= limit {
		return values
	}
	return append([]MarketOrderBookLevel(nil), values[:limit]...)
}

func limitDepthLevels(values []MarketDepthLevel, limit int) []MarketDepthLevel {
	if len(values) <= limit {
		return values
	}
	return append([]MarketDepthLevel(nil), values[:limit]...)
}

func limitBrokerageRecords(values []MarketBrokerageRecord, limit int) []MarketBrokerageRecord {
	if len(values) <= limit {
		return values
	}
	return append([]MarketBrokerageRecord(nil), values[:limit]...)
}

func limitCustodyRecords(values []MarketCustodyRecord, limit int) []MarketCustodyRecord {
	if len(values) <= limit {
		return values
	}
	return append([]MarketCustodyRecord(nil), values[:limit]...)
}

func filterBrokerageRecords(values []MarketBrokerageRecord, keep func(MarketBrokerageRecord) bool) []MarketBrokerageRecord {
	out := []MarketBrokerageRecord{}
	for _, value := range values {
		if keep(value) {
			out = append(out, value)
		}
	}
	return out
}

func mapFromAny(value any) map[string]any {
	if out, ok := value.(map[string]any); ok {
		return out
	}
	return nil
}

func arrayFromAny(value any) []any {
	if out, ok := value.([]any); ok {
		return out
	}
	return nil
}

func firstPresentAny(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func numberFromAny(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		out, _ := typed.Float64()
		return out
	case string:
		parsed := strings.ReplaceAll(strings.TrimSpace(typed), ",", ".")
		var out float64
		if _, err := fmt.Sscanf(parsed, "%f", &out); err == nil {
			return out
		}
	}
	return 0
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func parseAnyTime(value any) (time.Time, bool) {
	text := stringFromAny(value)
	if text == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
