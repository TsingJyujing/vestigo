package text

import (
	"testing"
)

func TestNewCJKNormalizer(t *testing.T) {
	tests := []struct {
		name    string
		useJp2t bool
		useT2s  bool
		wantErr bool
	}{
		{
			name:    "创建无转换的normalizer",
			useJp2t: false,
			useT2s:  false,
			wantErr: false,
		},
		{
			name:    "创建仅jp2t的normalizer",
			useJp2t: true,
			useT2s:  false,
			wantErr: false,
		},
		{
			name:    "创建仅t2s的normalizer",
			useJp2t: false,
			useT2s:  true,
			wantErr: false,
		},
		{
			name:    "创建完整的normalizer",
			useJp2t: true,
			useT2s:  true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewCJKNormalizer(tt.useJp2t, tt.useT2s)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCJKNormalizer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && n == nil {
				t.Errorf("NewCJKNormalizer() returned nil normalizer")
			}
		})
	}
}

func TestCJKNormalizer_Normalize_NFKC(t *testing.T) {
	n, err := NewCJKNormalizer(false, false)
	if err != nil {
		t.Fatalf("NewCJKNormalizer() error = %v", err)
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "普通ASCII文本",
			input:   "Hello World",
			want:    "Hello World",
			wantErr: false,
		},
		{
			name:    "空字符串",
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "全角字符转半角",
			input:   "Ｈｅｌｌｏ",
			want:    "Hello",
			wantErr: false,
		},
		{
			name:    "Unicode兼容字符",
			input:   "ℌ", // U+210C (Black-letter H)
			want:    "H",
			wantErr: false,
		},
		{
			name:    "中文字符不变",
			input:   "你好世界",
			want:    "你好世界",
			wantErr: false,
		},
		{
			name:    "日文假名不变",
			input:   "こんにちは",
			want:    "こんにちは",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := n.Normalize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Normalize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Normalize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCJKNormalizer_Normalize_JP2T(t *testing.T) {
	n, err := NewCJKNormalizer(true, false)
	if err != nil {
		t.Fatalf("NewCJKNormalizer() error = %v", err)
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "日本新字体转繁体",
			input:   "国",
			want:    "國",
			wantErr: false,
		},
		{
			name:    "混合文本",
			input:   "日本国",
			want:    "日本國",
			wantErr: false,
		},
		{
			name:    "普通文本不变",
			input:   "Hello",
			want:    "Hello",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := n.Normalize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Normalize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Normalize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCJKNormalizer_Normalize_T2S(t *testing.T) {
	n, err := NewCJKNormalizer(false, true)
	if err != nil {
		t.Fatalf("NewCJKNormalizer() error = %v", err)
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "繁体转简体",
			input:   "繁體中文",
			want:    "繁体中文",
			wantErr: false,
		},
		{
			name:    "已是简体不变",
			input:   "简体中文",
			want:    "简体中文",
			wantErr: false,
		},
		{
			name:    "混合文本",
			input:   "Hello 繁體",
			want:    "Hello 繁体",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := n.Normalize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Normalize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Normalize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCJKNormalizer_Normalize_Full(t *testing.T) {
	n, err := NewCJKNormalizer(true, true)
	if err != nil {
		t.Fatalf("NewCJKNormalizer() error = %v", err)
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "完整转换流程-日文到简体",
			input:   "日本国",
			want:    "日本国",
			wantErr: false,
		},
		{
			name:    "NFKC+JP2T+T2S组合",
			input:   "ＨｅｌｌｏＷｏｒｌｄ",
			want:    "HelloWorld",
			wantErr: false,
		},
		{
			name:    "复杂混合文本",
			input:   "Hello繁體日本国検索檢索",
			want:    "Hello繁体日本国检索检索",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := n.Normalize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Normalize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Normalize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCJKNormalizer_Interface(t *testing.T) {
	// 测试 CJKNormalizer 实现了 Normalizer 接口
	var _ Normalizer = &CJKNormalizer{}

	n, err := NewCJKNormalizer(false, false)
	if err != nil {
		t.Fatalf("NewCJKNormalizer() error = %v", err)
	}

	// 验证返回的是 Normalizer 接口类型
	var normalizer Normalizer = n
	result, err := normalizer.Normalize("test")
	if err != nil {
		t.Errorf("Normalize() through interface error = %v", err)
	}
	if result != "test" {
		t.Errorf("Normalize() through interface = %v, want %v", result, "test")
	}
}

// 基准测试
func BenchmarkCJKNormalizer_Normalize_NFKC(b *testing.B) {
	n, _ := NewCJKNormalizer(false, false)
	text := "ＨｅｌｌｏＷｏｒｌｄ你好世界"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = n.Normalize(text)
	}
}

func BenchmarkCJKNormalizer_Normalize_Full(b *testing.B) {
	n, _ := NewCJKNormalizer(true, true)
	text := "ＨｅｌｌｏＷｏｒｌｄ繁體中文日本国"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = n.Normalize(text)
	}
}
