package news

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hissebot/internal/domain"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

type Options struct {
	Ticker  string
	Limit   int
	RSSURLs []string
}

type Result struct {
	Tickers int `json:"tickers"`
	Items   int `json:"items"`
}

type File struct {
	Source    string    `json:"source"`
	Ticker    string    `json:"ticker"`
	FetchedAt time.Time `json:"fetched_at"`
	Items     []Item    `json:"items"`
}

type Item struct {
	Ticker      string         `json:"ticker"`
	Source      string         `json:"source"`
	Provider    string         `json:"provider,omitempty"`
	URL         string         `json:"url,omitempty"`
	Title       string         `json:"title,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	PublishedAt time.Time      `json:"published_at,omitempty"`
	ReceivedAt  time.Time      `json:"received_at"`
	Reliability float64        `json:"reliability"`
	Raw         map[string]any `json:"raw,omitempty"`
}

func Sync(ctx context.Context, store *storage.EquityStore, options Options) (Result, error) {
	tickers, err := syncTickers(store, options)
	if err != nil {
		return Result{}, err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	result := Result{}
	for _, ticker := range tickers {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		equity, err := store.Load(ticker)
		if err != nil {
			return result, err
		}
		items := []Item{}
		items = append(items, kapItems(store, ticker)...)
		items = append(items, commentItems(store, ticker)...)
		for _, rssURL := range options.RSSURLs {
			rssItems, err := fetchRSSItems(ctx, client, rssURL, ticker, equity.Name)
			if err != nil {
				fmt.Printf("news: rss %s failed for %s: %v\n", rssURL, ticker, err)
				continue
			}
			items = append(items, rssItems...)
		}
		items = dedupeItems(items)
		if options.Limit > 0 && len(items) > options.Limit {
			items = items[:options.Limit]
		}
		file := File{
			Source:    "hissebot_news_sync",
			Ticker:    ticker,
			FetchedAt: time.Now().UTC(),
			Items:     items,
		}
		if err := util.WriteJSON(store.NewsSentimentPath(ticker), file); err != nil {
			return result, err
		}
		result.Tickers++
		result.Items += len(items)
	}
	return result, nil
}

func syncTickers(store *storage.EquityStore, options Options) ([]string, error) {
	if ticker := storage.NormalizeTicker(options.Ticker); ticker != "" {
		return []string{ticker}, nil
	}
	equities, err := store.List()
	if err != nil {
		return nil, err
	}
	tickers := make([]string, 0, len(equities))
	for _, equity := range equities {
		if equity.AssetType != 2 {
			continue
		}
		tickers = append(tickers, equity.Ticker)
	}
	return tickers, nil
}

func kapItems(store *storage.EquityStore, ticker string) []Item {
	raw, err := os.ReadFile(store.KAPDisclosuresPath(ticker))
	if err != nil {
		return nil
	}
	var rows []map[string]any
	if json.Unmarshal(raw, &rows) != nil {
		return nil
	}
	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		title := stringValue(row["title"])
		summary := stringValue(row["summary"])
		if title == "" && summary == "" {
			continue
		}
		items = append(items, Item{
			Ticker:      ticker,
			Source:      "kap",
			Provider:    "KAP",
			Title:       title,
			Summary:     summary,
			PublishedAt: timeValue(row["publish_date"]),
			ReceivedAt:  time.Now().UTC(),
			Reliability: 0.95,
			Raw:         row,
		})
	}
	return items
}

func commentItems(store *storage.EquityStore, ticker string) []Item {
	var comments []domain.Comment
	if err := util.ReadJSON(store.CommentsPath(ticker), &comments); err != nil {
		return nil
	}
	items := make([]Item, 0, len(comments))
	for _, comment := range comments {
		text := strings.TrimSpace(comment.Text)
		if text == "" {
			continue
		}
		items = append(items, Item{
			Ticker:      ticker,
			Source:      "comment",
			Provider:    "InvestingComments",
			Title:       firstNonEmpty(comment.Title, comment.Username),
			Summary:     text,
			PublishedAt: comment.CreatedAt,
			ReceivedAt:  time.Now().UTC(),
			Reliability: 0.35,
			Raw:         comment.Raw,
		})
	}
	return items
}

type rssDoc struct {
	Channel rssChannel  `xml:"channel"`
	Entries []atomEntry `xml:"entry"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
}

type atomEntry struct {
	Title   string   `xml:"title"`
	Summary string   `xml:"summary"`
	Content string   `xml:"content"`
	Updated string   `xml:"updated"`
	Link    atomLink `xml:"link"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
}

func fetchRSSItems(ctx context.Context, client *http.Client, url string, ticker string, companyName string) ([]Item, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "hissebot-go/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var doc rssDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	items := []Item{}
	for _, row := range doc.Channel.Items {
		title := cleanText(row.Title)
		summary := cleanText(row.Description)
		if !matchesTicker(title+" "+summary, ticker, companyName) {
			continue
		}
		items = append(items, Item{
			Ticker:      ticker,
			Source:      "rss",
			Provider:    firstNonEmpty(doc.Channel.Title, filepath.Base(url)),
			URL:         row.Link,
			Title:       title,
			Summary:     summary,
			PublishedAt: parseRSSDate(row.PubDate),
			ReceivedAt:  time.Now().UTC(),
			Reliability: 0.65,
		})
	}
	for _, row := range doc.Entries {
		title := cleanText(row.Title)
		summary := cleanText(firstNonEmpty(row.Summary, row.Content))
		if !matchesTicker(title+" "+summary, ticker, companyName) {
			continue
		}
		items = append(items, Item{
			Ticker:      ticker,
			Source:      "rss",
			Provider:    filepath.Base(url),
			URL:         row.Link.Href,
			Title:       title,
			Summary:     summary,
			PublishedAt: timeValue(row.Updated),
			ReceivedAt:  time.Now().UTC(),
			Reliability: 0.65,
		})
	}
	return items, nil
}

func dedupeItems(items []Item) []Item {
	seen := map[string]bool{}
	out := make([]Item, 0, len(items))
	for _, item := range items {
		key := strings.ToUpper(strings.TrimSpace(item.Source + "|" + item.URL + "|" + item.Title + "|" + item.Summary))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func matchesTicker(text string, ticker string, companyName string) bool {
	text = strings.ToUpper(text)
	ticker = strings.ToUpper(ticker)
	if ticker != "" && strings.Contains(text, ticker) {
		return true
	}
	for _, token := range strings.Fields(strings.ToUpper(companyName)) {
		token = strings.Trim(token, ".,;:()[]{}")
		if len([]rune(token)) >= 5 && strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func cleanText(value string) string {
	value = html.UnescapeString(value)
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\t", " ")
	return strings.Join(strings.Fields(value), " ")
}

func parseRSSDate(value string) time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC3339Nano, "Mon, 02 Jan 2006 15:04:05 -0700"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func timeValue(value any) time.Time {
	text := stringValue(value)
	if text == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z07:00", "02.01.2006 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
