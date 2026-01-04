package text

import (
	"testing"
)

func TestNewGSETokenizer(t *testing.T) {
	tokenizer, err := NewGSETokenizer(false)
	if err != nil {
		t.Fatalf("NewGSETokenizer() error = %v", err)
	}
	if tokenizer == nil {
		t.Fatal("NewGSETokenizer() returned nil tokenizer")
	}
}

func TestGSETokenizer_Tokenize(t *testing.T) {
	tokenizer, err := NewGSETokenizer(false)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	tests := []struct {
		name  string
		text  string
		check func([]string) bool
	}{
		{
			name: "Chinese",
			text: "你好世界",
			check: func(tokens []string) bool {
				return len(tokens) > 0
			},
		},
		{
			name: "Japanese",
			text: "こんにちは世界",
			check: func(tokens []string) bool {
				return len(tokens) > 0
			},
		},
		{
			name: "English",
			text: "Hello World",
			check: func(tokens []string) bool {
				return len(tokens) > 0
			},
		},
		{
			name: "Mixed",
			text: "山达尔星联邦共和国Hello world, こんにちは世界。你好世界.",
			check: func(tokens []string) bool {
				return len(tokens) > 5
			},
		},
		{
			name: "Empty",
			text: "",
			check: func(tokens []string) bool {
				return len(tokens) == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenizer.Tokenize(tt.text)
			if !tt.check(tokens) {
				t.Errorf("Tokenize(%q) = %v, check failed", tt.text, tokens)
			}
		})
	}
}

func TestGSETokenizer_FilterStopWord(t *testing.T) {
	// Test with filterStopWord disabled
	tokenizerNoFilter, err := NewGSETokenizer(false)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	text := "我的世界是一个很好的游戏"
	tokensNoFilter := tokenizerNoFilter.Tokenize(text)

	// Test with filterStopWord enabled
	tokenizerWithFilter, err := NewGSETokenizer(true)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	tokensFiltered := tokenizerWithFilter.Tokenize(text)

	// Filtered result should be less than or equal to original tokens
	if len(tokensFiltered) > len(tokensNoFilter) {
		t.Errorf("Filtered tokens should be less than or equal to unfiltered, got %d > %d", len(tokensFiltered), len(tokensNoFilter))
	}
}

func BenchmarkGSETokenizer_Tokenize_Chinese(b *testing.B) {
	tokenizer, err := NewGSETokenizer(false)
	if err != nil {
		b.Fatalf("Failed to create tokenizer: %v", err)
	}
	text := "山达尔星联邦共和国联邦政府是一个强大的政治实体"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tokenizer.Tokenize(text)
	}
}

func BenchmarkGSETokenizer_Tokenize_Japanese(b *testing.B) {
	tokenizer, err := NewGSETokenizer(false)
	if err != nil {
		b.Fatalf("Failed to create tokenizer: %v", err)
	}
	text := "こんにちは世界、私はウツボです。"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tokenizer.Tokenize(text)
	}
}

func BenchmarkGSETokenizer_Tokenize_Mixed(b *testing.B) {
	tokenizer, err := NewGSETokenizer(false)
	if err != nil {
		b.Fatalf("Failed to create tokenizer: %v", err)
	}
	text := "山达尔星联邦共和国Hello world, Winter is coming! こんにちは世界、私はウツボです。你好世界."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tokenizer.Tokenize(text)
	}
}
