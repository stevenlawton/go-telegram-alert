
# go-telegram-log-alert (tlog)

`tlog` is a custom Go logger that wraps the standard logger and forwards log messages to a specified Telegram chat. This allows you to easily receive important log messages via Telegram, making it ideal for remote monitoring of applications.

## Features

- Log messages with different severity levels (`INFO`, `ERROR`, `FATAL`, `ALERT`).
- Forward logs to a Telegram chat.
- Control log levels via Telegram commands.
- Group `INFO` and `ERROR` logs periodically for reduced noise.
- Mute notifications temporarily for maintenance or to reduce interruptions.
- Filter out specific words from being logged to Telegram.
- Uses standard `log` package functions (`log.Println`, `log.Fatal`, etc.).

## Installation

To install the library, use:

```sh
go get github.com/stevenlawton/go-telegram-alert
```

## Usage

Here is an example of how to use `tlog` in your Go project.

```go
package main

import (
    "log"
    "github.com/stevenlawton/go-telegram-alert"
)

func main() {
    botToken := "YOUR_TELEGRAM_BOT_TOKEN"
    chatID := int64(YOUR_CHAT_ID) // Ensure this is an int64
    appName := "MyApp"

    err := tlog.NewTeleLogger(botToken, chatID, appName)
    if err != nil {
        log.Fatalf("Failed to create TeleLogger: %v", err)
    }

    // Now use log functions as usual
    log.Println("INFO: This is an informational message.")
    log.Println("ERROR: This is an error message.")
    log.Println("ALERT: This is an alert message.")
    log.Fatal("FATAL: This is a fatal error message.")
}
```

### Setting Log Levels via Telegram

You can control the verbosity of the logs sent to Telegram by using the following commands in your Telegram chat:

- `/set_log_level_info` - Sets the log level to `INFO` (default).
- `/set_log_level_error` - Sets the log level to `ERROR` (only `ERROR` and `FATAL` logs will be sent).
- `/set_log_level_fatal` - Sets the log level to `FATAL` (only `FATAL` logs will be sent).
- `/set_log_level_alert` - Sets the log level to `ALERT` (only `ALERT` logs will be sent).

### Additional Commands

- `/mute_notifications <duration>` - Mutes all log notifications for a specified duration. Example: `/mute_notifications 10m`.
- `/filter_word <word>` - Filters out logs containing a specific word. Example: `/filter_word debug`.

## API

### `NewTeleLogger(botToken string, chatID int64, appName string) error`

Initializes the Telegram logger and sets it as the default logger. You must provide your bot's token, the chat ID where you want the logs to be sent, and the app name to identify your application in the logs.

### Logging at Appropriate Levels

After initializing the logger with `NewTeleLogger`, use the standard `log` functions (`log.Println`, `log.Printf`, `log.Fatal`, etc.) to generate log messages. To ensure the log messages are categorized properly:

- **Informational Messages**: Use `log.Println("INFO: message")` for general information.
- **Error Messages**: Use `log.Println("ERROR: message")` to indicate an error.
- **Alert Messages**: Use `log.Println("ALERT: message")` for high-priority alerts.
- **Fatal Messages**: Use `log.Fatal("FATAL: message")` to log fatal issues that will stop the program.

These messages will be forwarded to the specified Telegram chat based on the set log level.

## Log Levels and Features

The logger allows setting different log levels to filter messages sent to Telegram:

- **INFO**: All log messages will be sent.
- **ERROR**: Only `ERROR` and `FATAL` log messages will be sent.
- **FATAL**: Only `FATAL` log messages will be sent.
- **ALERT**: Only `ALERT` log messages will be sent.

The default log level is `INFO`. You can change it by sending commands to the bot.

### Log Grouping

- **Grouped Logs**: `INFO` and `ERROR` logs can be grouped and sent in bulk every 5 minutes to reduce noise. This feature ensures you still receive all log messages, but without the constant interruptions.

### Notifications and Filtering

- **Mute Notifications**: You can mute notifications using the `/mute_notifications` command to reduce interruptions during specific time periods.
- **Filtered Words**: Use the `/filter_word` command to prevent logs containing specific words from being sent to Telegram, which helps reduce unnecessary messages.

## License

MIT License. See the [LICENSE](LICENSE) file for more details.

## Contributing

Feel free to open issues or submit pull requests to improve the library. Contributions are welcome!

## Acknowledgements

- [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) - Used for interacting with the Telegram Bot API.
