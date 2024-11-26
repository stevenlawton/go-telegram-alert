package tlog

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Log levels
const (
	LogLevelDebug = "DEBUG"
	LogLevelInfo  = "INFO"
	LogLevelError = "ERROR"
	LogLevelFatal = "FATAL"
	LogLevelAlert = "ALERT"
)

// BotAPI defines the methods used from the telegram.BotAPI.
type BotAPI interface {
	Send(c telegram.Chattable) (telegram.Message, error)
	GetUpdatesChan(u telegram.UpdateConfig) telegram.UpdatesChannel
	GetChat(config telegram.ChatInfoConfig) (telegram.Chat, error)
	Request(c telegram.Chattable) (*telegram.APIResponse, error)
}

// TelegramLogger is the custom logger type that wraps a standard logger and forwards logs to Telegram.
type TelegramLogger struct {
	logger         *log.Logger
	bot            BotAPI
	chatID         int64
	appName        string
	mutedUntil     time.Time
	config         Config
	groupedLogs    []string
	groupingTicker *time.Ticker
	stopGrouping   chan struct{}
	mutex          sync.Mutex
}

var defaultTeleLogger *TelegramLogger

// NewTLog creates a new instance of TelegramLogger and sets it as the default logger
func NewTLog(botToken string, chatID int64, appName string) error {
	bot, err := telegram.NewBotAPI(botToken)
	if err != nil {
		return err
	}
	return newTLogWithBot(bot, chatID, appName)
}

// newTLogWithBot allows injecting a custom BotAPI implementation (useful for testing)
func newTLogWithBot(bot BotAPI, chatID int64, appName string) error {
	// Load config from pinned message if available
	pinnedConfig, err := getPinnedConfig(bot, chatID)
	if err != nil {
		return err
	}

	defaultTeleLogger = &TelegramLogger{
		logger:       log.New(os.Stdout, "", log.LstdFlags),
		bot:          bot,
		chatID:       chatID,
		appName:      appName,
		config:       *pinnedConfig,
		groupedLogs:  []string{},
		stopGrouping: make(chan struct{}),
		mutex:        sync.Mutex{},
	}

	// Apply the loaded configuration
	defaultTeleLogger.applyPinnedConfig(pinnedConfig)

	// Set default logger output to our TelegramLogger
	log.SetOutput(defaultTeleLogger)

	// Set up a command listener to handle log level changes
	go defaultTeleLogger.listenForCommands()

	// Start log grouping process if GroupTime is greater than 0
	if pinnedConfig.GroupTime > 0 {
		defaultTeleLogger.startGroupingTicker(pinnedConfig.GroupTime)
	}

	defaultTeleLogger.setBotCommands()

	return nil
}

// startGroupingTicker starts the log grouping ticker for a given duration.
func (t *TelegramLogger) startGroupingTicker(groupTime time.Duration) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Stop existing ticker and goroutine if running
	if t.groupingTicker != nil {
		t.groupingTicker.Stop()
		close(t.stopGrouping)
		t.stopGrouping = make(chan struct{})
		t.groupingTicker = nil
	}

	// Do not start ticker if groupTime is zero or negative
	if groupTime <= 0 {
		return
	}

	t.groupingTicker = time.NewTicker(groupTime)
	go t.groupLogsSender()
}

// Write implements the io.Writer interface to capture log output
func (t *TelegramLogger) Write(p []byte) (n int, err error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	message := string(p)

	if t.isMuted() || t.isFiltered(message) {
		return len(p), nil
	}

	if t.shouldLogMessage(message) {
		if t.shouldGroupLogs() {
			t.groupedLogs = append(t.groupedLogs, message)
		} else {
			t.forwardToTelegram(message)
		}
	}

	t.logger.Print(message)
	return len(p), nil
}

// Internal method for forwarding log messages to Telegram
func (t *TelegramLogger) forwardToTelegram(message string) {
	if t.bot == nil {
		t.logger.Println("DEBUG: Bot is not properly configured.")
		return
	}

	var emoji string
	var disableNotification bool
	switch {
	case strings.HasPrefix(message, "DEBUG:"):
		emoji = "🐞"
		disableNotification = true
	case strings.HasPrefix(message, "INFO:"):
		emoji = "ℹ️"
		disableNotification = true
	case strings.HasPrefix(message, "ERROR:"):
		emoji = "⚠️"
		disableNotification = true
	case strings.HasPrefix(message, "FATAL:"):
		emoji = "❌"
		disableNotification = false
	case strings.HasPrefix(message, "ALERT:"):
		emoji = "🚨"
		disableNotification = false
	default:
		emoji = ""
		disableNotification = true
	}

	msg := telegram.NewMessage(t.chatID, fmt.Sprintf("[%s] %s %s", t.appName, emoji, message))
	msg.DisableNotification = disableNotification
	_, err := t.bot.Send(msg)
	if err != nil {
		t.logger.Printf("Error forwarding log to Telegram: %v\n", err)
	}
}

// Method to determine if a message should be logged based on the current log level
func (t *TelegramLogger) shouldLogMessage(message string) bool {
	switch t.config.LogLevel {
	case LogLevelDebug:
		return true
	case LogLevelInfo:
		return !strings.HasPrefix(message, "DEBUG:")
	case LogLevelError:
		return strings.HasPrefix(message, "ERROR:") || strings.HasPrefix(message, "FATAL:") || strings.HasPrefix(message, "ALERT:")
	case LogLevelFatal:
		return strings.HasPrefix(message, "FATAL:")
	case LogLevelAlert:
		return strings.HasPrefix(message, "ALERT:") || strings.HasPrefix(message, "FATAL:")
	default:
		return true
	}
}

// Listen for Telegram bot commands to set log levels and other configurations
func (t *TelegramLogger) listenForCommands() {
	u := telegram.NewUpdate(0)
	u.Timeout = 60

	updates := t.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			if update.Message.IsCommand() {
				t.handleCommand(update)
			}
		}
	}
}

// Handle individual commands for the logger
func (t *TelegramLogger) handleCommand(update telegram.Update) {
	command := update.Message.Command()
	args := update.Message.CommandArguments()

	switch command {
	case "set_log_level_info":
		t.config.LogLevel = LogLevelInfo
		t.pinConfig()
		t.forwardToTelegram("Log level set to INFO")
	case "set_log_level_error":
		t.config.LogLevel = LogLevelError
		t.pinConfig()
		t.forwardToTelegram("Log level set to ERROR")
	case "set_log_level_fatal":
		t.config.LogLevel = LogLevelFatal
		t.pinConfig()
		t.forwardToTelegram("Log level set to FATAL")
	case "set_log_level_alert":
		t.config.LogLevel = LogLevelAlert
		t.pinConfig()
		t.forwardToTelegram("Log level set to ALERT")
	case "mute_notifications":
		duration, err := time.ParseDuration(args)
		if err == nil {
			t.mutedUntil = time.Now().Add(duration)
			t.forwardToTelegram(fmt.Sprintf("Notifications muted for %s", duration))
		} else {
			t.forwardToTelegram("Invalid duration format. Example: 10m, 1h")
		}
	case "filter_word":
		if args != "" {
			t.config.FilteredWords = append(t.config.FilteredWords, args)
			t.pinConfig()
			t.forwardToTelegram(fmt.Sprintf("Filtered word added: %s", args))
		} else {
			t.forwardToTelegram("Please provide a word to filter.")
		}
	case "set_group_time":
		groupTime, err := time.ParseDuration(args)
		if err == nil {
			t.mutex.Lock()
			t.config.GroupTime = groupTime
			t.pinConfig()
			t.mutex.Unlock()

			t.startGroupingTicker(groupTime)

			if groupTime > 0 {
				t.forwardToTelegram(fmt.Sprintf("Log grouping time set to %s", groupTime))
			} else {
				t.forwardToTelegram("Log grouping disabled")
			}
		} else {
			t.forwardToTelegram("Invalid duration format. Example: 10m, 1h")
		}
	default:
		t.forwardToTelegram("Unknown command: " + command)
	}
}

// Check if notifications are muted
func (t *TelegramLogger) isMuted() bool {
	return time.Now().Before(t.mutedUntil)
}

// Check if a message should be filtered
func (t *TelegramLogger) isFiltered(message string) bool {
	for _, word := range t.config.FilteredWords {
		if strings.Contains(strings.ToLower(message), strings.ToLower(word)) {
			return true
		}
	}
	return false
}

// Check if a message should be grouped
func (t *TelegramLogger) shouldGroupLogs() bool {
	return t.config.GroupTime > 0 && t.groupingTicker != nil
}

// Send grouped logs periodically
func (t *TelegramLogger) groupLogsSender() {
	for {
		select {
		case <-t.groupingTicker.C:
			t.mutex.Lock()
			if len(t.groupedLogs) > 0 {
				groupedMessage := strings.Join(t.groupedLogs, "\n")
				t.forwardToTelegram(fmt.Sprintf("Grouped Logs:\n%s", groupedMessage))
				t.groupedLogs = []string{} // Clear grouped logs after sending
			}
			t.mutex.Unlock()
		case <-t.stopGrouping:
			return
		}
	}
}

// Convenience logging methods
func Debug(format string, v ...any) {
	defaultTeleLogger.Debug(format, v...)
}

func Info(format string, v ...any) {
	defaultTeleLogger.Info(format, v...)
}

func Error(format string, v ...any) {
	defaultTeleLogger.Error(format, v...)
}

func Alert(format string, v ...any) {
	defaultTeleLogger.Alert(format, v...)
}

func Fatal(format string, v ...any) {
	defaultTeleLogger.Fatal(format, v...)
}

// TeleLogger logging methods
func (t *TelegramLogger) Debug(format string, v ...any) {
	logMessage := fmt.Sprintf("DEBUG: %s", fmt.Sprintf(format, v...))
	t.Write([]byte(logMessage))
}

func (t *TelegramLogger) Info(format string, v ...any) {
	logMessage := fmt.Sprintf("INFO: %s", fmt.Sprintf(format, v...))
	t.Write([]byte(logMessage))
}

func (t *TelegramLogger) Error(format string, v ...any) {
	logMessage := fmt.Sprintf("ERROR: %s", fmt.Sprintf(format, v...))
	t.Write([]byte(logMessage))
}

func (t *TelegramLogger) Alert(format string, v ...any) {
	logMessage := fmt.Sprintf("ALERT: %s", fmt.Sprintf(format, v...))
	t.Write([]byte(logMessage))
}

func (t *TelegramLogger) Fatal(format string, v ...any) {
	logMessage := fmt.Sprintf("FATAL: %s", fmt.Sprintf(format, v...))
	t.Write([]byte(logMessage))
	os.Exit(1)
}

func (t *TelegramLogger) setBotCommands() {
	commands := []telegram.BotCommand{
		{Command: "set_log_level_info", Description: "Set the log level to INFO (receive all messages)"},
		{Command: "set_log_level_error", Description: "Set the log level to ERROR (receive only ERROR, ALERT, and FATAL messages)"},
		{Command: "set_log_level_fatal", Description: "Set the log level to FATAL (receive only FATAL messages)"},
		{Command: "set_log_level_alert", Description: "Set the log level to ALERT (receive only ALERT and FATAL messages)"},
		{Command: "mute_notifications", Description: "Mute all notifications for a specified duration (e.g., 10m or 1h)"},
		{Command: "filter_word", Description: "Add a word to the filter list to prevent specific messages from being sent to Telegram"},
		{Command: "set_group_time", Description: "Set the log grouping time (0 to disable grouping, e.g., 10m, 1h)"},
	}

	config := telegram.NewSetMyCommands(commands...)
	_, err := t.bot.Request(config)
	if err != nil {
		t.logger.Printf("Error setting bot commands: %v\n", err)
	}
}

// Example usage
func _() {
	botToken := "YOUR_TELEGRAM_BOT_TOKEN"
	chatID := int64(-1) // Ensure this is an int64
	appName := "MyApp"

	err := NewTLog(botToken, chatID, appName)
	if err != nil {
		log.Fatalf("FATAL: Failed to create TeleLogger: %v", err)
	}

	// Using the new convenience functions from tlog
	Info("This is an informational message.")
	Error("This is an error message.")
	Alert("This is an alert message.")
	Fatal("This is a fatal error message.") // Will exit after logging

	// Or use the standard library directly
	log.Println("INFO: This is a standard log informational message.")
	log.Println("ERROR: This is a standard log error message.")
}
