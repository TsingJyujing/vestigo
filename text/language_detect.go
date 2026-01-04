// Detect language of a given text using github.com/pemistahl/lingua-go
package text

import (
	"github.com/pemistahl/lingua-go"
)

// LanguageDetector detects the language of a given text
type LanguageDetector struct {
	detector lingua.LanguageDetector
}

// NewLanguageDetector creates a new language detector
// It only detects Chinese, Japanese, and English
// Returns lingua.Unknown for other languages or undetectable text
func NewLanguageDetector() *LanguageDetector {
	languages := []lingua.Language{
		lingua.Chinese,
		lingua.Japanese,
		lingua.English,
	}

	detector := lingua.NewLanguageDetectorBuilder().
		FromLanguages(languages...).
		Build()

	return &LanguageDetector{
		detector: detector,
	}
}

// Detect detects the language of the given text
// Returns one of: lingua.Chinese, lingua.Japanese, lingua.English, or lingua.Unknown
func (d *LanguageDetector) Detect(text string) lingua.Language {
	if text == "" {
		return lingua.Unknown
	}

	detectedLang, exists := d.detector.DetectLanguageOf(text)
	if !exists {
		return lingua.Unknown
	}

	return detectedLang
}
