package telegram

import (
	"strings"
	"testing"
)

func TestSplitMessageRespectsTelegramLimit(t *testing.T) {
	text := strings.Repeat("step detail ", 900)
	chunks := splitMessage(text, 3900)
	if len(chunks) < 2 {
		t.Fatal("expected long cooking guide to be chunked")
	}
	for _, chunk := range chunks {
		if len([]rune(chunk)) > 3900 {
			t.Fatalf("chunk exceeds Telegram limit: %d", len([]rune(chunk)))
		}
	}
	if strings.Join(chunks, "") == "" {
		t.Fatal("chunks unexpectedly empty")
	}
}
