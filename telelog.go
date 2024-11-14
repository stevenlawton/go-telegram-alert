package gotelalert

import (
	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"os"
)

// Log levels
const (
	LogLevelInfo  = "INFO"
	LogLevelError = "ERROR"
	LogLevelFatal = "FATAL"
)

// TeleLogger is the custom logger type that wraps a standard logger and forwards logs to Telegram.
type TeleLogger struct {
	logger   *log.Logger
	bot      *telegram.BotAPI
	chatID   int64
	logLevel string
}

var defaultTeleLogger *TeleLogger

// NewTeleLogger creates a new instance of TeleLogger and sets it as the default logger
func NewTeleLogger(botToken string, chatID int64) error {
	bot, err := telegram.NewBotAPI(botToken)
	if err != nil {
		return err
	}

	defaultTeleLogger = &TeleLogger{
		logger:   log.New(os.Stdout, "", log.LstdFlags),
		bot:      bot,
		chatID:   chatID,
		logLevel: LogLevelInfo, // Default log level is INFO
	}

	// Set default logger output to our TeleLogger
	log.SetOutput(defaultTeleLogger)

	// Set up a command listener to handle log level changes
	go defaultTeleLogger.listenForCommands()

	return nil
}

// Write implements the io.Writer interface to capture log output
func (t *TeleLogger) Write(p []byte) (n int, err error) {
	message := string(p)
	t.forwardToTelegram(message)
	t.logger.Print(message) // Corrected to use Print instead of Output to avoid error
	return len(p), nil
}

// Internal method for forwarding log messages to Telegram
func (t *TeleLogger) forwardToTelegram(message string) {
	if t.bot == nil {
		return // Skip sending if not properly configured
	}

	if t.shouldLogMessage(message) {
		msg := telegram.NewMessage(t.chatID, message)
		_, err := t.bot.Send(msg)
		if err != nil {
			t.logger.Printf("Error forwarding log to Telegram: %v\n", err)
		}
	}
}

// Method to determine if a message should be logged based on the current log level
func (t *TeleLogger) shouldLogMessage(message string) bool {
	switch t.logLevel {
	case LogLevelInfo:
		return true
	case LogLevelError:
		return !(message == "INFO")
	case LogLevelFatal:
		return message == "FATAL"
	default:
		return true
	}
}

// Listen for Telegram bot commands to set log levels
func (t *TeleLogger) listenForCommands() {
	u := telegram.NewUpdate(0)
	u.Timeout = 60

	updates := t.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil { // Check if the update contains a message
			if update.Message.IsCommand() {
				command := update.Message.Command()
				switch command {
				case "set_log_level_info":
					t.logLevel = LogLevelInfo
					t.forwardToTelegram("Log level set to INFO")
				case "set_log_level_error":
					t.logLevel = LogLevelError
					t.forwardToTelegram("Log level set to ERROR")
				case "set_log_level_fatal":
					t.logLevel = LogLevelFatal
					t.forwardToTelegram("Log level set to FATAL")
				default:
					t.forwardToTelegram("Unknown command: " + command)
				}
			}
		}
	}
}

// Example usage
func Example() {
	botToken := "YOUR_TELEGRAM_BOT_TOKEN"
	chatID := int64(-1) // Ensure this is an int64
	err := NewTeleLogger(botToken, chatID)
	if err != nil {
		log.Fatalf("Failed to create TeleLogger: %v", err)
	}

	log.Println("This is an informational message.")
	log.Fatal("This is a fatal error message.")
}
