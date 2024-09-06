package zerolog_test

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
	globallog "github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/tozd/go/errors"

	z "gitlab.com/tozd/go/zerolog"
)

//go:embed example.jsonl
var testExample []byte

//go:embed example.out
var testExpected []byte

var formattedLevels = map[string]zerolog.Level{} //nolint:gochecknoglobals

func init() { //nolint:gochecknoinits
	for l, f := range zerolog.FormattedLevels {
		formattedLevels[f] = l
	}
}

func expectNone() func(t *testing.T, actual string) {
	return func(t *testing.T, actual string) {
		t.Helper()
		assert.Equal(t, "", actual)
	}
}

func expectLog(level, message string, fieldValue ...string) func(t *testing.T, actual string) {
	return func(t *testing.T, actual string) {
		t.Helper()
		var v map[string]json.RawMessage
		errE := json.Unmarshal([]byte(actual), &v)
		require.NoError(t, errE, actual)
		fieldCount := 0
		if level != "" {
			fieldCount++
			assert.Equal(t, `"`+level+`"`, string(v["level"]))
		}
		if message != "" {
			fieldCount++
			assert.Equal(t, message, string(v["message"]))
		}
		tt, err := time.Parse(`"`+z.TimeFieldFormat+`"`, string(v["time"]))
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now(), tt, 5*time.Minute)
		assert.Equal(t, time.UTC, tt.Location())
		for i := 0; i < len(fieldValue); i += 2 {
			assert.Equal(t, fieldValue[i+1], string(v[fieldValue[i]]))
		}
		assert.Len(t, v, 1+fieldCount+len(fieldValue)/2)
	}
}

func extractColor(t *testing.T, str string) (int, string) {
	t.Helper()

	r := "^\x1b\\[(\\d+)m(.*)\x1b\\[0m$"
	match := regexp.MustCompile(r).FindStringSubmatch(str)
	require.NotEmpty(t, match, "%s\n%s\n", str, r)
	color, err := strconv.Atoi(match[1])
	require.NoError(t, err, str)
	return color, match[2]
}

func expectConsole(level, message string, color bool, hasErr error, fieldValues ...string) func(t *testing.T, actual string) {
	return func(t *testing.T, actual string) {
		t.Helper()
		r := `^(\S+) (\S+)(?: (.+?))?`
		if hasErr != nil {
			if color {
				r += regexp.QuoteMeta(fmt.Sprintf(" \x1b[36merror=\x1b[0m\x1b[31m\x1b[1m%s\x1b[0m\x1b[0m", strconv.Quote(hasErr.Error())))
			} else {
				r += regexp.QuoteMeta(fmt.Sprintf(" error=%q", hasErr.Error()))
			}
		}
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
		r += `\n`
		if hasErr != nil {
			if level == "ERR" {
				r += `((?:.*\n)+)`
			} else {
				r += `((?:.+=.*\n)+)`
			}
		}
		r += `$`
		match := regexp.MustCompile(r).FindStringSubmatch(actual)
		require.NotEmpty(t, match, "%s\n%s\n", actual, r)
		_, ok := formattedLevels[match[2]]
		var l string
		if !color || ok || match[2] == "???" {
			l = match[2]
		} else {
			var levelColor int
			levelColor, l = extractColor(t, match[2])
			level, ok := formattedLevels[l]
			assert.True(t, ok)
			assert.Equal(t, zerolog.LevelColors[level], levelColor)
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
		require.NoError(t, err)
		nyear, nmonth, nday := time.Now().Date()
		tt = time.Date(nyear, nmonth, nday, tt.Hour(), tt.Minute(), tt.Second(), tt.Nanosecond(), tt.Location())
		assert.WithinDuration(t, time.Now(), tt, 5*time.Minute)
		assert.Equal(t, time.Local, tt.Location())
		if hasErr != nil && level == "ERR" {
			c := func(s string) string {
				if color {
					return "\x1b\\[31m" + s + "\x1b\\[0m"
				}
				return s
			}
			helpLine := `.+:`
			detail := `.+=.*`
			line := `.+`
			location := `.+:\d+`
			errMsg := `.+`
			assert.Regexp(t, `^`+
				`(?:`+
				c(`\t*`+detail)+`\n`+ // Detail key-value.
				`)*`+
				c(`\t*`+helpLine)+`\n`+ // Help line.
				`(?:`+
				c(`\t*`+line)+`\n`+ // Function name.
				c(`\t*`+location)+`\n`+ // File name and line.
				`)+`+
				`(?:`+
				`\n`+
				c(`\t*`+helpLine)+`\n\n`+ // Help line.
				c(`\t*`+errMsg)+`\n`+ // Error.
				`(?:`+
				c(`\t*`+detail)+`\n`+ // Detail key-value.
				`)*`+
				c(`\t*`+helpLine)+`\n`+ // Help line.
				`(?:`+
				c(`\t*`+line)+`\n`+ // Function name.
				c(`\t*`+location)+`\n`+ // File name and line.
				`)+`+
				`)*`+
				`$`, match[4])
		}
	}
}

func TestZerolog(t *testing.T) {
	parentError := errors.New("parent error")
	errors.Details(parentError)["x"] = "y"
	logErr := errors.Wrap(parentError, "child error")
	errors.Details(logErr)["x"] = "z"

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
			ConsoleExpected: expectLog("info", `"test"`),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectLog("info", `"test"`),
		},
		{
			Name: "mixed_level_filter",
			Input: func(log zerolog.Logger) {
				log.Info().Msg("test")
			},
			ConsoleType:     "json",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectLog("info", `"test"`),
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
			FileExpected:    expectLog("info", `"test"`),
		},
		{
			Name: "no_escape_html",
			Input: func(log zerolog.Logger) {
				log.Info().Interface("body", "<body>").Msg("<test>")
			},
			ConsoleType:     "json",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectLog("info", `"<test>"`, "body", `"<body>"`),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectLog("info", `"<test>"`, "body", `"<body>"`),
		},
		{
			Name: "stdlog",
			Input: func(_ zerolog.Logger) {
				stdlog.Print("test")
			},
			ConsoleType:     "json",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectLog("", `"test"`),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectLog("", `"test"`),
		},
		{
			Name: "color_stdlog",
			Input: func(_ zerolog.Logger) {
				stdlog.Print("test")
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsole("???", `test`, true, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectLog("", `"test"`),
		},
		{
			Name: "global_log",
			Input: func(_ zerolog.Logger) {
				globallog.Info().Msg("test")
			},
			ConsoleType:     "json",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectLog("info", `"test"`),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectLog("info", `"test"`),
		},
		{
			Name: "nocolor_info",
			Input: func(log zerolog.Logger) {
				log.Info().Msg("test")
			},
			ConsoleType:     "nocolor",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsole("INF", `test`, false, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "nocolor_duration",
			Input: func(log zerolog.Logger) {
				log.Info().Dur("dur", time.Second+time.Millisecond+time.Microsecond).Msg("test")
			},
			ConsoleType:     "nocolor",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsole("INF", `test`, false, nil, "dur", "1.001"),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "nocolor_float",
			Input: func(log zerolog.Logger) {
				log.Info().Float64("float", 1.23456).Msg("test")
			},
			ConsoleType:     "nocolor",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsole("INF", `test`, false, nil, "float", "1.235"),
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
			ConsoleExpected: expectConsole("???", `test`, false, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectLog("", `"test"`),
		},
		{
			Name: "color_trace",
			Input: func(log zerolog.Logger) {
				log.Trace().Msg("test")
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.TraceLevel,
			ConsoleExpected: expectConsole("TRC", `test`, true, nil),
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
			ConsoleExpected: expectConsole("DBG", `test`, true, nil),
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
			ConsoleExpected: expectConsole("INF", `test`, true, nil),
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
			ConsoleExpected: expectConsole("WRN", `test`, true, nil),
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
			ConsoleExpected: expectConsole("ERR", `test`, true, nil),
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
			ConsoleExpected: expectConsole("???", `test`, true, nil),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectLog("", `"test"`),
		},
		{
			Name: "color_no_body",
			Input: func(log zerolog.Logger) {
				log.Info().Send()
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsole("INF", ``, true, nil),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectLog("info", ``),
		},
		{
			Name: "color_values",
			Input: func(log zerolog.Logger) {
				log.Info().Str("zzz", "value").Str("aaa", "value").Msg("test")
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsole("INF", `test`, true, nil, "zzz", "value", "aaa", "value"),
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
			ConsoleExpected: expectConsole("INF", `test`, false, nil, "zzz", "value", "aaa", "value"),
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
			ConsoleExpected: expectConsole("INF", ``, true, nil, "zzz", "value", "aaa", "value"),
			FileLevel:       zerolog.InfoLevel,
			FileExpected:    expectLog("info", ``, "zzz", `"value"`, "aaa", `"value"`),
		},
		{
			Name: "nocolor_values_no_body",
			Input: func(log zerolog.Logger) {
				log.Info().Str("zzz", "value").Str("aaa", "value").Send()
			},
			ConsoleType:     "nocolor",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsole("INF", ``, false, nil, "zzz", "value", "aaa", "value"),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "color_err",
			Input: func(log zerolog.Logger) {
				log.Info().Str("zzz", "value").Str("aaa", "value").Err(logErr).Msg("test")
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsole("INF", `test`, true, logErr, "zzz", "value", "aaa", "value"),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "nocolor_err",
			Input: func(log zerolog.Logger) {
				log.Info().Str("zzz", "value").Str("aaa", "value").Err(logErr).Msg("test")
			},
			ConsoleType:     "nocolor",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsole("INF", `test`, false, logErr, "zzz", "value", "aaa", "value"),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "color_err_no_body",
			Input: func(log zerolog.Logger) {
				log.Info().Str("zzz", "value").Str("aaa", "value").Err(logErr).Send()
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsole("INF", ``, true, logErr, "zzz", "value", "aaa", "value"),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "nocolor_err_no_body",
			Input: func(log zerolog.Logger) {
				log.Info().Str("zzz", "value").Str("aaa", "value").Err(logErr).Send()
			},
			ConsoleType:     "nocolor",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsole("INF", ``, false, logErr, "zzz", "value", "aaa", "value"),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "color_error_err",
			Input: func(log zerolog.Logger) {
				log.Error().Str("zzz", "value").Str("aaa", "value").Err(logErr).Msg("test")
			},
			ConsoleType:     "color",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsole("ERR", `test`, true, logErr, "zzz", "value", "aaa", "value"),
			FileLevel:       zerolog.PanicLevel,
			FileExpected:    expectNone(),
		},
		{
			Name: "nocolor_error_err",
			Input: func(log zerolog.Logger) {
				log.Error().Str("zzz", "value").Str("aaa", "value").Err(logErr).Msg("test")
			},
			ConsoleType:     "nocolor",
			ConsoleLevel:    zerolog.InfoLevel,
			ConsoleExpected: expectConsole("ERR", `test`, false, logErr, "zzz", "value", "aaa", "value"),
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
				Logger:      zerolog.Nop(),
				WithContext: nil,
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
					Main: z.Main{
						Level: zerolog.TraceLevel,
					},
					Context: z.Context{
						Level:            zerolog.TraceLevel,
						ConditionalLevel: zerolog.TraceLevel,
						TriggerLevel:     zerolog.TraceLevel,
					},
				},
			}
			ff, errE := z.New(&config)
			require.NoError(t, errE, "% -+#.1v", errE)
			t.Cleanup(func() {
				// We might double close but we do not care.
				ff.Close()
			})
			tt.Input(config.Logger)
			w.Close()
			console, err := io.ReadAll(r)
			r.Close()
			require.NoError(t, err)
			tt.ConsoleExpected(t, string(console))
			ff.Close()
			file, err := os.ReadFile(p)
			require.NoError(t, err)
			tt.FileExpected(t, string(file))
		})
	}
}

func TestPrettyLog(t *testing.T) {
	buffer := new(bytes.Buffer)
	errE := z.PrettyLog(false, bytes.NewReader(testExample), buffer)
	require.NoError(t, errE, "% -+#.1v", errE)
	assert.Equal(t, testExpected, buffer.Bytes())
}

func TestWithContext(t *testing.T) {
	for k, tt := range []struct {
		Test             func(t *testing.T, ctx context.Context, buffer *bytes.Buffer)
		AfterTrigger     func(t *testing.T, buffer *bytes.Buffer)
		ConsoleLevel     zerolog.Level
		ContextLevel     zerolog.Level
		ConditionalLevel zerolog.Level
		TriggerLevel     zerolog.Level
	}{
		{
			Test: func(t *testing.T, ctx context.Context, buffer *bytes.Buffer) {
				t.Helper()
				zerolog.Ctx(ctx).Debug().Msg("no")
				zerolog.Ctx(ctx).Info().Msg("yes1")
				assert.Regexp(t, `^\d{2}:\d{2} INF yes1\n$`, buffer.String())
				zerolog.Ctx(ctx).Error().Msg("yes2")
				assert.Regexp(t, `^\d{2}:\d{2} INF yes1\n\d{2}:\d{2} DBG no\n\d{2}:\d{2} ERR yes2\n$`, buffer.String())
			},
			AfterTrigger: func(t *testing.T, buffer *bytes.Buffer) {
				t.Helper()
				assert.Regexp(t, `^\d{2}:\d{2} INF yes1\n\d{2}:\d{2} DBG no\n\d{2}:\d{2} ERR yes2\n$`, buffer.String())
			},
			ConsoleLevel:     zerolog.DebugLevel,
			ContextLevel:     zerolog.DebugLevel,
			ConditionalLevel: zerolog.DebugLevel,
			TriggerLevel:     zerolog.ErrorLevel,
		},
		{
			Test: func(t *testing.T, ctx context.Context, buffer *bytes.Buffer) {
				t.Helper()
				zerolog.Ctx(ctx).Debug().Msg("no")
				zerolog.Ctx(ctx).Info().Msg("yes1")
				assert.Regexp(t, `^\d{2}:\d{2} INF yes1\n$`, buffer.String())
				zerolog.Ctx(ctx).Error().Msg("yes2")
				assert.Regexp(t, `^\d{2}:\d{2} INF yes1\n\d{2}:\d{2} ERR yes2\n$`, buffer.String())
			},
			AfterTrigger: func(t *testing.T, buffer *bytes.Buffer) {
				t.Helper()
				assert.Regexp(t, `^\d{2}:\d{2} INF yes1\n\d{2}:\d{2} ERR yes2\n$`, buffer.String())
			},
			ConsoleLevel:     zerolog.InfoLevel,
			ContextLevel:     zerolog.DebugLevel,
			ConditionalLevel: zerolog.DebugLevel,
			TriggerLevel:     zerolog.ErrorLevel,
		},
		{
			Test: func(t *testing.T, ctx context.Context, buffer *bytes.Buffer) {
				t.Helper()
				zerolog.Ctx(ctx).Debug().Msg("no")
				zerolog.Ctx(ctx).Info().Msg("yes1")
				assert.Regexp(t, `^\d{2}:\d{2} INF yes1\n$`, buffer.String())
				zerolog.Ctx(ctx).Error().Msg("yes2")
				assert.Regexp(t, `^\d{2}:\d{2} INF yes1\n\d{2}:\d{2} ERR yes2\n$`, buffer.String())
			},
			AfterTrigger: func(t *testing.T, buffer *bytes.Buffer) {
				t.Helper()
				assert.Regexp(t, `^\d{2}:\d{2} INF yes1\n\d{2}:\d{2} ERR yes2\n$`, buffer.String())
			},
			ConsoleLevel:     zerolog.DebugLevel,
			ContextLevel:     zerolog.InfoLevel,
			ConditionalLevel: zerolog.DebugLevel,
			TriggerLevel:     zerolog.ErrorLevel,
		},
		{
			Test: func(t *testing.T, ctx context.Context, buffer *bytes.Buffer) {
				t.Helper()
				zerolog.Ctx(ctx).Debug().Msg("no")
				zerolog.Ctx(ctx).Info().Msg("yes1")
				assert.Regexp(t, `^\d{2}:\d{2} DBG no\n\d{2}:\d{2} INF yes1\n$`, buffer.String())
				zerolog.Ctx(ctx).Error().Msg("yes2")
				assert.Regexp(t, `^\d{2}:\d{2} DBG no\n\d{2}:\d{2} INF yes1\n\d{2}:\d{2} ERR yes2\n$`, buffer.String())
			},
			AfterTrigger: func(t *testing.T, buffer *bytes.Buffer) {
				t.Helper()
				assert.Regexp(t, `^\d{2}:\d{2} DBG no\n\d{2}:\d{2} INF yes1\n\d{2}:\d{2} ERR yes2\n$`, buffer.String())
			},
			ConsoleLevel:     zerolog.DebugLevel,
			ContextLevel:     zerolog.DebugLevel,
			ConditionalLevel: zerolog.DebugLevel,
			TriggerLevel:     zerolog.DebugLevel,
		},
	} {
		t.Run(fmt.Sprintf("case=%d", k), func(t *testing.T) {
			buffer := new(bytes.Buffer)
			config := z.LoggingConfig{
				Logger:      zerolog.Nop(),
				WithContext: nil,
				Logging: z.Logging{
					Console: z.Console{
						Type:   "nocolor",
						Level:  tt.ConsoleLevel,
						Output: buffer,
					},
					File: z.File{
						Level: zerolog.Disabled,
						Path:  "",
					},
					Main: z.Main{
						Level: zerolog.Disabled,
					},
					Context: z.Context{
						Level:            tt.ContextLevel,
						ConditionalLevel: tt.ConditionalLevel,
						TriggerLevel:     tt.TriggerLevel,
					},
				},
			}
			_, errE := z.New(&config)
			require.NoError(t, errE, "% -+#.1v", errE)
			assert.Equal(t, zerolog.Disabled, config.Logger.GetLevel())
			require.NotNil(t, config.WithContext)
			ctx := context.Background()
			ctx, closeCtx, trigger := config.WithContext(ctx)
			t.Cleanup(closeCtx)
			tt.Test(t, ctx, buffer)
			trigger()
			tt.AfterTrigger(t, buffer)

			buffer.Reset()

			h := z.NewHandler(config.WithContext)(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
				tt.Test(t, req.Context(), buffer)
				panic(nil) //nolint:govet
			}))
			func() {
				defer func() {
					err := recover()
					if !assert.Equal(t, &runtime.PanicNilError{}, err) {
						panic(err)
					}
				}()
				h.ServeHTTP(nil, httptest.NewRequest(http.MethodGet, "/", nil))
			}()
			tt.AfterTrigger(t, buffer)
		})
	}
}

type kongConfig struct {
	z.LoggingConfig
}

func createKong(t *testing.T, expectExit bool, args []string) (kongConfig, bytes.Buffer, *kong.Context, error) {
	t.Helper()

	var buffer bytes.Buffer
	var config kongConfig
	parser := kong.Must(&config,
		kong.UsageOnError(),
		kong.Writers(
			&buffer,
			&buffer,
		),
		kong.Vars{
			"defaultLoggingConsoleType":             z.DefaultConsoleType,
			"defaultLoggingConsoleLevel":            z.DefaultConsoleLevel,
			"defaultLoggingFileLevel":               z.DefaultFileLevel,
			"defaultLoggingMainLevel":               z.DefaultMainLevel,
			"defaultLoggingContextLevel":            z.DefaultContextLevel,
			"defaultLoggingContextConditionalLevel": z.DefaultContextConditionalLevel,
			"defaultLoggingContextTriggerLevel":     z.DefaultContextTriggerLevel,
		},
		z.KongLevelTypeMapper,
		kong.Exit(func(int) {
			t.Helper()
			if !expectExit {
				assert.FailNow(t, "unexpected exit")
			}
		}),
	)
	ctx, err := parser.Parse(args)

	return config, buffer, ctx, err //nolint:wrapcheck
}

func TestKong(t *testing.T) {
	config, buffer, ctx, err := createKong(t, false, []string{"--logging.console.type=nocolor"})
	require.NoError(t, err)
	config.Logging.Console.Output = &buffer
	logFile, errE := z.New(&config)
	defer logFile.Close()
	require.NoError(t, errE)
	config.Logger.Info().Msgf("%s running", ctx.Model.Name)
	assert.Regexp(t, `\d{2}:\d{2} INF zerolog.test running\n`, buffer.String())
}

const expectedUsage = `Usage: zerolog.test

Flags:
  -h, --help                      Show context-sensitive help.
      --logging.console.type=TYPE
                                  Type of console logging. Possible:
                                  color,nocolor,json,disable. Default: color.
      --logging.console.level=LEVEL
                                  Filter out all log entries below the level.
                                  Possible: trace,debug,info,warn,error.
                                  Default: debug.
      --logging.file.path=PATH    Append log entries to a file (as well).
      --logging.file.level=LEVEL
                                  Filter out all log entries below the level.
                                  Possible: trace,debug,info,warn,error.
                                  Default: debug.
  -l, --logging.main.level=LEVEL
                                  Log entries at the level or higher. Possible:
                                  trace,debug,info,warn,error,disabled.
                                  Default: info. Environment variable:
                                  LOGGING_MAIN_LEVEL.
      --logging.context.level=LEVEL
                                  Log entries at the level or higher. Possible:
                                  trace,debug,info,warn,error,disabled. Default:
                                  debug.
      --logging.context.conditional=LEVEL
                                  Buffer log entries at the level and
                                  below until triggered. Possible:
                                  trace,debug,info,warn,error. Default: debug.
      --logging.context.trigger=LEVEL
                                  A log entry at the level or higher triggers.
                                  Possible: trace,debug,info,warn,error.
                                  Default: error.
`

func TestKongUsage(t *testing.T) {
	_, buffer, _, err := createKong(t, true, []string{"--help"})
	require.NoError(t, err)
	assert.Equal(t, expectedUsage, buffer.String())
}
