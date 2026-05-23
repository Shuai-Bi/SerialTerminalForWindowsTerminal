package main

import (
	"log"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/console"
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile | log.Lmsgprefix)
}

func main() {
	console.Run()
}
