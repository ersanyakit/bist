package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	DataDir                   string
	SeedDir                   string
	CacheDir                  string
	EquitiesDir               string
	CommentsDir               string
	TradingViewCacheDir       string
	BilancoCacheDir           string
	KAPCompaniesFile          string
	KAPDisclosuresFile        string
	KAPDisclosuresURL         string
	KAPDisclosuresToken       string
	KAPSectorsFile            string
	KAPSectorsURL             string
	InvestingTrackIDsFile     string
	TradingViewRequestsFile   string
	MKKNameOverridesFile      string
	SectorClassificationsFile string
	ValuationAssumptionsFile  string
	GoldenFinancialRatiosFile string
	TUIKGDPFile               string
	TUIKInflationFile         string
	TCMBDir                   string
	TCMBEVDSBaseURL           string
	TCMBEVDSDir               string
	VAPDir                    string
	VAPIndexPortfolioFile     string
	MarketWSURL               string
	MarketMQTTURL             string
	MarketMQTTTopics          string
	MarketMQTTClientID        string
	MarketMQTTUsername        string
	MarketMQTTPassword        string
	MarketMQTTConnectHex      string
	MarketMQTTProtocolName    string
	MarketMQTTProtocolLevel   string
	MarketSnapshotFile        string
	MatriksFormationsURL      string
	MatriksFormationsFile     string
	UniverseFile              string
	DelistedUniverseFile      string
	AuditLogPath              string
	LegacyBilancoFile         string
	EndpointURL               string
	EndpointToken             string
	MKKCookie                 string
	IsYatirimCookie           string
	TKGMCookie                string
	TradingViewHistoryURL     string
	TradingViewChartTransport string
	HTTPTimeout               time.Duration
	CommandTimeout            time.Duration
}

func Load() Config {
	dataDir := getenv("HISSEBOT_DATA_DIR", "data")
	seedDir := filepath.Join(dataDir, "seed")
	cacheDir := filepath.Join(dataDir, "cache")

	return Config{
		DataDir:                   dataDir,
		SeedDir:                   seedDir,
		CacheDir:                  cacheDir,
		EquitiesDir:               filepath.Join(dataDir, "equities"),
		CommentsDir:               filepath.Join(dataDir, "comments"),
		TradingViewCacheDir:       filepath.Join(cacheDir, "tradingview"),
		BilancoCacheDir:           filepath.Join(cacheDir, "bilanco"),
		KAPCompaniesFile:          filepath.Join(seedDir, "kap_companies.json"),
		KAPDisclosuresFile:        getenv("HISSEBOT_KAP_DISCLOSURES_FILE", filepath.Join(seedDir, "kap_disclosures.json")),
		KAPDisclosuresURL:         os.Getenv("HISSEBOT_KAP_DISCLOSURES_URL"),
		KAPDisclosuresToken:       os.Getenv("HISSEBOT_KAP_DISCLOSURES_TOKEN"),
		KAPSectorsFile:            getenv("HISSEBOT_KAP_SECTORS_FILE", filepath.Join(seedDir, "kap_sectors.json")),
		KAPSectorsURL:             getenv("HISSEBOT_KAP_SECTORS_URL", "https://kap.org.tr/tr/Sektorler"),
		InvestingTrackIDsFile:     filepath.Join(seedDir, "investing_track_ids.json"),
		TradingViewRequestsFile:   filepath.Join(seedDir, "tradingview_requests.json"),
		MKKNameOverridesFile:      filepath.Join(seedDir, "mkk_name_overrides.json"),
		SectorClassificationsFile: getenv("HISSEBOT_SECTOR_CLASSIFICATIONS_FILE", filepath.Join(seedDir, "sector_classifications.json")),
		ValuationAssumptionsFile:  getenv("HISSEBOT_VALUATION_ASSUMPTIONS_FILE", filepath.Join(seedDir, "valuation_assumptions.json")),
		GoldenFinancialRatiosFile: getenv("HISSEBOT_GOLDEN_FINANCIAL_RATIOS_FILE", filepath.Join(seedDir, "golden_financial_ratios.json")),
		TUIKGDPFile:               getenv("HISSEBOT_TUIK_GDP_FILE", filepath.Join(dataDir, "macro", "tuik_gdp.json")),
		TUIKInflationFile:         getenv("HISSEBOT_TUIK_INFLATION_FILE", filepath.Join(dataDir, "macro", "tuik_inflation_indices.json")),
		TCMBDir:                   getenv("HISSEBOT_TCMB_DIR", filepath.Join(dataDir, "macro", "tcmb")),
		TCMBEVDSBaseURL:           getenv("HISSEBOT_TCMB_EVDS_BASE_URL", "https://evds3.tcmb.gov.tr/igmevdsms-dis"),
		TCMBEVDSDir:               getenv("HISSEBOT_TCMB_EVDS_DIR", filepath.Join(dataDir, "macro", "tcmb_evds")),
		VAPDir:                    getenv("HISSEBOT_VAP_DIR", filepath.Join(dataDir, "macro", "vap")),
		VAPIndexPortfolioFile:     getenv("HISSEBOT_VAP_INDEX_PORTFOLIO_FILE", filepath.Join(dataDir, "macro", "vap", "bist_endeks_portfoy.json")),
		MarketWSURL:               os.Getenv("HISSEBOT_MARKET_WS_URL"),
		MarketMQTTURL:             os.Getenv("HISSEBOT_MARKET_MQTT_URL"),
		MarketMQTTTopics:          os.Getenv("HISSEBOT_MARKET_MQTT_TOPICS"),
		MarketMQTTClientID:        os.Getenv("HISSEBOT_MARKET_MQTT_CLIENT_ID"),
		MarketMQTTUsername:        os.Getenv("HISSEBOT_MARKET_MQTT_USERNAME"),
		MarketMQTTPassword:        os.Getenv("HISSEBOT_MARKET_MQTT_PASSWORD"),
		MarketMQTTConnectHex:      os.Getenv("HISSEBOT_MARKET_MQTT_CONNECT_HEX"),
		MarketMQTTProtocolName:    os.Getenv("HISSEBOT_MARKET_MQTT_PROTOCOL_NAME"),
		MarketMQTTProtocolLevel:   os.Getenv("HISSEBOT_MARKET_MQTT_PROTOCOL_LEVEL"),
		MarketSnapshotFile:        getenv("HISSEBOT_MARKET_SNAPSHOT_FILE", filepath.Join(dataDir, "market", "live_snapshot.json")),
		MatriksFormationsURL:      os.Getenv("HISSEBOT_MATRIKS_FORMATIONS_URL"),
		MatriksFormationsFile:     getenv("HISSEBOT_MATRIKS_FORMATIONS_FILE", filepath.Join(dataDir, "market", "matriks_formations.json")),
		UniverseFile:              getenv("HISSEBOT_UNIVERSE_FILE", filepath.Join(seedDir, "listed_universe.json")),
		DelistedUniverseFile:      getenv("HISSEBOT_DELISTED_UNIVERSE_FILE", filepath.Join(seedDir, "delisted_universe.json")),
		AuditLogPath:              getenv("HISSEBOT_AUDIT_LOG_PATH", filepath.Join(dataDir, "audit", "events.jsonl")),
		LegacyBilancoFile:         getenv("HISSEBOT_LEGACY_BILANCO_FILE", "tumBilancolar.json"),
		EndpointURL:               os.Getenv("HISSEBOT_ENDPOINT_URL"),
		EndpointToken:             os.Getenv("HISSEBOT_ENDPOINT_TOKEN"),
		MKKCookie:                 os.Getenv("HISSEBOT_MKK_COOKIE"),
		IsYatirimCookie:           os.Getenv("HISSEBOT_ISYATIRIM_COOKIE"),
		TKGMCookie:                os.Getenv("HISSEBOT_TKGM_COOKIE"),
		TradingViewHistoryURL:     os.Getenv("HISSEBOT_TV_HISTORY_URL"),
		TradingViewChartTransport: getenv("HISSEBOT_TV_CHART_TRANSPORT", "auto"),
		HTTPTimeout:               durationEnv("HISSEBOT_HTTP_TIMEOUT", 45*time.Second),
		CommandTimeout:            durationEnv("HISSEBOT_COMMAND_TIMEOUT", 30*time.Minute),
	}
}

type SecurityIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

func ValidateSecurity(cfg Config) []SecurityIssue {
	issues := []SecurityIssue{}
	if cfg.EndpointURL != "" && cfg.EndpointToken == "" {
		issues = append(issues, SecurityIssue{
			Severity: "critical",
			Code:     "endpoint_token_missing",
			Message:  "HISSEBOT_ENDPOINT_TOKEN must be provided by environment or secret manager; no hard-coded fallback is allowed",
		})
	}
	if cfg.EndpointToken == "HISSEYORUMCOINVESTINGBOT_TOKEN_AUTH" {
		issues = append(issues, SecurityIssue{
			Severity: "critical",
			Code:     "endpoint_token_uses_default_secret",
			Message:  "default endpoint token is not acceptable for production",
		})
	}
	return issues
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	if d, err := time.ParseDuration(value); err == nil {
		return d
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}
