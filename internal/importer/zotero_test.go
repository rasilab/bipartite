package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseZotero_ValidEntry(t *testing.T) {
	data := []byte(`{
		"config": {},
		"collections": [],
		"items": [{
			"itemKey": "GU8DNUPB",
			"itemType": "journalArticle",
			"citationKey": "Smith2026",
			"title": "Test Paper",
			"abstractNote": "This is a test abstract",
			"DOI": "10.1234/test",
			"date": "2026-03-15",
			"publicationTitle": "Test Journal",
			"extra": "PMID: 12345\nPMCID: PMC67890",
			"creators": [
				{"firstName": "John", "lastName": "Smith", "creatorType": "author"},
				{"firstName": "Jane", "lastName": "Doe", "creatorType": "author"},
				{"firstName": "Ed", "lastName": "Itor", "creatorType": "editor"}
			],
			"attachments": [
				{"key": "N7ZF7T44", "filename": "main.pdf", "contentType": "application/pdf"},
				{"key": "XY123456", "filename": "supp.pdf", "contentType": "application/pdf"},
				{"key": "AB000000", "filename": "notes.html", "contentType": "text/html"}
			]
		}]
	}`)

	refs, errs := ParseZotero(data)
	if len(errs) > 0 {
		t.Fatalf("ParseZotero() returned errors: %v", errs)
	}
	if len(refs) != 1 {
		t.Fatalf("ParseZotero() returned %d refs, want 1", len(refs))
	}

	ref := refs[0]

	if ref.ID != "Smith2026" {
		t.Errorf("ID = %v, want Smith2026", ref.ID)
	}
	if ref.DOI != "10.1234/test" {
		t.Errorf("DOI = %v, want 10.1234/test", ref.DOI)
	}
	if ref.Title != "Test Paper" {
		t.Errorf("Title = %v, want Test Paper", ref.Title)
	}
	if ref.Abstract != "This is a test abstract" {
		t.Errorf("Abstract = %v, want 'This is a test abstract'", ref.Abstract)
	}
	if ref.Venue != "Test Journal" {
		t.Errorf("Venue = %v, want Test Journal", ref.Venue)
	}

	// Authors: only creatorType == "author", not editor
	if len(ref.Authors) != 2 {
		t.Fatalf("Authors count = %d, want 2", len(ref.Authors))
	}
	if ref.Authors[0].First != "John" || ref.Authors[0].Last != "Smith" {
		t.Errorf("Authors[0] = %+v, want John Smith", ref.Authors[0])
	}

	// Date
	if ref.Published.Year != 2026 {
		t.Errorf("Published.Year = %d, want 2026", ref.Published.Year)
	}
	if ref.Published.Month != 3 {
		t.Errorf("Published.Month = %d, want 3", ref.Published.Month)
	}
	if ref.Published.Day != 15 {
		t.Errorf("Published.Day = %d, want 15", ref.Published.Day)
	}

	// PDF paths
	if ref.PDFPath != "N7ZF7T44/main.pdf" {
		t.Errorf("PDFPath = %v, want N7ZF7T44/main.pdf", ref.PDFPath)
	}
	if len(ref.SupplementPaths) != 1 || ref.SupplementPaths[0] != "XY123456/supp.pdf" {
		t.Errorf("SupplementPaths = %v, want [XY123456/supp.pdf]", ref.SupplementPaths)
	}

	// Source
	if ref.Source.Type != "zotero" {
		t.Errorf("Source.Type = %v, want zotero", ref.Source.Type)
	}
	if ref.Source.ID != "GU8DNUPB" {
		t.Errorf("Source.ID = %v, want GU8DNUPB", ref.Source.ID)
	}

	// External IDs
	if ref.PMID != "12345" {
		t.Errorf("PMID = %v, want 12345", ref.PMID)
	}
	if ref.PMCID != "PMC67890" {
		t.Errorf("PMCID = %v, want PMC67890", ref.PMCID)
	}
}

func TestParseZotero_NoCitationKey(t *testing.T) {
	data := []byte(`{
		"config": {},
		"collections": [],
		"items": [{
			"itemKey": "ABC12345",
			"itemType": "journalArticle",
			"citationKey": "",
			"title": "Test Paper",
			"date": "2026",
			"creators": [{"firstName": "John", "lastName": "Smith", "creatorType": "author"}]
		}]
	}`)

	refs, errs := ParseZotero(data)
	if len(errs) > 0 {
		t.Fatalf("ParseZotero() returned errors: %v", errs)
	}
	if refs[0].ID != "ABC12345" {
		t.Errorf("ID = %v, want ABC12345 (itemKey fallback)", refs[0].ID)
	}
}

func TestParseZotero_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "missing title",
			data: `{"items": [{"itemKey": "A", "itemType": "journalArticle", "date": "2026", "creators": [{"firstName": "J", "lastName": "S", "creatorType": "author"}]}]}`,
		},
		{
			name: "missing authors",
			data: `{"items": [{"itemKey": "A", "itemType": "journalArticle", "title": "T", "date": "2026", "creators": []}]}`,
		},
		{
			name: "missing date",
			data: `{"items": [{"itemKey": "A", "itemType": "journalArticle", "title": "T", "creators": [{"firstName": "J", "lastName": "S", "creatorType": "author"}]}]}`,
		},
		{
			name: "only editors",
			data: `{"items": [{"itemKey": "A", "itemType": "journalArticle", "title": "T", "date": "2026", "creators": [{"firstName": "E", "lastName": "D", "creatorType": "editor"}]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, errs := ParseZotero([]byte(tt.data))
			if len(errs) == 0 {
				t.Errorf("ParseZotero() expected error for %s, got refs: %+v", tt.name, refs)
			}
		})
	}
}

func TestParseZotero_DateFormats(t *testing.T) {
	tests := []struct {
		name  string
		date  string
		year  int
		month int
		day   int
	}{
		{"YYYY-MM-DD", "2026-03-15", 2026, 3, 15},
		{"YYYY-M-D", "1993-1-5", 1993, 1, 5},
		{"YYYY-MM", "2026-03", 2026, 3, 0},
		{"YYYY-M", "1998-7", 1998, 7, 0},
		{"YYYY", "2026", 2026, 0, 0},
		{"MM/DD/YYYY", "02/01/2006", 2006, 2, 1},
		{"MM/YYYY", "06/1970", 1970, 6, 0},
		{"Month DD, YYYY", "January 10, 2020", 2020, 1, 10},
		{"Month YYYY", "March 2019", 2019, 3, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pd, err := parseZoteroDate(tt.date)
			if err != nil {
				t.Fatalf("parseZoteroDate(%q) error: %v", tt.date, err)
			}
			if pd.Year != tt.year {
				t.Errorf("Year = %d, want %d", pd.Year, tt.year)
			}
			if pd.Month != tt.month {
				t.Errorf("Month = %d, want %d", pd.Month, tt.month)
			}
			if pd.Day != tt.day {
				t.Errorf("Day = %d, want %d", pd.Day, tt.day)
			}
		})
	}
}

func TestParseZotero_ExtraFieldParsing(t *testing.T) {
	extra := "PMID: 29785012\nPMCID: PMC6181777\narXiv: 1906.07748\nSome other note"
	pmid, pmcid, arxiv := parseZoteroExtra(extra)

	if pmid != "29785012" {
		t.Errorf("PMID = %v, want 29785012", pmid)
	}
	if pmcid != "PMC6181777" {
		t.Errorf("PMCID = %v, want PMC6181777", pmcid)
	}
	if arxiv != "1906.07748" {
		t.Errorf("ArXiv = %v, want 1906.07748", arxiv)
	}
}

func TestParseZotero_ExtraFieldEmpty(t *testing.T) {
	pmid, pmcid, arxiv := parseZoteroExtra("")
	if pmid != "" || pmcid != "" || arxiv != "" {
		t.Errorf("Expected empty results, got pmid=%q pmcid=%q arxiv=%q", pmid, pmcid, arxiv)
	}
}

func TestParseZotero_SkipsNotesAndAttachments(t *testing.T) {
	data := []byte(`{
		"items": [
			{"itemKey": "A", "itemType": "note", "title": "My notes"},
			{"itemKey": "B", "itemType": "attachment", "title": "Some file"},
			{"itemKey": "C", "itemType": "journalArticle", "citationKey": "Valid2026", "title": "Real Paper", "date": "2026", "creators": [{"firstName": "J", "lastName": "S", "creatorType": "author"}]}
		]
	}`)

	refs, errs := ParseZotero(data)
	if len(errs) > 0 {
		t.Fatalf("ParseZotero() returned errors: %v", errs)
	}
	if len(refs) != 1 {
		t.Fatalf("ParseZotero() returned %d refs, want 1", len(refs))
	}
	if refs[0].ID != "Valid2026" {
		t.Errorf("ID = %v, want Valid2026", refs[0].ID)
	}
}

func TestParseZotero_CorporateAuthor(t *testing.T) {
	data := []byte(`{
		"items": [{
			"itemKey": "A",
			"itemType": "journalArticle",
			"title": "Report",
			"date": "2026",
			"creators": [{"name": "World Health Organization", "creatorType": "author"}]
		}]
	}`)

	refs, errs := ParseZotero(data)
	if len(errs) > 0 {
		t.Fatalf("ParseZotero() returned errors: %v", errs)
	}
	if refs[0].Authors[0].Last != "World Health Organization" {
		t.Errorf("Authors[0].Last = %v, want 'World Health Organization'", refs[0].Authors[0].Last)
	}
}

func TestParseZotero_BookSectionVenue(t *testing.T) {
	data := []byte(`{
		"items": [{
			"itemKey": "A",
			"itemType": "bookSection",
			"title": "Chapter One",
			"date": "2026",
			"bookTitle": "The Big Book",
			"creators": [{"firstName": "A", "lastName": "B", "creatorType": "author"}]
		}]
	}`)

	refs, errs := ParseZotero(data)
	if len(errs) > 0 {
		t.Fatalf("ParseZotero() returned errors: %v", errs)
	}
	if refs[0].Venue != "The Big Book" {
		t.Errorf("Venue = %v, want 'The Big Book'", refs[0].Venue)
	}
}

func TestParseZotero_InvalidJSON(t *testing.T) {
	data := []byte(`not valid json`)
	refs, errs := ParseZotero(data)
	if len(errs) == 0 {
		t.Errorf("ParseZotero() expected error for invalid JSON, got refs: %+v", refs)
	}
}

func TestParseZotero_EmptyItems(t *testing.T) {
	data := []byte(`{"items": []}`)
	refs, errs := ParseZotero(data)
	if len(errs) > 0 {
		t.Fatalf("ParseZotero() returned errors: %v", errs)
	}
	if len(refs) != 0 {
		t.Errorf("ParseZotero() returned %d refs, want 0", len(refs))
	}
}

func TestParseZotero_PartialErrors(t *testing.T) {
	data := []byte(`{
		"items": [
			{"itemKey": "A", "itemType": "journalArticle", "citationKey": "Valid2026", "title": "Valid", "date": "2026", "creators": [{"lastName": "V", "creatorType": "author"}]},
			{"itemKey": "B", "itemType": "journalArticle", "title": "", "date": "2026", "creators": [{"lastName": "I", "creatorType": "author"}]},
			{"itemKey": "C", "itemType": "journalArticle", "citationKey": "AlsoValid2025", "title": "Also Valid", "date": "2025", "creators": [{"lastName": "A", "creatorType": "author"}]}
		]
	}`)

	refs, errs := ParseZotero(data)
	if len(refs) != 2 {
		t.Errorf("ParseZotero() returned %d valid refs, want 2", len(refs))
	}
	if len(errs) != 1 {
		t.Errorf("ParseZotero() returned %d errors, want 1", len(errs))
	}
}

func TestParseZotero_RealTestData(t *testing.T) {
	testFile := filepath.Join("..", "..", "testdata", "zotero_sample.json")
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Skipf("Test data file not found: %v", err)
	}

	refs, errs := ParseZotero(data)
	if len(errs) > 0 {
		t.Errorf("ParseZotero() returned %d errors parsing real test data: %v", len(errs), errs)
	}
	// Sample has 3 items: 1 journalArticle, 1 conferencePaper, 1 note (skipped)
	if len(refs) != 2 {
		t.Errorf("ParseZotero() returned %d refs, want 2", len(refs))
	}

	if len(refs) > 0 {
		ref := refs[0]
		if ref.ID == "" {
			t.Error("First ref has empty ID")
		}
		if ref.Title == "" {
			t.Error("First ref has empty Title")
		}
		if ref.Source.Type != "zotero" {
			t.Errorf("First ref Source.Type = %s, want zotero", ref.Source.Type)
		}
		if ref.PMID != "29785012" {
			t.Errorf("First ref PMID = %s, want 29785012", ref.PMID)
		}
	}
}
