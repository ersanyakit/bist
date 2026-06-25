# hissebot — Geliştirme Rehberi

> Otomatik üretildi: BMAD Document Project (kapsamlı tarama) — 2026-06-25

## Ön Koşullar

- **Go 1.26** (modül: `hissebot`)
- C derleyici (cgo) — `mattn/go-sqlite3` derlemesi için gerekli (`gcc`/`clang`)
- Git

## Kurulum

```bash
git clone <repo>
cd bist
go mod download
go build ./...      # tüm paketleri derle (cgo sqlite uyarısı zararsız)
```

## Yapılandırma (ortam değişkenleri)

Tüm konfig `internal/config/config.go` → `Load()` ile env'den okunur. Öne çıkanlar:

| Env | Varsayılan | Amaç |
|---|---|---|
| `HISSEBOT_DATA_DIR` | `data` | Tüm veri kökü |
| `HISSEBOT_KAP_DISCLOSURES_FILE` | `data/seed/kap_disclosures.json` | KAP bildirim dosyası |
| `HISSEBOT_KAP_DISCLOSURES_URL` / `_TOKEN` | — | KAP bildirim API |
| `HISSEBOT_KAP_SECTORS_URL` | — | Sektör verisi |
| Market MQTT/WS (`MarketMQTT*`, `MarketWSURL`) | — | Canlı market verisi |
| Çerezler (`MKKCookie`, `IsYatirimCookie`, `TKGMCookie`) | — | Korunan kaynaklara erişim |
| `HTTPTimeout`, `CommandTimeout` | — | Zaman aşımları |

> 51 alanlı düz struct; gizli bilgiler (token/çerez/MQTT şifresi) dizin yollarıyla aynı struct'ta. Üretimde bunları gerçek secret yönetiminden geçirin.

## Çalıştırma

```bash
# Tipik tam akış (tek hisse)
go run ./cmd/hissebot sync all-data
go run ./cmd/hissebot financials run -force
go run ./cmd/hissebot analyze -symbol ASELS

# Sunucu
go run ./cmd/hissebot serve api

# Yardım
go run ./cmd/hissebot help
```

Tüm komutlar için bkz. [api-contracts.md](./api-contracts.md).

## Test

```bash
go test ./...                 # tüm testler (128 test dosyası)
go test -race ./internal/...  # eşzamanlılık doğrulama (önerilir)
go test ./internal/quant/...  # iyi kaplı paket
```

Test deseni: Go standart `*_test.go`, tablo-tabanlı testler. **Açık:** `internal/analysis/*/scoring.go` ve `internal/confidence/score.go` testsiz — yeni test yazarken öncelik bunlar.

## Codegen Araçları

İndikatör/patern katalogları üretilmiştir (`internal/ta/{indicators,patterns}/generated/`). Yeniden üretmek için:

```bash
go run ./tools/indicator_catalog_gen
go run ./tools/pattern_catalog_gen
go run ./tools/pattern_audit_report
go run ./tools/analysis_report_audit -path data/equities/ASELS/analysis/<tarih>/analysis.json -spot-only=true
```

## Proje Konvansiyonları

- **Katman disiplini:** `internal/domain` saf tutulur (altyapı import etmez). Yeni iş tipi eklerken buraya koyun.
- **Kaynak izlenebilirliği:** Her veri yapısı `SourceMeta` taşır (source, source_url, as_of_date, data_version).
- **Hata sarma:** `fmt.Errorf("...: %w", err)` ile sarın (kod tabanında %73 tutarlılık).
- **Kütüphane kodunda panik yok:** `internal/` içinde panik kullanmayın (mevcut tek istisna `tradingview/charts.go:687` bir hatadır).
- **Eksik tooling:** Makefile/golangci-lint/test CI yok. Eklerseniz `go vet ./...` + `golangci-lint run` + `go test -race ./...` önerilir.

## Bilinen Teknik Borç

Detaylı liste: [review-2026-06-25.md](./review-2026-06-25.md). Özet: `ta` mega-paketi (722 dosya), tanrı dosyaları (`professional_report.go` 9.549 satır), yukarı bağımlılıklar (`ta → services`), sabit skorlama eşikleri.
