package services

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/agently/api/internal/domain"
)

// Literal config values the model writes when it does not know a user-specific
// value — "@your_chat_id_here", "<recipient email>", "YOUR_API_KEY". They pass
// every structural check and then fail at run time against the real service
// (Telegram answered a placeholder chat_id with an opaque 400/404), so the
// planner treats them as validation problems and asks the model to route the
// value through defaultInput, where the Run dialog lets the user fill it in.
var placeholderPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\byour[_-][a-z0-9_-]{0,40}[_-]here\b`),
	regexp.MustCompile(`^\s*<[^<>{}]{1,60}>\s*$`),
	regexp.MustCompile(`\b(?:YOUR|ENTER|INSERT|REPLACE)_[A-Z0-9_]{2,}\b`),
	regexp.MustCompile(`(?i)^\s*(?:x{3,}|placeholder|changeme|change_me|tbd|todo)\s*$`),
}

// isPlaceholder reports whether a literal config string is a stand-in rather
// than a real value. Template references ({{input.x}}) are never placeholders.
func isPlaceholder(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	for _, re := range placeholderPatterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// placeholderProblems lists every config value in the graph that looks like a
// made-up placeholder, as repair instructions for the graph author.
func placeholderProblems(nodes []domain.GraphNode) []string {
	var errs []string
	for _, n := range nodes {
		var found []string
		walkStrings(n.Config, "", func(path, v string) {
			if isPlaceholder(v) {
				found = append(found, fmt.Sprintf("config.%s (%q)", path, v))
			}
		})
		sort.Strings(found)
		for _, f := range found {
			errs = append(errs, fmt.Sprintf(
				"node %q: %s is a made-up placeholder, not a real value. If the user did not give it, "+
					"add the key to defaultInput with an empty string and reference it as {{input.<key>}} "+
					"so the user fills it in before running", n.Key, f))
		}
	}
	return errs
}

func walkStrings(v any, path string, fn func(path, s string)) {
	switch t := v.(type) {
	case string:
		fn(path, t)
	case map[string]any:
		for k, child := range t {
			p := k
			if path != "" {
				p = path + "." + k
			}
			walkStrings(child, p, fn)
		}
	case []any:
		for i, child := range t {
			walkStrings(child, fmt.Sprintf("%s[%d]", path, i), fn)
		}
	}
}
