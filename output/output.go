package output

import "fmt"

type OutputLevel int

const (
	ErrorLevel   OutputLevel = iota
	SilentLevel
	NormalLevel
	VerboseLevel
	DebugLevel
)


var Level OutputLevel = NormalLevel

func IsLevel(l OutputLevel) bool {
	return l <= Level
}

func output(l OutputLevel, s ...interface{}) {
	if Level >= l {
		fmt.Println(s...)
	}
}


func outputf(l OutputLevel, fs string, s ...interface{}) {
	if Level > l {
		fmt.Printf(fs, s...)
	}
}

func Error(s ...interface{}) {	output(ErrorLevel, s...) }
func Errorf(fmt string, s ...interface{}) {	outputf(ErrorLevel, fmt, s...) }

func Warn(s ...interface{}) {	output(SilentLevel, s...) }
func Warnf(fmt string, s ...interface{}) {	outputf(SilentLevel, fmt, s...) }

func Normal(s ...interface{}) {	output(NormalLevel, s...) }
func Normalf(fmt string, s ...interface{}) {	outputf(NormalLevel, fmt, s...) }

func Verbose(s ...interface{}) {	output(VerboseLevel, s...) }
func Verbosef(fmt string, s ...interface{}) {	outputf(VerboseLevel, fmt, s...) }

func Debug(s ...interface{}) {	output(DebugLevel, s...) }
func Debugf(fmt string, s ...interface{}) {	outputf(DebugLevel, fmt, s...) }

