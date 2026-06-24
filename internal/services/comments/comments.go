package comments

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/storage"
	"hissebot/internal/util"
	"hissebot/internal/wsclient"
)

const investingWebSocketURL = "wss://streaming.forexpros.com/echo/458/e5wec40v/websocket"

var htmlTags = regexp.MustCompile(`</?[^>]+>`)

type incomingEnvelope struct {
	Message string `json:"message"`
}

func Serve(ctx context.Context, cfg config.Config, store *storage.EquityStore) error {
	trackIDs, err := loadTrackIDs(cfg)
	if err != nil {
		return err
	}
	subscribePayload := buildSubscribePayload(trackIDs)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := wsclient.Dial(ctx, investingWebSocketURL)
		if err != nil {
			fmt.Printf("comments: websocket dial failed: %v\n", err)
			sleepOrDone(ctx, 3*time.Second)
			continue
		}
		fmt.Println("comments: websocket connected")
		if err := readLoop(ctx, store, conn, subscribePayload); err != nil {
			fmt.Printf("comments: websocket closed: %v\n", err)
		}
		_ = conn.Close()
		sleepOrDone(ctx, 3*time.Second)
	}
}

func readLoop(ctx context.Context, store *storage.EquityStore, conn *wsclient.Conn, subscribePayload string) error {
	lastHeartbeat := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		raw, err := conn.ReadText(ctx)
		if err != nil {
			return err
		}
		if raw == "o" {
			if err := conn.WriteText(subscribePayload); err != nil {
				return err
			}
			if err := conn.WriteText(`{"_event":"UID","UID":0}`); err != nil {
				return err
			}
			continue
		}

		if time.Since(lastHeartbeat) >= 10*time.Second {
			lastHeartbeat = time.Now()
			_ = conn.WriteText(`["{\"_event\":\"heartbeat\",\"data\":\"h\"}"]`)
		}

		comment, ok := decodeComment(raw)
		if !ok {
			continue
		}
		if err := saveComment(store, comment); err != nil {
			fmt.Printf("comments: save failed: %v\n", err)
		}
	}
}

func decodeComment(raw string) (domain.Comment, bool) {
	if raw == "" {
		return domain.Comment{}, false
	}
	if raw[0] != '[' {
		raw = raw[1:]
	}

	var outer any
	if err := json.Unmarshal([]byte(raw), &outer); err != nil {
		return domain.Comment{}, false
	}

	var inner string
	switch value := outer.(type) {
	case string:
		inner = value
	case []any:
		if len(value) == 0 {
			return domain.Comment{}, false
		}
		inner, _ = value[0].(string)
	default:
		return domain.Comment{}, false
	}
	if inner == "" {
		return domain.Comment{}, false
	}

	var envelope incomingEnvelope
	if err := json.Unmarshal([]byte(inner), &envelope); err != nil {
		return domain.Comment{}, false
	}
	parts := strings.SplitN(envelope.Message, "::", 2)
	if len(parts) != 2 {
		return domain.Comment{}, false
	}

	var rawComment map[string]any
	if err := json.Unmarshal([]byte(parts[1]), &rawComment); err != nil {
		return domain.Comment{}, false
	}

	comment := domain.Comment{
		Target:    stringValue(rawComment["target"]),
		Title:     stringValue(rawComment["title"]),
		Tooltip:   stringValue(rawComment["tooltip"]),
		Text:      cleanText(stringValue(rawComment["text"])),
		Username:  stringValue(rawComment["username"]),
		CreatedAt: time.Now().UTC(),
		Raw:       rawComment,
	}
	if user, ok := rawComment["user"].(map[string]any); ok {
		comment.User = user
	}
	return comment, comment.Target != "" || comment.Text != ""
}

func saveComment(store *storage.EquityStore, comment domain.Comment) error {
	if ticker, ok, err := findByPair(store, comment.Target); err != nil {
		return err
	} else if ok {
		if err := appendCommentFile(store.CommentsPath(ticker), comment); err != nil {
			return err
		}
		return store.Update(ticker, func(e *domain.Equity) error {
			e.Comments = append(e.Comments, comment)
			return nil
		})
	}
	return appendTargetComment(store, comment)
}

func findByPair(store *storage.EquityStore, pair string) (string, bool, error) {
	if pair == "" {
		return "", false, nil
	}
	equities, err := store.List()
	if err != nil {
		return "", false, err
	}
	for _, equity := range equities {
		if equity.Pair == pair {
			return equity.Ticker, true, nil
		}
	}
	return "", false, nil
}

func appendTargetComment(store *storage.EquityStore, comment domain.Comment) error {
	return appendCommentFile(store.UnmatchedCommentsPath(comment.Target), comment)
}

func appendCommentFile(path string, comment domain.Comment) error {
	var comments []domain.Comment
	if err := util.ReadJSON(path, &comments); err != nil && !os.IsNotExist(err) {
		return err
	}
	comments = append(comments, comment)
	return util.WriteJSON(path, comments)
}

func loadTrackIDs(cfg config.Config) ([]int64, error) {
	var ids []int64
	if err := util.ReadJSON(cfg.InvestingTrackIDsFile, &ids); err == nil {
		return ids, nil
	}
	cachePath := filepath.Join(cfg.CacheDir, "investing_track_ids.json")
	if err := util.ReadJSON(cachePath, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func buildSubscribePayload(ids []int64) string {
	parts := make([]string, 0, len(ids)+1)
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("cmt-10-5-%d", id))
	}
	parts = append(parts, "domain-10")
	message := strings.Join(parts, ":%%") + ":"
	obj := fmt.Sprintf(`{"_event":"bulk-subscribe","tzID":63,"message":%q}`, message)
	encoded, _ := json.Marshal([]string{obj})
	return string(encoded)
}

func cleanText(input string) string {
	input = htmlTags.ReplaceAllString(input, "")
	return strings.TrimSpace(html.UnescapeString(input))
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func sleepOrDone(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
