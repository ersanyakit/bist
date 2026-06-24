package seed

import (
	"context"
	"fmt"
	"path/filepath"

	"hissebot/internal/config"
	"hissebot/internal/util"
)

func ImportTrackIDs(ctx context.Context, cfg config.Config, file string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	var ids []int64
	if err := util.ReadJSON(file, &ids); err != nil {
		return err
	}
	out := cfg.InvestingTrackIDsFile
	if filepath.Clean(file) != filepath.Clean(out) {
		if err := util.WriteJSON(out, ids); err != nil {
			return err
		}
	}
	fmt.Printf("tracklist: %d investing ids written to %s\n", len(ids), out)
	return nil
}
