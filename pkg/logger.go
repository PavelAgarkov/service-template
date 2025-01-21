package pkg

import (
	"context"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
)

type ctxKey struct{}

type LoggerConfig struct {
	ServiceName string
	LogPath     string
}

func ProvideLoggerConfig(serviceName string, logPath string) LoggerConfig {
	return LoggerConfig{
		ServiceName: "simple_http_server",
		LogPath:     "logs/app.log",
	}
}

// func NewLogger(appName string, logfile string) *zap.Logger {
func NewLogger(cfg LoggerConfig) *zap.Logger {
	stdout := zapcore.AddSync(os.Stdout)

	file := zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.LogPath,
		MaxSize:    5,
		MaxBackups: 10,
		MaxAge:     14,
		Compress:   true,
	})

	// FIXME: change the level based on the config
	level := zap.DebugLevel

	logLevel := zap.NewAtomicLevelAt(level)

	productionCfg := zap.NewProductionEncoderConfig()

	developmentCfg := zap.NewDevelopmentEncoderConfig()
	developmentCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder

	consoleEncoder := zapcore.NewConsoleEncoder(developmentCfg)
	fileEncoder := zapcore.NewJSONEncoder(productionCfg)

	// log to multiple destinations (console and file)
	// extra fields are added to the JSON output alone
	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, stdout, logLevel),
		zapcore.NewCore(fileEncoder, file, logLevel),
	)

	return zap.New(core).With(zap.String("application", cfg.ServiceName))
}

func LoggerFromCtx(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*zap.Logger); ok {
		return l
	}

	return nil
}

func LoggerWithCtx(ctx context.Context, l *zap.Logger) context.Context {
	if lp, ok := ctx.Value(ctxKey{}).(*zap.Logger); ok {
		if lp == l {
			return ctx
		}
	}

	return context.WithValue(ctx, ctxKey{}, l)
}
