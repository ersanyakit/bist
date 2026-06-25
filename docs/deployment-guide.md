# hissebot — Dağıtım Rehberi

> Otomatik üretildi: BMAD Document Project (kapsamlı tarama) — 2026-06-25

`hissebot` durumsuz (stateless) bir Go ikilisidir; durum dosya sisteminde (`data/`) tutulur. Dağıtım = ikiliyi derle + zamanlanmış veri toplama job'larını çalıştır. Veritabanı/konteyner orkestrasyonu yoktur.

## 1. Derleme

```bash
go build -o hissebot ./cmd/hissebot
```

`.gitignore` ile kök `hissebot` ikilisi ve `data/` izlenmez.

## 2. KAP Attachment Worker (GitHub Actions)

Tek CI/CD workflow'u: `.github/workflows/kap-attachment-worker.yml`. `workflow_dispatch` ile manuel tetiklenir. KAP eklerini indirir, periyodik commit/push yapar.

Parametreler:
| Girdi | Varsayılan | Anlam |
|---|---|---|
| `duration_minutes` | 300 | Commit/push öncesi yumuşak çalışma süresi |
| `limit_per_ticker` | 1 | Commit öncesi ticker başına maks yeni dosya |
| `delay` | 2s | KAP istekleri arası gecikme |
| `request_timeout` | 90s | İstek başına zaman aşımı |
| `run_disclosures` | false | İndirmeden önce bildirim metadata'sını yenile |
| `disclosure_from` | 2010-01-01 | Bildirim metadata başlangıç tarihi |
| `attachment_from` / `attachment_to` | — | Opsiyonel ek tarih aralığı |

## 3. macOS launchd (yerel sürekli worker)

`ops/launchagents/com.hissebot.kap.attachments.plist` — yerel makinede KAP ek indirmeyi sürekli çalıştırmak için launchd agent'ı.

```bash
cp ops/launchagents/com.hissebot.kap.attachments.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.hissebot.kap.attachments.plist
```

## 4. Paralel KAP İndirme

```bash
WORKERS=4 ./scripts/sync_kap_attachments_parallel.sh
```

`sync kap-attachments` dayanıklılık bayraklarıyla (retry, transient-error-sleep, rate-limit-sleep, min-free-bytes) uzun süreli çalışacak şekilde tasarlanmıştır.

## 5. Sunucu Dağıtımı

```bash
./hissebot serve api       # HTTP API (Fiber v3)
./hissebot serve reports   # Rapor sunucusu
./hissebot serve comments  # Yorum sunucusu
```

Sağlık kontrolü: `GET /healthz`. Ters proxy/process manager (systemd, supervisor) arkasına alın.

## 6. Ortam

Gerekli env değişkenleri için bkz. [development-guide.md](./development-guide.md). Gizli bilgiler (KAP token, MKK/IsYatırım çerezleri, MQTT şifresi) deploy ortamında secret yöneticisinden enjekte edilmeli — repoya commit edilmemeli.

## 7. Eksik / Önerilen

- **Otomatik test CI yok.** Önerilen: PR'larda `go build ./...` + `go test -race ./...` + `golangci-lint` çalıştıran bir workflow.
- **Konteynerleştirme yok.** İkili statik derlenebilir (cgo sqlite hariç); Dockerfile eklenebilir.
- **İzleme yok.** `/healthz` dışında metrik/log toplama tanımlı değil.
