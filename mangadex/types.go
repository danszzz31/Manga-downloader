package mangadex

// --- MangaDex API Response types ---

// MangaList is the response from GET /manga
type MangaList struct {
	Result string   `json:"result"`
	Data   []Manga  `json:"data"`
}

// Manga represents a manga resource
type Manga struct {
	ID         string       `json:"id"`
	Type       string       `json:"type"`
	Attributes MangaAttrs   `json:"attributes"`
	Relations  []Relation   `json:"relationships"`
}

type MangaAttrs struct {
	Title       map[string]string `json:"title"`
	AltTitles   []map[string]string `json:"altTitles"`
	Description map[string]string `json:"description"`
}

type Relation struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// ChapterList is the response from GET /manga/{id}/feed
type ChapterList struct {
	Result string    `json:"result"`
	Data   []Chapter `json:"data"`
}

// Chapter represents a chapter resource
type Chapter struct {
	ID         string       `json:"id"`
	Type       string       `json:"type"`
	Attributes ChapterAttrs `json:"attributes"`
}

type ChapterAttrs struct {
	Chapter        string            `json:"chapter"`
	Volume         *string           `json:"volume"`
	Title          string            `json:"title"`
	TranslatedLanguage string        `json:"translatedLanguage"`
	Pages          int               `json:"pages"`
	PublishAt      string            `json:"publishAt"`
	ExternalURL    *string           `json:"externalUrl"`
}

// AtHomeResponse is the response from GET /at-home/server/{chapterID}
type AtHomeResponse struct {
	Result  string          `json:"result"`
	BaseURL string          `json:"baseUrl"`
	Chapter AtHomeChapter   `json:"chapter"`
}

type AtHomeChapter struct {
	Hash     string   `json:"hash"`
	Data     []string `json:"data"`
	DataSaver []string `json:"dataSaver"`
}
