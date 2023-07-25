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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-colorable"
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

// Console is configuration of logging human-friendly formatted (and colorized) logs to the console (stdout by default).
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

// File is configuration of logging logs as JSON to a file.
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
// function New and function New returns the logger in its Log field.
type LoggingConfig struct {
	Log     zerolog.Logger `json:"-" kong:"-"       yaml:"-"`
	Logging Logging        `embed:"" json:"logging" prefix:"logging." yaml:"logging"`
}

// filteredWriter writes only logs at Level or above.
type filteredWriter struct {
	Writer zerolog.LevelWriter
	Level  zerolog.Level
}

func (w *filteredWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err == io.EOF { //nolint:errorlint
		// See: https://github.com/golang/go/issues/39155
		return n, io.EOF
	}
	return n, errors.WithStack(err)
}

func (w *filteredWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if level >= w.Level {
		n, err := w.Writer.WriteLevel(level, p)
		if err == io.EOF { //nolint:errorlint
			// See: https://github.com/golang/go/issues/39155
			return n, io.EOF
		}
		return n, errors.WithStack(err)
	}
	return len(p), nil
}

// Copied from zerolog/writer.go.
type levelWriterAdapter struct {
	io.Writer
}

func (lw levelWriterAdapter) WriteLevel(_ zerolog.Level, p []byte) (int, error) {
	n, err := lw.Write(p)
	if err == io.EOF { //nolint:errorlint
		// See: https://github.com/golang/go/issues/39155
		return n, io.EOF
	}
	return n, errors.WithStack(err)
}

// Copied from zerolog/console.go.
func colorize(s interface{}, c int, disabled bool) string {
	if disabled {
		return fmt.Sprintf("%s", s)
	}
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
}

// formatError extracts just the error message from error's JSON.
func formatError(noColor bool) zerolog.Formatter {
	return func(i interface{}) string {
		j, ok := i.([]byte)
		if !ok {
			return colorize("[error: value is not []byte]", colorRed, noColor)
		}
		var e struct {
			Error string `json:"error,omitempty"`
		}
		err := json.Unmarshal(json.RawMessage(j), &e)
		if err != nil {
			return colorize(fmt.Sprintf("[error: %s]", err.Error()), colorRed, noColor)
		}
		return colorize(colorize(e.Error, colorRed, noColor), colorBold, noColor)
	}
}

// Based on zerolog/console.go, but with different colors.
func formatLevel(noColor bool) zerolog.Formatter {
	return func(i interface{}) string {
		var l string
		if ll, ok := i.(string); ok {
			switch ll {
			case zerolog.LevelTraceValue:
				l = colorize("TRC", colorBlue, noColor)
			case zerolog.LevelDebugValue:
				l = "DBG"
			case zerolog.LevelInfoValue:
				l = colorize("INF", colorGreen, noColor)
			case zerolog.LevelWarnValue:
				l = colorize("WRN", colorYellow, noColor)
			case zerolog.LevelErrorValue:
				l = colorize("ERR", colorRed, noColor)
			case zerolog.LevelFatalValue:
				l = colorize("FTL", colorRed, noColor)
			case zerolog.LevelPanicValue:
				l = colorize("PNC", colorRed, noColor)
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

// Based on zerolog/console.go, but formatted to local timezone.
// See: https://github.com/rs/zerolog/pull/415
func formatTimestamp(timeFormat string, noColor bool) zerolog.Formatter {
	if timeFormat == "" {
		timeFormat = time.Kitchen
	}
	return func(i interface{}) string {
		t := "<nil>"
		switch tt := i.(type) {
		case string:
			ts, err := time.Parse(zerolog.TimeFieldFormat, tt)
			if err != nil {
				t = tt
			} else {
				t = ts.Local().Format(timeFormat) //nolint:gosmopolitan
			}
		case json.Number:
			i, err := tt.Int64()
			if err != nil {
				t = tt.String()
			} else {
				var sec, nsec int64 = i, 0
				switch zerolog.TimeFieldFormat {
				case zerolog.TimeFormatUnixMs:
					nsec = int64(time.Duration(i) * time.Millisecond)
					sec = 0
				case zerolog.TimeFormatUnixMicro:
					nsec = int64(time.Duration(i) * time.Microsecond)
					sec = 0
				}
				ts := time.Unix(sec, nsec)
				t = ts.Format(timeFormat)
			}
		}
		return colorize(t, colorDarkGray, noColor)
	}
}

type eventError struct {
	Error string `json:"error,omitempty"`
	Stack []struct {
		Name string `json:"name,omitempty"`
		File string `json:"file,omitempty"`
		Line int    `json:"line,omitempty"`
	} `json:"stack,omitempty"`
	Cause *eventError `json:"cause,omitempty"`
}

type eventWithError struct {
	Error *eventError `json:"error,omitempty"`
	Level string      `json:"level,omitempty"`
}

// consoleWriter writes stack traces for errors after the line with the log.
type consoleWriter struct {
	zerolog.ConsoleWriter
	buf  *bytes.Buffer
	out  io.Writer
	lock sync.Mutex
}

func newConsoleWriter(noColor bool, output *os.File) *consoleWriter {
	buf := &bytes.Buffer{}
	w := zerolog.NewConsoleWriter()
	// Embedded ConsoleWriter writes to a buffer, to which this consoleWriter
	// appends a stack trace and only then writes it to stdout.
	w.Out = buf
	w.NoColor = noColor
	w.TimeFormat = "15:04"
	w.FormatErrFieldValue = formatError(w.NoColor)
	w.FormatLevel = formatLevel(w.NoColor)
	w.FormatTimestamp = formatTimestamp(w.TimeFormat, w.NoColor)

	return &consoleWriter{
		ConsoleWriter: w,
		buf:           buf,
		out:           colorable.NewColorable(output),
		lock:          sync.Mutex{},
	}
}

func makeMessageBold(p []byte) ([]byte, errors.E) {
	var event map[string]interface{}
	d := json.NewDecoder(bytes.NewReader(p))
	d.UseNumber()
	err := d.Decode(&event)
	if err != nil {
		return p, errors.Errorf("cannot decode event: %w", err)
	}

	if event[zerolog.MessageFieldName] == "" || event[zerolog.MessageFieldName] == nil {
		return p, nil
	}

	switch event[zerolog.LevelFieldName] {
	case zerolog.LevelInfoValue, zerolog.LevelWarnValue, zerolog.LevelErrorValue, zerolog.LevelFatalValue, zerolog.LevelPanicValue:
		// Passthrough.
	default:
		return p, nil
	}

	event[zerolog.MessageFieldName] = colorize(fmt.Sprintf("%s", event[zerolog.MessageFieldName]), colorBold, false)
	return x.MarshalWithoutEscapeHTML(event)
}

func (w *consoleWriter) Write(p []byte) (int, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	defer w.buf.Reset()

	// Remember the length before we maybe modify p.
	n := len(p)

	var errE errors.E
	if !w.NoColor {
		p, errE = makeMessageBold(p)
		if errE != nil {
			return 0, errE
		}
	}

	_, err := w.ConsoleWriter.Write(p)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	var event eventWithError
	err = json.Unmarshal(p, &event)
	if err != nil {
		return 0, errors.Errorf("cannot decode event: %w", err)
	}

	level, _ := zerolog.ParseLevel(event.Level)

	// Print a stack trace only on error or above levels.
	if level < zerolog.ErrorLevel {
		_, err = w.buf.WriteTo(w.out)
		return n, errors.WithStack(err)
	}

	ee := event.Error
	first := true
	for ee != nil {
		if !first {
			w.buf.WriteString(colorize("\nthe above error was caused by the following error:\n\n", colorRed, w.NoColor))
			if ee.Error != "" {
				w.buf.WriteString(colorize(colorize(ee.Error, colorRed, w.NoColor), colorBold, w.NoColor))
				w.buf.WriteString("\n")
			}
		}
		first = false
		if len(ee.Stack) > 0 {
			w.buf.WriteString(colorize("stack trace (most recent call first):\n", colorRed, w.NoColor))
			for _, s := range ee.Stack {
				w.buf.WriteString(colorize(s.Name, colorRed, w.NoColor))
				w.buf.WriteString("\n\t")
				w.buf.WriteString(colorize(s.File, colorRed, w.NoColor))
				w.buf.WriteString(colorize(":", colorRed, w.NoColor))
				w.buf.WriteString(colorize(strconv.Itoa(s.Line), colorRed, w.NoColor))
				w.buf.WriteString("\n")
			}
		}
		ee = ee.Cause
	}

	_, err = w.buf.WriteTo(w.out)
	return n, errors.WithStack(err)
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

	return nil, errors.Errorf("logging config not found in struct %T", config)
}

// New configures and initializes zerolog and Go's standard log package for logging.
//
// New expects configuration anywhere nested inside config as a LoggingConfig struct
// and returns the logger in its Log field.
//
// Returned file handle belongs to the file to which logs are appended (if file
// logging is enabled in configuration). Closing it is caller's responsibility.
//
// For details on what all is configured and initialized see package's README.
func New(config interface{}) (*os.File, errors.E) {
	loggingConfig, errE := extractLoggingConfig(config)
	if errE != nil {
		return nil, errors.Errorf("cannot extract logging config: %w", errE)
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
		writers = append(writers, &filteredWriter{
			Writer: levelWriterAdapter{w},
			Level:  loggingConfig.Logging.Console.Level,
		})
		if loggingConfig.Logging.Console.Level < level {
			level = loggingConfig.Logging.Console.Level
		}
	case "json":
		w := output
		writers = append(writers, &filteredWriter{
			Writer: levelWriterAdapter{w},
			Level:  loggingConfig.Logging.Console.Level,
		})
		if loggingConfig.Logging.Console.Level < level {
			level = loggingConfig.Logging.Console.Level
		}
	case "disable":
		// Nothing.
	default:
		return nil, errors.Errorf("invalid console logging type: %s", loggingConfig.Logging.Console.Type)
	}
	if loggingConfig.Logging.File.Path != "" {
		w, err := os.OpenFile(loggingConfig.Logging.File.Path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, fileMode)
		if err != nil {
			return nil, errors.Errorf("cannot open logging file: %w", err)
		}
		file = w
		writers = append(writers, &filteredWriter{
			Writer: levelWriterAdapter{w},
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
	zerolog.InterfaceMarshalFunc = func(v interface{}) ([]byte, error) {
		return x.MarshalWithoutEscapeHTML(v)
	}
	log.Logger = logger
	loggingConfig.Log = logger
	stdlog.SetFlags(0)
	stdlog.SetOutput(logger)

	return file, nil
}
