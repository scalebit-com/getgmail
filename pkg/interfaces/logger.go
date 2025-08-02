package interfaces

type Logger interface {
	Info(message string)
	Error(message string)
	Warn(message string)
	Debug(message string)
}