// Package zerolog provides opinionated configuration of the https://github.com/rs/zerolog package.
//
// For details on what all is configured and initialized see package's README.
package zerolog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/tozd/go/errors"
	"gitlab.com/tozd/go/x"
	"gopkg.in/yaml.v3"
)

const (
	fileMode = 0o600
)

// Copied from zerolog/console.go.
const (
	colorRed = iota + 31
	colorGreen
	colorYellow
	colorBlue

	colorBold     = 1
	colorDarkGray = 90
)

const (
	DefaultConsoleType  = "color"
	DefaultConsoleLevel = "info"
	DefaulFileLevel     = "info"
)

const TimeFieldFormat = "2006-01-02T15:04:05.000Z07:00"

var LevelColors = map[string]int{ //nolint:gochecknoglobals
	"TRC": colorBlue,
	"DBG": 0,
	"INF": colorGreen,
	"WRN": colorYellow,
	"ERR": colorRed,
	"FTL": colorRed,
	"PNC": colorRed,
}

// Console is configuration of logging logs to the console (stdout by default).
//
// Type can be the following values: color (human-friendly formatted and colorized),
// nocolor (just human-friendly formatted), json, disable (do not log to the console).
//
// Level can be trace, debug, info, warn, and error.
//
//nolint:lll
type Console struct {
	Type   string        `default:"${defaultLoggingConsoleType}"  enum:"color,nocolor,json,disable"  help:"Type of console logging. Possible: ${enum}. Default: ${defaultLoggingConsoleType}."                                                                   json:"type"  placeholder:"TYPE"  yaml:"type"`
	Level  zerolog.Level `default:"${defaultLoggingConsoleLevel}" enum:"trace,debug,info,warn,error" help:"All logs with a level greater than or equal to this level will be written to the console. Possible: ${enum}. Default: ${defaultLoggingConsoleLevel}." json:"level" placeholder:"LEVEL" short:"l"   yaml:"level"`
	Output *os.File      `json:"-"                                kong:"-"                           yaml:"-"`
}

func (c *Console) UnmarshalYAML(value *yaml.Node) error {
	var tmp struct {
		Type  string `yaml:"type"`
		Level string `yaml:"level"`
	}

	// TODO: Limit only to known fields.
	//       See: https://github.com/go-yaml/yaml/issues/460
	err := value.Decode(&tmp)
	if err != nil {
		return errors.WithStack(err)
	}
	level, err := zerolog.ParseLevel(tmp.Level)
	if err != nil {
		return errors.WithStack(err)
	}

	c.Type = tmp.Type
	c.Level = level

	return nil
}

func (c *Console) UnmarshalJSON(b []byte) error {
	var tmp struct {
		Type  string `json:"type"`
		Level string `json:"level"`
	}

	errE := x.UnmarshalWithoutUnknownFields(b, &tmp)
	if errE != nil {
		return errE
	}
	level, err := zerolog.ParseLevel(tmp.Level)
	if err != nil {
		return errors.WithStack(err)
	}

	c.Type = tmp.Type
	c.Level = level

	return nil
}

// File is configuration of logging logs as JSON by appending them to a file at path.
//
// Level can be trace, debug, info, warn, and error.
//
//nolint:lll
type File struct {
	Path  string        `help:"Append logs to a file (as well)." json:"path"                        placeholder:"PATH"                                                                                                                                    type:"path"  yaml:"path"`
	Level zerolog.Level `default:"${defaultLoggingFileLevel}"    enum:"trace,debug,info,warn,error" help:"All logs with a level greater than or equal to this level will be written to the file. Possible: ${enum}. Default: ${defaultLoggingFileLevel}." json:"level" placeholder:"LEVEL" yaml:"level"`
}

func (f *File) UnmarshalYAML(value *yaml.Node) error {
	var tmp struct {
		Path  string `yaml:"path"`
		Level string `yaml:"level"`
	}

	// TODO: Limit only to known fields.
	//       See: https://github.com/go-yaml/yaml/issues/460
	err := value.Decode(&tmp)
	if err != nil {
		return errors.WithStack(err)
	}
	level, err := zerolog.ParseLevel(tmp.Level)
	if err != nil {
		return errors.WithStack(err)
	}

	f.Path = tmp.Path
	f.Level = level

	return nil
}

func (f *File) UnmarshalJSON(b []byte) error {
	var tmp struct {
		Path  string `json:"path"`
		Level string `json:"level"`
	}

	errE := x.UnmarshalWithoutUnknownFields(b, &tmp)
	if errE != nil {
		return errE
	}
	level, err := zerolog.ParseLevel(tmp.Level)
	if err != nil {
		return errors.WithStack(err)
	}

	f.Path = tmp.Path
	f.Level = level

	return nil
}

// Logging is configuration for console and file logging.
type Logging struct {
	Console Console `embed:"" json:"console" prefix:"console." yaml:"console"`
	File    File    `embed:"" json:"file"    prefix:"file."    yaml:"file"`
}

// LoggingConfig struct can be provided anywhere inside the config argument to
// function New and function New returns the logger in its Logger field.
type LoggingConfig struct {
	Logger  zerolog.Logger `json:"-" kong:"-"       yaml:"-"`
	Logging Logging        `embed:"" json:"logging" prefix:"logging." yaml:"logging"`
}

// Based on zerolog/console.go, but made only for strings and with c==0 condition.
func colorize(s string, c int, disabled bool) string {
	if disabled || c == 0 {
		return s
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", c, s)
}

// formatError extracts just the error message from error's JSON.
//
// Stack trace is written out separately in formatExtra.
func formatError(noColor bool) zerolog.Formatter {
	return func(i interface{}) string {
		j, ok := i.([]byte)
		if !ok {
			return colorize("[error: value is not []byte]", colorRed, noColor)
		}
		err, errE := errors.UnmarshalJSON(j)
		if errE != nil {
			return colorize(fmt.Sprintf("[error: %s]", errE.Error()), colorRed, noColor)
		}
		return colorize(colorize(err.Error(), colorBold, noColor), colorRed, noColor)
	}
}

// formatExtra extracts stack trace from the error and adds it after the current log line in the buffer.
func formatExtra(noColor bool) func(map[string]interface{}, *bytes.Buffer) error {
	return func(event map[string]interface{}, buf *bytes.Buffer) error {
		eData, ok := event[zerolog.ErrorFieldName]
		if !ok {
			return nil
		}

		if event[zerolog.LevelFieldName] == nil {
			return nil
		}

		l, ok := event[zerolog.LevelFieldName].(string)
		if !ok {
			return nil
		}

		level, err := zerolog.ParseLevel(l)
		if err != nil {
			return errors.WithStack(err)
		}

		// Print a stack trace only on error or above levels.
		if level < zerolog.ErrorLevel {
			return nil
		}

		eJSON, errE := x.Marshal(eData)
		if errE != nil {
			return errE
		}

		e, errE := errors.UnmarshalJSON(eJSON)
		if errE != nil {
			return errE
		}

		formatter := errors.Formatter{
			Error: e,
			GetMessage: func(err error) string {
				// We want error messages to be bold, recursively.
				return colorize(err.Error(), colorBold, noColor)
			},
		}

		// " " if the message format makes sure that the string ends with a newline.
		message := fmt.Sprintf("% v", formatter)
		full := fmt.Sprintf("% -+#.1v", formatter)
		// Message has already been included in formatError.
		full = strings.TrimPrefix(full, message)

		if len(full) == 0 {
			return nil
		}

		// Zerolog always adds a newline at the end.
		// So we add one ourselves now and remove one from "full".
		buf.WriteString("\n")
		full = strings.TrimSuffix(full, "\n")
		lines := strings.Split(full, "\n")
		for i, line := range lines {
			if len(line) > 0 {
				buf.WriteString(colorize(line, colorRed, noColor))
			}
			if i < len(lines)-1 {
				// We to not write a newline for the last line.
				// Zerolog always adds a newline at the end.
				buf.WriteString("\n")
			}
		}

		return nil
	}
}

// Based on zerolog/console.go, but with different colors.
func formatLevel(noColor bool) zerolog.Formatter {
	return func(i interface{}) string {
		var l string
		if ll, ok := i.(string); ok {
			switch ll {
			case zerolog.LevelTraceValue:
				l = colorize("TRC", LevelColors["TRC"], noColor)
			case zerolog.LevelDebugValue:
				l = colorize("DBG", LevelColors["DBG"], noColor)
			case zerolog.LevelInfoValue:
				l = colorize("INF", LevelColors["INF"], noColor)
			case zerolog.LevelWarnValue:
				l = colorize("WRN", LevelColors["WRN"], noColor)
			case zerolog.LevelErrorValue:
				l = colorize("ERR", LevelColors["ERR"], noColor)
			case zerolog.LevelFatalValue:
				l = colorize("FTL", LevelColors["FTL"], noColor)
			case zerolog.LevelPanicValue:
				l = colorize("PNC", LevelColors["PNC"], noColor)
			default:
				l = "???"
			}
		} else {
			if i == nil {
				l = "???"
			} else {
				l = strings.ToUpper(fmt.Sprintf("%s", i))[0:3]
			}
		}
		return l
	}
}

// We use FormatPrepare to make the message bold only at info level and above.
// FormatMessage does not have access to the level.
func formatPrepare(noColor bool) func(map[string]interface{}) error {
	return func(event map[string]interface{}) error {
		if event[zerolog.MessageFieldName] == "" || event[zerolog.MessageFieldName] == nil {
			return nil
		}

		switch event[zerolog.LevelFieldName] {
		case zerolog.LevelInfoValue, zerolog.LevelWarnValue, zerolog.LevelErrorValue, zerolog.LevelFatalValue, zerolog.LevelPanicValue:
			// Passthrough.
		default:
			return nil
		}

		event[zerolog.MessageFieldName] = colorize(fmt.Sprintf("%s", event[zerolog.MessageFieldName]), colorBold, noColor)

		return nil
	}
}

// consoleWriter writes stack traces for errors after the line with the log.
type consoleWriter struct {
	zerolog.ConsoleWriter
}

func newConsoleWriter(noColor bool, output *os.File) *consoleWriter {
	w := zerolog.NewConsoleWriter()
	w.Out = output
	w.NoColor = noColor
	w.TimeFormat = "15:04"
	w.FormatErrFieldValue = formatError(w.NoColor)
	w.FormatLevel = formatLevel(w.NoColor)
	w.FormatExtra = formatExtra(w.NoColor)
	w.FormatPrepare = formatPrepare(w.NoColor)

	return &consoleWriter{
		ConsoleWriter: w,
	}
}

func extractLoggingConfig(config interface{}) (*LoggingConfig, errors.E) {
	configType := reflect.TypeOf(LoggingConfig{}) //nolint:exhaustruct
	val := reflect.ValueOf(config).Elem()
	typ := val.Type()
	if typ == configType {
		return val.Addr().Interface().(*LoggingConfig), nil //nolint:forcetypeassert
	}
	fields := reflect.VisibleFields(typ)
	for _, field := range fields {
		if field.Type == configType {
			return val.FieldByIndex(field.Index).Addr().Interface().(*LoggingConfig), nil //nolint:forcetypeassert
		}
	}

	errE := errors.New("logging config not found in struct")
	errors.Details(errE)["type"] = fmt.Sprintf("%T", config)
	return nil, errE
}

// New configures and initializes zerolog and Go's standard log package for logging.
//
// New expects configuration anywhere nested inside config as a LoggingConfig struct
// and returns the logger in its Logger field.
//
// Returned file handle belongs to the file to which logs are appended (if file
// logging is enabled in configuration). Closing it is caller's responsibility.
//
// For details on what all is configured and initialized see package's README.
func New(config interface{}) (*os.File, errors.E) {
	loggingConfig, errE := extractLoggingConfig(config)
	if errE != nil {
		return nil, errors.WithMessage(errE, "cannot extract logging config")
	}

	level := zerolog.Disabled
	writers := []io.Writer{}
	output := loggingConfig.Logging.Console.Output
	if output == nil {
		output = os.Stdout
	}
	var file *os.File
	switch loggingConfig.Logging.Console.Type {
	case "color", "nocolor":
		w := newConsoleWriter(loggingConfig.Logging.Console.Type == "nocolor", output)
		writers = append(writers, &zerolog.FilteredLevelWriter{
			Writer: zerolog.LevelWriterAdapter{Writer: w},
			Level:  loggingConfig.Logging.Console.Level,
		})
		if loggingConfig.Logging.Console.Level < level {
			level = loggingConfig.Logging.Console.Level
		}
	case "json":
		w := output
		writers = append(writers, &zerolog.FilteredLevelWriter{
			Writer: zerolog.LevelWriterAdapter{Writer: w},
			Level:  loggingConfig.Logging.Console.Level,
		})
		if loggingConfig.Logging.Console.Level < level {
			level = loggingConfig.Logging.Console.Level
		}
	case "disable":
		// Nothing.
	default:
		errE := errors.New("invalid console logging type")
		errors.Details(errE)["value"] = loggingConfig.Logging.Console.Type
		return nil, errE
	}
	if loggingConfig.Logging.File.Path != "" {
		w, err := os.OpenFile(loggingConfig.Logging.File.Path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, fileMode)
		if err != nil {
			return nil, errors.WithMessage(err, "cannot open logging file")
		}
		file = w
		writers = append(writers, &zerolog.FilteredLevelWriter{
			Writer: zerolog.LevelWriterAdapter{Writer: w},
			Level:  loggingConfig.Logging.File.Level,
		})
		if loggingConfig.Logging.Console.Level < level {
			level = loggingConfig.Logging.File.Level
		}
	}

	writer := zerolog.MultiLevelWriter(writers...)
	logger := zerolog.New(writer).Level(level).With().Timestamp().Logger()

	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().UTC()
	}
	zerolog.TimeFieldFormat = TimeFieldFormat
	zerolog.ErrorMarshalFunc = func(ee error) interface{} { //nolint:reassign
		if ee == nil {
			return json.RawMessage("null")
		}

		var j []byte
		var err error
		switch e := ee.(type) { //nolint:errorlint
		case interface {
			MarshalJSON() ([]byte, error)
		}:
			j, err = e.MarshalJSON()
		default:
			j, err = x.MarshalWithoutEscapeHTML(struct {
				Error string `json:"error,omitempty"`
			}{
				Error: ee.Error(),
			})
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "marshaling error \"%s\" into JSON during logging failed: %s\n", ee.Error(), err.Error())
		}
		return json.RawMessage(j)
	}
	// See: https://github.com/rs/zerolog/pull/568
	zerolog.InterfaceMarshalFunc = func(v interface{}) ([]byte, error) {
		return x.MarshalWithoutEscapeHTML(v)
	}
	log.Logger = logger
	loggingConfig.Logger = logger
	stdlog.SetFlags(0)
	stdlog.SetOutput(logger)

	return file, nil
}

var KongLevelTypeMapper = kong.TypeMapper(reflect.TypeOf(zerolog.Level(0)), kong.MapperFunc(func(ctx *kong.DecodeContext, target reflect.Value) error {
	var l string
	err := ctx.Scan.PopValueInto("level", &l)
	if err != nil {
		return err
	}
	level, err := zerolog.ParseLevel(l)
	if err != nil {
		return errors.WithStack(err)
	}
	target.Set(reflect.ValueOf(level))
	return nil
}))
