package tlog

import (
	"fmt"
	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"strings"
	"time"
)

// Config represents the configuration for the logger.
type Config struct {
	LogLevel      string        `json:"log_level"`
	GroupTime     time.Duration `json:"group_time"`
	FilteredWords []string      `json:"filtered_words"`
}

// Pin configuration to the chat
func (t *TelegramLogger) pinConfig() {
	configString := t.generateConfigString()

	chatInfoConfig := telegram.ChatInfoConfig{
		ChatConfig: telegram.ChatConfig{
			ChatID: t.chatID,
		},
	}

	chat, err := t.bot.GetChat(chatInfoConfig)
	if err != nil {
		t.logger.Printf("Error getting chat info: %v", err)
		return
	}

	if chat.PinnedMessage != nil && strings.Contains(chat.PinnedMessage.Text, "Log Level:") {
		// Edit the existing pinned message
		editMsg := telegram.NewEditMessageText(t.chatID, chat.PinnedMessage.MessageID, configString)
		_, err := t.bot.Send(editMsg)
		if err != nil {
			t.logger.Printf("Error editing pinned config message: %v", err)
		}
	} else {
		// Create and pin a new message if no suitable pinned message exists
		msg := telegram.NewMessage(t.chatID, configString)
		msg.DisableNotification = true

		sentMsg, err := t.bot.Send(msg)
		if err != nil {
			t.logger.Printf("Error sending config to chat: %v", err)
			return
		}

		pinConfigMsg := telegram.PinChatMessageConfig{
			ChatID:    t.chatID,
			MessageID: sentMsg.MessageID,
		}
		_, err = t.bot.Request(pinConfigMsg)
		if err != nil {
			t.logger.Printf("Error pinning config message: %v", err)
		}
	}
}

func (t *TelegramLogger) applyPinnedConfig(config *Config) {
	if config != nil {
		t.mutex.Lock()
		defer t.mutex.Unlock()

		// Apply log level configuration
		t.config.LogLevel = config.LogLevel

		// Stop existing ticker and goroutine
		if t.groupingTicker != nil {
			t.groupingTicker.Stop()
			close(t.stopGrouping)
			t.stopGrouping = make(chan struct{})
			t.groupingTicker = nil
		}

		// Apply grouping time configuration
		if config.GroupTime > 0 {
			t.startGroupingTicker(config.GroupTime)
		}

		t.config.GroupTime = config.GroupTime

		// Apply filtered words
		t.config.FilteredWords = config.FilteredWords
	}
}

// Retrieve pinned config from the chat
func getPinnedConfig(bot BotAPI, chatID int64) (*Config, error) {
	chatInfoConfig := telegram.ChatInfoConfig{
		ChatConfig: telegram.ChatConfig{
			ChatID: chatID,
		},
	}

	chat, err := bot.GetChat(chatInfoConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat info: %w", err)
	}

	if chat.PinnedMessage != nil {
		return parseConfigFromMessage(chat.PinnedMessage.Text), nil
	}

	return &Config{
		LogLevel:      LogLevelInfo,
		GroupTime:     0,
		FilteredWords: []string{},
	}, nil
}

// Parse config from a pinned message
func parseConfigFromMessage(message string) *Config {
	var logLevel string
	var groupTimeStr string
	var filteredWords []string

	lines := strings.Split(message, "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "Log Level":
			logLevel = value
		case "Group Time":
			groupTimeStr = value
		case "Filtered Words":
			filteredWords = strings.Split(value, ",")
		}
	}

	groupTime, err := time.ParseDuration(groupTimeStr)
	if err != nil {
		groupTime = 0
	}

	return &Config{
		LogLevel:      logLevel,
		GroupTime:     groupTime,
		FilteredWords: filteredWords,
	}
}

// Generate a human-readable config string for pinning
func (t *TelegramLogger) generateConfigString() string {
	return fmt.Sprintf("Log Level: %s\nGroup Time: %s\nFiltered Words: %v", t.config.LogLevel, t.config.GroupTime, t.config.FilteredWords)
}
