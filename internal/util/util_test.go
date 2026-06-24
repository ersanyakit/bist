package util

import (
	"path/filepath"
	"reflect"
	"testing"
)

type jsonRoundTripPayload struct {
	Symbol string   `json:"symbol"`
	Values []int    `json:"values"`
	Flags  []string `json:"flags"`
}

func TestSlugTRNormalizesTurkishCharacters(t *testing.T) {
	got := SlugTR("İş GYO Çelik A.Ş. 123!")
	want := "isgyocelikas123"
	if got != want {
		t.Fatalf("SlugTR() = %q, want %q", got, want)
	}
}

func TestWriteReadJSONRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "payload.json")
	want := jsonRoundTripPayload{
		Symbol: "EUPWR",
		Values: []int{1, 2, 3},
		Flags:  []string{"computed", "guarded"},
	}

	if err := WriteJSON(path, want); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	var got jsonRoundTripPayload
	if err := ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON() error = %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadJSON() = %#v, want %#v", got, want)
	}
}
