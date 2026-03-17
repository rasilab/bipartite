package pubmed

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/matsen/bipartite/internal/reference"
)

// MapPubmedToReference converts a PubmedArticle to a Reference.
func MapPubmedToReference(article PubmedArticle) reference.Reference {
	pmid := article.MedlineCitation.PMID.Value
	doi, pmcid := extractArticleIDs(article.PubmedData.ArticleIDList)

	ref := reference.Reference{
		ID:       generateCiteKey(article),
		DOI:      doi,
		Title:    article.MedlineCitation.Article.Title,
		Abstract: buildAbstract(article.MedlineCitation.Article.Abstract),
		Venue:    article.MedlineCitation.Article.Journal.ISOAbbreviation,
		Authors:  mapAuthors(article.MedlineCitation.Article.AuthorList),
		Source: reference.ImportSource{
			Type: "pubmed",
			ID:   pmid,
		},
		PMID:  pmid,
		PMCID: pmcid,
	}

	ref.Published = parsePublicationDate(article)

	return ref
}

// extractArticleIDs extracts DOI and PMCID from the article ID list.
func extractArticleIDs(idList ArticleIDList) (doi, pmcid string) {
	for _, aid := range idList.ArticleIDs {
		switch aid.IDType {
		case "doi":
			doi = aid.Value
		case "pmc":
			pmcid = aid.Value
		}
	}
	return doi, pmcid
}

// buildAbstract concatenates abstract segments into a single string.
func buildAbstract(abstract AbstractText) string {
	if len(abstract.Texts) == 0 {
		return ""
	}
	if len(abstract.Texts) == 1 {
		return strings.TrimSpace(abstract.Texts[0].Text)
	}
	var parts []string
	for _, seg := range abstract.Texts {
		text := strings.TrimSpace(seg.Text)
		if text == "" {
			continue
		}
		if seg.Label != "" {
			parts = append(parts, seg.Label+": "+text)
		} else {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

// mapAuthors converts PubMed authors to Reference authors.
func mapAuthors(authorList AuthorList) []reference.Author {
	authors := make([]reference.Author, 0, len(authorList.Authors))
	for _, a := range authorList.Authors {
		if a.CollectiveName != "" {
			authors = append(authors, reference.Author{Last: a.CollectiveName})
			continue
		}
		authors = append(authors, reference.Author{
			First: a.ForeName,
			Last:  a.LastName,
		})
	}
	return authors
}

// parsePublicationDate extracts a publication date from the article.
// Prefers the electronic publication date, then falls back to journal issue date.
func parsePublicationDate(article PubmedArticle) reference.PublicationDate {
	// Try electronic publication date first
	for _, d := range article.MedlineCitation.Article.ArticleDate {
		if d.DateType == "Electronic" {
			pub := parseDateFields(d.Year, d.Month, d.Day)
			if pub.Year > 0 {
				return pub
			}
		}
	}

	// Fall back to journal issue pub date
	pd := article.MedlineCitation.Article.Journal.JournalIssue.PubDate
	return parseDateFields(pd.Year, pd.Month, pd.Day)
}

// parseDateFields converts string year/month/day to a PublicationDate.
func parseDateFields(yearStr, monthStr, dayStr string) reference.PublicationDate {
	pub := reference.PublicationDate{}
	if y, err := strconv.Atoi(yearStr); err == nil {
		pub.Year = y
	}
	pub.Month = parseMonth(monthStr)
	if d, err := strconv.Atoi(dayStr); err == nil && d >= 1 && d <= 31 {
		pub.Day = d
	}
	return pub
}

// monthMap maps month abbreviations and names to month numbers.
var monthMap = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
	"january": 1, "february": 2, "march": 3, "april": 4,
	"june": 6, "july": 7, "august": 8, "september": 9,
	"october": 10, "november": 11, "december": 12,
}

// parseMonth parses a month string (numeric, abbreviation, or full name).
func parseMonth(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Try numeric
	if m, err := strconv.Atoi(s); err == nil && m >= 1 && m <= 12 {
		return m
	}
	// Try name/abbreviation
	if m, ok := monthMap[strings.ToLower(s)]; ok {
		return m
	}
	return 0
}

// generateCiteKey generates a citation key from PubMed article metadata.
// Format: LastName + Year + suffix (e.g., "Zhang2018-vi")
func generateCiteKey(article PubmedArticle) string {
	lastName := "Unknown"
	if len(article.MedlineCitation.Article.AuthorList.Authors) > 0 {
		a := article.MedlineCitation.Article.AuthorList.Authors[0]
		if a.LastName != "" {
			lastName = sanitizeForCiteKey(a.LastName)
		} else if a.CollectiveName != "" {
			lastName = sanitizeForCiteKey(a.CollectiveName)
		}
	}

	year := 0
	pub := parsePublicationDate(article)
	year = pub.Year
	if year == 0 {
		year = 9999
	}

	suffix := generateTitleSuffix(article.MedlineCitation.Article.Title)

	return fmt.Sprintf("%s%d-%s", lastName, year, suffix)
}

// sanitizeForCiteKey removes non-alphanumeric characters.
func sanitizeForCiteKey(s string) string {
	var result strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// generateTitleSuffix creates a 2-letter suffix from the title.
func generateTitleSuffix(title string) string {
	words := strings.Fields(strings.ToLower(title))
	stopWords := map[string]bool{"a": true, "an": true, "the": true, "of": true, "and": true, "in": true, "on": true, "for": true, "to": true, "with": true}

	var suffix strings.Builder
	for _, word := range words {
		if !stopWords[word] && len(word) > 0 {
			suffix.WriteByte(word[0])
			if suffix.Len() >= 2 {
				break
			}
		}
	}

	for suffix.Len() < 2 {
		suffix.WriteByte('x')
	}

	return suffix.String()
}
