package zerolog

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/tozd/go/errors"
)

// TODO: Remove code below when corresponding PRs get merged.
//       See: https://github.com/rs/zerolog/pull/599
//       See: https://github.com/rs/zerolog/pull/602

const (
	colorGreen = iota + 32
	colorYellow
	colorBlue
)

var LevelColors = map[zerolog.Level]int{
	zerolog.TraceLevel: colorBlue,
	zerolog.DebugLevel: 0,
	zerolog.InfoLevel:  colorGreen,
	zerolog.WarnLevel:  colorYellow,
	zerolog.ErrorLevel: colorRed,
	zerolog.FatalLevel: colorRed,
	zerolog.PanicLevel: colorRed,
}

var FormattedLevels = map[zerolog.Level]string{
	zerolog.TraceLevel: "TRC",
	zerolog.DebugLevel: "DBG",
	zerolog.InfoLevel:  "INF",
	zerolog.WarnLevel:  "WRN",
	zerolog.ErrorLevel: "ERR",
	zerolog.FatalLevel: "FTL",
	zerolog.PanicLevel: "PNC",
}

func formatLevel(noColor bool) zerolog.Formatter {
	return func(i interface{}) string {
		var l string
		if ll, ok := i.(string); ok {
			level, _ := zerolog.ParseLevel(ll)
			fl, ok := FormattedLevels[level]
			if ok {
				l = colorize(fl, LevelColors[level], noColor)
			} else {
				l = strings.ToUpper(ll)[0:3]
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

var triggerWriterPool = &sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// newTriggerLevelWriter returns a new TriggerLevelWriter.
//
// It obtains a buffer from the pool and you must
// call Close to return the buffer to the pool.
func newTriggerLevelWriter(w io.Writer, conditionalLevel, triggerLevel zerolog.Level) *triggerLevelWriter {
	return &triggerLevelWriter{
		Writer:           w,
		ConditionalLevel: conditionalLevel,
		TriggerLevel:     triggerLevel,
		buf:              triggerWriterPool.Get().(*bytes.Buffer),
	}
}

// triggerLevelWriter buffers log lines at the ConditionalLevel or below
// until a trigger level (or higher) line is emitted. Log lines with level
// higher than ConditionalLevel are always written out to the destination
// writer. If trigger never happens, buffered log lines are never written out.
//
// It can be used to configure "log level per request". You should create a
// new triggerLevelWriter (using NewTriggerLevelWriter) per request.
type triggerLevelWriter struct {
	// Destination writer. If LevelWriter is provided (usually), its WriteLevel is used
	// instead of Write.
	io.Writer

	// ConditionalLevel is the level (and below) at which lines are buffered until
	// a trigger level (or higher) line is emitted. Usually this is set to DebugLevel.
	ConditionalLevel zerolog.Level

	// TriggerLevel is the lowest level that triggers the sending of the conditional
	// level lines. Usually this is set to ErrorLevel.
	TriggerLevel zerolog.Level

	buf       *bytes.Buffer
	triggered bool
	mu        sync.Mutex
}

func (w *triggerLevelWriter) WriteLevel(l zerolog.Level, p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.buf == nil {
		return 0, errors.New("invalid writer")
	}

	// At first trigger level or above log line, we flush the buffer and change the
	// trigger state to triggered.
	if !w.triggered && l >= w.TriggerLevel {
		err := w.trigger()
		if err != nil {
			return 0, err
		}
	}

	// Unless triggered, we buffer everything at and below ConditionalLevel.
	if !w.triggered && l <= w.ConditionalLevel {
		// We prefix each log line with a byte with the level.
		// Hopefully we will never have a level value which equals a newline
		// (which could interfere with reconstruction of log lines in the trigger method).
		w.buf.WriteByte(byte(l))
		w.buf.Write(p)
		return len(p), nil
	}

	// Anything above ConditionalLevel is always passed through.
	// Once triggered, everything is passed through.
	if lw, ok := w.Writer.(zerolog.LevelWriter); ok {
		return lw.WriteLevel(l, p)
	}
	return w.Write(p)
}

// trigger expects lock to be held.
func (w *triggerLevelWriter) trigger() error {
	if w.triggered {
		return nil
	}
	w.triggered = true
	defer w.buf.Reset()

	p := w.buf.Bytes()
	for len(p) > 0 {
		// We do not use bufio.Scanner here because we already have full buffer
		// in the memory and we do not want extra copying from the buffer to
		// scanner's token slice, nor we want to hit scanner's token size limit,
		// and we also want to preserve newlines.
		i := bytes.IndexByte(p, '\n')
		line := p[0 : i+1]
		p = p[i+1:]
		// We prefixed each log line with a byte with the level.
		level := zerolog.Level(line[0])
		line = line[1:]
		var err error
		if lw, ok := w.Writer.(zerolog.LevelWriter); ok {
			_, err = lw.WriteLevel(level, line)
		} else {
			_, err = w.Write(line)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// Trigger forces flushing the buffer and change the trigger state to
// triggered, if the writer has not already been triggered before.
func (w *triggerLevelWriter) Trigger() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.buf == nil {
		return errors.New("invalid writer")
	}

	return w.trigger()
}

// Close closes the writer and returns the buffer to the pool.
func (w *triggerLevelWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.buf == nil {
		return nil
	}

	w.buf.Reset()
	triggerWriterPool.Put(w.buf)
	w.buf = nil

	return nil
}
