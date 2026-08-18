package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

const dateLayout = "2006-01-02"

func main() {
	date := flag.String("date", time.Now().UTC().AddDate(0, 0, -1).Format(dateLayout), "Hacker News archive date (YYYY-MM-DD)")
	flag.Parse()

	if err := run(*date); err != nil {
		log.Fatal(err)
	}
}

func run(date string) error {
	if _, err := time.Parse(dateLayout, date); err != nil {
		return fmt.Errorf("invalid date %q (expected YYYY-MM-DD): %w", date, err)
	}

	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("start playwright: %w", err)
	}
	defer func() {
		if err := pw.Stop(); err != nil {
			log.Printf("could not stop playwright: %v", err)
		}
	}()

	browser, err := pw.Chromium.Launch()
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	defer func() {
		if err := browser.Close(); err != nil {
			log.Printf("could not close browser: %v", err)
		}
	}()

	page, err := browser.NewPage()
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}

	frontPageURL := "https://news.ycombinator.com/front?day=" + url.QueryEscape(date)
	response, err := page.Goto(frontPageURL)
	if err != nil {
		return fmt.Errorf("open Hacker News front page: %w", err)
	}
	if response != nil && response.Status() >= 400 {
		return fmt.Errorf("open Hacker News front page: returned HTTP %d", response.Status())
	}

	html, err := page.Content()
	if err != nil {
		return fmt.Errorf("read Hacker News front page: %w", err)
	}
	stories, err := ParseFrontPageStories(date, html)
	if err != nil {
		return fmt.Errorf("parse Hacker News stories: %w", err)
	}
	if len(stories) == 0 {
		return fmt.Errorf("Hacker News returned no stories")
	}

	pdfDir, err := os.MkdirTemp("", "hackernewspaper-pdfs-*")
	if err != nil {
		return fmt.Errorf("create temporary PDF directory: %w", err)
	}
	defer os.RemoveAll(pdfDir)

	pdfFiles := make([]string, 0, len(stories)+1)
	skippedStories := 0
	frontPagePDF := filepath.Join(pdfDir, "000.pdf")
	frontPageBytes, err := page.PDF()
	if err != nil {
		return fmt.Errorf("render Hacker News front page as PDF: %w", err)
	}
	if err := os.WriteFile(frontPagePDF, frontPageBytes, 0644); err != nil {
		return fmt.Errorf("write Hacker News front page PDF: %w", err)
	}
	pdfFiles = append(pdfFiles, frontPagePDF)

	for index, story := range stories {
		pdfPath := filepath.Join(pdfDir, fmt.Sprintf("%03d.pdf", index+1))
		log.Printf("rendering story %d/%d: %s", index+1, len(stories), story.Title)

		response, err := page.Goto(story.URL)
		if err != nil {
			log.Printf("skipping story %d/%d %q: open failed: %v", index+1, len(stories), story.Title, err)
			skippedStories++
			continue
		}
		if response != nil && response.Status() >= 400 {
			log.Printf("skipping story %d/%d %q: returned HTTP %d", index+1, len(stories), story.Title, response.Status())
			skippedStories++
			continue
		}

		bytes, err := page.PDF()
		if err != nil {
			log.Printf("skipping story %d/%d %q: render failed: %v", index+1, len(stories), story.Title, err)
			skippedStories++
			continue
		}
		if err := os.WriteFile(pdfPath, bytes, 0644); err != nil {
			log.Printf("skipping story %d/%d %q: write failed: %v", index+1, len(stories), story.Title, err)
			skippedStories++
			continue
		}
		pdfFiles = append(pdfFiles, pdfPath)
	}

	outputPDF := fmt.Sprintf("hackernewspaper-%s.pdf", date)
	if err := api.MergeCreateFile(pdfFiles, outputPDF, false, nil); err != nil {
		return fmt.Errorf("merge PDFs: %w", err)
	}

	log.Printf("merged front page and %d/%d stories into %s (%d skipped)", len(stories)-skippedStories, len(stories), outputPDF, skippedStories)
	return nil
}
