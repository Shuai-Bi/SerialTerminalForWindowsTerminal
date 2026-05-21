package main

import (
	"bytes"
	"fmt"
	"github.com/trzsz/trzsz-go/trzsz"
	"github.com/zimolab/charsetconv"
	"go.bug.st/serial"
	"io"
	"os"
	"strings"
	"time"
)

var (
	serialPort  serial.Port
	out         io.Writer = os.Stdout
	trzszFilter *trzsz.TrzszFilter
	clientIn    *io.PipeReader
	stdoutPipe  *io.PipeReader
	stdinPipe   *io.PipeWriter
	clientOut   *io.PipeWriter
)

func convertChunk(chunk []byte, srcCode, dstCode string) ([]byte, error) {
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

func formatHexFrame(frame []byte, withTimestamp bool, tsFmt string) string {
	if withTimestamp {
		return fmt.Sprintf("%v % X %q \n", time.Now().Format(tsFmt), frame, frame)
	}

	return fmt.Sprintf("% X %q \n", frame, frame)
}
