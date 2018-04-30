package output

import (
	"fmt"
	"sync"
)

type OutputLevel int

const (
	ErrorLevel OutputLevel = iota
	SilentLevel
	NormalLevel
	VerboseLevel
	DebugLevel
)

var errorWG sync.WaitGroup = sync.WaitGroup{}
var errorCount int
var errorChannel chan bool = make(chan bool)

func init() {
	errorWG.Add(1)
	go func() {
		defer errorWG.Done()
		for range errorChannel {
			errorCount++
		}
	}()
}

func ErrorCount() int {
	close(errorChannel)
	errorWG.Wait()
	return errorCount
}

var Level OutputLevel = NormalLevel

func IsLevel(l OutputLevel) bool {
	return l <= Level
}

func output(l OutputLevel, s ...interface{}) {
	if l > ErrorLevel {
		errorChannel <- true
	}
	if Level >= l {
		fmt.Println(s...)
	}
}

func outputf(l OutputLevel, fs string, s ...interface{}) {
	if l > ErrorLevel {
		errorChannel <- true
	}
	if Level > l {
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
