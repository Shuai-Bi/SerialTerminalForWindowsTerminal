// Package charset provides character-set conversion and hex formatting utilities.
package charset

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/zimolab/charsetconv"
)

// ConvertChunk converts a byte chunk from srcCode charset to dstCode charset.
// Returns nil, nil when input is empty. Returns a copied slice when charsets match.
func ConvertChunk(chunk []byte, srcCode, dstCode string) ([]byte, error) {
	if len(chunk) == 0 {
		return nil, nil
	}

	if strings.EqualFold(srcCode, dstCode) {
		dup := make([]byte, len(chunk))
		copy(dup, chunk)
		return dup, nil
	}

	var buf bytes.Buffer
	err := charsetconv.ConvertWith(bytes.NewReader(chunk), charsetconv.Charset(srcCode), &buf, charsetconv.Charset(dstCode), false)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// FormatHexFrame formats a byte frame as hex + printable representation.
// Optionally prefixes with a timestamp using the given format string.
func FormatHexFrame(frame []byte, withTimestamp bool, tsFmt string) string {
	if withTimestamp {
		return fmt.Sprintf("%v % X %q \n", time.Now().Format(tsFmt), frame, frame)
	}

	return fmt.Sprintf("% X %q \n", frame, frame)
}
