package pubmed

import (
	"regexp"
	"strings"
)

// pmidPattern matches a bare numeric PMID.
var pmidPattern = regexp.MustCompile(`^\d+$`)

// doiPattern matches a DOI (starts with 10.).
var doiPattern = regexp.MustCompile(`^10\.\d{4,}`)

// pmcidPattern matches a PMCID (PMC followed by digits).
var pmcidPattern = regexp.MustCompile(`^PMC\d+$`)

// ParsePaperID parses a paper identifier string into a PaperIdentifier.
// Supports formats:
//   - PMID:12345 or bare numeric string
//   - DOI:10.1038/nature12373
//   - PMCID:PMC1234567
//   - Local IDs (anything else)
func ParsePaperID(id string) PaperIdentifier {
	id = strings.TrimSpace(id)

	// Check for explicit PMID: prefix
	upper := strings.ToUpper(id)
	if strings.HasPrefix(upper, "PMID:") {
		return PaperIdentifier{Type: "PMID", Value: id[5:]}
	}

	// Check for DOI: prefix
	if strings.HasPrefix(upper, "DOI:") {
		return PaperIdentifier{Type: "DOI", Value: id[4:]}
	}

	// Check for PMCID: prefix
	if strings.HasPrefix(upper, "PMCID:") {
		return PaperIdentifier{Type: "PMCID", Value: id[6:]}
	}

	// Bare numeric PMID
	if pmidPattern.MatchString(id) {
		return PaperIdentifier{Type: "PMID", Value: id}
	}

	// Bare DOI (starts with 10.)
	if doiPattern.MatchString(id) {
		return PaperIdentifier{Type: "DOI", Value: id}
	}

	// Bare PMCID
	if pmcidPattern.MatchString(strings.ToUpper(id)) {
		return PaperIdentifier{Type: "PMCID", Value: strings.ToUpper(id)}
	}

	return PaperIdentifier{Type: "LOCAL", Value: id}
}

// IsExternalID returns true if the identifier represents an external
// paper ID (PMID, DOI, PMCID) rather than a local collection ID.
func (p PaperIdentifier) IsExternalID() bool {
	return p.Type != "LOCAL"
}
