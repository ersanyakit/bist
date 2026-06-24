package macro

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hissebot/pkg/mathutil"
)

const DefaultTUIKGDPFile = "data/macro/tuik_gdp.json"

type GDPDataset struct {
	Source      string             `json:"source"`
	SourceURL   string             `json:"source_url"`
	MetadataURL string             `json:"metadata_url,omitempty"`
	Methodology string             `json:"methodology"`
	FetchedAt   string             `json:"fetched_at"`
	Indicators  []GDPIndicatorMeta `json:"indicators,omitempty"`
	Points      []GDPPoint         `json:"points"`
}

type GDPIndicatorMeta struct {
	Code   string `json:"code"`
	NameTR string `json:"name_tr"`
	NameEN string `json:"name_en,omitempty"`
	Unit   string `json:"unit"`
}

type GDPPoint struct {
	Year              int     `json:"year"`
	GDPThousandTRY    float64 `json:"gdp_thousand_try"`
	PerCapitaTRY      float64 `json:"per_capita_try"`
	PerCapitaUSD      float64 `json:"per_capita_usd"`
	ImpliedPopulation float64 `json:"implied_population"`
}

type GDPContext struct {
	Computed            bool     `json:"computed"`
	Source              string   `json:"source,omitempty"`
	SourceURL           string   `json:"source_url,omitempty"`
	MetadataURL         string   `json:"metadata_url,omitempty"`
	Methodology         string   `json:"methodology,omitempty"`
	FetchedAt           string   `json:"fetched_at,omitempty"`
	ReferenceYear       int      `json:"reference_year,omitempty"`
	LatestYear          int      `json:"latest_year,omitempty"`
	PreviousYear        int      `json:"previous_year,omitempty"`
	ObservationLagYears int      `json:"observation_lag_years,omitempty"`
	FreshnessStatus     string   `json:"freshness_status,omitempty"`
	GDPThousandTRY      float64  `json:"gdp_thousand_try,omitempty"`
	GDPThousandTRYYoY   float64  `json:"gdp_thousand_try_yoy,omitempty"`
	PerCapitaTRY        float64  `json:"per_capita_try,omitempty"`
	PerCapitaTRYYoY     float64  `json:"per_capita_try_yoy,omitempty"`
	PerCapitaUSD        float64  `json:"per_capita_usd,omitempty"`
	PerCapitaUSDYoY     float64  `json:"per_capita_usd_yoy,omitempty"`
	ImpliedPopulation   float64  `json:"implied_population,omitempty"`
	Score               float64  `json:"score,omitempty"`
	Regime              string   `json:"regime,omitempty"`
	EquityImpact        string   `json:"equity_impact,omitempty"`
	Interpretation      string   `json:"interpretation,omitempty"`
	RequiredCaveats     []string `json:"required_caveats,omitempty"`
	DataQualityWarning  string   `json:"data_quality_warning,omitempty"`
}

func SaveGDPDataset(path string, dataset GDPDataset) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("gdp dataset path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create macro dir: %w", err)
	}
	raw, err := json.MarshalIndent(dataset, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal gdp dataset: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write gdp dataset: %w", err)
	}
	return nil
}

func LoadGDPDataset(path string) (GDPDataset, bool, error) {
	if strings.TrimSpace(path) == "" {
		return GDPDataset{}, false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return GDPDataset{}, false, nil
		}
		return GDPDataset{}, false, fmt.Errorf("read gdp dataset: %w", err)
	}
	var dataset GDPDataset
	if err := json.Unmarshal(raw, &dataset); err != nil {
		return GDPDataset{}, false, fmt.Errorf("parse gdp dataset: %w", err)
	}
	return dataset, true, nil
}

func AnalyzeGDP(dataset GDPDataset) GDPContext {
	points := validGDPPoints(dataset.Points)
	if len(points) < 2 {
		return GDPContext{
			Computed:           false,
			Source:             dataset.Source,
			SourceURL:          dataset.SourceURL,
			MetadataURL:        dataset.MetadataURL,
			Methodology:        dataset.Methodology,
			FetchedAt:          dataset.FetchedAt,
			DataQualityWarning: "TÜİK GSYH makro etkisi için en az iki yıllık veri gerekli.",
			RequiredCaveats:    []string{"GSYH etkisi veri eksikliği nedeniyle skorlanmadı."},
		}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Year > points[j].Year })
	latest := points[0]
	previous := points[1]
	gdpYoY := pctChange(latest.GDPThousandTRY, previous.GDPThousandTRY)
	perCapTRYYoY := pctChange(latest.PerCapitaTRY, previous.PerCapitaTRY)
	perCapUSDYoY := pctChange(latest.PerCapitaUSD, previous.PerCapitaUSD)
	score := gdpMacroScore(gdpYoY, perCapTRYYoY, perCapUSDYoY)
	referenceYear := latest.Year + 1
	if fetchedAt, err := time.Parse(time.RFC3339, dataset.FetchedAt); err == nil && fetchedAt.Year() > referenceYear {
		referenceYear = fetchedAt.Year()
	}
	observationLagYears := referenceYear - latest.Year
	freshnessStatus := "current_annual_actual"
	dataQualityWarning := ""
	if observationLagYears >= 2 {
		freshnessStatus = "stale_annual_actual"
		dataQualityWarning = fmt.Sprintf("GSYH dosyası %d yılında çekilmiş olsa da son gerçekleşen gözlem %d; veri dönemi ile indirme tarihi aynı şey değildir.", referenceYear, latest.Year)
		score = math.Min(score, 59)
	}
	regime := "nötr / makro destek sınırlı"
	switch {
	case score >= 70:
		regime = "büyüme ve dolar bazlı gelir destekleyici"
	case score <= 40:
		regime = "makro gelir/demand teyidi zayıf"
	}
	impact := "GSYH ve kişi başına gelir artışı şirket gelirleri için talep tabanını destekler; bu tek başına alım sinyali değildir, bilanço kalitesi, sektör duyarlılığı ve fiyat/teyit ile birlikte okunur."
	if perCapUSDYoY < 0 {
		impact = "TL bazlı büyüme olsa bile kişi başına dolar GSYH zayıflıyor; kur ve dış alım gücü baskısı nedeniyle makro etki temkinli okunur."
	} else if score >= 70 {
		impact = "Kişi başına dolar GSYH ve toplam GSYH aynı yönde güçleniyor; iç talep, ciro büyümesi ve yabancı yatırımcı algısı için destekleyici makro zemin var."
	}
	interpretation := fmt.Sprintf(
		"%d verisinde kişi başına GSYH %.0f $ (yıllık %.1f%%), %.0f TL (yıllık %.1f%%), toplam GSYH %.0f bin TL (yıllık %.1f%%).",
		latest.Year,
		latest.PerCapitaUSD,
		perCapUSDYoY,
		latest.PerCapitaTRY,
		perCapTRYYoY,
		latest.GDPThousandTRY,
		gdpYoY,
	)
	return GDPContext{
		Computed:            true,
		Source:              empty(dataset.Source, "TÜİK CİP"),
		SourceURL:           dataset.SourceURL,
		MetadataURL:         dataset.MetadataURL,
		Methodology:         dataset.Methodology,
		FetchedAt:           dataset.FetchedAt,
		ReferenceYear:       referenceYear,
		LatestYear:          latest.Year,
		PreviousYear:        previous.Year,
		ObservationLagYears: observationLagYears,
		FreshnessStatus:     freshnessStatus,
		GDPThousandTRY:      latest.GDPThousandTRY,
		GDPThousandTRYYoY:   gdpYoY,
		PerCapitaTRY:        latest.PerCapitaTRY,
		PerCapitaTRYYoY:     perCapTRYYoY,
		PerCapitaUSD:        latest.PerCapitaUSD,
		PerCapitaUSDYoY:     perCapUSDYoY,
		ImpliedPopulation:   latest.ImpliedPopulation,
		Score:               score,
		Regime:              regime,
		EquityImpact:        impact,
		Interpretation:      interpretation,
		RequiredCaveats: []string{
			"CİP verisi il düzeyinden Türkiye toplamına agregedir.",
			"TL bazlı GSYH nominaldir; enflasyon/kur ayrıştırması ayrıca yapılmalıdır.",
			"GSYH makro rüzgarı gösterir, tek başına hisse al/sat kararı değildir.",
		},
		DataQualityWarning: dataQualityWarning,
	}
}

func DefaultGDPPathFromEquitiesDir(equitiesDir string) string {
	if strings.TrimSpace(equitiesDir) == "" {
		return DefaultTUIKGDPFile
	}
	return filepath.Join(filepath.Dir(equitiesDir), "macro", "tuik_gdp.json")
}

func NewDatasetFetchedNow(points []GDPPoint, indicators []GDPIndicatorMeta, sourceURL, metadataURL string) GDPDataset {
	sort.Slice(points, func(i, j int) bool { return points[i].Year > points[j].Year })
	return GDPDataset{
		Source:      "TÜİK Coğrafi İstatistik Portalı",
		SourceURL:   sourceURL,
		MetadataURL: metadataURL,
		Methodology: "CİP il göstergeleri duzey=3 verisi kullanıldı; GSYH (bin TL) illerden toplandı, kişi başına GSYH TL ve $ değerleri il GSYH/per-capita ile türetilen nüfus üzerinden ağırlıklandırıldı.",
		FetchedAt:   time.Now().UTC().Format(time.RFC3339),
		Indicators:  indicators,
		Points:      points,
	}
}

func validGDPPoints(points []GDPPoint) []GDPPoint {
	out := make([]GDPPoint, 0, len(points))
	for _, point := range points {
		if point.Year > 0 && point.GDPThousandTRY > 0 && point.PerCapitaTRY > 0 && point.PerCapitaUSD > 0 {
			out = append(out, point)
		}
	}
	return out
}

func pctChange(current, previous float64) float64 {
	if previous == 0 || math.IsNaN(current) || math.IsNaN(previous) {
		return 0
	}
	return (current - previous) / previous * 100
}

func gdpMacroScore(gdpYoY, perCapTRYYoY, perCapUSDYoY float64) float64 {
	nominalPulse := (gdpYoY + perCapTRYYoY) / 2
	score := 50.0
	score += mathutil.Clamp(perCapUSDYoY*1.4, -24, 24)
	score += mathutil.Clamp((nominalPulse-25)*0.35, -10, 12)
	if perCapUSDYoY < 0 {
		score -= mathutil.Clamp(math.Abs(perCapUSDYoY)*0.8, 0, 12)
	}
	return mathutil.Clamp(score, 0, 100)
}

func empty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
