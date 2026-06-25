---
project_name: 'bist (hissebot)'
user_name: 'ersan'
date: '2026-06-25'
sections_completed: ['technology_stack', 'language_rules', 'framework_rules', 'testing_rules', 'code_quality', 'workflow_rules', 'dont_miss_rules']
existing_patterns_found: 12
status: 'complete'
rule_count: 33
optimized_for_llm: true
---

# Project Context for AI Agents

_Bu dosya, `bist` (modül: `hissebot`) projesinde kod üretirken AI ajanlarının uyması gereken kritik kuralları içerir. Aşikar olmayan, LLM'lerin kaçırabileceği detaylara odaklanır. Genel Go bilgisi tekrarlanmaz._

---

## Technology Stack & Versions

- **Go 1.26** — modül adı `hissebot` (import yolları `hissebot/internal/...`).
- **HTTP:** gofiber/fiber **v3.3.0** (`serve api`). Not: Fiber v3, v2'den farklı API'ye sahip — v2 örneklerini kopyalama.
- **WebSocket:** gorilla/websocket v1.5.3 (`internal/wsclient`, canlı market).
- **Excel:** xuri/excelize v2.8.1 · **PDF:** ledongthuc/pdf · **XLS (legacy):** extrame/xls v0.0.1.
- **SQLite:** mattn/go-sqlite3 v1.14.32 — **cgo gerektirir** (C derleyici şart; saf-Go build çalışmaz).
- **Görsel:** golang.org/x/image (grafik render, `internal/ta/chart`).
- Build/lint/test tooling **yok**: Makefile yok, golangci yok, test CI yok. Eklersen `go vet` + `golangci-lint` + `go test -race` öner.

## Critical Implementation Rules

### Mimari & Katman Kuralları (en önemli)

- **Kalıcılık dosya tabanlıdır, PostgreSQL DEĞİL.** Veri hisse bazlı JSON'da: `data/equities/{TICKER}/*.json`. Okuma/yazma **yalnızca** `internal/storage` `EquityStore` üzerinden (`storage.NewEquityStore(root)`). `database/sql` veya ORM ekleme. `migrations/*.sql` sadece **referans şemadır**, runtime'da bağlanılmaz.
- **`internal/domain` saf tutulur.** Domain paketleri altyapı (`services`, `storage`, `datasources`, `net/http`, `database/sql`, 3rd-party lib) import ETMEZ. Yeni iş tipi → buraya saf struct olarak.
- **`ta → services/datasources/kapingest` yukarı bağımlılığını ÇOĞALTMA.** Bu mevcut bir mimari kusurdur (P1). Hesaplama çekirdeği (`internal/ta`) I/O servislerine doğrudan bağlanmamalı; ihtiyaç varsa küçük bir arayüz (port) enjekte et, somut `services/*` import etme.
- **`internal/ta` bir mega-pakettir (kod tabanının %66'sı, 30 alt-paket).** Yeni metrik eklerken kanonik evi seç: teknik/forecast → `ta`, portföy/oran/volatilite matematiği → `quant`, ham veri çekme → `services`. Risk hesabı üç yerde tekrarlı (`ta/risk`, `analysis/risk`, `quant`) — yenisini eklemeden önce hangisinin genişletileceğini kontrol et.

### Veri & İzlenebilirlik Kuralları

- **Her veri yapısı `SourceMeta` taşır** (`source`, `source_url`, `as_of_date`, `data_version`). Yeni domain tipi eklerken bu deseni koru — izlenebilirlik zorunlu.
- **Versiyonlama:** Kayıtlar `data_version` + `as_of_date` ile sürümlenir (tarihsel revizyon). KAP belgeleri ayrıca `version` + `latest` flag taşır.
- **KAP merkezi cache:** `data/equities/_kap/` altında manifest/registry/failures. Ek indirme idempotent (aynı işin tekrar çalışması güvenli) olmalı; `attachments_manifest.jsonl` sha256 ile bütünlük doğrular, `attachments_failures.jsonl` tekrar denenir.
- `data/` git'te izlenmez (`.gitignore`). Üretilen veriyi commit etme (KAP attachment worker'ın özel commit akışı hariç).

### Dil (Go) Kuralları

- **Hata sarma idiomu:** `fmt.Errorf("bağlam: %w", err)` — bağlam mesajı + `%w`. Kod tabanı bu konuda tutarlı; aynısını uygula. Sentinel hatalar paket düzeyinde `var Err... = errors.New(...)`.
- **Kütüphane kodunda panik YASAK.** `internal/` içinde `panic`/`log.Fatal` kullanma; hata döndür ve yukarı taşı. `log.Fatalf` yalnızca `main()` içinde. (Mevcut tek istisna `services/tradingview/charts.go:687` bir bugdur, örnek alma.)
- **`context.Context` propagasyonu:** Çağıranın ctx'ini ilet; sıcak yolda taze `context.Background()`/`context.TODO()` başlatma (iptal/timeout zincirini koparır). Mevcut kötü örnekler: `ta/analysis/engine.go:2566`, `ta/storage/writer.go:315`.
- **Kaynak temizliği:** HTTP gövdesini hata kontrolünden hemen sonra `defer resp.Body.Close()`. Cevap okumalarını `io.LimitReader` ile sınırla (örn. 8MB).
- **Eşzamanlılık deseni:** Worker pool için `internal/services/financials/fetch.go:155-185` kanonik örnek (WaitGroup + `defer close(jobCh)` + `wg.Wait()` sonra `close(resultCh)` + ilk fatal hatada `cancel()`). Paylaşılan durumu mutex ile koru.
- **Paket düzeyinde değişebilir global state ekleme.** Mevcut `var`lar değişmez lookup tabloları/regex'ler. Singleton/global yerine değer geç.

### Framework / Servis Kuralları

- **CLI:** Tüm komutlar `cmd/hissebot/main.go` içinde `run()` → `args[0]` üzerinde flat `switch`. Her komut kendi `flag.FlagSet`'ini ayrıştırır. Yeni komut eklerken bu deseni izle; merkezi DI container ekleme.
- **HTTP:** Fiber **v3** handler imzaları (`func(c fiber.Ctx) error`). `/healthz` sağlık endpoint'i korunur. Yeni rota eklerken mevcut sektör endpoint desenini (`api_server.go`) örnek al.
- **Config:** Tüm ayar `internal/config/config.go` `Load()` ile env'den okunur (51 alanlı düz struct). Yeni ayar → `Config` struct + `Load()` içinde `getenv(...)`. **Gizli bilgileri** (KAP token, MKK/IsYatırım çerezleri, MQTT şifresi) koda gömme; env'den oku.

### Test Kuralları

- Go standart `*_test.go`, **tablo-tabanlı** testler. `internal/quant` iyi kaplı bir referanstır.
- **Testsiz kritik alanlar (yeni test önceliği):** `internal/analysis/{fundamental,risk,technical,valuation}/scoring.go` ve `internal/confidence/score.go` (0.75 inceleme kapısı). Bunlar karar matematiği — değiştirirsen test ekle.
- Eşzamanlılık değiştiren PR'larda `go test -race ./...` çalıştır.

### Kod Kalitesi & Stil

- **İsimlendirme:** Go idiomatik — paket adları kısa/küçük harf, export `PascalCase`, dosya adları `snake_case.go`. Sembolleri gereksiz yeniden adlandırma; mevcut konvansiyona uy.
- **Sabit eşikleri isimlendir.** Finansal/skor eşiklerini çıplak literal bırakma → `const`. Örn. `confidence/score.go` ağırlıkları (`0.20/0.35/-0.25`) ve `0.75` kapısı isimlendirilmeli; indikatör sabitleri (CCI `0.015`, Fib seviyeleri) tek yerde.
- **Tanrı dosyası üretme.** Mevcut sorun: `ta/storage/professional_report.go` 9.549 satır, `reportLabel` 719 satır. Yeni büyük çeviri/switch tabloları yerine data map / codegen kullan.
- **Codegen:** `internal/ta/{indicators,patterns}/generated/` üretilmiştir — elle düzenleme; `tools/indicator_catalog_gen` / `tools/pattern_catalog_gen` ile yeniden üret.
- Paylaşılan matematik için `pkg/mathutil` (Max/Min/Clamp/SafeDiv) kullan; yeniden implemente etme (en yüksek fan-in'li çekirdek).

### Geliştirme Workflow Kuralları

- **Dil:** Kod yorumları/dokümanlar Türkçe yazılabilir (README ve docs Türkçe). Mevcut dil tarzına uy.
- **Commit:** İstenmedikçe commit/push yapma. `data/` ve kök `hissebot` ikilisini commit etme.
- **Build doğrulama:** Değişiklik sonrası `go build ./...` (cgo sqlite uyarısı zararsız, beklenir).
- **KAP worker:** Tek CI workflow'u `.github/workflows/kap-attachment-worker.yml` (manuel `workflow_dispatch`). Uzun süreli ingestion `sync kap-attachments` dayanıklılık bayraklarıyla (retry/rate-limit-sleep/min-free-bytes) çalışır.

### Kritik "Kaçırma" Kuralları (anti-pattern özeti)

- ❌ PostgreSQL/ORM/`database/sql` ekleme — kalıcılık dosya tabanlı.
- ❌ `internal/domain`'e altyapı import etme.
- ❌ `internal/ta`'dan `services`/`datasources`/`kapingest`'e yeni doğrudan bağımlılık ekleme.
- ❌ Kütüphane kodunda `panic`/`log.Fatal`.
- ❌ Sıcak yolda `context.Background()`.
- ❌ Skor/finansal eşikleri çıplak literal.
- ❌ Gizli bilgileri (token/çerez/şifre) koda gömme.
- ❌ `SourceMeta`'sız yeni veri yapısı.
- ✅ Hataları `%w` ile sar, ctx'i ilet, kaynakları `defer` ile kapat, idempotent ingestion yaz.

---

## Usage Guidelines

**AI Ajanları için:**
- Kod yazmadan ÖNCE bu dosyayı oku ve tüm kuralları aynen uygula.
- Tereddütte daha kısıtlayıcı seçeneği tercih et (örn. yeni bağımlılık eklemek yerine arayüz enjekte et).
- Yeni bir kalıcı desen ortaya çıkarsa bu dosyayı güncelle.
- Daha derin mimari bağlam için: `docs/index.md`, `docs/architecture.md`, `docs/review-2026-06-25.md`.

**İnsanlar için:**
- Dosyayı yalın ve ajan ihtiyacına odaklı tut.
- Teknoloji yığını veya desenler değişince güncelle (özellikle Fiber/Go sürümü, kalıcılık modeli).
- Aşikar hale gelen kuralları çıkar; mimari kusurlar (P1) düzeltildikçe ilgili "kaçırma" kurallarını sadeleştir.

Last Updated: 2026-06-25
