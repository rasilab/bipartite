package importer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/matsen/bipartite/internal/reference"
)

// ZoteroExport represents the top-level structure of a Better BibTeX JSON export.
type ZoteroExport struct {
	Config      json.RawMessage `json:"config"`
	Collections json.RawMessage `json:"collections"`
	Items       []ZoteroItem    `json:"items"`
}

// ZoteroItem represents a single item from a Better BibTeX JSON export.
type ZoteroItem struct {
	ItemKey          string             `json:"itemKey"`
	ItemType         string             `json:"itemType"`
	CitationKey      string             `json:"citationKey"`
	Title            string             `json:"title"`
	AbstractNote     string             `json:"abstractNote"`
	DOI              string             `json:"DOI"`
	Date             string             `json:"date"`
	PublicationTitle string             `json:"publicationTitle"`
	BookTitle        string             `json:"bookTitle"`
	ProceedingsTitle string             `json:"proceedingsTitle"`
	Extra            string             `json:"extra"`
	Creators         []ZoteroCreator    `json:"creators"`
	Attachments      []ZoteroAttachment `json:"attachments"`
}

// ZoteroCreator represents an author, editor, or other contributor.
type ZoteroCreator struct {
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	CreatorType string `json:"creatorType"`
	Name        string `json:"name"` // single-field name (e.g., corporate authors)
}

// ZoteroAttachment represents a file attachment on a Zotero item.
type ZoteroAttachment struct {
	Key         string `json:"key"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	LocalPath   string `json:"localPath"`
}

// ParseZotero parses a Better BibTeX JSON export and returns references.
func ParseZotero(data []byte) ([]reference.Reference, []error) {
	var export ZoteroExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, []error{fmt.Errorf("parsing Zotero JSON: %w", err)}
	}

	var refs []reference.Reference
	var errs []error

	for i, item := range export.Items {
		// Skip non-document item types
		if item.ItemType == "attachment" || item.ItemType == "note" {
			continue
		}

		ref, err := zoteroItemToReference(item)
		if err != nil {
			id := item.CitationKey
			if id == "" {
				id = item.ItemKey
			}
			errs = append(errs, fmt.Errorf("entry %d (%s): %w", i+1, id, err))
			continue
		}
		refs = append(refs, ref)
	}

	return refs, errs
}

// zoteroItemToReference converts a Zotero item to our Reference type.
func zoteroItemToReference(item ZoteroItem) (reference.Reference, error) {
	if item.Title == "" {
		return reference.Reference{}, fmt.Errorf("missing required field 'title'")
	}

	// Extract authors (only creatorType == "author")
	var authors []reference.Author
	for _, c := range item.Creators {
		if c.CreatorType != "author" {
			continue
		}
		a := reference.Author{
			First: c.FirstName,
			Last:  c.LastName,
		}
		// Handle single-field names (corporate authors)
		if a.First == "" && a.Last == "" && c.Name != "" {
			a.Last = c.Name
		}
		authors = append(authors, a)
	}
	if len(authors) == 0 {
		return reference.Reference{}, fmt.Errorf("missing required field 'creators' (no authors)")
	}

	// Parse date
	pubDate, err := parseZoteroDate(item.Date)
	if err != nil {
		return reference.Reference{}, fmt.Errorf("invalid date %q: %w", item.Date, err)
	}

	// Extract PDFs from attachments
	var pdfPath string
	var supplementPaths []string
	for _, att := range item.Attachments {
		if att.ContentType != "application/pdf" {
			continue
		}
		path := att.Key + "/" + att.Filename
		if pdfPath == "" {
			pdfPath = path
		} else {
			supplementPaths = append(supplementPaths, path)
		}
	}

	// ID: prefer citationKey, fall back to itemKey
	id := item.CitationKey
	if id == "" {
		id = item.ItemKey
	}

	// Venue: depends on item type
	venue := item.PublicationTitle
	if venue == "" {
		venue = item.BookTitle
	}
	if venue == "" {
		venue = item.ProceedingsTitle
	}

	// Parse extra field for external IDs
	pmid, pmcid, arxiv := parseZoteroExtra(item.Extra)

	ref := reference.Reference{
		ID:              id,
		DOI:             item.DOI,
		Title:           item.Title,
		Authors:         authors,
		Abstract:        item.AbstractNote,
		Venue:           venue,
		Published:       pubDate,
		PDFPath:         pdfPath,
		SupplementPaths: supplementPaths,
		Source: reference.ImportSource{
			Type: "zotero",
			ID:   item.ItemKey,
		},
		PMID:    pmid,
		PMCID:   pmcid,
		ArXivID: arxiv,
	}

	return ref, nil
}

var (
	reYMD      = regexp.MustCompile(`^(\d{4})-(\d{1,2})-(\d{1,2})$`)
	reYM       = regexp.MustCompile(`^(\d{4})-(\d{1,2})$`)
	reYOnly    = regexp.MustCompile(`^(\d{4})$`)
	reMDY      = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})/(\d{4})$`)
	reMY       = regexp.MustCompile(`^(\d{1,2})/(\d{4})$`)
	rePMID     = regexp.MustCompile(`(?m)^PMID:\s*(\d+)`)
	rePMCID    = regexp.MustCompile(`(?m)^PMCID:\s*(PMC\d+)`)
	reArXiv    = regexp.MustCompile(`(?mi)^arXiv:\s*(\S+)`)
)

// parseZoteroDate parses the various date formats used by Zotero.
func parseZoteroDate(dateStr string) (reference.PublicationDate, error) {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return reference.PublicationDate{}, fmt.Errorf("empty date")
	}

	// Try YYYY-MM-DD or YYYY-M-D
	if m := reYMD.FindStringSubmatch(dateStr); m != nil {
		return buildDate(m[1], m[2], m[3])
	}

	// Try YYYY-MM or YYYY-M
	if m := reYM.FindStringSubmatch(dateStr); m != nil {
		return buildDate(m[1], m[2], "")
	}

	// Try YYYY
	if m := reYOnly.FindStringSubmatch(dateStr); m != nil {
		return buildDate(m[1], "", "")
	}

	// Try MM/DD/YYYY
	if m := reMDY.FindStringSubmatch(dateStr); m != nil {
		return buildDate(m[3], m[1], m[2])
	}

	// Try MM/YYYY
	if m := reMY.FindStringSubmatch(dateStr); m != nil {
		return buildDate(m[2], m[1], "")
	}

	// Try "Month DD, YYYY" (e.g., "January 10, 2020")
	t, err := time.Parse("January 2, 2006", dateStr)
	if err == nil {
		return reference.PublicationDate{
			Year:  t.Year(),
			Month: int(t.Month()),
			Day:   t.Day(),
		}, nil
	}

	// Try "Month YYYY" (e.g., "January 2020")
	t, err = time.Parse("January 2006", dateStr)
	if err == nil {
		return reference.PublicationDate{
			Year:  t.Year(),
			Month: int(t.Month()),
		}, nil
	}

	return reference.PublicationDate{}, fmt.Errorf("unrecognized format")
}

// buildDate constructs a PublicationDate from string components.
func buildDate(yearStr, monthStr, dayStr string) (reference.PublicationDate, error) {
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return reference.PublicationDate{}, fmt.Errorf("invalid year: %s", yearStr)
	}

	pd := reference.PublicationDate{Year: year}

	if monthStr != "" {
		month, err := strconv.Atoi(monthStr)
		if err == nil && month >= 1 && month <= 12 {
			pd.Month = month
		}
	}

	if dayStr != "" {
		day, err := strconv.Atoi(dayStr)
		if err == nil && day >= 1 && day <= 31 {
			pd.Day = day
		}
	}

	return pd, nil
}

// parseZoteroExtra extracts PMID, PMCID, and ArXiv ID from the free-text extra field.
func parseZoteroExtra(extra string) (pmid, pmcid, arxiv string) {
	if m := rePMID.FindStringSubmatch(extra); m != nil {
		pmid = m[1]
	}
	if m := rePMCID.FindStringSubmatch(extra); m != nil {
		pmcid = m[1]
	}
	if m := reArXiv.FindStringSubmatch(extra); m != nil {
		arxiv = m[1]
	}
	return
}
