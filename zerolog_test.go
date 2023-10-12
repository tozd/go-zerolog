package zerolog_test

import (
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
	globallog "github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	z "gitlab.com/tozd/go/zerolog"
)

func expectNone() func(t *testing.T, actual string) {
	return func(t *testing.T, actual string) {
		t.Helper()
		assert.Equal(t, "", actual)
	}
}

func expectLogWithMessage(level, message string, fieldValue ...string) func(t *testing.T, actual string) {
	return func(t *testing.T, actual string) {
		t.Helper()
		var v map[string]json.RawMessage
		errE := json.Unmarshal([]byte(actual), &v)
		require.NoError(t, errE)
		if level != "" {
			assert.Equal(t, `"`+level+`"`, string(v["level"]))
		}
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

func extractColor(t *testing.T, str string) (int, string) {
	t.Helper()

	match := regexp.MustCompile("^\x1b\\[(\\d+)m(.*)\x1b\\[0m$").FindStringSubmatch(str)
	require.NotEmpty(t, match, str)
	color, err := strconv.Atoi(match[1])
	require.NoError(t, err, str)
	return color, match[2]
}

func expectConsoleWithMessage(level, message string, color bool, hasErr error, fieldValues ...string) func(t *testing.T, actual string) {
	return func(t *testing.T, actual string) {
		t.Helper()
		r := `^(\S+) (\S+)(?: (.+?))?`
		extraFields := map[string]string{}
		fieldKeys := []string{}
		for i := 0; i < len(fieldValues); i += 2 {
			extraFields[fieldValues[i]] = fieldValues[i+1]
			fieldKeys = append(fieldKeys, fieldValues[i])
		}
		sort.Strings(fieldKeys)
		for _, key := range fieldKeys {
			if color {
				r += regexp.QuoteMeta(fmt.Sprintf(" \x1b[36m%s=\x1b[0m%s", key, extraFields[key]))
			} else {
				r += regexp.QuoteMeta(fmt.Sprintf(" %s=%s", key, extraFields[key]))
			}
		}
		r += `\n$`
		match := regexp.MustCompile(r).FindStringSubmatch(actual)
		require.NotEmpty(t, match, actual)
		_, ok := z.LevelColors[match[2]]
		var l string
		if !color || ok || match[2] == "???" {
			l = match[2]
		} else {
			var levelColor int
			levelColor, l = extractColor(t, match[2])
			assert.Equal(t, z.LevelColors[l], levelColor)
		}
		assert.Equal(t, level, l)
		if len(match[3]) > 0 {
			if color {
				switch l {
				case "INF", "WRN", "ERR", "FTL", "PNC":
					messageColor, m := extractColor(t, match[3])
					assert.Equal(t, 1, messageColor)
					assert.Equal(t, message, m)
				default:
					assert.Equal(t, message, match[3])
				}
			} else {
				assert.Equal(t, message, match[3])
			}
		}
		var ti string
		if color {
			var timeColor int
			timeColor, ti = extractColor(t, match[1])
			assert.Equal(t, 90, timeColor)
		} else {
			ti = match[1]
		}
		tt, err := time.ParseInLocation("15:04", ti, time.Local)
		assert.NoError(t, err)
		nyear, nmonth, nday := time.Now().Date()
		tt = time.Date(nyear, nmonth, nday, tt.Hour(), tt.Minute(), tt.Second(), tt.Nanosecond(), tt.Location())
		assert.WithinDuration(t, time.Now(), tt, 5*time.Minute)
		assert.Equal(t, time.Local, tt.Location())
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
			ConsoleExpected: expectNone(),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectNone(),
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
			FileExpected:    expectNone(),
		},
		{
			Name: "disable_console",
			Input: func(log zerolog.Logger) {
				log.Info().Msg("test")
			},
			ConsoleType:     "disable",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectNone(),
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
		{
			Name: "stdlog",
			Input: func(_ zerolog.Logger) {
				stdlog.Print("test")
			},
			ConsoleType:     "json",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectLogWithMessage("", `"test"`),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectLogWithMessage("", `"test"`),
		},
		{
			Name: "color_stdlog",
			Input: func(_ zerolog.Logger) {
				stdlog.Print("test")
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsoleWithMessage("???", `test`, true, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectLogWithMessage("", `"test"`),
		},
		{
			Name: "global_log",
			Input: func(_ zerolog.Logger) {
				globallog.Info().Msg("test")
			},
			ConsoleType:     "json",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectLogWithMessage("info", `"test"`),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectLogWithMessage("info", `"test"`),
		},
		{
			Name: "nocolor_info",
			Input: func(log zerolog.Logger) {
				log.Info().Msg("test")
			},
			ConsoleType:     "nocolor",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsoleWithMessage("INF", `test`, false, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "nocolor_no_level",
			Input: func(log zerolog.Logger) {
				log.Log().Msg("test")
			},
			ConsoleType:     "nocolor",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsoleWithMessage("???", `test`, false, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectLogWithMessage("", `"test"`),
		},
		{
			Name: "color_trace",
			Input: func(log zerolog.Logger) {
				log.Trace().Msg("test")
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.TraceLevel,
			ConsoleExpected: expectConsoleWithMessage("TRC", `test`, true, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "color_debug",
			Input: func(log zerolog.Logger) {
				log.Debug().Msg("test")
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.TraceLevel,
			ConsoleExpected: expectConsoleWithMessage("DBG", `test`, true, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "color_info",
			Input: func(log zerolog.Logger) {
				log.Info().Msg("test")
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsoleWithMessage("INF", `test`, true, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "warn",
			Input: func(log zerolog.Logger) {
				log.Warn().Msg("test")
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsoleWithMessage("WRN", `test`, true, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "color_error",
			Input: func(log zerolog.Logger) {
				log.Error().Msg("test")
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsoleWithMessage("ERR", `test`, true, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "color_no_level",
			Input: func(log zerolog.Logger) {
				log.Log().Msg("test")
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsoleWithMessage("???", `test`, true, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectLogWithMessage("", `"test"`),
		},
		{
			Name: "color_no_body",
			Input: func(log zerolog.Logger) {
				log.Info().Send()
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsoleWithMessage("INF", ``, true, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "color_values",
			Input: func(log zerolog.Logger) {
				log.Info().Str("zzz", "value").Str("aaa", "value").Msg("test")
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsoleWithMessage("INF", `test`, true, nil, "zzz", "value", "aaa", "value"),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "nocolor_values",
			Input: func(log zerolog.Logger) {
				log.Info().Str("zzz", "value").Str("aaa", "value").Msg("test")
			},
			ConsoleType:     "nocolor",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsoleWithMessage("INF", `test`, false, nil, "zzz", "value", "aaa", "value"),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},

		{
			Name: "color_values_no_body",
			Input: func(log zerolog.Logger) {
				log.Info().Str("zzz", "value").Str("aaa", "value").Send()
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsoleWithMessage("INF", ``, true, nil, "zzz", "value", "aaa", "value"),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "nocolor_values_no_body",
			Input: func(log zerolog.Logger) {
				log.Info().Str("zzz", "value").Str("aaa", "value").Send()
			},
			ConsoleType:     "nocolor",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsoleWithMessage("INF", ``, false, nil, "zzz", "value", "aaa", "value"),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
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
