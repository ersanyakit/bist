package financials

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"hissebot/internal/domain"
	"hissebot/internal/services/kap"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

type legacyBilancoItem struct {
	Ticker string                    `json:"ticker"`
	Data   map[string]map[string]any `json:"data"`
}

func ImportLegacyBilanco(ctx context.Context, store *storage.EquityStore, file string) error {
	bytes, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	var raw []map[string]legacyBilancoItem
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return err
	}

	imported := 0
	for _, wrapper := range raw {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		for ticker, item := range wrapper {
			if item.Ticker != "" {
				ticker = item.Ticker
			}
			info := &domain.BilancoInfo{
				Ticker: storage.NormalizeTicker(ticker),
				Data:   map[string]domain.BilancoField{},
			}
			for code, rawField := range item.Data {
				field := domain.BilancoField{Years: map[string][]*float64{}}
				if desc, ok := rawField["descTR"].(string); ok {
					field.DescTR = desc
				}
				if desc, ok := rawField["descEN"].(string); ok {
					field.DescEN = desc
				}
				for key, value := range rawField {
					if key == "descTR" || key == "descEN" {
						continue
					}
					if values := numberSlice(value); len(values) > 0 {
						field.Years[key] = values
					}
				}
				if len(field.Years) > 0 {
					info.Data[code] = field
				}
			}
			domain.ApplyBilancoSourceMetadata(info, "legacy_import", "", "", time.Now().UTC())
			domain.AppendLineage(info, domain.DataLineageEvent{
				Stage:     "legacy_financial_import",
				Source:    "legacy_json",
				Transform: "import_legacy_bilanco",
				Version:   domain.FinancialMetadataVersion,
				CreatedAt: time.Now().UTC(),
				Notes:     []string{"publish_date_source=unavailable"},
			})
			if _, err := kap.ApplyFinancialPublishDatesFromStore(store, ticker, info); err != nil {
				return err
			}
			if err := store.Update(ticker, func(e *domain.Equity) error {
				e.AssetType = 2
				e.BilancoInfo = info
				return nil
			}); err != nil {
				return err
			}
			if err := util.WriteJSON(store.FinancialInfoPath(ticker), info); err != nil {
				return err
			}
			versionStore, err := upsertStatementVersionStore(store, ticker, info)
			if err != nil {
				return err
			}
			if len(versionStore.Versions) > 0 {
				if err := util.WriteJSON(store.FinancialStatementVersionsPath(ticker), versionStore); err != nil {
					return err
				}
			}
			imported++
		}
	}

	fmt.Printf("financials: %d legacy bilanco entries imported into equity json files\n", imported)
	return nil
}

func numberSlice(value any) []*float64 {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	values := make([]*float64, 0, len(raw))
	for _, item := range raw {
		switch v := item.(type) {
		case float64:
			if math.IsNaN(v) || math.IsInf(v, 0) {
				values = append(values, nil)
				continue
			}
			vv := v
			values = append(values, &vv)
		case nil:
			values = append(values, nil)
		default:
			values = append(values, nil)
		}
	}
	return values
}
