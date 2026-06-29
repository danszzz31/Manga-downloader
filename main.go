package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mangadown/mangadex"
)

var logFile *os.File

func main() {
	client := mangadex.NewClient()
	initLogger()
	defer logFile.Close()

	log("MangaDex Downloader started")
	log("━━━━━━━━━━━━━━━━━━━━━━━━━")

	// --- Step 1: Search for manga ---
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("🔍 Search manga: ")
	query, _ := reader.ReadString('\n')
	query = strings.TrimSpace(query)
	if query == "" {
		fmt.Println("❌ Search query cannot be empty.")
		os.Exit(1)
	}

	fmt.Println("  Searching...")
	mangas, err := client.SearchManga(query)
	if err != nil {
		fmt.Printf("❌ Search failed: %v\n", err)
		os.Exit(1)
	}

	if len(mangas) == 0 {
		fmt.Println("❌ No manga found with that title.")
		os.Exit(1)
	}

	fmt.Println("\n📚 Results:")
	for i, m := range mangas {
		title := getBestTitle(m.Attributes.Title)
		fmt.Printf("  [%d] %s\n", i+1, title)
	}
	fmt.Printf("\nSelect manga [1-%d]: ", len(mangas))
	selStr, _ := reader.ReadString('\n')
	selStr = strings.TrimSpace(selStr)
	sel, err := strconv.Atoi(selStr)
	if err != nil || sel < 1 || sel > len(mangas) {
		fmt.Println("❌ Invalid selection.")
		os.Exit(1)
	}
	selected := mangas[sel-1]
	title := getBestTitle(selected.Attributes.Title)
	fmt.Printf("  ✓ Selected: %s\n", title)
	log("Selected manga: %s (ID: %s)", title, selected.ID)

	// --- Step 2: Show available languages ---
	fmt.Println("\n🌐 Fetching available languages...")
	langs, err := client.GetAvailableLanguages(selected.ID)
	if err != nil {
		fmt.Printf("❌ Failed to get languages: %v\n", err)
		os.Exit(1)
	}
	if len(langs) == 0 {
		fmt.Println("❌ No chapters found for this manga.")
		os.Exit(1)
	}

	fmt.Println("\n🌐 Available languages:")
	for i, lang := range langs {
		fmt.Printf("  [%d] %s (%s)\n", i+1, languageDisplayName(lang), lang)
	}
	fmt.Printf("\nSelect language [1-%d]: ", len(langs))
	langStr, _ := reader.ReadString('\n')
	langStr = strings.TrimSpace(langStr)
	langSel, err := strconv.Atoi(langStr)
	if err != nil || langSel < 1 || langSel > len(langs) {
		fmt.Println("❌ Invalid selection.")
		os.Exit(1)
	}
	chosenLang := langs[langSel-1]
	fmt.Printf("  ✓ Language: %s\n", languageDisplayName(chosenLang))
	log("Selected language: %s", chosenLang)

	// --- Step 3: Fetch all chapters ---
	fmt.Println("\n📖 Fetching chapters...")
	chapters, err := client.GetChapters(selected.ID, chosenLang)
	if err != nil {
		fmt.Printf("❌ Failed to get chapters: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Found %d chapters\n", len(chapters))
	log("Found %d chapters in %s", len(chapters), chosenLang)

	if len(chapters) == 0 {
		fmt.Println("❌ No chapters available.")
		os.Exit(1)
	}

	// --- Step 4: Choose concurrency ---
	fmt.Print("\n⚡ Concurrent downloads. (default 5, press Enter for default): ")
	concStr, _ := reader.ReadString('\n')
	concStr = strings.TrimSpace(concStr)
	concurrency := 5
	if concStr != "" {
		if c, err := strconv.Atoi(concStr); err == nil && c > 0 && c <= 20 {
			concurrency = c
		} else {
			fmt.Printf("  Using default 5 (valid range: 1-20)\n")
		}
	}
	fmt.Printf("  Downloading %d chapters at a time\n", concurrency)
	log("Concurrency: %d", concurrency)

	// --- Step 5: Create output directory ---
	safeName := sanitizeDirName(title)
	baseDir := filepath.Join(".", safeName)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		fmt.Printf("❌ Failed to create directory: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("📁 Output directory: %s\n", baseDir)

	// --- Step 6: Download all chapters concurrently ---
	fmt.Println("\n⬇️  Starting download...")
	log("Starting download of %d chapters to %s (concurrency: %d)", len(chapters), baseDir, concurrency)

	startTime := time.Now()

	progressFn := func(done, total int) {}

	downloaded, failed := client.DownloadAllChapters(chapters, baseDir, concurrency, 3, progressFn, func(s string) { log("%s", s) })

	elapsed := time.Since(startTime)
	total := downloaded + failed
	fmt.Printf("\n✅ Download complete! %d/%d chapters in %s\n", downloaded, total, formatDuration(elapsed))
	if failed > 0 {
		fmt.Printf("⚠  %d chapters failed (see log)\n", failed)
	}
	fmt.Printf("📁 Saved to: %s\n", baseDir)
	log("Download complete! %d/%d successful in %s (failures: %d)", downloaded, total, formatDuration(elapsed), failed)
}

func initLogger() {
	logDir := "logs"
	os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, fmt.Sprintf("mangadown_%s.log", time.Now().Format("20060102_150405")))
	f, err := os.Create(logPath)
	if err != nil {
		fmt.Printf("⚠ Could not create log file: %v\n", err)
		f, _ = os.CreateTemp("", "mangadown_*.log")
		if f != nil {
			logPath = f.Name()
		}
	}
	logFile = f
	fmt.Printf("📝 Log file: %s\n", logPath)
}

func log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("    %s\n", msg)
	if logFile != nil {
		fmt.Fprintf(logFile, "[%s] %s\n", timestamp, msg)
	}
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func getBestTitle(titles map[string]string) string {
	if t, ok := titles["en"]; ok {
		return t
	}
	for _, t := range titles {
		return t
	}
	return "Unknown Title"
}

func languageDisplayName(code string) string {
	names := map[string]string{
		"en": "English", "ja": "Japanese", "ko": "Korean",
		"zh": "Chinese", "zh-hk": "Chinese (HK)", "zh-ro": "Chinese (RO)",
		"ar": "Arabic", "bg": "Bulgarian", "bn": "Bengali",
		"ca": "Catalan", "cs": "Czech", "da": "Danish",
		"de": "German", "el": "Greek", "es": "Spanish",
		"es-la": "Spanish (Latin America)", "fa": "Persian",
		"fi": "Finnish", "fr": "French", "he": "Hebrew",
		"hi": "Hindi", "hr": "Croatian", "hu": "Hungarian",
		"id": "Indonesian", "it": "Italian", "lt": "Lithuanian",
		"mn": "Mongolian", "ms": "Malay", "my": "Burmese",
		"nl": "Dutch", "no": "Norwegian", "pl": "Polish",
		"pt": "Portuguese", "pt-br": "Portuguese (Brazil)",
		"ro": "Romanian", "ru": "Russian", "sk": "Slovak",
		"sl": "Slovenian", "sr": "Serbian", "sv": "Swedish",
		"th": "Thai", "tl": "Tagalog", "tr": "Turkish",
		"uk": "Ukrainian", "vi": "Vietnamese",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return code
}

func sanitizeDirName(name string) string {
	replacer := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "-",
		"?", "-", "\"", "'", "<", "-", ">", "-",
		"|", "-", " ", "_",
	)
	return strings.TrimSpace(replacer.Replace(name))
}
