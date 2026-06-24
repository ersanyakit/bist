package kap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

const defaultCompanyItemsURL = "https://www.kap.org.tr/tr/api/company/items/%s/%s"

type CompanySyncOptions struct {
	URL        string
	MemberType string
	State      string
	OutputFile string
}

func SyncCompanies(ctx context.Context, cfg config.Config, store *storage.EquityStore, opts CompanySyncOptions) error {
	companies, err := FetchCompanies(ctx, cfg, opts)
	if err != nil {
		return err
	}
	if opts.OutputFile != "" {
		if err := util.WriteJSON(opts.OutputFile, companies); err != nil {
			return err
		}
	}
	return importCompanyList(ctx, store, companies, firstNonEmpty(opts.OutputFile, "kap_live_company_items"))
}

func FetchCompanies(ctx context.Context, cfg config.Config, opts CompanySyncOptions) ([]map[string]any, error) {
	memberType := firstNonEmpty(opts.MemberType, "IGS")
	state := firstNonEmpty(opts.State, "A")
	url := firstNonEmpty(opts.URL, fmt.Sprintf(defaultCompanyItemsURL, memberType, state))
	client := &http.Client{Timeout: firstDuration(cfg.HTTPTimeout, 45*time.Second)}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	setKAPRequestHeaders(req)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kap companies %s/%s: unexpected HTTP status %s: %s", memberType, state, resp.Status, strings.TrimSpace(string(raw)))
	}
	var companies []map[string]any
	if err := json.Unmarshal(raw, &companies); err != nil {
		return nil, err
	}
	return companies, nil
}

func ImportCompanies(ctx context.Context, store *storage.EquityStore, file string) error {
	var companies []map[string]any
	if err := util.ReadJSON(file, &companies); err != nil {
		return err
	}
	return importCompanyList(ctx, store, companies, file)
}

func importCompanyList(ctx context.Context, store *storage.EquityStore, companies []map[string]any, source string) error {
	updated := 0
	for _, company := range companies {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		stockCode, _ := company["stockCode"].(string)
		if stockCode == "" {
			continue
		}
		title, _ := company["kapMemberTitle"].(string)
		for _, ticker := range splitStockCodes(stockCode) {
			if ticker == "" {
				continue
			}
			err := store.Update(ticker, func(e *domain.Equity) error {
				e.AssetType = 2
				if e.Name == "" && title != "" {
					e.Name = title
				}
				e.KAPInfo = company
				return nil
			})
			if err != nil {
				return err
			}
			if err := util.WriteJSON(store.KAPPath(ticker), company); err != nil {
				return err
			}
			updated++
		}
	}

	fmt.Printf("kap: %d ticker updated from %s\n", updated, source)
	return nil
}

func splitStockCodes(input string) []string {
	input = strings.ReplaceAll(input, " ", "")
	parts := strings.Split(input, ",")
	for i := range parts {
		parts[i] = storage.NormalizeTicker(parts[i])
	}
	return parts
}

func firstDuration(values ...time.Duration) time.Duration {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
