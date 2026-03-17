// Package pubmed provides a client for the NCBI PubMed E-utilities API.
package pubmed

import "encoding/xml"

// XML types for efetch responses.

// PubmedArticleSet is the root element of an efetch XML response.
type PubmedArticleSet struct {
	XMLName  xml.Name        `xml:"PubmedArticleSet"`
	Articles []PubmedArticle `xml:"PubmedArticle"`
}

// PubmedArticle represents a single article in the efetch response.
type PubmedArticle struct {
	MedlineCitation MedlineCitation `xml:"MedlineCitation"`
	PubmedData      PubmedData      `xml:"PubmedData"`
}

// MedlineCitation contains the main article metadata.
type MedlineCitation struct {
	PMID    PMID    `xml:"PMID"`
	Article Article `xml:"Article"`
}

// PMID is a PubMed identifier element.
type PMID struct {
	Value string `xml:",chardata"`
}

// Article contains article-level metadata.
type Article struct {
	Journal     Journal       `xml:"Journal"`
	Title       string        `xml:"ArticleTitle"`
	Abstract    AbstractText  `xml:"Abstract"`
	AuthorList  AuthorList    `xml:"AuthorList"`
	Language    []string      `xml:"Language"`
	ArticleDate []ArticleDate `xml:"ArticleDate"`
}

// AbstractText contains one or more abstract text segments.
type AbstractText struct {
	Texts []AbstractSegment `xml:"AbstractText"`
}

// AbstractSegment is a single segment of an abstract (may have a label).
type AbstractSegment struct {
	Label string `xml:"Label,attr"`
	Text  string `xml:",chardata"`
}

// Journal contains journal-level metadata.
type Journal struct {
	ISOAbbreviation string       `xml:"ISOAbbreviation"`
	Title           string       `xml:"Title"`
	JournalIssue    JournalIssue `xml:"JournalIssue"`
}

// JournalIssue contains issue-level metadata including publication date.
type JournalIssue struct {
	PubDate PubDate `xml:"PubDate"`
}

// PubDate represents a publication date.
type PubDate struct {
	Year        string `xml:"Year"`
	Month       string `xml:"Month"`
	Day         string `xml:"Day"`
	MedlineDate string `xml:"MedlineDate"`
}

// ArticleDate represents an electronic publication date.
type ArticleDate struct {
	DateType string `xml:"DateType,attr"`
	Year     string `xml:"Year"`
	Month    string `xml:"Month"`
	Day      string `xml:"Day"`
}

// AuthorList contains the list of authors.
type AuthorList struct {
	Authors []PubmedAuthor `xml:"Author"`
}

// PubmedAuthor represents an author with structured name fields.
type PubmedAuthor struct {
	LastName       string `xml:"LastName"`
	ForeName       string `xml:"ForeName"`
	Initials       string `xml:"Initials"`
	CollectiveName string `xml:"CollectiveName"`
}

// PubmedData contains additional data like article IDs.
type PubmedData struct {
	ArticleIDList ArticleIDList `xml:"ArticleIdList"`
}

// ArticleIDList contains various article identifiers.
type ArticleIDList struct {
	ArticleIDs []ArticleID `xml:"ArticleId"`
}

// ArticleID is a single article identifier with a type attribute.
type ArticleID struct {
	IDType string `xml:"IdType,attr"`
	Value  string `xml:",chardata"`
}

// JSON types for esearch responses.

// ESearchResult is the response from the esearch endpoint.
type ESearchResult struct {
	Header ESearchHeader `json:"header"`
	Result ESearchBody   `json:"esearchresult"`
}

// ESearchHeader contains response metadata.
type ESearchHeader struct {
	Type    string `json:"type"`
	Version string `json:"version"`
}

// ESearchBody contains the search results.
type ESearchBody struct {
	Count    string   `json:"count"`
	RetMax   string   `json:"retmax"`
	RetStart string   `json:"retstart"`
	IDList   []string `json:"idlist"`
}

// JSON types for esummary responses.

// ESummaryResult is the response from the esummary endpoint.
type ESummaryResult struct {
	Result map[string]interface{} `json:"result"`
}

// DocSummary represents a document summary from esummary.
type DocSummary struct {
	UID        string                `json:"uid"`
	PubDate    string                `json:"pubdate"`
	Source     string                `json:"source"`
	Title      string                `json:"title"`
	Authors    []DocSummaryAuthor    `json:"authors"`
	Volume     string                `json:"volume"`
	Issue      string                `json:"issue"`
	Pages      string                `json:"pages"`
	DOI        string                `json:"elocationid"`
	ArticleIDs []DocSummaryArticleID `json:"articleids"`
}

// DocSummaryAuthor is an author in a document summary.
type DocSummaryAuthor struct {
	Name   string `json:"name"`
	AuthID string `json:"authtype"`
}

// DocSummaryArticleID is an article ID in a document summary.
type DocSummaryArticleID struct {
	IDType string `json:"idtype"`
	Value  string `json:"value"`
}

// PaperIdentifier represents a parsed paper identifier for PubMed.
type PaperIdentifier struct {
	Type  string // PMID, DOI, PMCID, LOCAL
	Value string
}
