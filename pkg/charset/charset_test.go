package charset

import (
	"bytes"
	"strings"
	"testing"
)

func TestConvertChunk(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out, err := ConvertChunk(nil, "UTF-8", "UTF-8")
		if err != nil {
			t.Fatalf("ConvertChunk(nil) unexpected error: %v", err)
		}
		if out != nil {
			t.Fatalf("ConvertChunk(nil) expected nil output")
		}
	})

	t.Run("same-charset-copy", func(t *testing.T) {
		in := []byte("hello")
		out, err := ConvertChunk(in, "UTF-8", "UTF-8")
		if err != nil {
			t.Fatalf("ConvertChunk same charset unexpected error: %v", err)
		}
		if !bytes.Equal(out, in) {
			t.Fatalf("ConvertChunk same charset mismatch got=%q want=%q", out, in)
		}

		out[0] = 'H'
		if in[0] != 'h' {
			t.Fatalf("ConvertChunk should return a copied slice")
		}
	})
}

func TestFormatHexFrame(t *testing.T) {
	frame := []byte("AB")
	out := FormatHexFrame(frame, false, "")
	if !strings.Contains(out, "41 42") {
		t.Fatalf("FormatHexFrame missing hex bytes: %q", out)
	}
	if !strings.Contains(out, "\"AB\"") {
		t.Fatalf("FormatHexFrame missing quoted bytes: %q", out)
	}

	outTS := FormatHexFrame([]byte("A"), true, "2006")
	if !strings.Contains(outTS, "41") || !strings.Contains(outTS, "\"A\"") {
		t.Fatalf("FormatHexFrame(withTimestamp) malformed output: %q", outTS)
	}
}

func TestConvertChunkCharsetConversion(t *testing.T) {
	t.Run("gbk-to-utf8", func(t *testing.T) {
		// Chinese "你好" in GBK: 0xC4 0xE3 0xBA 0xC3
		gbkHello := []byte{0xC4, 0xE3, 0xBA, 0xC3}
		out, err := ConvertChunk(gbkHello, "GBK", "UTF-8")
		if err != nil {
			t.Fatalf("ConvertChunk GBK->UTF-8 unexpected error: %v", err)
		}
		if string(out) != "你好" {
			t.Fatalf("ConvertChunk GBK->UTF-8 got=%q want=%q", string(out), "你好")
		}
	})

	t.Run("same-charset-different-case", func(t *testing.T) {
		in := []byte("hello")
		out, err := ConvertChunk(in, "utf-8", "UTF-8")
		if err != nil {
			t.Fatalf("ConvertChunk case-diff unexpected error: %v", err)
		}
		if !bytes.Equal(out, in) {
			t.Fatalf("ConvertChunk case-diff mismatch got=%q want=%q", out, in)
		}
	})

	t.Run("invalid-charset", func(t *testing.T) {
		_, err := ConvertChunk([]byte("hello"), "INVALID-CHARSET-NAME", "UTF-8")
		if err == nil {
			t.Fatalf("ConvertChunk invalid charset should error")
		}
	})

	t.Run("empty-input", func(t *testing.T) {
		out, err := ConvertChunk([]byte{}, "GBK", "UTF-8")
		if err != nil {
			t.Fatalf("ConvertChunk empty unexpected error: %v", err)
		}
		if out != nil {
			t.Fatalf("ConvertChunk empty input should return nil")
		}
	})
}
