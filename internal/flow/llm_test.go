package flow

import (
	"strconv"
	"strings"
	"testing"
)

func TestParseSummaryResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantErr  bool
		wantKeys []string
	}{
		{
			name:     "valid JSON",
			response: `{"matsengrp/repo#123": "summary here"}`,
			wantErr:  false,
			wantKeys: []string{"matsengrp/repo#123"},
		},
		{
			name: "JSON in markdown code block",
			response: "```json\n" +
				`{"matsengrp/repo#123": "summary here"}` + "\n```",
			wantErr:  false,
			wantKeys: []string{"matsengrp/repo#123"},
		},
		{
			name: "JSON in plain code block",
			response: "```\n" +
				`{"matsengrp/repo#123": "summary here"}` + "\n```",
			wantErr:  false,
			wantKeys: []string{"matsengrp/repo#123"},
		},
		{
			name: "multiple items",
			response: `{
				"matsengrp/repo#1": "first summary",
				"matsengrp/repo#2": "second summary"
			}`,
			wantErr:  false,
			wantKeys: []string{"matsengrp/repo#1", "matsengrp/repo#2"},
		},
		{
			name:     "invalid JSON",
			response: "This is not JSON at all",
			wantErr:  true,
		},
		{
			name: "JSON with preamble",
			response: `Here are the summaries:
{"matsengrp/repo#123": "summary"}`,
			wantErr: true,
		},
		{
			name:     "empty response",
			response: "",
			wantErr:  true,
		},
		{
			name: "whitespace around JSON",
			response: `

  {"matsengrp/repo#123": "summary"}

`,
			wantErr:  false,
			wantKeys: []string{"matsengrp/repo#123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSummaryResponse(tt.response)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseSummaryResponse() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("parseSummaryResponse() unexpected error: %v", err)
				return
			}
			for _, key := range tt.wantKeys {
				if _, ok := result[key]; !ok {
					t.Errorf("parseSummaryResponse() missing key %q", key)
				}
			}
		})
	}
}

func TestExtractFromCodeBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "json code block",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "plain code block",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "no closing fence",
			input:    "```json\n{\"key\": \"value\"}",
			expected: "{\"key\": \"value\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFromCodeBlock(tt.input)
			if got != tt.expected {
				t.Errorf("extractFromCodeBlock() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBuildSummaryPrompt(t *testing.T) {
	t.Run("builds prompt for issue", func(t *testing.T) {
		items := []ItemDetails{
			{
				Ref:    "matsengrp/repo#123",
				Title:  "Test issue",
				Author: "testuser",
				Body:   "Issue body text",
				IsPR:   false,
			},
		}
		prompt := buildSummaryPrompt(items)

		if !strings.Contains(prompt, "REF: matsengrp/repo#123") {
			t.Error("missing REF")
		}
		if !strings.Contains(prompt, "TYPE: Issue") {
			t.Error("missing TYPE: Issue")
		}
		if !strings.Contains(prompt, "TITLE: Test issue") {
			t.Error("missing TITLE")
		}
		if !strings.Contains(prompt, "AUTHOR: testuser") {
			t.Error("missing AUTHOR")
		}
		if !strings.Contains(prompt, "BODY: Issue body text") {
			t.Error("missing BODY")
		}
	})

	t.Run("builds prompt for PR", func(t *testing.T) {
		items := []ItemDetails{
			{
				Ref:    "matsengrp/repo#456",
				Title:  "Test PR",
				Author: "prauthor",
				Body:   "PR description",
				IsPR:   true,
			},
		}
		prompt := buildSummaryPrompt(items)

		if !strings.Contains(prompt, "TYPE: PR") {
			t.Error("missing TYPE: PR")
		}
	})

	t.Run("includes comments", func(t *testing.T) {
		items := []ItemDetails{
			{
				Ref:    "matsengrp/repo#123",
				Title:  "Test",
				Author: "author",
				IsPR:   false,
				Comments: []CommentSummary{
					{Author: "commenter1", Body: "First comment"},
					{Author: "commenter2", Body: "Second comment"},
				},
			},
		}
		prompt := buildSummaryPrompt(items)

		if !strings.Contains(prompt, "@commenter1: First comment") {
			t.Error("missing first comment")
		}
		if !strings.Contains(prompt, "@commenter2: Second comment") {
			t.Error("missing second comment")
		}
	})

	t.Run("truncates long comments to 200 chars", func(t *testing.T) {
		longComment := strings.Repeat("x", 300)
		items := []ItemDetails{
			{
				Ref:    "matsengrp/repo#123",
				Title:  "Test",
				Author: "author",
				IsPR:   false,
				Comments: []CommentSummary{
					{Author: "commenter", Body: longComment},
				},
			},
		}
		prompt := buildSummaryPrompt(items)

		if !strings.Contains(prompt, strings.Repeat("x", 200)) {
			t.Error("should contain truncated comment")
		}
		if strings.Contains(prompt, strings.Repeat("x", 201)) {
			t.Error("should not contain more than 200 x's")
		}
	})

	t.Run("truncates long body to 300 chars", func(t *testing.T) {
		longBody := strings.Repeat("y", 500)
		items := []ItemDetails{
			{
				Ref:    "matsengrp/repo#123",
				Title:  "Test",
				Author: "author",
				Body:   longBody,
				IsPR:   false,
			},
		}
		prompt := buildSummaryPrompt(items)

		if !strings.Contains(prompt, strings.Repeat("y", 300)) {
			t.Error("should contain truncated body")
		}
		if strings.Contains(prompt, strings.Repeat("y", 301)) {
			t.Error("should not contain more than 300 y's")
		}
	})

	t.Run("limits to last 5 comments", func(t *testing.T) {
		var comments []CommentSummary
		for i := 0; i < 10; i++ {
			comments = append(comments, CommentSummary{
				Author: "user" + strconv.Itoa(i),
				Body:   "comment " + strconv.Itoa(i),
			})
		}
		items := []ItemDetails{
			{
				Ref:      "matsengrp/repo#123",
				Title:    "Test",
				Author:   "author",
				IsPR:     false,
				Comments: comments,
			},
		}
		prompt := buildSummaryPrompt(items)

		// Should have comments 5-9, not 0-4
		if !strings.Contains(prompt, "@user5: comment 5") {
			t.Error("should contain user5's comment")
		}
		if !strings.Contains(prompt, "@user9: comment 9") {
			t.Error("should contain user9's comment")
		}
		if strings.Contains(prompt, "@user0: comment 0") {
			t.Error("should NOT contain user0's comment")
		}
		if strings.Contains(prompt, "@user4: comment 4") {
			t.Error("should NOT contain user4's comment")
		}
	})

	t.Run("includes output format instructions", func(t *testing.T) {
		items := []ItemDetails{
			{
				Ref:    "matsengrp/repo#123",
				Title:  "Test",
				Author: "author",
				IsPR:   false,
			},
		}
		prompt := buildSummaryPrompt(items)

		if !strings.Contains(prompt, "JSON object") {
			t.Error("should mention JSON object")
		}
		if !strings.Contains(prompt, "Return ONLY the JSON object") {
			t.Error("should have JSON-only instruction")
		}
	})
}

func TestBuildPersonDigestPrompt(t *testing.T) {
	t.Run("includes author header", func(t *testing.T) {
		items := []DigestItem{
			{Number: 123, Title: "Test issue", Author: "testuser", IsPR: false, State: "open", HTMLURL: "https://github.com/org/repo/issues/123"},
		}
		prompt := buildPersonDigestPrompt(items, "testuser")

		if !strings.Contains(prompt, "Author: @testuser") {
			t.Error("missing Author header")
		}
		if !strings.Contains(prompt, "*@testuser*") {
			t.Error("missing bold author in example")
		}
	})

	t.Run("formats issue", func(t *testing.T) {
		items := []DigestItem{
			{Number: 123, Title: "Test issue", Author: "testuser", IsPR: false, State: "open", HTMLURL: "https://github.com/org/repo/issues/123"},
		}
		prompt := buildPersonDigestPrompt(items, "testuser")

		if !strings.Contains(prompt, "[Issue]") {
			t.Error("missing [Issue]")
		}
		if !strings.Contains(prompt, "#123") {
			t.Error("missing #123")
		}
		if !strings.Contains(prompt, "Test issue") {
			t.Error("missing title")
		}
	})

	t.Run("formats merged PR", func(t *testing.T) {
		items := []DigestItem{
			{Number: 456, Title: "Test PR", Author: "prauthor", IsPR: true, State: "closed", Merged: true, HTMLURL: "https://github.com/org/repo/pull/456"},
		}
		prompt := buildPersonDigestPrompt(items, "prauthor")

		if !strings.Contains(prompt, "[PR]") {
			t.Error("missing [PR]")
		}
		if !strings.Contains(prompt, "merged") {
			t.Error("missing merged state")
		}
	})

	t.Run("includes Slack format instructions", func(t *testing.T) {
		items := []DigestItem{
			{Number: 123, Title: "Test", Author: "user", IsPR: false, State: "open", HTMLURL: "https://github.com/org/repo/issues/123"},
		}
		prompt := buildPersonDigestPrompt(items, "user")

		if !strings.Contains(prompt, "Slack") {
			t.Error("missing Slack")
		}
		if !strings.Contains(prompt, "mrkdwn") {
			t.Error("missing mrkdwn")
		}
		if !strings.Contains(prompt, "Slack-style links") {
			t.Error("missing Slack link format instruction")
		}
	})
}

func TestGenerateDigestPerPersonEmpty(t *testing.T) {
	messages, err := GenerateDigestPerPerson(nil, "dasm2", "Jan 12-18")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if !strings.Contains(messages[0], "*This week in dasm2*") {
		t.Error("should contain header")
	}
	if !strings.Contains(messages[0], "No activity") {
		t.Error("should contain No activity message")
	}
}
