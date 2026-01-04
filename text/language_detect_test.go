package text

import (
	"testing"

	"github.com/pemistahl/lingua-go"
)

func TestNewLanguageDetector(t *testing.T) {
	detector := NewLanguageDetector()
	if detector == nil {
		t.Fatal("NewLanguageDetector() returned nil")
	}
	if detector.detector == nil {
		t.Error("LanguageDetector.detector is nil")
	}
}

func TestLanguageDetector_Detect(t *testing.T) {
	detector := NewLanguageDetector()

	tests := []struct {
		name string
		text string
		want lingua.Language
	}{
		{
			name: "空字符串",
			text: "",
			want: lingua.Unknown,
		},
		{
			name: "简体中文",
			text: "这是一段简体中文文本，用于测试语言检测功能。",
			want: lingua.Chinese,
		},
		{
			name: "繁体中文",
			text: "這是一段繁體中文文本，用於測試語言檢測功能。",
			want: lingua.Chinese,
		},
		{
			name: "日文平假名",
			text: "これは日本語のテキストです。言語検出機能をテストします。",
			want: lingua.Japanese,
		},
		{
			name: "日文片假名",
			text: "コンピュータプログラミング",
			want: lingua.Japanese,
		},
		{
			name: "日文汉字混合",
			text: "日本語の文章です。",
			want: lingua.Japanese,
		},
		{
			name: "英文",
			text: "This is an English text for testing language detection.",
			want: lingua.English,
		},
		{
			name: "纯数字",
			text: "12345",
			want: lingua.Unknown,
		},
		{
			name: "纯符号",
			text: "!@#$%^&*()",
			want: lingua.Unknown,
		},
		{
			name: "短中文",
			text: "你好",
			want: lingua.Chinese,
		},
		{
			name: "短日文",
			text: "こんにちは",
			want: lingua.Japanese,
		},
		{
			name: "短英文",
			text: "Hello",
			want: lingua.English,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.Detect(tt.text)
			if got != tt.want {
				t.Errorf("Detect() = %v (%s), want %v (%s)", got, got.String(), tt.want, tt.want.String())
			}
		})
	}
}

func TestLanguageDetector_MixedText(t *testing.T) {
	detector := NewLanguageDetector()

	tests := []struct {
		name     string
		text     string
		wantLang lingua.Language
	}{
		{
			name:     "中英混合-中文为主",
			text:     "这是中文文本 with some English words 在里面。",
			wantLang: lingua.Chinese,
		},
		{
			name:     "中英混合-英文为主",
			text:     "This is English text 带有一些中文词汇 inside.",
			wantLang: lingua.English,
		},
		{
			name:     "日英混合-日文为主",
			text:     "これは日本語のテキストで some English words が含まれています。",
			wantLang: lingua.Japanese,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.Detect(tt.text)
			if got != tt.wantLang {
				t.Logf("Mixed text detected as %s instead of %s (this may be acceptable)",
					got.String(), tt.wantLang.String())
			}
		})
	}
}

func TestLanguageDetector_SupportedLanguages(t *testing.T) {
	detector := NewLanguageDetector()

	// 测试支持的三种语言
	supportedTexts := map[lingua.Language]string{
		lingua.Chinese:  "这是中文",
		lingua.Japanese: "これは日本語です",
		lingua.English:  "This is English",
	}

	for expectedLang, text := range supportedTexts {
		t.Run(expectedLang.String(), func(t *testing.T) {
			got := detector.Detect(text)
			if got != expectedLang {
				t.Errorf("Detect(%q) = %v, want %v", text, got, expectedLang)
			}
		})
	}
}

// 基准测试
func BenchmarkLanguageDetector_Detect_Chinese(b *testing.B) {
	detector := NewLanguageDetector()
	text := "这是一段用于基准测试的中文文本，包含了足够的信息来进行语言检测。"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.Detect(text)
	}
}

func BenchmarkLanguageDetector_Detect_Japanese(b *testing.B) {
	detector := NewLanguageDetector()
	text := "これはベンチマークテスト用の日本語テキストで、言語検出を行うための十分な情報が含まれています。"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.Detect(text)
	}
}

func BenchmarkLanguageDetector_Detect_English(b *testing.B) {
	detector := NewLanguageDetector()
	text := "This is an English text for benchmark testing that contains sufficient information for language detection."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.Detect(text)
	}
}
