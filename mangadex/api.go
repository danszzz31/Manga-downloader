package mangadex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

const baseURL = "https://api.mangadex.org"

// Client handles MangaDex API calls
type Client struct {
	http         *http.Client
	downloadHTTP *http.Client // separate client for image downloads (longer timeout)
}

// NewClient creates a new MangaDex API client
func NewClient() *Client {
	return &Client{
		http:         &http.Client{Timeout: 30 * time.Second},
		downloadHTTP: &http.Client{Timeout: 300 * time.Second},
	}
}

// SearchManga searches for manga by title
func (c *Client) SearchManga(title string) ([]Manga, error) {
	u := fmt.Sprintf("%s/manga?title=%s&limit=10", baseURL, url.QueryEscape(title))
	resp, err := c.http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search returned status %d: %s", resp.StatusCode, string(body))
	}

	var ml MangaList
	if err := json.NewDecoder(resp.Body).Decode(&ml); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	if ml.Result != "ok" {
		return nil, fmt.Errorf("search API result: %s", ml.Result)
	}

	return ml.Data, nil
}

// GetAvailableLanguages returns the set of languages available for a manga
func (c *Client) GetAvailableLanguages(mangaID string) ([]string, error) {
	u := fmt.Sprintf("%s/manga/%s/feed?limit=1&offset=0", baseURL, mangaID)
	resp, err := c.http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("languages request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("languages returned status %d", resp.StatusCode)
	}

	return c.getAllLanguages(mangaID)
}

func (c *Client) getAllLanguages(mangaID string) ([]string, error) {
	langSet := make(map[string]struct{})
	limit := 500
	offset := 0

	for {
		u := fmt.Sprintf("%s/manga/%s/feed?limit=%d&offset=%d&order[chapter]=asc",
			baseURL, mangaID, limit, offset)
		resp, err := c.http.Get(u)
		if err != nil {
			return nil, fmt.Errorf("feed request: %w", err)
		}

		var cl ChapterList
		if err := json.NewDecoder(resp.Body).Decode(&cl); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode feed: %w", err)
		}
		resp.Body.Close()

		if cl.Result != "ok" {
			resp.Body.Close()
			return nil, fmt.Errorf("feed result: %s", cl.Result)
		}

		for _, ch := range cl.Data {
			langSet[ch.Attributes.TranslatedLanguage] = struct{}{}
		}

		if len(cl.Data) < limit {
			break
		}
		offset += limit
	}

	langs := make([]string, 0, len(langSet))
	for l := range langSet {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	return langs, nil
}

// GetChapters returns all chapters for a manga in a given language
func (c *Client) GetChapters(mangaID, lang string) ([]Chapter, error) {
	var all []Chapter
	limit := 500
	offset := 0

	for {
		u := fmt.Sprintf("%s/manga/%s/feed?limit=%d&offset=%d&translatedLanguage[]=%s&order[chapter]=asc",
			baseURL, mangaID, limit, offset, url.QueryEscape(lang))
		resp, err := c.http.Get(u)
		if err != nil {
			return nil, fmt.Errorf("feed request: %w", err)
		}

		var cl ChapterList
		if err := json.NewDecoder(resp.Body).Decode(&cl); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode feed: %w", err)
		}
		resp.Body.Close()

		if cl.Result != "ok" {
			return nil, fmt.Errorf("feed result: %s", cl.Result)
		}

		// Filter out external chapters (those hosted elsewhere)
		for _, ch := range cl.Data {
			if ch.Attributes.ExternalURL == nil || *ch.Attributes.ExternalURL == "" {
				all = append(all, ch)
			}
		}

		if len(cl.Data) < limit {
			break
		}
		offset += limit
	}

	return normalizeChapters(all), nil
}

func normalizeChapters(chapters []Chapter) []Chapter {
	// Filter out ad/donation chapters: MangaDex uploaders often combine
	// multiple chapters into one upload with "+" in the chapter number or title,
	// padding with ads/donation pages. These are never the clean standalone chapters.
	clean := make([]Chapter, 0, len(chapters))
	for _, ch := range chapters {
		if strings.Contains(ch.Attributes.Chapter, "+") || strings.Contains(ch.Attributes.Title, "+") {
			continue
		}
		clean = append(clean, ch)
	}

	unique := make([]Chapter, 0, len(clean))
	seen := make(map[string]int, len(clean))

	for _, ch := range clean {
		key := chapterDedupeKey(ch)
		if idx, ok := seen[key]; ok {
			if shouldReplaceChapter(ch, unique[idx]) {
				unique[idx] = ch
			}
			continue
		}

		seen[key] = len(unique)
		unique = append(unique, ch)
	}

	sort.SliceStable(unique, func(i, j int) bool {
		return chapterLess(unique[i], unique[j])
	})

	return unique
}

func chapterDedupeKey(ch Chapter) string {
	lang := strings.ToLower(strings.TrimSpace(ch.Attributes.TranslatedLanguage))
	chapter := normalizeChapterPart(ch.Attributes.Chapter)
	if chapter != "" {
		return lang + "|chapter|" + chapter
	}

	title := strings.ToLower(strings.TrimSpace(ch.Attributes.Title))
	volume := normalizeChapterPart(chapterVolume(ch))
	if title != "" {
		return lang + "|untitled-chapter|" + volume + "|" + title
	}

	return lang + "|id|" + ch.ID
}

func shouldReplaceChapter(candidate, current Chapter) bool {
	if candidate.Attributes.Pages != current.Attributes.Pages {
		return candidate.Attributes.Pages > current.Attributes.Pages
	}
	if candidate.Attributes.PublishAt != current.Attributes.PublishAt {
		return candidate.Attributes.PublishAt > current.Attributes.PublishAt
	}
	return candidate.ID < current.ID
}

func chapterLess(a, b Chapter) bool {
	if cmp := compareChapterPart(a.Attributes.Chapter, b.Attributes.Chapter); cmp != 0 {
		return cmp < 0
	}
	if cmp := compareChapterPart(chapterVolume(a), chapterVolume(b)); cmp != 0 {
		return cmp < 0
	}
	if a.Attributes.PublishAt != b.Attributes.PublishAt {
		return a.Attributes.PublishAt < b.Attributes.PublishAt
	}
	return a.ID < b.ID
}

func chapterVolume(ch Chapter) string {
	if ch.Attributes.Volume == nil {
		return ""
	}
	return *ch.Attributes.Volume
}

func compareChapterPart(a, b string) int {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)

	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}

	af, aok := parseChapterPart(a)
	bf, bok := parseChapterPart(b)
	if aok && bok {
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		default:
			return strings.Compare(normalizeChapterPart(a), normalizeChapterPart(b))
		}
	}
	if aok {
		return -1
	}
	if bok {
		return 1
	}

	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

func normalizeChapterPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if f, ok := parseChapterPart(value); ok {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strings.ToLower(value)
}

func parseChapterPart(value string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// GetChapterPages gets the page image URLs for a chapter
func (c *Client) GetChapterPages(chapterID string) (imgBaseURL string, filenames []string, err error) {
	u := fmt.Sprintf("%s/at-home/server/%s", baseURL, chapterID)
	resp, err := c.http.Get(u)
	if err != nil {
		return "", nil, fmt.Errorf("at-home request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("at-home returned status %d", resp.StatusCode)
	}

	var ar AtHomeResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return "", nil, fmt.Errorf("decode at-home: %w", err)
	}

	if ar.Result != "ok" {
		return "", nil, fmt.Errorf("at-home result: %s", ar.Result)
	}

	return ar.BaseURL + "/data/" + ar.Chapter.Hash + "/", ar.Chapter.Data, nil
}

// DownloadChapterPages downloads all pages of a chapter concurrently
func (c *Client) DownloadChapterPages(chapter Chapter, chDir string, pageConcurrency int, logFn func(string)) error {
	base, pages, err := c.GetChapterPages(chapter.ID)
	if err != nil {
		return fmt.Errorf("get pages: %w", err)
	}

	if err := os.MkdirAll(chDir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	bar := progressbar.NewOptions(len(pages),
		progressbar.OptionSetDescription(fmt.Sprintf("Ch. %s", chapter.Attributes.Chapter)),
		progressbar.OptionSetWidth(30),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowIts(),
		progressbar.OptionClearOnFinish(),
	)

	// Download pages concurrently within a chapter
	sem := make(chan struct{}, pageConcurrency)
	var wg sync.WaitGroup
	errs := make(chan error, len(pages))

	for i, filename := range pages {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, fname string) {
			defer wg.Done()
			defer func() { <-sem }()

			imgURL := base + fname
			ext := filepath.Ext(fname)
			if ext == "" {
				ext = ".jpg"
			}
			pageNum := fmt.Sprintf("%s%03d%s", "p_", idx+1, ext)
			destPath := filepath.Join(chDir, pageNum)

			if err := c.downloadImage(imgURL, destPath); err != nil {
				errs <- fmt.Errorf("page %d: %w", idx+1, err)
				bar.Add(1)
				return
			}
			bar.Add(1)
		}(i, filename)
	}

	wg.Wait()
	close(errs)

	var firstErr error
	for e := range errs {
		if firstErr == nil {
			firstErr = e
		}
	}
	return firstErr
}

// DownloadChapter downloads a single chapter (for backward compat / sequential use)
func (c *Client) DownloadChapter(chapter Chapter, baseDir string, logFn func(string)) error {
	vol := ""
	if chapter.Attributes.Volume != nil && *chapter.Attributes.Volume != "" {
		vol = "Vol. " + *chapter.Attributes.Volume + " - "
	}
	chNum := chapter.Attributes.Chapter
	chTitle := chapter.Attributes.Title

	dirName := fmt.Sprintf("%sCh. %s", vol, chNum)
	if chTitle != "" {
		dirName += " - " + sanitizeFilename(chTitle)
	}
	chDir := filepath.Join(baseDir, sanitizeFilename(dirName))

	logFn(fmt.Sprintf("  → Downloading Ch. %s", chNum))

	err := c.DownloadChapterPages(chapter, chDir, 2, logFn)
	if err != nil {
		return err
	}

	logFn(fmt.Sprintf("  ✓ Ch. %s done", chNum))
	return nil
}

// DownloadAllChapters downloads all chapters concurrently, limiting concurrency
func (c *Client) DownloadAllChapters(chapters []Chapter, baseDir string, concurrency, pageConc int, progressFn func(int, int), logFn func(string)) (int, int) {
	chapters = normalizeChapters(chapters)

	type result struct {
		idx   int
		err   error
		chNum string
	}

	results := make(chan result, len(chapters))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, ch := range chapters {
		sem <- struct{}{}

		chNum := ch.Attributes.Chapter
		if chNum == "" {
			chNum = "0"
		}

		vol := ""
		if ch.Attributes.Volume != nil && *ch.Attributes.Volume != "" {
			vol = "Vol. " + *ch.Attributes.Volume + " - "
		}
		chTitle := ch.Attributes.Title
		dirName := fmt.Sprintf("%sCh. %s", vol, chNum)
		if chTitle != "" {
			dirName += " - " + sanitizeFilename(chTitle)
		}
		chDir := filepath.Join(baseDir, sanitizeFilename(dirName))

		logFn(fmt.Sprintf("[%d/%d] Ch. %s", i+1, len(chapters), chNum))

		wg.Add(1)
		go func(idx int, chapter Chapter, chNum, chDir string) {
			defer wg.Done()
			defer func() { <-sem }()

			err := c.DownloadChapterPages(chapter, chDir, pageConc, logFn)
			if err != nil {
				results <- result{idx, err, chNum}
				return
			}
			results <- result{idx, nil, chNum}
		}(i, ch, chNum, chDir)
	}

	// Close results when all downloads finish
	go func() {
		wg.Wait()
		close(results)
	}()

	downloaded := 0
	failed := 0
	for r := range results {
		progressFn(downloaded+failed+1, len(chapters))
		if r.err != nil {
			logFn(fmt.Sprintf("  ✗ Ch. %s failed: %v", r.chNum, r.err))
			failed++
		} else {
			downloaded++
		}
	}

	return downloaded, failed
}

func (c *Client) downloadImage(url, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return nil // already downloaded
	}

	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := c.downloadHTTP.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("http get: %w", err)
			if attempt < maxRetries {
				backoff := time.Duration(attempt*2) * time.Second
				time.Sleep(backoff)
				continue
			}
			return lastErr
		}

		// Read response body early so we can close the connection on retry
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read body: %w", err)
			if attempt < maxRetries {
				backoff := time.Duration(attempt*2) * time.Second
				time.Sleep(backoff)
				continue
			}
			return lastErr
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			if attempt < maxRetries {
				backoff := time.Duration(attempt*2) * time.Second
				time.Sleep(backoff)
				continue
			}
			return lastErr
		}

		out, err := os.Create(dest)
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}

		_, err = out.Write(body)
		out.Close()
		if err != nil {
			lastErr = fmt.Errorf("write file: %w", err)
			os.Remove(dest)
			if attempt < maxRetries {
				backoff := time.Duration(attempt*2) * time.Second
				time.Sleep(backoff)
				continue
			}
			return lastErr
		}

		return nil
	}

	return lastErr
}

// FormatChapterNumber returns the chapter number with padding for sorting
func FormatChapterNumber(ch string) string {
	f, err := strconv.ParseFloat(ch, 64)
	if err != nil {
		return ch
	}
	return strconv.FormatFloat(f, 'f', 1, 64)
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "-",
		"?", "-", "\"", "'", "<", "-", ">", "-",
		"|", "-",
	)
	return strings.TrimSpace(replacer.Replace(name))
}
