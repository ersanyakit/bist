package mkk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

type publicResponse struct {
	Data []publicCompany `json:"data"`
}

type publicCompany struct {
	TradeName string `json:"tradeName"`
	CompanyID any    `json:"companyId"`
}

type companyResponse struct {
	Data map[string]any `json:"data"`
}

func Sync(ctx context.Context, cfg config.Config, store *storage.EquityStore) error {
	overrides, err := loadOverrides(cfg.MKKNameOverridesFile)
	if err != nil {
		return err
	}

	companies, err := fetchPublicCompanies(ctx, cfg)
	if err != nil {
		return err
	}

	equities, err := store.List()
	if err != nil {
		return err
	}

	bySlug := map[string][]string{}
	for _, equity := range equities {
		name := equity.Name
		if override := overrides[equity.Ticker]; override != "" {
			name = override
			if err := store.Update(equity.Ticker, func(e *domain.Equity) error {
				e.Name = override
				if e.MKKID == 0 {
					e.MKKID = -1
				}
				return nil
			}); err != nil {
				return err
			}
			if err := util.WriteJSON(store.MKKPath(equity.Ticker), map[string]any{
				"source":     "mkk",
				"ticker":     equity.Ticker,
				"name":       override,
				"mkk_id":     -1,
				"matched":    false,
				"override":   true,
				"updated_at": time.Now().UTC(),
			}); err != nil {
				return err
			}
		}
		if name == "" && equity.KAPInfo != nil {
			name, _ = equity.KAPInfo["kapMemberTitle"].(string)
		}
		if name == "" {
			continue
		}
		bySlug[util.SlugTR(name)] = append(bySlug[util.SlugTR(name)], equity.Ticker)
	}

	matched := 0
	for _, company := range companies {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		id := companyID(company.CompanyID)
		if id == 0 || company.TradeName == "" {
			continue
		}
		for _, ticker := range bySlug[util.SlugTR(company.TradeName)] {
			if err := store.Update(ticker, func(e *domain.Equity) error {
				e.AssetType = 2
				e.Name = company.TradeName
				e.MKKID = id
				return nil
			}); err != nil {
				return err
			}
			if err := util.WriteJSON(store.MKKPath(ticker), map[string]any{
				"source":     "mkk",
				"ticker":     ticker,
				"name":       company.TradeName,
				"mkk_id":     id,
				"matched":    true,
				"raw":        company,
				"updated_at": time.Now().UTC(),
			}); err != nil {
				return err
			}
			matched++
		}
	}

	infoUpdated := 0
	equities, err = store.List()
	if err != nil {
		return err
	}
	for _, equity := range equities {
		if equity.MKKID <= 0 {
			continue
		}
		info, err := fetchCompanyInfo(ctx, cfg, equity.MKKID)
		if err != nil {
			fmt.Printf("mkk: %s company info skipped: %v\n", equity.Ticker, err)
			continue
		}
		delete(info, "logo")
		if err := util.WriteJSON(store.MKKCompanyInfoPath(equity.Ticker), info); err != nil {
			return err
		}
		if err := store.Update(equity.Ticker, func(e *domain.Equity) error {
			e.CompanyInfo = info
			return nil
		}); err != nil {
			return err
		}
		infoUpdated++
	}

	fmt.Printf("mkk: %d mkk ids matched, %d company info files updated\n", matched, infoUpdated)
	return nil
}

func loadOverrides(path string) (map[string]string, error) {
	overrides := map[string]string{}
	if err := util.ReadJSON(path, &overrides); err != nil {
		return nil, err
	}
	normalized := map[string]string{}
	for ticker, name := range overrides {
		normalized[storage.NormalizeTicker(ticker)] = name
	}
	return normalized, nil
}

func fetchPublicCompanies(ctx context.Context, cfg config.Config) ([]publicCompany, error) {
	var response publicResponse
	if err := getJSON(ctx, cfg, "https://e-sirket.mkk.com.tr/api/companies/public", &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func fetchCompanyInfo(ctx context.Context, cfg config.Config, companyID int64) (map[string]any, error) {
	var response companyResponse
	url := fmt.Sprintf("https://e-sirket.mkk.com.tr/api/companies/company-by-company-id/%d", companyID)
	if err := getJSON(ctx, cfg, url, &response); err != nil {
		return nil, err
	}
	if response.Data == nil {
		return map[string]any{}, nil
	}
	return response.Data, nil
}

func getJSON(ctx context.Context, cfg config.Config, url string, target any) error {
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Authorization", "Bearer")
	req.Header.Set("Company", "")
	req.Header.Set("Referer", "https://e-sirket.mkk.com.tr/")
	req.Header.Set("User-Agent", "hissebot-go/1.0")
	if cfg.MKKCookie != "" {
		req.Header.Set("Cookie", cfg.MKKCookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func companyID(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case json.Number:
		id, _ := v.Int64()
		return id
	case string:
		id, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return id
	default:
		return 0
	}
}
