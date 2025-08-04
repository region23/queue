package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogLevel определяет уровень логирования
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

var levelNames = map[LogLevel]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
	LevelFatal: "FATAL",
}

// Logger представляет структурированный логгер
type Logger struct {
	level  LogLevel
	logger *log.Logger
}

// New создает новый логгер
func New(level LogLevel) *Logger {
	return &Logger{
		level:  level,
		logger: log.New(os.Stdout, "", 0),
	}
}

// SetLevel устанавливает уровень логирования
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// Debug записывает debug сообщение
func (l *Logger) Debug(msg string, fields ...Field) {
	l.log(LevelDebug, msg, fields...)
}

// Info записывает info сообщение
func (l *Logger) Info(msg string, fields ...Field) {
	l.log(LevelInfo, msg, fields...)
}

// Warn записывает warning сообщение
func (l *Logger) Warn(msg string, fields ...Field) {
	l.log(LevelWarn, msg, fields...)
}

// Error записывает error сообщение
func (l *Logger) Error(msg string, fields ...Field) {
	l.log(LevelError, msg, fields...)
}

// Fatal записывает fatal сообщение и завершает программу
func (l *Logger) Fatal(msg string, fields ...Field) {
	l.log(LevelFatal, msg, fields...)
	os.Exit(1)
}

// WithContext возвращает логгер с контекстом
func (l *Logger) WithContext(ctx context.Context) *ContextLogger {
	return &ContextLogger{
		logger: l,
		ctx:    ctx,
	}
}

// WithFields возвращает логгер с предустановленными полями
func (l *Logger) WithFields(fields ...Field) *FieldLogger {
	return &FieldLogger{
		logger: l,
		fields: fields,
	}
}

// log выполняет фактическое логирование
func (l *Logger) log(level LogLevel, msg string, fields ...Field) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelName := levelNames[level]

	// Получить информацию о вызывающем коде
	_, file, line, ok := runtime.Caller(3)
	caller := "unknown"
	if ok {
		caller = fmt.Sprintf("%s:%d", getShortFileName(file), line)
	}

	// Формировать строку с полями
	fieldsStr := ""
	if len(fields) > 0 {
		var parts []string
		for _, field := range fields {
			parts = append(parts, field.String())
		}
		fieldsStr = " " + strings.Join(parts, " ")
	}

	logLine := fmt.Sprintf("[%s] %s %s %s%s",
		timestamp, levelName, caller, msg, fieldsStr)

	l.logger.Println(logLine)
}

// getShortFileName возвращает короткое имя файла
func getShortFileName(file string) string {
	parts := strings.Split(file, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return file
}

// ContextLogger оборачивает логгер с контекстом
type ContextLogger struct {
	logger *Logger
	ctx    context.Context
}

// Debug записывает debug сообщение с контекстом
func (cl *ContextLogger) Debug(msg string, fields ...Field) {
	cl.logger.Debug(msg, fields...)
}

// Info записывает info сообщение с контекстом
func (cl *ContextLogger) Info(msg string, fields ...Field) {
	cl.logger.Info(msg, fields...)
}

// Warn записывает warning сообщение с контекстом
func (cl *ContextLogger) Warn(msg string, fields ...Field) {
	cl.logger.Warn(msg, fields...)
}

// Error записывает error сообщение с контекстом
func (cl *ContextLogger) Error(msg string, fields ...Field) {
	cl.logger.Error(msg, fields...)
}

// Fatal записывает fatal сообщение с контекстом
func (cl *ContextLogger) Fatal(msg string, fields ...Field) {
	cl.logger.Fatal(msg, fields...)
}

// FieldLogger оборачивает логгер с предустановленными полями
type FieldLogger struct {
	logger *Logger
	fields []Field
}

// Debug записывает debug сообщение с предустановленными полями
func (fl *FieldLogger) Debug(msg string, fields ...Field) {
	allFields := append(fl.fields, fields...)
	fl.logger.Debug(msg, allFields...)
}

// Info записывает info сообщение с предустановленными полями
func (fl *FieldLogger) Info(msg string, fields ...Field) {
	allFields := append(fl.fields, fields...)
	fl.logger.Info(msg, allFields...)
}

// Warn записывает warning сообщение с предустановленными полями
func (fl *FieldLogger) Warn(msg string, fields ...Field) {
	allFields := append(fl.fields, fields...)
	fl.logger.Warn(msg, allFields...)
}

// Error записывает error сообщение с предустановленными полями
func (fl *FieldLogger) Error(msg string, fields ...Field) {
	allFields := append(fl.fields, fields...)
	fl.logger.Error(msg, allFields...)
}

// Fatal записывает fatal сообщение с предустановленными полями
func (fl *FieldLogger) Fatal(msg string, fields ...Field) {
	allFields := append(fl.fields, fields...)
	fl.logger.Fatal(msg, allFields...)
}

// Field представляет поле логирования
type Field struct {
	Key   string
	Value interface{}
}

// String возвращает строковое представление поля
func (f Field) String() string {
	return fmt.Sprintf("%s=%v", f.Key, f.Value)
}

// Вспомогательные функции для создания полей
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

func Error(err error) Field {
	return Field{Key: "error", Value: err}
}

func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Глобальный логгер по умолчанию
var defaultLogger = New(LevelInfo)

// Глобальные функции логирования
func Debug(msg string, fields ...Field) {
	defaultLogger.Debug(msg, fields...)
}

func Info(msg string, fields ...Field) {
	defaultLogger.Info(msg, fields...)
}

func Warn(msg string, fields ...Field) {
	defaultLogger.Warn(msg, fields...)
}

func ErrorLog(msg string, fields ...Field) {
	defaultLogger.Error(msg, fields...)
}

func Fatal(msg string, fields ...Field) {
	defaultLogger.Fatal(msg, fields...)
}

func SetLevel(level LogLevel) {
	defaultLogger.SetLevel(level)
}
