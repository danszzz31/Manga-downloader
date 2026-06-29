package mangadex

import "testing"

func TestNormalizeChaptersDedupesAndSortsNumerically(t *testing.T) {
	chapters := []Chapter{
		testChapter("ch-2-old", "2", 18, "2026-01-01T00:00:00+00:00"),
		testChapter("ch-1-old", "1", 20, "2026-01-01T00:00:00+00:00"),
		testChapter("ch-10", "10", 24, "2026-01-01T00:00:00+00:00"),
		testChapter("ch-1-better", "1.0", 22, "2026-01-02T00:00:00+00:00"),
		testChapter("ch-3", "3", 19, "2026-01-01T00:00:00+00:00"),
		testChapter("ch-2-better", "2", 21, "2026-01-02T00:00:00+00:00"),
	}

	got := normalizeChapters(chapters)
	gotIDs := make([]string, 0, len(got))
	for _, ch := range got {
		gotIDs = append(gotIDs, ch.ID)
	}

	wantIDs := []string{"ch-1-better", "ch-2-better", "ch-3", "ch-10"}
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("got %d chapters %v, want %d %v", len(gotIDs), gotIDs, len(wantIDs), wantIDs)
	}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("got ids %v, want %v", gotIDs, wantIDs)
		}
	}
}

func testChapter(id, chapter string, pages int, publishAt string) Chapter {
	return Chapter{
		ID: id,
		Attributes: ChapterAttrs{
			Chapter:            chapter,
			TranslatedLanguage: "en",
			Pages:              pages,
			PublishAt:          publishAt,
		},
	}
}
