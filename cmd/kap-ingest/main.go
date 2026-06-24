package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"hissebot/internal/kapingest"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("kap-ingest", flag.ExitOnError)
	input := fs.String("input", "data/equities", "PDF'lerin bulunduğu ana klasör")
	output := fs.String("output", "data/processed", "JSONL çıktı klasörü")
	workers := fs.Int("workers", kapingest.DefaultWorkers, "paralel worker sayısı")
	limit := fs.Int("limit", 0, "en fazla işlenecek PDF sayısı, 0 limitsiz")
	resume := fs.Bool("resume", true, "processed_files.jsonl içindeki hash'leri atla")
	llm := fs.Bool("llm", false, "KAPEvent extraction çalıştır")
	dryRun := fs.Bool("dry-run", false, "işlem planını göster ama yazma")
	timeout := fs.Duration("timeout", kapingest.DefaultExtractTimeout, "PDF başına pdftotext timeout")
	ocr := fs.Bool("ocr", true, "düşük kaliteli veya metinsiz PDF'lerde OCR fallback çalıştır")
	ocrTimeout := fs.Duration("ocr-timeout", kapingest.DefaultOCRTimeout, "PDF başına OCR fallback timeout")
	ocrLang := fs.String("ocr-lang", "", "Tesseract OCR dili; boşsa tur+eng varsa onu, yoksa eng kullanılır")
	assetsOnly := fs.Bool("assets-only", false, "mevcut raw_documents.jsonl üstünden sadece asset extraction çalıştır")
	rawDocuments := fs.String("raw-documents", "", "assets-only için raw_documents.jsonl yolu; boşsa output/raw_documents.jsonl")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx := context.Background()
	if *assetsOnly {
		rawPath := *rawDocuments
		if rawPath == "" {
			rawPath = *input
		}
		if rawPath == "" || rawPath == "data/equities" {
			rawPath = ""
		}
		summary, err := kapingest.ExtractAssetsFromRawDocuments(ctx, kapingest.AssetExtractionOptions{
			RawDocumentsPath: rawPath,
			OutputDir:        *output,
			Now:              time.Now,
		})
		if encodeErr := printJSON(summary); encodeErr != nil {
			return encodeErr
		}
		return err
	}
	summary, err := kapingest.Run(ctx, kapingest.Options{
		InputDir:       *input,
		OutputDir:      *output,
		Workers:        *workers,
		Limit:          *limit,
		Resume:         *resume,
		LLM:            *llm,
		DryRun:         *dryRun,
		ExtractTimeout: *timeout,
		DisableOCR:     !*ocr,
		OCRTimeout:     *ocrTimeout,
		OCRLanguages:   *ocrLang,
		Now:            time.Now,
	})
	if encodeErr := printJSON(summary); encodeErr != nil {
		return encodeErr
	}
	return err
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("json output: %w", err)
	}
	return nil
}
