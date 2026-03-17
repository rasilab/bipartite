package pubmed

import (
	"testing"
)

func TestMapPubmedToReference(t *testing.T) {
	article := PubmedArticle{
		MedlineCitation: MedlineCitation{
			PMID: PMID{Value: "19872477"},
			Article: Article{
				Title: "Genome-wide analysis in vivo of translation with nucleotide resolution using ribosome profiling",
				Abstract: AbstractText{
					Texts: []AbstractSegment{
						{Text: "Cells regulate gene expression through translation."},
					},
				},
				Journal: Journal{
					ISOAbbreviation: "Science",
					JournalIssue: JournalIssue{
						PubDate: PubDate{Year: "2009", Month: "Apr", Day: "10"},
					},
				},
				AuthorList: AuthorList{
					Authors: []PubmedAuthor{
						{LastName: "Ingolia", ForeName: "Nicholas T"},
						{LastName: "Ghaemmaghami", ForeName: "Sina"},
						{LastName: "Newman", ForeName: "John R S"},
						{LastName: "Weissman", ForeName: "Jonathan S"},
					},
				},
			},
		},
		PubmedData: PubmedData{
			ArticleIDList: ArticleIDList{
				ArticleIDs: []ArticleID{
					{IDType: "doi", Value: "10.1126/science.1168978"},
					{IDType: "pmc", Value: "PMC2746483"},
					{IDType: "pubmed", Value: "19213877"},
				},
			},
		},
	}

	ref := MapPubmedToReference(article)

	if ref.PMID != "19872477" {
		t.Errorf("PMID = %q, want %q", ref.PMID, "19872477")
	}
	if ref.DOI != "10.1126/science.1168978" {
		t.Errorf("DOI = %q, want %q", ref.DOI, "10.1126/science.1168978")
	}
	if ref.PMCID != "PMC2746483" {
		t.Errorf("PMCID = %q, want %q", ref.PMCID, "PMC2746483")
	}
	if ref.Title != "Genome-wide analysis in vivo of translation with nucleotide resolution using ribosome profiling" {
		t.Errorf("Title = %q", ref.Title)
	}
	if ref.Venue != "Science" {
		t.Errorf("Venue = %q, want %q", ref.Venue, "Science")
	}
	if ref.Source.Type != "pubmed" {
		t.Errorf("Source.Type = %q, want %q", ref.Source.Type, "pubmed")
	}
	if ref.Source.ID != "19872477" {
		t.Errorf("Source.ID = %q, want %q", ref.Source.ID, "19872477")
	}
	if len(ref.Authors) != 4 {
		t.Fatalf("len(Authors) = %d, want 4", len(ref.Authors))
	}
	if ref.Authors[0].First != "Nicholas T" || ref.Authors[0].Last != "Ingolia" {
		t.Errorf("Authors[0] = %+v, want Ingolia, Nicholas T", ref.Authors[0])
	}
	if ref.Published.Year != 2009 {
		t.Errorf("Published.Year = %d, want 2009", ref.Published.Year)
	}
	if ref.Published.Month != 4 {
		t.Errorf("Published.Month = %d, want 4", ref.Published.Month)
	}
}

func TestMapPubmedToReference_CollectiveName(t *testing.T) {
	article := PubmedArticle{
		MedlineCitation: MedlineCitation{
			PMID: PMID{Value: "12345"},
			Article: Article{
				Title: "A study by a consortium",
				Journal: Journal{
					ISOAbbreviation: "Nature",
					JournalIssue: JournalIssue{
						PubDate: PubDate{Year: "2020"},
					},
				},
				AuthorList: AuthorList{
					Authors: []PubmedAuthor{
						{CollectiveName: "Human Genome Consortium"},
					},
				},
			},
		},
	}

	ref := MapPubmedToReference(article)

	if len(ref.Authors) != 1 {
		t.Fatalf("len(Authors) = %d, want 1", len(ref.Authors))
	}
	if ref.Authors[0].Last != "Human Genome Consortium" {
		t.Errorf("Authors[0].Last = %q, want %q", ref.Authors[0].Last, "Human Genome Consortium")
	}
}

func TestBuildAbstract(t *testing.T) {
	tests := []struct {
		name     string
		abstract AbstractText
		want     string
	}{
		{
			name:     "empty",
			abstract: AbstractText{},
			want:     "",
		},
		{
			name: "single segment",
			abstract: AbstractText{
				Texts: []AbstractSegment{{Text: "  Hello world.  "}},
			},
			want: "Hello world.",
		},
		{
			name: "labeled segments",
			abstract: AbstractText{
				Texts: []AbstractSegment{
					{Label: "BACKGROUND", Text: "Background text."},
					{Label: "METHODS", Text: "Methods text."},
				},
			},
			want: "BACKGROUND: Background text. METHODS: Methods text.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAbstract(tt.abstract)
			if got != tt.want {
				t.Errorf("buildAbstract() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseMonth(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"Jan", 1},
		{"apr", 4},
		{"December", 12},
		{"6", 6},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseMonth(tt.input); got != tt.want {
				t.Errorf("parseMonth(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateCiteKey(t *testing.T) {
	article := PubmedArticle{
		MedlineCitation: MedlineCitation{
			PMID: PMID{Value: "12345"},
			Article: Article{
				Title: "Ribosome profiling reveals pervasive translation",
				Journal: Journal{
					JournalIssue: JournalIssue{
						PubDate: PubDate{Year: "2012"},
					},
				},
				AuthorList: AuthorList{
					Authors: []PubmedAuthor{
						{LastName: "Ingolia", ForeName: "Nicholas T"},
					},
				},
			},
		},
	}

	key := generateCiteKey(article)
	if key != "Ingolia2012-rp" {
		t.Errorf("generateCiteKey() = %q, want %q", key, "Ingolia2012-rp")
	}
}

func TestSanitizeForCiteKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"O'Brien", "OBrien"},
		{"van der Waals", "vanderWaals"},
		{"Zhang", "Zhang"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := sanitizeForCiteKey(tt.input); got != tt.want {
				t.Errorf("sanitizeForCiteKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
