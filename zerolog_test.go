package zerolog_test

import (
	"encoding/json"
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

func expectString(expected string) func(t *testing.T, actual string) {
	return func(t *testing.T, actual string) {
		t.Helper()
		assert.Equal(t, expected, actual)
	}
}

func expectLogWithMessage(level, message string, fieldValue ...string) func(t *testing.T, actual string) {
	return func(t *testing.T, actual string) {
		t.Helper()
		var v map[string]json.RawMessage
		errE := json.Unmarshal([]byte(actual), &v)
		require.NoError(t, errE)
		assert.Equal(t, `"`+level+`"`, string(v["level"]))
		assert.Equal(t, message, string(v["message"]))
		tt, err := time.Parse(`"`+z.TimeFieldFormat+`"`, string(v["time"]))
		assert.NoError(t, err)
		assert.WithinDuration(t, time.Now(), tt, 5*time.Minute)
		assert.Equal(t, time.UTC, tt.Location())
		for i := 0; i < len(fieldValue); i += 2 {
			assert.Equal(t, fieldValue[i+1], string(v[fieldValue[i]]))
		}
	}
}

func TestZerolog(t *testing.T) {
	for _, tt := range []struct {
		Name            string
		Input           func(log zerolog.Logger)
		ConsoleType     string
		ConsoleLevel    zerolog.Level
		ConsoleExpected func(t *testing.T, actual string)
		FileLevel       zerolog.Level
		FileExpected    func(t *testing.T, actual string)
	}{
		{
			Name: "basic_level_filter",
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
			Name: "basic_logging",
			Input: func(log zerolog.Logger) {
				log.Info().Msg("test")
			},
			ConsoleType:     "json",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectLogWithMessage("info", `"test"`),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectLogWithMessage("info", `"test"`),
		},
		{
			Name: "mixed_level_filter",
			Input: func(log zerolog.Logger) {
				log.Info().Msg("test")
			},
			ConsoleType:     "json",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectLogWithMessage("info", `"test"`),
			FileLevel:       zerolog.ErrorLevel,
			FileExpected:    expectString(``),
		},
		{
			Name: "disable_console",
			Input: func(log zerolog.Logger) {
				log.Info().Msg("test")
			},
			ConsoleType:     "disable",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectString(``),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectLogWithMessage("info", `"test"`),
		},
		{
			Name: "no_escape_html",
			Input: func(log zerolog.Logger) {
				log.Info().Interface("body", "<body>").Msg("<test>")
			},
			ConsoleType:     "json",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectLogWithMessage("info", `"<test>"`, "body", `"<body>"`),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectLogWithMessage("info", `"<test>"`, "body", `"<body>"`),
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
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
