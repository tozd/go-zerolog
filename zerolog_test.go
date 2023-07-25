package zerolog_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	z "gitlab.com/tozd/go/zerolog"
)

type logWithMessage struct {
	Level   string `json:"level"`
	Time    string `json:"time"`
	Message string `json:"message"`
}

func expectString(expected string) func(t *testing.T, actual string) {
	return func(t *testing.T, actual string) {
		t.Helper()
		assert.Equal(t, expected, actual)
	}
}

func expectLogWithMessage(level, message string) func(t *testing.T, actual string) {
	return func(t *testing.T, actual string) {
		t.Helper()
		var v logWithMessage
		errE := json.Unmarshal([]byte(actual), &v)
		require.NoError(t, errE)
		assert.Equal(t, level, v.Level)
		assert.Equal(t, message, v.Message)
		tt, err := time.Parse(z.TimeFieldFormat, v.Time)
		assert.NoError(t, err)
		assert.WithinDuration(t, time.Now(), tt, 5*time.Minute)
	}
}

func TestZerolog(t *testing.T) {
	for i, tt := range []struct {
		Input           func(log zerolog.Logger)
		ConsoleType     string
		ConsoleLevel    zerolog.Level
		ConsoleExpected func(t *testing.T, actual string)
		FileLevel       zerolog.Level
		FileExpected    func(t *testing.T, actual string)
	}{
		{
			Input: func(log zerolog.Logger) {
				log.Debug().Msg("test")
			},
			ConsoleType:     "json",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectString(``),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectString(``),
		},
		{
			Input: func(log zerolog.Logger) {
				log.Info().Msg("test")
			},
			ConsoleType:     "json",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectLogWithMessage("info", "test"),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectLogWithMessage("info", "test"),
		},
	} {
		t.Run(fmt.Sprintf("%d:%s/%s/%s", i, tt.ConsoleType, tt.ConsoleLevel, tt.FileLevel), func(t *testing.T) {
			dir := t.TempDir()
			p := path.Join(dir, "log")
			r, w, err := os.Pipe()
			t.Cleanup(func() {
				// We might double close but we do not care.
				r.Close()
				w.Close()
			})
			require.NoError(t, err)
			config := z.LoggingConfig{
				Log: zerolog.Nop(),
				Logging: z.Logging{
					Console: z.Console{
						Type:   tt.ConsoleType,
						Level:  tt.ConsoleLevel,
						Output: w,
					},
					File: z.File{
						Level: tt.FileLevel,
						Path:  p,
					},
				},
			}
			ff, err := z.New(&config)
			require.NoError(t, err)
			t.Cleanup(func() {
				// We might double close but we do not care.
				ff.Close()
			})
			tt.Input(config.Log)
			w.Close()
			console, err := io.ReadAll(r)
			r.Close()
			assert.NoError(t, err)
			tt.ConsoleExpected(t, string(console))
			ff.Close()
			file, err := os.ReadFile(p)
			assert.NoError(t, err)
			tt.FileExpected(t, string(file))
		})
	}
}
