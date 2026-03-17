package pubmed

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/matsen/bipartite/internal/config"
	"golang.org/x/time/rate"
)

const (
	// BaseURL is the NCBI E-utilities base URL.
	BaseURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"

	// DefaultTimeout is the default HTTP request timeout.
	DefaultTimeout = 30 * time.Second

	// UnauthenticatedRateLimit is 3 requests per second without an API key.
	UnauthenticatedRateLimit = 3.0

	// AuthenticatedRateLimit is 10 requests per second with an API key.
	AuthenticatedRateLimit = 10.0
)

// Client is a rate-limited HTTP client for the PubMed E-utilities API.
type Client struct {
	httpClient *http.Client
	limiter    *rate.Limiter
	apiKey     string
	email      string
	baseURL    string
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithAPIKey sets the API key for authenticated requests.
func WithAPIKey(key string) ClientOption {
	return func(c *Client) {
		c.apiKey = key
	}
}

// WithEmail sets the email for requests (recommended by NCBI).
func WithEmail(email string) ClientOption {
	return func(c *Client) {
		c.email = email
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithBaseURL sets a custom base URL (for testing).
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// NewClient creates a new PubMed E-utilities API client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		baseURL:    BaseURL,
	}

	// Check for API key and email in global config
	if key := config.GetPubMedAPIKey(); key != "" {
		c.apiKey = key
	}
	if email := config.GetPubMedEmail(); email != "" {
		c.email = email
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.apiKey != "" {
		c.limiter = rate.NewLimiter(rate.Limit(AuthenticatedRateLimit), 1)
	} else {
		c.limiter = rate.NewLimiter(rate.Limit(UnauthenticatedRateLimit), 1)
	}

	return c
}

// GetPaper fetches a paper by PMID using efetch (XML).
func (c *Client) GetPaper(ctx context.Context, pmid string) (*PubmedArticle, error) {
	endpoint := fmt.Sprintf("%s/efetch.fcgi", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	q.Set("db", "pubmed")
	q.Set("id", pmid)
	q.Set("retmode", "xml")
	c.addAuthParams(q)
	req.URL.RawQuery = q.Encode()

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var articleSet PubmedArticleSet
	if err := xml.Unmarshal(body, &articleSet); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}

	if len(articleSet.Articles) == 0 {
		return nil, ErrNotFound
	}

	return &articleSet.Articles[0], nil
}

// Search searches PubMed by query using esearch (JSON).
func (c *Client) Search(ctx context.Context, query string, limit int) (*ESearchBody, error) {
	if limit <= 0 {
		limit = 20
	}

	endpoint := fmt.Sprintf("%s/esearch.fcgi", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	q.Set("db", "pubmed")
	q.Set("term", query)
	q.Set("retmax", strconv.Itoa(limit))
	q.Set("retmode", "json")
	c.addAuthParams(q)
	req.URL.RawQuery = q.Encode()

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, err
	}

	var result ESearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result.Result, nil
}

// GetSummaries fetches document summaries for a list of PMIDs using esummary (JSON).
func (c *Client) GetSummaries(ctx context.Context, pmids []string) (map[string]*DocSummary, error) {
	if len(pmids) == 0 {
		return map[string]*DocSummary{}, nil
	}

	endpoint := fmt.Sprintf("%s/esummary.fcgi", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	q.Set("db", "pubmed")
	q.Set("id", strings.Join(pmids, ","))
	q.Set("retmode", "json")
	c.addAuthParams(q)
	req.URL.RawQuery = q.Encode()

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Parse the raw JSON to extract per-PMID summaries
	var raw struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	summaries := make(map[string]*DocSummary)
	for _, pmid := range pmids {
		rawDoc, ok := raw.Result[pmid]
		if !ok {
			continue
		}
		var doc DocSummary
		if err := json.Unmarshal(rawDoc, &doc); err != nil {
			continue
		}
		summaries[pmid] = &doc
	}

	return summaries, nil
}

// ResolveDOI searches PubMed for a DOI and returns the corresponding PMID.
func (c *Client) ResolveDOI(ctx context.Context, doi string) (string, error) {
	query := fmt.Sprintf("%s[doi]", doi)
	result, err := c.Search(ctx, query, 1)
	if err != nil {
		return "", err
	}
	if len(result.IDList) == 0 {
		return "", ErrNotFound
	}
	return result.IDList[0], nil
}

// addAuthParams adds API key and email to query parameters.
func (c *Client) addAuthParams(q interface{ Set(string, string) }) {
	if c.apiKey != "" {
		q.Set("api_key", c.apiKey)
	}
	if c.email != "" {
		q.Set("email", c.email)
	}
}

// do executes an HTTP request with rate limiting.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	if err := c.limiter.Wait(req.Context()); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}

	return resp, nil
}

// checkResponse checks for API errors.
func checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	msg := string(body)
	if msg == "" {
		msg = resp.Status
	}

	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		Message:    strings.TrimSpace(msg),
	}

	if resp.StatusCode == 429 {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if secs, err := strconv.Atoi(retryAfter); err == nil {
				apiErr.RetryAfter = secs
			}
		}
		return fmt.Errorf("%w: %v", ErrRateLimited, apiErr)
	}

	return apiErr
}
