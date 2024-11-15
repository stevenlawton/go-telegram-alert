# go-telegram-log-alert (gotelalert)

`gotelalert` is a custom Go logger that wraps the standard logger and forwards log messages to a specified Telegram chat. This allows you to easily receive important log messages via Telegram, making it ideal for remote monitoring of applications.

## Features

- Log messages with different severity levels (`INFO`, `ERROR`, `FATAL`).
- Forward logs to a Telegram chat.
- Control log levels via Telegram commands.
- Uses standard `log` package functions (`log.Println`, `log.Fatal`, etc.).

## Installation

To install the library, use:

```sh
go get github.com/stevenlawton/go-telegram-alert
```

## Usage

Here is an example of how to use `gotelalert` in your Go project.

```go
package main

import (
	"log"
	"github.com/stevenlawton/go-telegram-alert"
)

func main() {
	botToken := "YOUR_TELEGRAM_BOT_TOKEN"
	chatID := int64(YOUR_CHAT_ID) // Ensure this is an int64

	err := gotelalert.NewTeleLogger(botToken, chatID)
	if err != nil {
		log.Fatalf("Failed to create TeleLogger: %v", err)
	}

	// Now use log functions as usual
	log.Println("INFO: This is an informational message.")
	log.Println("ERROR: This is an error message.")
	log.Fatal("FATAL: This is a fatal error message.")
}
```

### Setting Log Levels via Telegram

You can control the verbosity of the logs sent to Telegram by using the following commands in your Telegram chat:

- `/set_log_level_info` - Sets the log level to `INFO` (default).
- `/set_log_level_error` - Sets the log level to `ERROR` (only `ERROR` and `FATAL` logs will be sent).
- `/set_log_level_fatal` - Sets the log level to `FATAL` (only `FATAL` logs will be sent).

## API

### `NewTeleLogger(botToken string, chatID int64) error`

Initializes the Telegram logger and sets it as the default logger. You must provide your bot's token and the chat ID where you want the logs to be sent.

### Logging at Appropriate Levels

After initializing the logger with `NewTeleLogger`, use the standard `log` functions (`log.Println`, `log.Printf`, `log.Fatal`, etc.) to generate log messages. To ensure the log messages are categorized properly:

- **Informational Messages**: Use `log.Println("INFO: message")` for general information.
- **Error Messages**: Use `log.Println("ERROR: message")` to indicate an error.
- **Fatal Messages**: Use `log.Fatal("FATAL: message")` to log fatal issues that will stop the program.

These messages will be forwarded to the specified Telegram chat based on the set log level.

## Log Levels

The logger allows setting different log levels to filter messages sent to Telegram:

- **INFO**: All log messages will be sent.
- **ERROR**: Only `ERROR` and `FATAL` log messages will be sent.
- **FATAL**: Only `FATAL` log messages will be sent.

The default log level is `INFO`. You can change it by sending commands to the bot.

## License

MIT License. See the [LICENSE](LICENSE) file for more details.

## Contributing

Feel free to open issues or submit pull requests to improve the library. Contributions are welcome!

## Acknowledgements

- [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) - Used for interacting with the Telegram Bot API.

