package logger

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/perarneng/getgmail/pkg/interfaces"
)

type ColorLogger struct{}

func NewLogger() interfaces.Logger {
	return &ColorLogger{}
}

func (l *ColorLogger) log(level, message string, colorFunc func(...interface{}) string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("%s %s %s\n", timestamp, colorFunc(level), message)
}

func (l *ColorLogger) Info(message string) {
	l.log("INFO", message, color.New(color.FgGreen).SprintFunc())
}

func (l *ColorLogger) Error(message string) {
	l.log("ERROR", message, color.New(color.FgRed).SprintFunc())
}

func (l *ColorLogger) Warn(message string) {
	l.log("WARN", message, color.New(color.FgYellow).SprintFunc())
}

func (l *ColorLogger) Debug(message string) {
	l.log("DEBUG", message, color.New(color.FgCyan).SprintFunc())
}