package main

import (
	"fmt"
	"os"

	"gitlab.com/tozd/go/zerolog"
)

func main() {
	errE := zerolog.PrettyLog(false, os.Stdin, os.Stdout)
	if errE != nil {
		fmt.Fprintf(os.Stderr, "error: % -+#.1v", errE)
		os.Exit(1)
	}
}
