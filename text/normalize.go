package text

import (
	"strings"

	"github.com/longbridgeapp/opencc"
	"golang.org/x/text/unicode/norm"
)

type Normalizer interface {
	Normalize(text string) (string, error)
}

type CJKNormalizer struct {
	jp2t *opencc.OpenCC
	t2s  *opencc.OpenCC
}

// NewCJKNormalizer creates a CJK normalizer.
// It performs the following normalization steps:
// 1. Unicode NFKC normalization
// 2. Japanese New Kanji -> Traditional Chinese Kanji/Japanese Old Kanji
// 3. Traditional Chinese Kanji -> Simplified Chinese Kanji
// The parameters control whether to apply the respective conversions.
func NewCJKNormalizer(useJp2t, useT2s bool) (Normalizer, error) {
	var jp2t, t2s *opencc.OpenCC = nil, nil
	var err error
	if useJp2t { // TODO jp2t is not valid yet
		jp2t, err = opencc.New("jp2t") // JP new Kanji -> Traditional Chinese Kanji / JP old Kanji
		if err != nil {
			return nil, err
		}
	}
	if useT2s {
		t2s, err = opencc.New("t2s") // Traditional Chinese Kanji -> Simplified Chinese Kanji
		if err != nil {
			return nil, err
		}
	}
	return &CJKNormalizer{jp2t: jp2t, t2s: t2s}, nil
}

func (n *CJKNormalizer) Normalize(text string) (string, error) {
	// 1. Unicode NFKC
	s := norm.NFKC.String(text)
	var err error
	// 2. Japanese New Kanji -> Traditional Chinese Kanji/Japanese Old Kanji
	if n.jp2t != nil {
		s, err = n.jp2t.Convert(s)
		if err != nil {
			return "", err
		}
	}
	// 3. Traditional Chinese Kanji -> Simplified Chinese Kanji
	if n.t2s != nil {
		s, err = n.t2s.Convert(s)
		if err != nil {
			return "", err
		}
	}
	return strings.ToLower(s), nil
}
