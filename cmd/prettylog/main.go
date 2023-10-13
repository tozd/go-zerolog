package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"gitlab.com/tozd/go/errors"

	"gitlab.com/tozd/go/zerolog"
)

func main() {
	// First we initialize global zerolog configuration by calling zerolog.New
	// with configuration with all logging disabled.
	config := zerolog.LoggingConfig{ //nolint:exhaustruct
		Logging: zerolog.Logging{ //nolint:exhaustruct
			Console: zerolog.Console{ //nolint:exhaustruct
				Type: "disable",
			},
		},
	}
	_, errE := zerolog.New(&config)
	if errE != nil {
		panic(errE)
	}

	writer := zerolog.NewConsoleWriter(false, os.Stdout)

	// Writer expects a whole line at once, so we
	// use a scanner to read input line by line.
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) > 0 {
			_, err := writer.Write(line)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				fmt.Fprintf(os.Stderr, "error: %s\n%s\n", err, line)
			}
		}
	}

	err := scanner.Err()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
	}
}
