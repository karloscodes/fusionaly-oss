package logging

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	Level   string
	Verbose bool
	LogDir  string
	Quiet   bool
	LogFile string // Specify the log file name
}

type Logger struct {
	*logrus.Logger
	config      Config // Store the configuration
	fileLogging bool
}

func NewLogger(config Config) *Logger {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp:       false,      // Enable timestamps for console logs
		TimestampFormat:        "15:04:05", // Use a short time format (HH:MM:SS)
		DisableColors:          false,      // Keep colors for console logs
		DisableQuote:           true,
		ForceColors:            true, // Ensure colors even if output is redirected
		FullTimestamp:          true,
		DisableLevelTruncation: true,
		PadLevelText:           false,
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyLevel: "", // Remove the level prefix
			logrus.FieldKeyMsg:   "", // Remove the msg prefix
			logrus.FieldKeyTime:  "", // We'll prepend the timestamp manually
		},
	})

	switch config.Level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}
	if config.Verbose {
		logger.SetLevel(logrus.DebugLevel)
	}
	if config.Quiet {
		logger.SetLevel(logrus.ErrorLevel)
	}

	return &Logger{
		Logger:      logger,
		config:      config,
		fileLogging: false,
	}
}

func NewFileLogger(config Config) *Logger {
	logger := NewLogger(config)
	logger.fileLogging = true

	logDir := config.LogDir
	if logDir == "" {
		logDir = os.Getenv("LOG_DIR")
		if logDir == "" {
			logDir = "/opt/fusionaly/logs"
		}
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		logger.Errorf("Failed to create log directory %s: %v", logDir, err)
	}

	// Use the LogFile field if specified, otherwise default to fusionaly-cli.log
	logFileName := config.LogFile
	if logFileName == "" {
		logFileName = "fusionaly-cli.log"
	}
	logFile := filepath.Join(logDir, logFileName)

	logger.AddHook(&FileHook{
		Writer: &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		},
		Formatter: &logrus.JSONFormatter{
			TimestampFormat: "15:04:05",
		},
	})

	return logger
}

type FileHook struct {
	Writer    io.Writer
	Formatter logrus.Formatter
}

func (h *FileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *FileHook) Fire(entry *logrus.Entry) error {
	line, err := h.Formatter.Format(entry)
	if err != nil {
		return err
	}
	_, err = h.Writer.Write(line)
	return err
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.Logger.Debugf(format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.Logger.Infof(format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.Logger.Warnf(format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.Logger.Errorf(format, args...)
}

func (l *Logger) Success(format string, args ...interface{}) {
	l.Logger.Infof("✔ "+format, args...)
	if l.fileLogging {
		l.Logger.WithField("status", "success").Infof(format, args...)
	}
}

func (l *Logger) Step(step, total int, format string, args ...interface{}) {
	l.Logger.Infof("➜ Step %d/%d: "+format, append([]interface{}{step, total}, args...)...)
	if l.fileLogging {
		l.Logger.WithFields(logrus.Fields{
			"step":  step,
			"total": total,
		}).Infof(format, args...)
	}
}

func (l *Logger) InfoWithTime(format string, args ...interface{}) {
	// For console output, Logrus's TextFormatter already includes the timestamp
	l.Logger.Infof(format, args...)
}

func (l *Logger) GetVerbose() bool {
	return l.config.Verbose
}

func (l *Logger) GetQuiet() bool {
	return l.config.Quiet
}

func DefaultConfig() Config {
	return Config{
		Level:   "info",
		Verbose: false,
		LogDir:  "",
		Quiet:   false,
		LogFile: "", // Default to empty, will use fusionaly-cli.log
	}
}
