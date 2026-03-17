package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/matsen/bipartite/internal/config"
	"github.com/matsen/bipartite/internal/pubmed"
	"github.com/matsen/bipartite/internal/reference"
	"github.com/matsen/bipartite/internal/storage"
	"github.com/spf13/cobra"
)

var (
	pubmedLookupFields string
	pubmedLookupExists bool
)

var pubmedLookupCmd = &cobra.Command{
	Use:   "lookup <paper-id>",
	Short: "Query PubMed for paper information without adding",
	Long: `Query PubMed for paper information without adding to collection.

Useful for checking paper details before adding.

Supported paper ID formats:
  PMID:19872477                PubMed ID
  19872477                     Bare numeric PMID
  DOI:10.1038/nature12373      DOI (resolved via PubMed)

Examples:
  bip pubmed lookup PMID:19872477
  bip pubmed lookup DOI:10.1038/nature12373 --human
  bip pubmed lookup 19872477 --fields title,authors --human`,
	Args: cobra.ExactArgs(1),
	RunE: runPubmedLookup,
}

func init() {
	pubmedCmd.AddCommand(pubmedLookupCmd)
	pubmedLookupCmd.Flags().StringVarP(&pubmedLookupFields, "fields", "f", "", "Comma-separated fields to return (default: all)")
	pubmedLookupCmd.Flags().BoolVarP(&pubmedLookupExists, "exists", "e", false, "Include whether paper exists in local collection")
}

// PubmedLookupResult is the JSON output for the lookup command.
type PubmedLookupResult struct {
	PMID          string               `json:"pmid"`
	DOI           string               `json:"doi,omitempty"`
	PMCID         string               `json:"pmcid,omitempty"`
	Title         string               `json:"title"`
	Authors       []PubmedLookupAuthor `json:"authors,omitempty"`
	Abstract      string               `json:"abstract,omitempty"`
	Year          int                  `json:"year,omitempty"`
	Venue         string               `json:"venue,omitempty"`
	ExistsLocally *bool                `json:"existsLocally,omitempty"`
	LocalID       string               `json:"localId,omitempty"`
	Error         *S2ErrorResult       `json:"error,omitempty"`
}

// PubmedLookupAuthor represents an author in lookup results.
type PubmedLookupAuthor struct {
	First string `json:"first,omitempty"`
	Last  string `json:"last"`
}

func runPubmedLookup(cmd *cobra.Command, args []string) error {
	paperID := args[0]
	ctx := context.Background()

	client := pubmed.NewClient()

	// Parse and resolve to PMID
	parsed := pubmed.ParsePaperID(paperID)
	var pmid string

	switch parsed.Type {
	case "PMID":
		pmid = parsed.Value
	case "DOI":
		resolved, err := client.ResolveDOI(ctx, parsed.Value)
		if err != nil {
			if pubmed.IsNotFound(err) {
				return outputGenericNotFound(paperID, "DOI not found in PubMed")
			}
			return outputGenericError(ExitPubMedAPIError, "api_error", "resolving DOI", err)
		}
		pmid = resolved
	case "PMCID":
		result, err := client.Search(ctx, parsed.Value+"[pmcid]", 1)
		if err != nil {
			return outputGenericError(ExitPubMedAPIError, "api_error", "resolving PMCID", err)
		}
		if len(result.IDList) == 0 {
			return outputGenericNotFound(paperID, "PMCID not found in PubMed")
		}
		pmid = result.IDList[0]
	default:
		return outputGenericNotFound(paperID, "Unrecognized identifier format for PubMed")
	}

	// Fetch paper
	article, err := client.GetPaper(ctx, pmid)
	if err != nil {
		if pubmed.IsNotFound(err) {
			return outputGenericNotFound(paperID, "Paper not found in PubMed")
		}
		if pubmed.IsRateLimited(err) {
			return outputPubmedRateLimited(err)
		}
		return outputGenericError(ExitPubMedAPIError, "api_error", "fetching paper", err)
	}

	// Map to reference for easy field extraction
	ref := pubmed.MapPubmedToReference(*article)

	result := PubmedLookupResult{
		PMID:     ref.PMID,
		DOI:      ref.DOI,
		PMCID:    ref.PMCID,
		Title:    ref.Title,
		Abstract: ref.Abstract,
		Year:     ref.Published.Year,
		Venue:    ref.Venue,
	}

	for _, a := range ref.Authors {
		result.Authors = append(result.Authors, PubmedLookupAuthor{
			First: a.First,
			Last:  a.Last,
		})
	}

	// Check local existence if requested
	if pubmedLookupExists {
		exists, localID := checkLocalExistence(ref)
		result.ExistsLocally = &exists
		result.LocalID = localID
	}

	// Filter fields if requested
	if pubmedLookupFields != "" {
		result = filterPubmedLookupFields(result, pubmedLookupFields)
	}

	if humanOutput {
		outputPubmedLookupHuman(result)
	} else {
		outputJSON(result)
	}
	return nil
}

func checkLocalExistence(ref reference.Reference) (bool, string) {
	repoRoot := mustFindRepository()
	refsPath := config.RefsPath(repoRoot)
	refs, err := storage.ReadAll(refsPath)
	if err != nil {
		return false, ""
	}

	// Check by DOI
	if ref.DOI != "" {
		for _, existing := range refs {
			if existing.DOI != "" && normalizeDOI(existing.DOI) == normalizeDOI(ref.DOI) {
				return true, existing.ID
			}
		}
	}

	// Check by PMID
	if ref.PMID != "" {
		for _, existing := range refs {
			if existing.PMID != "" && existing.PMID == ref.PMID {
				return true, existing.ID
			}
		}
	}

	return false, ""
}

func filterPubmedLookupFields(result PubmedLookupResult, fieldsStr string) PubmedLookupResult {
	fields := make(map[string]bool)
	for _, f := range strings.Split(fieldsStr, ",") {
		fields[strings.TrimSpace(strings.ToLower(f))] = true
	}

	filtered := PubmedLookupResult{
		PMID: result.PMID, // Always include
	}

	if fields["doi"] {
		filtered.DOI = result.DOI
	}
	if fields["pmcid"] {
		filtered.PMCID = result.PMCID
	}
	if fields["title"] {
		filtered.Title = result.Title
	}
	if fields["authors"] {
		filtered.Authors = result.Authors
	}
	if fields["abstract"] {
		filtered.Abstract = result.Abstract
	}
	if fields["year"] {
		filtered.Year = result.Year
	}
	if fields["venue"] {
		filtered.Venue = result.Venue
	}
	if fields["existslocally"] || result.ExistsLocally != nil {
		filtered.ExistsLocally = result.ExistsLocally
		filtered.LocalID = result.LocalID
	}

	return filtered
}

func outputPubmedLookupHuman(result PubmedLookupResult) {
	fmt.Printf("%s\n", result.Title)
	if len(result.Authors) > 0 {
		names := make([]string, 0, len(result.Authors))
		for _, a := range result.Authors {
			if a.First != "" {
				names = append(names, a.First+" "+a.Last)
			} else {
				names = append(names, a.Last)
			}
		}
		fmt.Printf("  Authors: %s\n", strings.Join(names, ", "))
	}
	if result.Year > 0 {
		fmt.Printf("  Year: %d\n", result.Year)
	}
	if result.Venue != "" {
		fmt.Printf("  Venue: %s\n", result.Venue)
	}
	if result.PMID != "" {
		fmt.Printf("  PMID: %s\n", result.PMID)
	}
	if result.DOI != "" {
		fmt.Printf("  DOI: %s\n", result.DOI)
	}
	if result.PMCID != "" {
		fmt.Printf("  PMCID: %s\n", result.PMCID)
	}
	if result.ExistsLocally != nil {
		if *result.ExistsLocally {
			fmt.Printf("  In collection: Yes (%s)\n", result.LocalID)
		} else {
			fmt.Printf("  In collection: No\n")
		}
	}
}
