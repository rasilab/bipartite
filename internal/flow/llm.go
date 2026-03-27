package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	// maxSummarizeConcurrency limits parallel Claude CLI calls for summarization.
	// Set to 10 to stay well under typical API rate limits while providing good throughput.
	maxSummarizeConcurrency = 10

	// maxCommentLength limits comment body length in take-home prompts.
	maxCommentLength = 200

	// maxBodyPreviewLength limits body preview in take-home prompts.
	maxBodyPreviewLength = 300

	// maxBodyForSummary limits input body size for single-item summarization.
	maxBodyForSummary = 1000

	// maxSummaryWords is the target word limit for single-item summaries.
	maxSummaryWords = 15
)

// truncateUTF8 safely truncates text to approximately maxLen bytes
// without splitting multi-byte UTF-8 characters. Adds "..." if truncated.
func truncateUTF8(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	// Find the last valid UTF-8 character boundary before maxLen
	validLen := maxLen
	for validLen > 0 && !utf8.RuneStart(text[validLen]) {
		validLen--
	}

	if validLen == 0 {
		return ""
	}

	return text[:validLen] + "..."
}

// CallClaude calls the claude CLI with the given prompt.
func CallClaude(prompt string, model string) (string, error) {
	if model == "" {
		model = "haiku"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "--model", model, "-p", prompt)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("claude CLI timed out after 5m")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude CLI error: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude CLI error: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GenerateTakehomeSummaries generates take-home summaries for a batch of items.
func GenerateTakehomeSummaries(items []ItemDetails) (TakehomeSummary, error) {
	if len(items) == 0 {
		return TakehomeSummary{}, nil
	}

	prompt := buildSummaryPrompt(items)
	response, err := CallClaude(prompt, "haiku")
	if err != nil {
		return nil, err
	}

	return parseSummaryResponse(response)
}

// buildSummaryPrompt builds the prompt for take-home summary generation.
func buildSummaryPrompt(items []ItemDetails) string {
	var itemsText strings.Builder

	for _, item := range items {
		itemType := "Issue"
		if item.IsPR {
			itemType = "PR"
		}

		// Format comments (last 5, truncated to 200 chars each)
		var commentsText strings.Builder
		start := 0
		if len(item.Comments) > 5 {
			start = len(item.Comments) - 5
		}
		for _, c := range item.Comments[start:] {
			body := truncateUTF8(c.Body, maxCommentLength)
			commentsText.WriteString(fmt.Sprintf("    @%s: %s\n", c.Author, body))
		}

		bodyPreview := truncateUTF8(item.Body, maxBodyPreviewLength)

		itemsText.WriteString(fmt.Sprintf(`
---
REF: %s
TYPE: %s
TITLE: %s
AUTHOR: %s
BODY: %s
RECENT_COMMENTS:
%s---`, item.Ref, itemType, item.Title, item.Author, bodyPreview, commentsText.String()))
	}

	return fmt.Sprintf(`You are helping triage GitHub activity. For each item below, provide a brief take-home summary (1 short sentence) that tells the user what happened and whether they need to act.

Focus on:
- What's the current state/what happened?
- Does the user need to do anything?
- If waiting, what are they waiting for?

Examples of good summaries:
- "Will responded to your review - ready for re-review"
- "David acknowledged suggestion - no action needed"
- "Kevin asked about data format - decision needed"
- "New issue from Hugh about flu data - needs triage"
- "CI failed on your PR - needs fix"
- "Merged successfully - no action"

Output format: Return a JSON object mapping each REF to its summary.
Example: {"org/repo#123": "summary here", "org/repo#456": "another summary"}

Items to summarize:
%s

Return ONLY the JSON object, no other text.`, itemsText.String())
}

// parseSummaryResponse parses the LLM response into a TakehomeSummary.
func parseSummaryResponse(response string) (TakehomeSummary, error) {
	text := strings.TrimSpace(response)

	// Handle markdown code blocks
	if strings.HasPrefix(text, "```") {
		text = extractFromCodeBlock(text)
	}

	var result TakehomeSummary
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
	}

	return result, nil
}

// extractFromCodeBlock extracts content from a markdown code block.
func extractFromCodeBlock(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return text
	}

	// Remove first line (```json or ```)
	start := 1
	// Remove last line if it's ```
	end := len(lines)
	if strings.TrimSpace(lines[len(lines)-1]) == "```" {
		end = len(lines) - 1
	}

	return strings.Join(lines[start:end], "\n")
}

// GenerateDigestPerPerson generates one digest message per author.
// Returns a slice of Slack-formatted messages: header first, then one per person.
func GenerateDigestPerPerson(items []DigestItem, channel, dateRange string) ([]string, error) {
	if len(items) == 0 {
		return []string{fmt.Sprintf("*This week in %s* (%s)\n\nNo activity this period.", channel, dateRange)}, nil
	}

	// Group items by author, preserving first-seen order
	authorItems := make(map[string][]DigestItem)
	var authorOrder []string
	for _, item := range items {
		if _, seen := authorItems[item.Author]; !seen {
			authorOrder = append(authorOrder, item.Author)
		}
		authorItems[item.Author] = append(authorItems[item.Author], item)
	}

	// Generate one message per author
	var messages []string
	messages = append(messages, fmt.Sprintf("*This week in %s* (%s)", channel, dateRange))

	for _, author := range authorOrder {
		prompt := buildPersonDigestPrompt(authorItems[author], author)
		response, err := CallClaude(prompt, "haiku")
		if err != nil {
			return nil, fmt.Errorf("generating summary for @%s: %w", author, err)
		}
		messages = append(messages, strings.TrimSpace(response))
	}

	return messages, nil
}

// buildPersonDigestPrompt builds a prompt for a single person's digest section.
func buildPersonDigestPrompt(items []DigestItem, author string) string {
	var itemsText strings.Builder
	for _, item := range items {
		itemType := "Issue"
		if item.IsPR {
			itemType = "PR"
		}
		state := item.State
		if item.Merged {
			state = "merged"
		}
		itemsText.WriteString(fmt.Sprintf("- [%s] #%d: %s (%s) URL: %s\n",
			itemType, item.Number, item.Title, state, item.HTMLURL))
	}

	return fmt.Sprintf(`Summarize this person's GitHub activity as a compact Slack message section.

Author: @%s

Activity:
%s

CRITICAL REQUIREMENTS:
- Include EVERY item listed above. Do NOT skip or omit any.
- Keep descriptions very short (a few words each).

Format as Slack mrkdwn:
- Start with bold author header: *@%s*
- Use bullet points (•) grouped by status: Merged, Open PRs, Open issues
- Within each status bullet, list items comma-separated with short descriptions and Slack-style links
- Skip status lines with no items

Example:
*@alice*
• Merged: structure-aware loss (<https://github.com/org/repo/pull/142|#142>), dataset registry (<https://github.com/org/repo/pull/138|#138>)
• Open PRs: attention refactor (<https://github.com/org/repo/pull/147|#147>)
• Open issues: OOM on large batches (<https://github.com/org/repo/issues/156|#156>)

Return ONLY the formatted section, no other text.`, author, itemsText.String(), author)
}

// SummarizeDigestItems generates AI summaries for digest items with controlled concurrency.
//
// For items with non-empty Body fields, this function calls Claude Haiku to generate
// one-sentence summaries. The Summary field is populated on successful generation.
//
// Concurrency behavior:
//   - Runs up to maxSummarizeConcurrency (10) Claude CLI calls in parallel
//   - Stops processing on first error encountered
//   - Returns error immediately if any summarization fails
//   - Items without bodies are skipped (no API call, no error)
//
// Returns a new slice to avoid modifying the input (preserves original state).
// Note: DigestItem structs are shallow-copied, not deep-cloned.
func SummarizeDigestItems(items []DigestItem) ([]DigestItem, error) {
	if len(items) == 0 {
		return items, nil
	}

	// Create result slice with same capacity
	result := make([]DigestItem, len(items))
	copy(result, items)

	// Semaphore for bounded concurrency
	sem := make(chan struct{}, maxSummarizeConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i := range result {
		// Skip items with no body
		if result[i].Body == "" {
			continue
		}

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check if we should abort due to earlier error
			mu.Lock()
			if firstErr != nil {
				mu.Unlock()
				return
			}
			mu.Unlock()

			// Generate summary
			summary, err := summarizeSingleItem(result[idx])
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("summarizing %s: %w", result[idx].Ref, err)
				}
				mu.Unlock()
				return
			}

			// No mutex needed - each goroutine writes to its own unique index
			result[idx].Summary = summary
		}(i)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return result, nil
}

// summarizeSingleItem generates a one-sentence summary for a single item.
func summarizeSingleItem(item DigestItem) (string, error) {
	itemType := "Issue"
	if item.IsPR {
		itemType = "PR"
	}

	// Truncate body to avoid token limits (UTF-8 safe)
	body := truncateUTF8(item.Body, maxBodyForSummary)

	prompt := fmt.Sprintf(`Summarize this GitHub %s in ONE short sentence (max %d words).
Focus on what it does or proposes, not implementation details.

Title: %s
Body:
%s

Return ONLY the summary sentence, nothing else.`, itemType, maxSummaryWords, item.Title, body)

	return CallClaude(prompt, "haiku")
}
