package tlog

import (
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MockBotAPI is a mock implementation of the BotAPI interface
type MockBotAPI struct {
	mu           sync.Mutex
	sentMessages []telegram.Chattable
	updatesChan  chan telegram.Update
}

func (m *MockBotAPI) Send(c telegram.Chattable) (telegram.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMessages = append(m.sentMessages, c)
	return telegram.Message{}, nil
}

func (m *MockBotAPI) GetUpdatesChan(u telegram.UpdateConfig) telegram.UpdatesChannel {
	return m.updatesChan
}

func (m *MockBotAPI) GetChat(config telegram.ChatInfoConfig) (telegram.Chat, error) {
	// Return a chat with no pinned message for simplicity
	return telegram.Chat{}, nil
}

func (m *MockBotAPI) Request(c telegram.Chattable) (*telegram.APIResponse, error) {
	// For setting bot commands and pinning messages
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMessages = append(m.sentMessages, c)
	return &telegram.APIResponse{Ok: true}, nil
}

// Test Initialization of the TelegramLogger
func TestNewTLog(t *testing.T) {
	// Arrange
	chatID := int64(123456)
	appName := "TestApp"

	// Use the mock bot
	mockBot := &MockBotAPI{
		sentMessages: []telegram.Chattable{},
		updatesChan:  make(chan telegram.Update),
	}

	// Act
	err := newTLogWithBot(mockBot, chatID, appName)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if defaultTeleLogger == nil {
		t.Error("Expected defaultTeleLogger to be initialized")
	}

	if defaultTeleLogger.appName != appName {
		t.Errorf("Expected appName %s, got %s", appName, defaultTeleLogger.appName)
	}
}

// Test logging at different levels
func TestLoggingLevels(t *testing.T) {
	// Arrange
	chatID := int64(123456)
	appName := "TestApp"

	// Use the mock bot
	mockBot := &MockBotAPI{
		sentMessages: []telegram.Chattable{},
		updatesChan:  make(chan telegram.Update),
	}

	// Initialize the logger with the mock bot
	err := newTLogWithBot(mockBot, chatID, appName)
	if err != nil {
		t.Fatalf("Failed to initialize tlog: %v", err)
	}

	// Set log level to INFO
	defaultTeleLogger.config.LogLevel = LogLevelInfo

	// Act
	Debug("This is a debug message")
	Info("This is an info message")
	Error("This is an error message")
	Alert("This is an alert message")

	// Assert
	mockBot.mu.Lock()
	defer mockBot.mu.Unlock()

	if len(mockBot.sentMessages) != 3 {
		t.Errorf("Expected 3 messages to be sent, got %d", len(mockBot.sentMessages))
	}

	// Check that the debug message was not sent
	for _, msg := range mockBot.sentMessages {
		c, ok := msg.(telegram.MessageConfig)
		if !ok {
			continue
		}
		if strings.Contains(c.Text, "DEBUG") {
			t.Errorf("Did not expect DEBUG message to be sent at INFO level")
		}
	}
}

// Test configuration changes via commands
func TestHandleCommand(t *testing.T) {
	// Arrange
	chatID := int64(123456)
	appName := "TestApp"
	mockBot := &MockBotAPI{
		sentMessages: []telegram.Chattable{},
		updatesChan:  make(chan telegram.Update),
	}

	// Initialize the logger with the mock bot
	defaultTeleLogger = &TelegramLogger{
		logger:       log.New(os.Stdout, "", log.LstdFlags),
		bot:          mockBot,
		chatID:       chatID,
		appName:      appName,
		config:       Config{LogLevel: LogLevelInfo},
		groupedLogs:  []string{},
		stopGrouping: make(chan struct{}),
		mutex:        sync.Mutex{},
	}

	// Create a command update to change log level to ERROR
	update := telegram.Update{
		Message: &telegram.Message{
			Text: "/set_log_level_error",
			Chat: &telegram.Chat{
				ID: chatID,
			},
			From: &telegram.User{
				ID: 987654,
			},
			Entities: []telegram.MessageEntity{
				{
					Type:   "bot_command",
					Offset: 0,
					Length: len("/set_log_level_error"),
				},
			},
		},
	}

	// Act
	defaultTeleLogger.handleCommand(update)

	// Assert
	if defaultTeleLogger.config.LogLevel != LogLevelError {
		t.Errorf("Expected log level to be ERROR, got %s", defaultTeleLogger.config.LogLevel)
	}

	// Verify that a confirmation message was sent
	mockBot.mu.Lock()
	defer mockBot.mu.Unlock()

	foundConfirmation := false
	for _, msg := range mockBot.sentMessages {
		c, ok := msg.(telegram.MessageConfig)
		if !ok {
			continue
		}
		if strings.Contains(c.Text, "Log level set to ERROR") {
			foundConfirmation = true
			break
		}
	}
	if !foundConfirmation {
		t.Error("Expected confirmation message for log level change")
	}
}

// Test that logs are correctly grouped when GroupTime is set
func TestLogGrouping(t *testing.T) {
	// Arrange
	chatID := int64(123456)
	appName := "TestApp"
	mockBot := &MockBotAPI{
		sentMessages: []telegram.Chattable{},
		updatesChan:  make(chan telegram.Update),
	}

	// Initialize the logger with the mock bot and short group time
	defaultTeleLogger = &TelegramLogger{
		logger:       log.New(os.Stdout, "", log.LstdFlags),
		bot:          mockBot,
		chatID:       chatID,
		appName:      appName,
		config:       Config{LogLevel: LogLevelInfo, GroupTime: 100 * time.Millisecond},
		groupedLogs:  []string{},
		stopGrouping: make(chan struct{}),
		mutex:        sync.Mutex{},
	}

	defaultTeleLogger.startGroupingTicker(100 * time.Millisecond)
	defer func() {
		defaultTeleLogger.groupingTicker.Stop()
		close(defaultTeleLogger.stopGrouping)
	}()

	// Act
	Info("First message")
	Info("Second message")

	// Wait for group time to elapse
	time.Sleep(200 * time.Millisecond)

	// Assert
	mockBot.mu.Lock()
	defer mockBot.mu.Unlock()

	if len(mockBot.sentMessages) == 0 {
		t.Error("Expected grouped message to be sent")
	}

	// Check that messages were grouped
	foundGroupedMessage := false
	for _, msg := range mockBot.sentMessages {
		c, ok := msg.(telegram.MessageConfig)
		if !ok {
			continue
		}
		if strings.Contains(c.Text, "Grouped Logs:") {
			if strings.Contains(c.Text, "First message") && strings.Contains(c.Text, "Second message") {
				foundGroupedMessage = true
				break
			}
		}
	}

	if !foundGroupedMessage {
		t.Error("Expected grouped log messages to be sent")
	}
}

// Test that logs are sent immediately when GroupTime is zero
func TestLogGroupingDisabled(t *testing.T) {
	// Arrange
	chatID := int64(123456)
	appName := "TestApp"
	mockBot := &MockBotAPI{
		sentMessages: []telegram.Chattable{},
		updatesChan:  make(chan telegram.Update),
	}

	// Initialize the logger with the mock bot and zero group time
	defaultTeleLogger = &TelegramLogger{
		logger:       log.New(os.Stdout, "", log.LstdFlags),
		bot:          mockBot,
		chatID:       chatID,
		appName:      appName,
		config:       Config{LogLevel: LogLevelInfo, GroupTime: 0},
		groupedLogs:  []string{},
		stopGrouping: make(chan struct{}),
		mutex:        sync.Mutex{},
	}

	// Act
	Info("Immediate message")

	// Assert
	mockBot.mu.Lock()
	defer mockBot.mu.Unlock()

	if len(mockBot.sentMessages) == 0 {
		t.Error("Expected message to be sent immediately")
	}

	// Check that message was sent immediately and not grouped
	for _, msg := range mockBot.sentMessages {
		c, ok := msg.(telegram.MessageConfig)
		if !ok {
			continue
		}
		if strings.Contains(c.Text, "Grouped Logs:") {
			t.Error("Did not expect grouped logs when grouping is disabled")
		}
		if strings.Contains(c.Text, "Immediate message") {
			// Success
			return
		}
	}

	t.Error("Expected immediate message to be sent")
}

// Test filtering of messages
func TestMessageFiltering(t *testing.T) {
	// Arrange
	chatID := int64(123456)
	appName := "TestApp"
	mockBot := &MockBotAPI{
		sentMessages: []telegram.Chattable{},
		updatesChan:  make(chan telegram.Update),
	}

	// Initialize the logger with the mock bot and add a filtered word
	defaultTeleLogger = &TelegramLogger{
		logger:       log.New(os.Stdout, "", log.LstdFlags),
		bot:          mockBot,
		chatID:       chatID,
		appName:      appName,
		config:       Config{LogLevel: LogLevelInfo, FilteredWords: []string{"secret"}},
		groupedLogs:  []string{},
		stopGrouping: make(chan struct{}),
		mutex:        sync.Mutex{},
	}

	// Act
	Info("This is a normal message")
	Info("This message contains a secret")

	// Assert
	mockBot.mu.Lock()
	defer mockBot.mu.Unlock()

	if len(mockBot.sentMessages) == 1 {
		c, ok := mockBot.sentMessages[0].(telegram.MessageConfig)
		if !ok {
			t.Error("Expected MessageConfig type")
		}
		if strings.Contains(c.Text, "secret") {
			t.Error("Did not expect message containing filtered word to be sent")
		}
	} else {
		t.Errorf("Expected 1 message to be sent, got %d", len(mockBot.sentMessages))
	}
}

// Test mute notifications
func TestMuteNotifications(t *testing.T) {
	// Arrange
	chatID := int64(123456)
	appName := "TestApp"
	mockBot := &MockBotAPI{
		sentMessages: []telegram.Chattable{},
		updatesChan:  make(chan telegram.Update),
	}

	// Initialize the logger with the mock bot
	defaultTeleLogger = &TelegramLogger{
		logger:       log.New(os.Stdout, "", log.LstdFlags),
		bot:          mockBot,
		chatID:       chatID,
		appName:      appName,
		config:       Config{LogLevel: LogLevelInfo},
		groupedLogs:  []string{},
		stopGrouping: make(chan struct{}),
		mutex:        sync.Mutex{},
	}

	// Act
	defaultTeleLogger.mutedUntil = time.Now().Add(100 * time.Millisecond)
	Info("This message should be muted")

	// Wait for mute to expire
	time.Sleep(150 * time.Millisecond)
	Info("This message should be sent")

	// Assert
	mockBot.mu.Lock()
	defer mockBot.mu.Unlock()

	if len(mockBot.sentMessages) != 1 {
		t.Errorf("Expected 1 message to be sent after mute expires, got %d", len(mockBot.sentMessages))
	}

	c, ok := mockBot.sentMessages[0].(telegram.MessageConfig)
	if !ok {
		t.Error("Expected MessageConfig type")
	}

	if !strings.Contains(c.Text, "This message should be sent") {
		t.Error("Expected message to be sent after mute expires")
	}
}
