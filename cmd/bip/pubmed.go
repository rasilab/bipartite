package main

import (
	"github.com/spf13/cobra"
)

// Exit codes for pubmed commands (mirrors S2 exit codes).
const (
	ExitPubMedNotFound  = 1 // Paper not found in PubMed
	ExitPubMedDuplicate = 2 // Paper already exists (without --update)
	ExitPubMedAPIError  = 3 // API error (rate limit, network)
)

var pubmedCmd = &cobra.Command{
	Use:   "pubmed",
	Short: "PubMed (NCBI E-utilities) integration commands",
	Long: `Commands for integrating with PubMed via NCBI E-utilities API.

Add papers by PMID or DOI, search PubMed by keyword, and look up paper metadata.

All commands output JSON by default for agent consumption.
Use --human flag for human-readable output.`,
}

func init() {
	rootCmd.AddCommand(pubmedCmd)
}
