package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/matsen/bipartite/internal/config"
	"github.com/matsen/bipartite/internal/pubmed"
	"github.com/matsen/bipartite/internal/reference"
	"github.com/matsen/bipartite/internal/storage"
	"github.com/spf13/cobra"
)

var (
	pubmedAddUpdate bool
	pubmedAddLink   string
)

var pubmedAddCmd = &cobra.Command{
	Use:   "add <paper-id>",
	Short: "Add a paper by fetching metadata from PubMed",
	Long: `Add a paper to the collection by fetching its metadata from PubMed.

Supported paper ID formats:
  PMID:19872477                PubMed ID
  19872477                     Bare numeric PMID
  DOI:10.1038/nature12373      DOI (resolved via PubMed)

Examples:
  bip pubmed add PMID:19872477
  bip pubmed add 19872477 --link ~/papers/paper.pdf
  bip pubmed add DOI:10.1038/nature12373 --human`,
	Args: cobra.ExactArgs(1),
	RunE: runPubmedAdd,
}

func init() {
	pubmedCmd.AddCommand(pubmedAddCmd)
	pubmedAddCmd.Flags().BoolVarP(&pubmedAddUpdate, "update", "u", false, "Update metadata if paper already exists")
	pubmedAddCmd.Flags().StringVarP(&pubmedAddLink, "link", "l", "", "Set pdf_path to the given file path")
}

// PubmedAddResult is the JSON output for the add command.
type PubmedAddResult struct {
	Action string                 `json:"action"` // added, updated, skipped
	Paper  *PubmedAddPaperSummary `json:"paper,omitempty"`
	Error  *S2ErrorResult         `json:"error,omitempty"`
}

// PubmedAddPaperSummary is a summary of the added paper.
type PubmedAddPaperSummary struct {
	ID      string   `json:"id"`
	DOI     string   `json:"doi,omitempty"`
	PMID    string   `json:"pmid"`
	Title   string   `json:"title"`
	Authors []string `json:"authors"`
	Year    int      `json:"year"`
	Venue   string   `json:"venue,omitempty"`
}

func runPubmedAdd(cmd *cobra.Command, args []string) error {
	paperID := args[0]
	ctx := context.Background()

	// Find repository
	repoRoot := mustFindRepository()
	refsPath := config.RefsPath(repoRoot)

	// Load existing refs
	refs, err := storage.ReadAll(refsPath)
	if err != nil {
		return outputPubmedError(ExitPubMedAPIError, "reading refs", err)
	}

	// Create client
	client := pubmed.NewClient()

	// Parse paper ID and resolve to PMID
	parsed := pubmed.ParsePaperID(paperID)
	var pmid string

	switch parsed.Type {
	case "PMID":
		pmid = parsed.Value
	case "DOI":
		resolved, err := client.ResolveDOI(ctx, parsed.Value)
		if err != nil {
			if pubmed.IsNotFound(err) {
				return outputPubmedNotFound(paperID, "DOI not found in PubMed")
			}
			return outputPubmedError(ExitPubMedAPIError, "resolving DOI", err)
		}
		pmid = resolved
	case "PMCID":
		// Search for PMCID to find PMID
		result, err := client.Search(ctx, parsed.Value+"[pmcid]", 1)
		if err != nil {
			return outputPubmedError(ExitPubMedAPIError, "resolving PMCID", err)
		}
		if len(result.IDList) == 0 {
			return outputPubmedNotFound(paperID, "PMCID not found in PubMed")
		}
		pmid = result.IDList[0]
	default:
		return outputPubmedNotFound(paperID, "Unrecognized identifier format for PubMed")
	}

	// Fetch paper from PubMed
	article, err := client.GetPaper(ctx, pmid)
	if err != nil {
		if pubmed.IsNotFound(err) {
			return outputPubmedNotFound(paperID, "Paper not found in PubMed")
		}
		if pubmed.IsRateLimited(err) {
			return outputPubmedRateLimited(err)
		}
		return outputPubmedError(ExitPubMedAPIError, "fetching paper", err)
	}

	// Map to reference
	ref := pubmed.MapPubmedToReference(*article)

	// Set PDF path if requested
	if pubmedAddLink != "" {
		ref.PDFPath = pubmedAddLink
	}

	// Check for duplicates by DOI
	if ref.DOI != "" {
		for _, existing := range refs {
			if existing.DOI != "" && normalizeDOI(existing.DOI) == normalizeDOI(ref.DOI) {
				if !pubmedAddUpdate {
					return outputPubmedDuplicate(existing.ID, ref.DOI)
				}
				return updateExistingPubmedPaper(refsPath, refs, existing.ID, ref)
			}
		}
	}

	// Check for duplicates by PMID
	for _, existing := range refs {
		if existing.PMID != "" && existing.PMID == ref.PMID {
			if !pubmedAddUpdate {
				return outputPubmedDuplicate(existing.ID, "")
			}
			return updateExistingPubmedPaper(refsPath, refs, existing.ID, ref)
		}
	}

	// Generate unique ID
	ref.ID = storage.GenerateUniqueID(refs, ref.ID)

	// Append to refs
	if err := storage.Append(refsPath, ref); err != nil {
		return outputPubmedError(ExitPubMedAPIError, "saving reference", err)
	}

	return outputPubmedAddResult("added", ref)
}

// normalizeDOI normalizes a DOI for comparison.
func normalizeDOI(doi string) string {
	return strings.ToLower(strings.TrimSpace(doi))
}

func updateExistingPubmedPaper(refsPath string, refs []reference.Reference, existingID string, newRef reference.Reference) error {
	for i, ref := range refs {
		if ref.ID == existingID {
			newRef.ID = existingID
			if pubmedAddLink == "" && ref.PDFPath != "" {
				newRef.PDFPath = ref.PDFPath
			}
			refs[i] = newRef
			break
		}
	}

	if err := storage.WriteAll(refsPath, refs); err != nil {
		return outputPubmedError(ExitPubMedAPIError, "saving reference", err)
	}

	return outputPubmedAddResult("updated", newRef)
}

func outputPubmedAddResult(action string, ref reference.Reference) error {
	authors := formatAuthors(ref.Authors)

	result := PubmedAddResult{
		Action: action,
		Paper: &PubmedAddPaperSummary{
			ID:      ref.ID,
			DOI:     ref.DOI,
			PMID:    ref.PMID,
			Title:   ref.Title,
			Authors: authors,
			Year:    ref.Published.Year,
			Venue:   ref.Venue,
		},
	}

	if humanOutput {
		fmt.Printf("%s: %s\n", capitalizeFirst(action), ref.ID)
		fmt.Printf("  Title: %s\n", ref.Title)
		fmt.Printf("  Authors: %s\n", joinAuthorsDisplay(authors))
		fmt.Printf("  Year: %d\n", ref.Published.Year)
		if ref.PMID != "" {
			fmt.Printf("  PMID: %s\n", ref.PMID)
		}
		if ref.Venue != "" {
			fmt.Printf("  Venue: %s\n", ref.Venue)
		}
	} else {
		outputJSON(result)
	}
	return nil
}

func outputPubmedNotFound(paperID, message string) error {
	return outputGenericNotFound(paperID, message)
}

func outputPubmedDuplicate(existingID, doi string) error {
	result := PubmedAddResult{
		Action: "skipped",
		Error: &S2ErrorResult{
			Code:       "duplicate",
			Message:    "Paper already exists in collection",
			PaperID:    existingID,
			Suggestion: "Use --update flag to refresh metadata",
		},
	}

	if humanOutput {
		fmt.Fprintf(os.Stderr, "Paper already exists: %s\n", existingID)
		if doi != "" {
			fmt.Fprintf(os.Stderr, "  DOI: %s\n", doi)
		}
		fmt.Fprintf(os.Stderr, "  Use --update to refresh metadata\n")
	} else {
		outputJSON(result)
	}
	os.Exit(ExitPubMedDuplicate)
	return nil
}

func outputPubmedRateLimited(err error) error {
	apiErr, _ := err.(*pubmed.APIError)
	retryAfter := 60
	if apiErr != nil && apiErr.RetryAfter > 0 {
		retryAfter = apiErr.RetryAfter
	}

	result := GenericErrorResult{
		Error: &S2ErrorResult{
			Code:       "rate_limited",
			Message:    "PubMed rate limit exceeded",
			Suggestion: fmt.Sprintf("Wait %d seconds or add pubmed_api_key to ~/.config/bip/config.yml", retryAfter),
			RetryAfter: retryAfter,
		},
	}

	if humanOutput {
		fmt.Fprintf(os.Stderr, "Error: Rate limit exceeded\n")
		fmt.Fprintf(os.Stderr, "  Wait %d seconds before retrying\n", retryAfter)
	} else {
		outputJSON(result)
	}
	os.Exit(ExitPubMedAPIError)
	return nil
}

func outputPubmedError(exitCode int, context string, err error) error {
	return outputGenericError(exitCode, "api_error", context, err)
}
