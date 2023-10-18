// Package zerolog provides opinionated configuration of the [zerolog] package.
//
// For details on what all is configured and initialized see package's README.
//
// [zerolog]: https://github.com/rs/zerolog
package zerolog

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"reflect"
	"strconv"
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
	colorRed  = iota + 31
	colorBold = 1
)

// Defaults to be used with [Kong]
// initialization for LoggingConfig struct:
//
//	kong.Vars{
//		"defaultLoggingConsoleType":             DefaultConsoleType,
//		"defaultLoggingConsoleLevel":            DefaultConsoleLevel,
//		"defaultLoggingFileLevel":               DefaultFileLevel,
//		"defaultLoggingMainLevel":               DefaultMainLevel,
//		"defaultLoggingContextLevel":            DefaultContextLevel,
//		"defaultLoggingContextConditionalLevel": DefaultContextConditionalLevel,
//		"defaultLoggingContextTriggerLevel":     DefaultContextTriggerLevel,
//	}
//
// [Kong]: https://github.com/alecthomas/kong
const (
	DefaultConsoleType             = "color"
	DefaultConsoleLevel            = "debug"
	DefaultFileLevel               = "debug"
	DefaultMainLevel               = "info"
	DefaultContextLevel            = "debug"
	DefaultContextConditionalLevel = "debug"
	DefaultContextTriggerLevel     = "error"
)

// TimeFieldFormat is the format for timestamps in log entries.
const TimeFieldFormat = "2006-01-02T15:04:05.000Z07:00"

// Console is configuration of logging log entries to the console (stdout by default).
//
// Type can be the following values: color (human-friendly formatted and colorized),
// nocolor (just human-friendly formatted), json, disable (do not log to the console).
//
// Level can be trace, debug, info, warn, and error.
//
//nolint:lll
type Console struct {
	Type   string        `default:"${defaultLoggingConsoleType}"  enum:"color,nocolor,json,disable"  help:"Type of console logging. Possible: ${enum}. Default: ${defaultLoggingConsoleType}."                     json:"type"  placeholder:"TYPE"  yaml:"type"`
	Level  zerolog.Level `default:"${defaultLoggingConsoleLevel}" enum:"trace,debug,info,warn,error" help:"Filter out all log entries below the level. Possible: ${enum}. Default: ${defaultLoggingConsoleLevel}." json:"level" placeholder:"LEVEL" yaml:"level"`
	Output io.Writer     `json:"-"                                kong:"-"                           yaml:"-"`
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

// File is configuration of logging log entries as JSON by appending them to a file at path.
//
// Level can be trace, debug, info, warn, and error.
//
//nolint:lll
type File struct {
	Path  string        `help:"Append log entries to a file (as well)." json:"path"                        placeholder:"PATH"                                                                                         type:"path"  yaml:"path"`
	Level zerolog.Level `default:"${defaultLoggingFileLevel}"           enum:"trace,debug,info,warn,error" help:"Filter out all log entries below the level. Possible: ${enum}. Default: ${defaultLoggingFileLevel}." json:"level" placeholder:"LEVEL" yaml:"level"`
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

// Main is configuration of the main logger.
//
// Level can be trace, debug, info, warn, and error.
// Level can be also disabled to disable main logger.
//
//nolint:lll
type Main struct {
	Level zerolog.Level `default:"${defaultLoggingMainLevel}" enum:"trace,debug,info,warn,error,disabled" help:"Log entries at the level or higher. Possible: ${enum}. Default: ${defaultLoggingContextLevel}." json:"level" placeholder:"LEVEL" short:"l" yaml:"level"`
}

func (m *Main) UnmarshalYAML(value *yaml.Node) error {
	var tmp struct {
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

	m.Level = level

	return nil
}

func (m *Main) UnmarshalJSON(b []byte) error {
	var tmp struct {
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

	m.Level = level

	return nil
}

// Context is configuration of the context logger.
//
// Levels can be trace, debug, info, warn, and error.
// Level can be also disabled to disable context logger.
//
// It supports buffering log lines at the ConditionalLevel or below until triggered by a log
// entry at the TriggerLevel or higher. To disable this behavior, set Level and TriggerLevel
// to the same level.
//
//nolint:lll
type Context struct {
	Level            zerolog.Level `default:"${defaultLoggingContextLevel}"            enum:"trace,debug,info,warn,error,disabled" help:"Log entries at the level or higher. Possible: ${enum}. Default: ${defaultLoggingContextLevel}."                                   json:"level"            placeholder:"LEVEL" yaml:"level"`
	ConditionalLevel zerolog.Level `default:"${defaultLoggingContextConditionalLevel}" enum:"trace,debug,info,warn,error"          help:"Buffer log entries at the level and below until triggered. Possible: ${enum}. Default: ${defaultLoggingContextConditionalLevel}." json:"conditionalLevel" name:"conditional"  placeholder:"LEVEL" yaml:"conditionalLevel"`
	TriggerLevel     zerolog.Level `default:"${defaultLoggingContextTriggerLevel}"     enum:"trace,debug,info,warn,error"          help:"A log entry at the level or higher triggers. Possible: ${enum}. Default: ${defaultLoggingContextTriggerLevel}."                   json:"triggerLevel"     name:"trigger"      placeholder:"LEVEL" yaml:"triggerLevel"`
}

func (c *Context) UnmarshalYAML(value *yaml.Node) error {
	var tmp struct {
		Level            string `yaml:"level"`
		ConditionalLevel string `yaml:"conditionalLevel"`
		TriggerLevel     string `yaml:"triggerLevel"`
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
	conditionalLevel, err := zerolog.ParseLevel(tmp.ConditionalLevel)
	if err != nil {
		return errors.WithStack(err)
	}
	triggerLevel, err := zerolog.ParseLevel(tmp.TriggerLevel)
	if err != nil {
		return errors.WithStack(err)
	}

	c.Level = level
	c.ConditionalLevel = conditionalLevel
	c.TriggerLevel = triggerLevel

	return nil
}

func (c *Context) UnmarshalJSON(b []byte) error {
	var tmp struct {
		Level            string `json:"level"`
		ConditionalLevel string `json:"conditionalLevel"`
		TriggerLevel     string `json:"triggerLevel"`
	}

	errE := x.UnmarshalWithoutUnknownFields(b, &tmp)
	if errE != nil {
		return errE
	}
	level, err := zerolog.ParseLevel(tmp.Level)
	if err != nil {
		return errors.WithStack(err)
	}
	conditionalLevel, err := zerolog.ParseLevel(tmp.ConditionalLevel)
	if err != nil {
		return errors.WithStack(err)
	}
	triggerLevel, err := zerolog.ParseLevel(tmp.TriggerLevel)
	if err != nil {
		return errors.WithStack(err)
	}

	c.Level = level
	c.ConditionalLevel = conditionalLevel
	c.TriggerLevel = triggerLevel

	return nil
}

// Logging is configuration for console and file logging.
type Logging struct {
	Console Console `embed:"" json:"console" prefix:"console." yaml:"console"`
	File    File    `embed:"" json:"file"    prefix:"file."    yaml:"file"`
	Main    Main    `embed:"" json:"main"    prefix:"main."    yaml:"main"`
	Context Context `embed:"" json:"context" prefix:"context." yaml:"context"`
}

// LoggingConfig struct can be provided anywhere inside the config argument to
// function New and function New returns the logger in its Logger field and
// sets its WithContext field.
type LoggingConfig struct {
	Logger      zerolog.Logger                                          `json:"-" kong:"-"       yaml:"-"`
	WithContext func(context.Context) (context.Context, func(), func()) `json:"-" kong:"-"       yaml:"-"`
	Logging     Logging                                                 `embed:"" json:"logging" prefix:"logging." yaml:"logging"`
}

// Copied from zerolog/console.go.
func needsQuote(s string) bool {
	for i := range s {
		if s[i] < 0x20 || s[i] > 0x7e || s[i] == ' ' || s[i] == '\\' || s[i] == '"' {
			return true
		}
	}
	return false
}

// Copied from zerolog/console.go.
func colorize(s interface{}, c int, disabled bool) string {
	e := os.Getenv("NO_COLOR")
	if e != "" || c == 0 {
		disabled = true
	}

	if disabled {
		return fmt.Sprintf("%s", s)
	}
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
}

// formatError formats errors which have been marshaled into JSON object
// using gitlab.com/tozd/go/errors's Formatter.
//
// It extracts just the error message from error's JSON.
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
		msg := err.Error()
		if needsQuote(msg) {
			msg = strconv.Quote(msg)
		}
		return colorize(colorize(msg, colorBold, noColor), colorRed, noColor)
	}
}

// formatExtra extracts details from the error (it the error exists)
// and formats them after the current log line in the buffer.
//
// On error and above levels it also formats error's stack trace
// and recurses into joined and cause errors.
//
// The error message itself is extracted in formatError.
//
// The error should have been marshaled into JSON object using
// gitlab.com/tozd/go/errors's Formatter.
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

		format := "%#v"
		if level >= zerolog.ErrorLevel {
			format = "% -+#.1v"
		}

		// " " if the message format makes sure that the string ends with a newline.
		message := fmt.Sprintf("% v", formatter)
		full := fmt.Sprintf(format, formatter)
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
				// We do not write a newline for the last line.
				// Zerolog always adds a newline at the end.
				buf.WriteString("\n")
			}
		}

		return nil
	}
}

// newConsoleWriter creates and initializes a new ConsoleWriter with 24-hour time
// format and formatting of errors which have been marshaled into JSON object
// using gitlab.com/tozd/go/errors's Formatter.
func newConsoleWriter(noColor bool, output io.Writer) *zerolog.ConsoleWriter {
	w := zerolog.NewConsoleWriter()
	w.Out = output
	w.NoColor = noColor
	w.TimeFormat = "15:04"
	w.FormatErrFieldValue = formatError(w.NoColor)
	w.FormatExtra = formatExtra(w.NoColor)
	w.FormatLevel = formatLevel(w.NoColor)

	return &w
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
// and returns the logger in its Logger field and sets its WithContext field.
// LoggingConfig can be initially populated with configuration using [Kong].
//
// Returned file handle belongs to the file to which log entries are appended (if file
// logging is enabled in configuration). Closing it is caller's responsibility.
//
// For details on what all is configured and initialized see package's README.
//
// [Kong]: https://github.com/alecthomas/kong
func New(config interface{}) (*os.File, errors.E) {
	loggingConfig, errE := extractLoggingConfig(config)
	if errE != nil {
		return nil, errors.WithMessage(errE, "cannot extract logging config")
	}

	minOutputLevel := zerolog.Disabled
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
		if loggingConfig.Logging.Console.Level < minOutputLevel {
			minOutputLevel = loggingConfig.Logging.Console.Level
		}
	case "json":
		w := output
		writers = append(writers, &zerolog.FilteredLevelWriter{
			Writer: zerolog.LevelWriterAdapter{Writer: w},
			Level:  loggingConfig.Logging.Console.Level,
		})
		if loggingConfig.Logging.Console.Level < minOutputLevel {
			minOutputLevel = loggingConfig.Logging.Console.Level
		}
	case "disable":
		// Nothing.
	default:
		errE = errors.New("invalid console logging type")
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
		if loggingConfig.Logging.Console.Level < minOutputLevel {
			minOutputLevel = loggingConfig.Logging.File.Level
		}
	}

	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().UTC()
	}
	zerolog.TimeFieldFormat = TimeFieldFormat
	// Marshal errors into JSON as an object and not a string
	// using gitlab.com/tozd/go/errors's Formatter.
	zerolog.ErrorMarshalFunc = func(ee error) interface{} { //nolint:reassign
		j, err := x.MarshalWithoutEscapeHTML(errors.Formatter{Error: ee}) //nolint:exhaustruct
		if err != nil {
			fmt.Fprintf(os.Stderr, `zerolog: marshaling error "%s" into JSON failed: % -+#.1v`, ee.Error(), err)
		}
		return json.RawMessage(j)
	}
	// See: https://github.com/rs/zerolog/pull/568
	zerolog.InterfaceMarshalFunc = func(v interface{}) ([]byte, error) {
		return x.MarshalWithoutEscapeHTML(v)
	}
	zerolog.ErrorHandler = func(err error) { //nolint:reassign
		fmt.Fprintf(os.Stderr, "zerolog: could not write event: % -+#.1v", errors.Formatter{Error: err}) //nolint:exhaustruct
	}

	writer := zerolog.MultiLevelWriter(writers...)

	mainLogger := zerolog.Nop()
	mainLoggerLevel := max(minOutputLevel, loggingConfig.Logging.Main.Level)
	if len(writers) > 0 && mainLoggerLevel < zerolog.Disabled {
		mainLogger = zerolog.New(writer).Level(mainLoggerLevel).With().Timestamp().Logger()
	}

	log.Logger = mainLogger
	loggingConfig.Logger = mainLogger
	stdlog.SetFlags(0)
	stdlog.SetOutput(mainLogger)

	ctxLoggerLevel := max(minOutputLevel, loggingConfig.Logging.Context.Level)
	if len(writers) > 0 && ctxLoggerLevel < zerolog.Disabled {
		loggingConfig.WithContext = func(ctx context.Context) (context.Context, func(), func()) {
			w := newTriggerLevelWriter(
				writer,
				loggingConfig.Logging.Context.ConditionalLevel,
				loggingConfig.Logging.Context.TriggerLevel,
			)
			ctxLogger := zerolog.New(w).Level(ctxLoggerLevel).With().Timestamp().Logger()
			closeCtx := func() {
				_ = w.Close()
			}
			trigger := func() {
				_ = w.Trigger()
			}
			return ctxLogger.WithContext(ctx), closeCtx, trigger
		}
	} else {
		loggingConfig.WithContext = func(ctx context.Context) (context.Context, func(), func()) {
			ctxLogger := zerolog.Nop()
			closeCtx := func() {}
			trigger := func() {}
			return ctxLogger.WithContext(ctx), closeCtx, trigger
		}
	}

	return file, errE
}

// We initialize kongLevelTypeMapper here so that whole definition does not end
// up in documentation.
var kongLevelTypeMapper = kong.TypeMapper( //nolint:gochecknoglobals
	reflect.TypeOf(zerolog.Level(0)),
	kong.MapperFunc(func(ctx *kong.DecodeContext, target reflect.Value) error {
		var l string
		err := ctx.Scan.PopValueInto("level", &l)
		if err != nil {
			return errors.WithStack(err)
		}
		level, err := zerolog.ParseLevel(l)
		if err != nil {
			return errors.WithStack(err)
		}
		target.Set(reflect.ValueOf(level))
		return nil
	}),
)

// KongLevelTypeMapper should be used with [Kong]
// initialization so that logging levels found in LoggingConfig struct can be
// correctly parsed on populated.
//
// [Kong]: https://github.com/alecthomas/kong
//
//nolint:gochecknoglobals
var KongLevelTypeMapper = kongLevelTypeMapper

// PrettyLog reads JSON lines from the input and writes to the output
// pretty-printed console lines using ConsoleWriter configured and
// initialized in the same way as New does.
func PrettyLog(noColor bool, input io.Reader, output io.Writer) errors.E {
	// First we initialize global zerolog configuration by calling zerolog.New
	// with configuration with all logging disabled.
	config := LoggingConfig{ //nolint:exhaustruct
		Logging: Logging{ //nolint:exhaustruct
			Console: Console{ //nolint:exhaustruct
				Type: "disable",
			},
		},
	}
	_, errE := New(&config)
	if errE != nil {
		return errE
	}

	writer := newConsoleWriter(noColor, output)

	// Writer expects a whole line at once, so we
	// use a scanner to read input line by line.
	scanner := bufio.NewScanner(input)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) > 0 {
			_, err := writer.Write(line)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				// We have on purpose an empty line between the error and the line.
				fmt.Fprintf(os.Stderr, "error: % -+#.1v\n%s\n", errors.Formatter{Error: err}, line) //nolint:exhaustruct
			}
		}
	}

	err := scanner.Err()
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
