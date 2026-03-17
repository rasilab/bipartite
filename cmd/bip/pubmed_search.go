package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/matsen/bipartite/internal/pubmed"
	"github.com/spf13/cobra"
)

var pubmedSearchLimit int

var pubmedSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search PubMed by keyword",
	Long: `Search PubMed by keyword using NCBI E-utilities.

Supports PubMed search syntax including field tags:
  [au]    Author
  [ti]    Title
  [mh]    MeSH terms
  [dp]    Publication date

Examples:
  bip pubmed search "CRISPR Cas9" --human
  bip pubmed search "Subramaniam AR[au]" --limit 10 --human
  bip pubmed search "ribosome profiling[ti]" --human`,
	Args: cobra.ExactArgs(1),
	RunE: runPubmedSearch,
}

func init() {
	pubmedCmd.AddCommand(pubmedSearchCmd)
	pubmedSearchCmd.Flags().IntVarP(&pubmedSearchLimit, "limit", "n", 20, "Maximum number of results")
}

// PubmedSearchResult is the JSON output for the search command.
type PubmedSearchResult struct {
	Query   string                   `json:"query"`
	Count   string                   `json:"count"`
	Results []PubmedSearchResultItem `json:"results"`
}

// PubmedSearchResultItem is a single search result.
type PubmedSearchResultItem struct {
	PMID    string   `json:"pmid"`
	Title   string   `json:"title"`
	Authors []string `json:"authors"`
	Source  string   `json:"source"`
	PubDate string   `json:"pubdate"`
	DOI     string   `json:"doi,omitempty"`
}

func runPubmedSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	ctx := context.Background()

	client := pubmed.NewClient()

	// Search for PMIDs
	searchResult, err := client.Search(ctx, query, pubmedSearchLimit)
	if err != nil {
		if pubmed.IsRateLimited(err) {
			return outputPubmedRateLimited(err)
		}
		return outputGenericError(ExitPubMedAPIError, "api_error", "searching PubMed", err)
	}

	if len(searchResult.IDList) == 0 {
		result := PubmedSearchResult{
			Query:   query,
			Count:   searchResult.Count,
			Results: []PubmedSearchResultItem{},
		}
		if humanOutput {
			fmt.Printf("No results found for: %s\n", query)
		} else {
			outputJSON(result)
		}
		return nil
	}

	// Fetch summaries for the PMIDs
	summaries, err := client.GetSummaries(ctx, searchResult.IDList)
	if err != nil {
		if pubmed.IsRateLimited(err) {
			return outputPubmedRateLimited(err)
		}
		return outputGenericError(ExitPubMedAPIError, "api_error", "fetching summaries", err)
	}

	// Build results in PMID order
	var items []PubmedSearchResultItem
	for _, pmid := range searchResult.IDList {
		doc, ok := summaries[pmid]
		if !ok {
			continue
		}

		var authors []string
		for _, a := range doc.Authors {
			authors = append(authors, a.Name)
		}

		// Extract DOI from article IDs
		doi := ""
		for _, aid := range doc.ArticleIDs {
			if aid.IDType == "doi" {
				doi = aid.Value
				break
			}
		}

		items = append(items, PubmedSearchResultItem{
			PMID:    pmid,
			Title:   doc.Title,
			Authors: authors,
			Source:  doc.Source,
			PubDate: doc.PubDate,
			DOI:     doi,
		})
	}

	result := PubmedSearchResult{
		Query:   query,
		Count:   searchResult.Count,
		Results: items,
	}

	if humanOutput {
		fmt.Printf("PubMed search: %s (%s total results)\n\n", query, searchResult.Count)
		for i, item := range items {
			fmt.Printf("%d. [PMID:%s] %s\n", i+1, item.PMID, item.Title)
			if len(item.Authors) > 0 {
				authorStr := strings.Join(item.Authors, ", ")
				if len(item.Authors) > 3 {
					authorStr = strings.Join(item.Authors[:3], ", ") + ", et al."
				}
				fmt.Printf("   %s (%s)\n", authorStr, item.PubDate)
			}
			if item.Source != "" {
				fmt.Printf("   %s\n", item.Source)
			}
			fmt.Println()
		}
	} else {
		outputJSON(result)
	}

	return nil
}
