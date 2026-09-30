package utils

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/SebastiaanKlippert/go-wkhtmltopdf"
)

type PDFGenerator struct{}

func NewPDFGenerator() *PDFGenerator {
	return &PDFGenerator{}
}

// landscapePage matches templates that declare a full-bleed A4 landscape page,
// such as the built-in diploma samples: @page { size: A4 landscape; margin: 0 }.
var landscapePage = regexp.MustCompile(`(?is)@page\s*\{[^}]*size\s*:\s*A4\s+landscape`)

func (pg *PDFGenerator) ConvertHTMLToPDF(html string) ([]byte, error) {
	pdfg, err := wkhtmltopdf.NewPDFGenerator()
	if err != nil {
		return convertHTMLToPDFWithChrome(html, err)
	}

	page := wkhtmltopdf.NewPageReader(bytes.NewReader([]byte(html)))
	page.Encoding.Set("utf-8")
	// Lay out with the template's print styles at true size and give web fonts
	// and the text-fitting script time to finish.
	page.PrintMediaType.Set(true)
	page.DisableSmartShrinking.Set(true)
	page.Zoom.Set(1)
	page.JavascriptDelay.Set(1500)

	if landscapePage.MatchString(html) {
		pdfg.PageSize.Set(wkhtmltopdf.PageSizeA4)
		pdfg.Orientation.Set(wkhtmltopdf.OrientationLandscape)
		pdfg.MarginTop.Set(0)
		pdfg.MarginBottom.Set(0)
		pdfg.MarginLeft.Set(0)
		pdfg.MarginRight.Set(0)
	}

	pdfg.AddPage(page)

	err = pdfg.Create()
	if err != nil {
		return convertHTMLToPDFWithChrome(html, err)
	}

	return pdfg.Bytes(), nil
}

func convertHTMLToPDFWithChrome(html string, wkhtmlErr error) ([]byte, error) {
	chromePath := findChromePath()
	if chromePath == "" {
		return nil, fmt.Errorf("wkhtmltopdf unavailable and Chrome was not found: %w", wkhtmlErr)
	}

	htmlFile, err := os.CreateTemp("", "vbs-pqc-*.html")
	if err != nil {
		return nil, fmt.Errorf("create temporary HTML: %w", err)
	}
	htmlPath := htmlFile.Name()
	defer os.Remove(htmlPath)
	if _, err := htmlFile.WriteString(html); err != nil {
		htmlFile.Close()
		return nil, fmt.Errorf("write temporary HTML: %w", err)
	}
	if err := htmlFile.Close(); err != nil {
		return nil, fmt.Errorf("close temporary HTML: %w", err)
	}

	pdfFile, err := os.CreateTemp("", "vbs-pqc-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("create temporary PDF: %w", err)
	}
	pdfPath := pdfFile.Name()
	pdfFile.Close()
	defer os.Remove(pdfPath)
	profileDir, err := os.MkdirTemp("", "vbs-pqc-chrome-profile-*")
	if err != nil {
		return nil, fmt.Errorf("create Chrome profile: %w", err)
	}
	defer os.RemoveAll(profileDir)

	fileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(htmlPath)}).String()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, chromePath,
		"--headless=new",
		"--disable-gpu",
		"--no-sandbox",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--user-data-dir="+profileDir,
		// Page size and margins come from the template's @page rule.
		"--no-pdf-header-footer",
		"--print-to-pdf-no-header",
		// Let web fonts load and the text-fitting script run before printing.
		"--virtual-time-budget=10000",
		"--run-all-compositor-stages-before-draw",
		"--print-to-pdf="+pdfPath,
		fileURL,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("wkhtmltopdf failed (%v), Chrome PDF fallback failed: %w; output: %s", wkhtmlErr, err, string(output))
	}

	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("read Chrome-generated PDF: %w", err)
	}
	if len(pdfBytes) == 0 {
		return nil, fmt.Errorf("Chrome generated an empty PDF")
	}
	return pdfBytes, nil
}

func findChromePath() string {
	// Windows development machines and Linux servers (Google Chrome or Chromium).
	for _, name := range []string{"chrome.exe", "google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	if runtime.GOOS == "windows" {
		candidates := []string{
			`C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe`,
			`C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe`,
			`C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe`,
			`C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe`,
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	return ""
}
