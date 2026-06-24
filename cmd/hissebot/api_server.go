package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	appconfig "hissebot/internal/config"
	"hissebot/internal/services/classification"
	"hissebot/internal/services/kapsectors"
	corestorage "hissebot/internal/storage"
)

type apiServerOptions struct {
	Addr string
}

type sectorSymbolResponse struct {
	Symbol         string                    `json:"symbol"`
	KAPSector      *kapsectors.CompanySector `json:"kap_sector,omitempty"`
	Classification *classification.Entry     `json:"classification,omitempty"`
}

type sectorGroup struct {
	MainSector string   `json:"main_sector"`
	Sector     string   `json:"sector"`
	Symbols    []string `json:"symbols"`
	Count      int      `json:"count"`
}

func runAPIServer(ctx context.Context, cfg appconfig.Config, store *corestorage.EquityStore, args []string) error {
	opts := apiServerOptions{Addr: "127.0.0.1:1454"}
	fs := flag.NewFlagSet("serve api", flag.ExitOnError)
	fs.StringVar(&opts.Addr, "addr", opts.Addr, "Fiber v3 API dinleme adresi")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, _, err := net.SplitHostPort(opts.Addr); err != nil {
		return fmt.Errorf("addr geçersiz: %w", err)
	}
	server := &apiServer{cfg: cfg, store: store}
	app := fiber.New(fiber.Config{
		AppName:        "hissebot-api",
		ReadBufferSize: 256 * 1024,
	})
	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.JSON(map[string]string{"status": "ok"})
	})
	api := app.Group("/api/v1")
	api.Get("/sectors", server.handleKAPSectors)
	api.Get("/sectors/:symbol", server.handleSectorSymbol)
	api.Get("/sector-groups", server.handleSectorGroups)
	api.Get("/sector-classifications", server.handleSectorClassifications)
	api.Get("/sector-classifications/:symbol", server.handleSectorClassificationSymbol)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = app.ShutdownWithContext(shutdownCtx)
	}()
	return app.Listen(opts.Addr)
}

type apiServer struct {
	cfg          appconfig.Config
	store        *corestorage.EquityStore
	kapSectorsMu sync.Mutex
}

func (s *apiServer) handleKAPSectors(c fiber.Ctx) error {
	file, err := s.loadKAPSectors()
	if err != nil {
		return fiberError(c, fiber.StatusNotFound, "kap_sectors_not_found", err.Error())
	}
	if !apiBool(c.Query("include_entries", "true")) {
		file.Entries = nil
	}
	return c.JSON(file)
}

func (s *apiServer) handleSectorSymbol(c fiber.Ctx) error {
	symbol := corestorage.NormalizeTicker(c.Params("symbol"))
	if symbol == "" {
		return fiberError(c, fiber.StatusBadRequest, "bad_request", "symbol zorunlu")
	}
	response := sectorSymbolResponse{Symbol: symbol}
	if file, err := s.loadKAPSectors(); err == nil {
		if entry, ok := file.Entries[symbol]; ok {
			response.KAPSector = &entry
		}
	}
	if file, err := s.loadClassifications(); err == nil {
		if entry, ok := file.Entries[symbol]; ok {
			response.Classification = &entry
		}
	}
	if response.KAPSector == nil && response.Classification == nil {
		return fiberError(c, fiber.StatusNotFound, "sector_not_found", "symbol için sektör kaydı bulunamadı")
	}
	return c.JSON(response)
}

func (s *apiServer) handleSectorGroups(c fiber.Ctx) error {
	file, err := s.loadKAPSectors()
	if err != nil {
		return fiberError(c, fiber.StatusNotFound, "kap_sectors_not_found", err.Error())
	}
	groups := map[string]*sectorGroup{}
	for symbol, entry := range file.Entries {
		key := entry.MainSector + "\x00" + entry.Sector
		group := groups[key]
		if group == nil {
			group = &sectorGroup{MainSector: entry.MainSector, Sector: entry.Sector}
			groups[key] = group
		}
		group.Symbols = append(group.Symbols, symbol)
	}
	out := make([]sectorGroup, 0, len(groups))
	for _, group := range groups {
		sort.Strings(group.Symbols)
		group.Count = len(group.Symbols)
		out = append(out, *group)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MainSector == out[j].MainSector {
			return out[i].Sector < out[j].Sector
		}
		return out[i].MainSector < out[j].MainSector
	})
	return c.JSON(map[string]any{
		"source":       file.Source,
		"source_url":   file.SourceURL,
		"generated_at": file.GeneratedAt,
		"groups":       out,
	})
}

func (s *apiServer) handleSectorClassifications(c fiber.Ctx) error {
	file, err := s.loadClassifications()
	if err != nil {
		return fiberError(c, fiber.StatusNotFound, "sector_classifications_not_found", err.Error())
	}
	if !apiBool(c.Query("include_entries", "true")) {
		file.Entries = nil
	}
	return c.JSON(file)
}

func (s *apiServer) handleSectorClassificationSymbol(c fiber.Ctx) error {
	symbol := corestorage.NormalizeTicker(c.Params("symbol"))
	if symbol == "" {
		return fiberError(c, fiber.StatusBadRequest, "bad_request", "symbol zorunlu")
	}
	file, err := s.loadClassifications()
	if err != nil {
		return fiberError(c, fiber.StatusNotFound, "sector_classifications_not_found", err.Error())
	}
	entry, ok := file.Entries[symbol]
	if !ok {
		return fiberError(c, fiber.StatusNotFound, "sector_classification_not_found", "symbol için sınıflandırma kaydı bulunamadı")
	}
	return c.JSON(map[string]any{
		"symbol": symbol,
		"entry":  entry,
	})
}

func (s *apiServer) loadKAPSectors() (kapsectors.File, error) {
	file, err := kapsectors.Load(s.cfg.KAPSectorsFile)
	if err == nil {
		return file, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return kapsectors.File{}, err
	}

	s.kapSectorsMu.Lock()
	defer s.kapSectorsMu.Unlock()

	file, err = kapsectors.Load(s.cfg.KAPSectorsFile)
	if err == nil {
		return file, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return kapsectors.File{}, err
	}

	timeout := s.cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	result, err := kapsectors.Sync(ctx, s.cfg, kapsectors.Options{Timeout: timeout})
	if err != nil {
		return kapsectors.File{}, fmt.Errorf("%s bulunamadı; KAP sync denendi ama başarısız: %w", s.cfg.KAPSectorsFile, err)
	}
	file, err = kapsectors.Load(result.OutputPath)
	if err != nil {
		return kapsectors.File{}, fmt.Errorf("%s KAP sync sonrası okunamadı: %w", result.OutputPath, err)
	}
	return file, nil
}

func (s *apiServer) loadClassifications() (classification.File, error) {
	path := s.cfg.SectorClassificationsFile
	raw, err := os.ReadFile(path)
	if err != nil {
		return classification.File{}, fmt.Errorf("%s okunamadı; önce sync sectors -refresh-kap çalıştırın: %w", path, err)
	}
	var file classification.File
	if err := json.Unmarshal(raw, &file); err != nil {
		return classification.File{}, err
	}
	if file.Entries == nil {
		file.Entries = map[string]classification.Entry{}
	}
	return file, nil
}

func fiberError(c fiber.Ctx, status int, code string, message string) error {
	return c.Status(status).JSON(map[string]string{
		"status":  "error",
		"code":    code,
		"message": message,
	})
}

func apiBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "1", "true", "yes", "evet":
		return true
	default:
		return false
	}
}
