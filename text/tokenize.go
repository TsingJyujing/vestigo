package text

import (
	"github.com/go-ego/gse"
)

// Tokenizer is the interface for text tokenization
type Tokenizer interface {
	// Tokenize splits text into tokens
	Tokenize(text string) []string
}

// GSETokenizer is a tokenizer implementation based on GSE
// It supports Chinese, Japanese and English tokenization
type GSETokenizer struct {
	seg            gse.Segmenter
	filterStopWord bool
}

// NewGSETokenizer creates a new GSE tokenizer
// filterStopWord: if true, stop words will be automatically filtered in Tokenize
func NewGSETokenizer(filterStopWord bool) (*GSETokenizer, error) {
	t := &GSETokenizer{
		filterStopWord: filterStopWord,
	}

	// Enable alphanumeric recognition
	t.seg.AlphaNum = true

	// Load Japanese dictionary first
	if err := t.seg.LoadDictEmbed("ja"); err != nil {
		return nil, err
	}

	// Load Chinese dictionary
	if err := t.seg.LoadDictEmbed("zh"); err != nil {
		return nil, err
	}

	// Load stop words
	if err := t.seg.LoadStopEmbed(); err != nil {
		return nil, err
	}

	return t, nil
}

// Tokenize splits text into tokens
// If filterStopWord is enabled, stop words will be automatically filtered
func (t *GSETokenizer) Tokenize(text string) []string {
	tokens := t.seg.Slice(text)
	if t.filterStopWord {
		return t.filterStopWords(tokens)
	}
	return tokens
}

// filterStopWords removes stop words from tokens (internal use)
func (t *GSETokenizer) filterStopWords(tokens []string) []string {
	if t.seg.StopWordMap == nil {
		return tokens
	}

	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if !t.seg.StopWordMap[token] {
			filtered = append(filtered, token)
		}
	}
	return filtered
}
