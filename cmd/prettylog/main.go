package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"gitlab.com/tozd/go/zerolog"
)

func main() {
	// First we initialize global zerolog configuration by calling zerolog.New
	// with configuration with all logging disabled.
	config := zerolog.LoggingConfig{
		Logging: zerolog.Logging{
			Console: zerolog.Console{
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
				fmt.Fprintf(os.Stderr, "error: %s\n%s\n", err, line)
			}
		}
	}

	err := scanner.Err()
	// Reader can get closed and we ignore that.
	if err != nil && !errors.Is(err, os.ErrClosed) {
		fmt.Fprintf(os.Stderr, "error: %s", err)
	}
}
