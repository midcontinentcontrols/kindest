package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger interface {
	Info(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Debug(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	With(fields ...zap.Field) Logger
}

type fakeLogger struct{}

func (l *fakeLogger) Info(msg string, fields ...zap.Field)  {}
func (l *fakeLogger) Error(msg string, fields ...zap.Field) {}
func (l *fakeLogger) Debug(msg string, fields ...zap.Field) {}
func (l *fakeLogger) Warn(msg string, fields ...zap.Field)  {}
func (l *fakeLogger) With(fields ...zap.Field) Logger       { return l }

func NewFakeLogger() Logger {
	return &fakeLogger{}
}

type zapLogger struct {
	log *zap.Logger
}

func (l *zapLogger) Info(msg string, fields ...zap.Field)  { l.log.Info(msg, fields...) }
func (l *zapLogger) Error(msg string, fields ...zap.Field) { l.log.Error(msg, fields...) }
func (l *zapLogger) Debug(msg string, fields ...zap.Field) { l.log.Debug(msg, fields...) }
func (l *zapLogger) Warn(msg string, fields ...zap.Field)  { l.log.Warn(msg, fields...) }
func (l *zapLogger) With(fields ...zap.Field) Logger       { return NewZapLogger(l.log.With(fields...)) }

func NewZapLogger(log *zap.Logger) Logger {
	return &zapLogger{log}
}

func NewZapLoggerFromEnv() Logger {
	atom := zap.NewAtomicLevel()
	if debug, ok := os.LookupEnv("DEBUG"); ok && debug != "0" {
		atom.SetLevel(zap.DebugLevel)
	} else if logLevel, ok := os.LookupEnv("LOG_LEVEL"); ok {
		switch logLevel {
		case "info":
			atom.SetLevel(zap.InfoLevel)
		case "debug":
			atom.SetLevel(zap.DebugLevel)
		case "warn":
			atom.SetLevel(zap.WarnLevel)
		case "error":
			atom.SetLevel(zap.ErrorLevel)
		case "dpanic":
			atom.SetLevel(zap.DPanicLevel)
		case "panic":
			atom.SetLevel(zap.PanicLevel)
		case "fatal":
			atom.SetLevel(zap.FatalLevel)
		default:
			panic(fmt.Sprintf("unknown log level '%s'", logLevel))
		}
	}
	encoderCfg := zap.NewProductionEncoderConfig()
	return NewZapLogger(zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	)))
}
