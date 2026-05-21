package main

import (
	"github.com/trzsz/trzsz-go/trzsz"
	"go.bug.st/serial"
	"io"
	"os"
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
