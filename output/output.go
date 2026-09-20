package output

import (
	"fmt"
	"sync/atomic"
)

type OutputLevel int

const (
	ErrorLevel OutputLevel = iota
	SilentLevel
	NormalLevel
	VerboseLevel
	DebugLevel
)

// errorCount counts genuine errors, and only those: main uses it to decide the
// process exit status.
//
// This was a channel fed by a counting goroutine, which had two faults.  The
// guard was `l >= ErrorLevel` and ErrorLevel is iota, i.e. 0, so *every*
// leveled call incremented it -- including Debug calls the level gate
// suppressed -- and locksmith therefore exited 1 on every invocation, which
// makes it unusable from a script, a cron job or a CI step.  And ErrorCount()
// closed the channel, so calling it twice, or emitting any output afterwards,
// panicked.  An atomic has neither problem and is less code.
var errorCount atomic.Int64

// ErrorCount returns the number of errors reported so far.  It is safe to call
// more than once.
func ErrorCount() int {
	return int(errorCount.Load())
}

var Level OutputLevel = NormalLevel

func IsLevel(l OutputLevel) bool {
	return l <= Level
}

func output(l OutputLevel, s ...interface{}) {
	if l == ErrorLevel {
		errorCount.Add(1)
	}
	if Level >= l {
		fmt.Println(s...)
	}
}

func outputf(l OutputLevel, fs string, s ...interface{}) {
	if l == ErrorLevel {
		errorCount.Add(1)
	}
	if Level >= l {
		fmt.Printf(fs, s...)
	}
}

func Error(s ...interface{})              { output(ErrorLevel, s...) }
func Errorf(fmt string, s ...interface{}) { outputf(ErrorLevel, fmt, s...) }

func Warn(s ...interface{})              { output(SilentLevel, s...) }
func Warnf(fmt string, s ...interface{}) { outputf(SilentLevel, fmt, s...) }

func Normal(s ...interface{})              { output(NormalLevel, s...) }
func Normalf(fmt string, s ...interface{}) { outputf(NormalLevel, fmt, s...) }

func Verbose(s ...interface{})              { output(VerboseLevel, s...) }
func Verbosef(fmt string, s ...interface{}) { outputf(VerboseLevel, fmt, s...) }

func Debug(s ...interface{})              { output(DebugLevel, s...) }
func Debugf(fmt string, s ...interface{}) { outputf(DebugLevel, fmt, s...) }
