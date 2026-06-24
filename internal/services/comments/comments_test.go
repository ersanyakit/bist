package comments

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeCommentParsesSockJSPayloadAndCleansHTML(t *testing.T) {
	rawComment := `{"target":"BIST:EUPWR","title":"Baslik","tooltip":"ipucu","text":"<b>Al &amp; sat</b>","username":"ersan","user":{"id":7}}`
	envelope, err := json.Marshal(incomingEnvelope{Message: "comment::" + rawComment})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	outer, err := json.Marshal([]string{string(envelope)})
	if err != nil {
		t.Fatalf("marshal outer: %v", err)
	}

	comment, ok := decodeComment(string(outer))
	if !ok {
		t.Fatalf("decodeComment() ok = false")
	}

	if comment.Target != "BIST:EUPWR" || comment.Username != "ersan" {
		t.Fatalf("decodeComment() identity = %#v", comment)
	}
	if comment.Text != "Al & sat" {
		t.Fatalf("decodeComment() text = %q, want cleaned text", comment.Text)
	}
	if comment.User["id"] != float64(7) {
		t.Fatalf("decodeComment() user = %#v, want id 7", comment.User)
	}
}

func TestDecodeCommentRejectsMalformedPayloads(t *testing.T) {
	for _, raw := range []string{"", "not-json", `["{}"]`, `["{\"message\":\"missing separator\"}"]`} {
		t.Run(raw, func(t *testing.T) {
			if _, ok := decodeComment(raw); ok {
				t.Fatalf("decodeComment(%q) ok = true, want false", raw)
			}
		})
	}
}

func TestBuildSubscribePayloadIncludesTrackIDsAndDomainChannel(t *testing.T) {
	payload := buildSubscribePayload([]int64{11, 22})

	var wrapped []string
	if err := json.Unmarshal([]byte(payload), &wrapped); err != nil {
		t.Fatalf("subscribe payload is not JSON array: %v", err)
	}
	if len(wrapped) != 1 {
		t.Fatalf("subscribe payload array length = %d, want 1", len(wrapped))
	}
	if !strings.Contains(wrapped[0], "cmt-10-5-11:%%cmt-10-5-22:%%domain-10:") {
		t.Fatalf("subscribe payload = %q, missing channels", wrapped[0])
	}
	if !strings.Contains(wrapped[0], `"bulk-subscribe"`) {
		t.Fatalf("subscribe payload = %q, missing event", wrapped[0])
	}
}
