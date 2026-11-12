package services

import (
	"strings"
	"testing"

	"github.com/agently/api/internal/domain"
)

func TestIsPlaceholder(t *testing.T) {
	for _, s := range []string{"@your_chat_id_here", "your-email-here", "<recipient email>", "YOUR_API_KEY", "Bearer YOUR_TOKEN", "xxxx", "changeme"} {
		if !isPlaceholder(s) {
			t.Errorf("isPlaceholder(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "{{input.chat_id}}", "@techdigest", "-1001234567890", "Here are your top stories", "<b>{{outputs.x.result}}</b>", "Your daily brief is here"} {
		if isPlaceholder(s) {
			t.Errorf("isPlaceholder(%q) = true, want false", s)
		}
	}
}

func TestPlaceholderProblemsNamesNodeAndPath(t *testing.T) {
	nodes := []domain.GraphNode{{
		Key: "send", Type: "pieces.telegram-bot.send_text_message",
		Config: map[string]any{"chat_id": "@your_chat_id_here", "message": "{{outputs.write.text}}"},
	}}
	errs := placeholderProblems(nodes)
	if len(errs) != 1 || !strings.Contains(errs[0], `"send"`) || !strings.Contains(errs[0], "config.chat_id") || !strings.Contains(errs[0], "defaultInput") {
		t.Fatalf("placeholderProblems = %v", errs)
	}
}
