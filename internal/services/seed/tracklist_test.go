package seed

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"hissebot/internal/config"
	"hissebot/internal/util"
)

func TestImportTrackIDsCopiesInputToConfiguredOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	output := filepath.Join(dir, "cache", "investing_track_ids.json")
	want := []int64{101, 202, 303}
	if err := util.WriteJSON(input, want); err != nil {
		t.Fatalf("write input: %v", err)
	}

	err := ImportTrackIDs(context.Background(), config.Config{InvestingTrackIDsFile: output}, input)
	if err != nil {
		t.Fatalf("ImportTrackIDs() error = %v", err)
	}

	var got []int64
	if err := util.ReadJSON(output, &got); err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("written ids = %#v, want %#v", got, want)
	}
}

func TestImportTrackIDsHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ImportTrackIDs(ctx, config.Config{}, "unused.json")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ImportTrackIDs() error = %v, want context.Canceled", err)
	}
}
