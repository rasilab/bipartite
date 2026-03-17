package pubmed

import "testing"

func TestParsePaperID(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
		wantVal  string
	}{
		{"PMID:19872477", "PMID", "19872477"},
		{"pmid:19872477", "PMID", "19872477"},
		{"19872477", "PMID", "19872477"},
		{"DOI:10.1038/nature12373", "DOI", "10.1038/nature12373"},
		{"doi:10.1038/nature12373", "DOI", "10.1038/nature12373"},
		{"10.1038/nature12373", "DOI", "10.1038/nature12373"},
		{"PMCID:PMC1234567", "PMCID", "PMC1234567"},
		{"pmcid:PMC1234567", "PMCID", "PMC1234567"},
		{"PMC1234567", "PMCID", "PMC1234567"},
		{"Zhang2018-vi", "LOCAL", "Zhang2018-vi"},
		{"  PMID:123  ", "PMID", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParsePaperID(tt.input)
			if got.Type != tt.wantType {
				t.Errorf("ParsePaperID(%q).Type = %q, want %q", tt.input, got.Type, tt.wantType)
			}
			if got.Value != tt.wantVal {
				t.Errorf("ParsePaperID(%q).Value = %q, want %q", tt.input, got.Value, tt.wantVal)
			}
		})
	}
}

func TestIsExternalID(t *testing.T) {
	tests := []struct {
		id   PaperIdentifier
		want bool
	}{
		{PaperIdentifier{Type: "PMID", Value: "123"}, true},
		{PaperIdentifier{Type: "DOI", Value: "10.1038/x"}, true},
		{PaperIdentifier{Type: "PMCID", Value: "PMC123"}, true},
		{PaperIdentifier{Type: "LOCAL", Value: "Zhang2018"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.id.Type, func(t *testing.T) {
			if got := tt.id.IsExternalID(); got != tt.want {
				t.Errorf("IsExternalID() = %v, want %v", got, tt.want)
			}
		})
	}
}
