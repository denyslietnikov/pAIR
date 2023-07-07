package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/caarlos0/env/v7"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	openai "github.com/sashabaranov/go-openai"
	//"github.com/sourcegraph/conc"
)

var cfg struct {
	TelegramAPIToken                    string  `env:"TELEGRAM_APITOKEN,required"`
	OpenAIAPIKey                        string  `env:"OPENAI_API_KEY,required"`
	MoodeBaseURL                        string  `env:"MOODE_BASE_URL,required"`
	OPENAIModel                         string  `env:"OPENAI_MODEL" envDefault:"gpt-3.5-turbo"`
	OpenAIBaseURL                       string  `env:"OPENAI_BASE_URL" envDefault:"https://api.openai.com"`
	ModelTemperature                    float32 `env:"MODEL_TEMPERATURE" envDefault:"1.0"`
	AllowedTelegramID                   []int64 `env:"ALLOWED_TELEGRAM_ID" envSeparator:","`
	ConversationIdleTimeoutSeconds      int     `env:"CONVERSATION_IDLE_TIMEOUT_SECONDS" envDefault:"900"`
	NotifyUserOnConversationIdleTimeout bool    `env:"NOTIFY_USER_ON_CONVERSATION_IDLE_TIMEOUT" envDefault:"false"`
}

type User struct {
	TelegramID     int64
	LastActiveTime time.Time
	HistoryMessage []openai.ChatCompletionMessage
	LatestMessage  tgbotapi.Message
}

func main() {

	fileContentChan := make(chan string)
	go runSongService(fileContentChan)
	//go readFileAndStoreValue()

	if err := env.Parse(&cfg); err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}

	baseURL, err := url.JoinPath(cfg.OpenAIBaseURL, "/v1")
	if err != nil {
		panic(err)
	}
	cfg.OpenAIBaseURL = baseURL

	if cfg.OPENAIModel != "gpt-3.5-turbo" && cfg.OPENAIModel != "gpt-4" {
		log.Fatalf("Invalid OPENAI_MODEL: %s", cfg.OPENAIModel)
	}

	bot, err := tgbotapi.NewBotAPI(cfg.TelegramAPIToken)
	if err != nil {
		panic(err)
	}

	gpt := NewGPT()

	log.Printf("Authorized on account %s", bot.Self.UserName)

	_, _ = bot.Request(tgbotapi.NewSetMyCommands([]tgbotapi.BotCommand{
		{
			Command:     "start",
			Description: "Start a new chat",
		},
	}...))

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message updates
			continue
		}
		userID := update.Message.Chat.ID

		if len(cfg.AllowedTelegramID) != 0 {
			var userAllowed bool
			for _, allowedID := range cfg.AllowedTelegramID {
				if allowedID == update.Message.Chat.ID {
					userAllowed = true
				}
			}
			if !userAllowed {
				_, err := bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("You are not allowed to use this bot. User ID: %d", update.Message.Chat.ID)))
				if err != nil {
					log.Print(err)
				}
				continue
			}
		}
		go handleUpdates(bot, gpt, userID, fileContentChan)
		select {}
	}
}

func handleUpdates(bot *tgbotapi.BotAPI, gpt *GPT, userID int64, fileContentChan <-chan string) {
	currentFileContent := ""
	for {
		// Ждем получения нового значения fileContent
		fileContent := <-fileContentChan

		// Проверяем, изменилось ли значение fileContent
		if fileContent != currentFileContent {
			// Обновляем текущее значение fileContent
			currentFileContent = fileContent

			// Вызываем processQueries с новым значением fileContent
			processQueries(bot, gpt, userID, currentFileContent)
		}

		// Добавляем небольшую задержку перед следующей проверкой
		time.Sleep(time.Second)
	}
}

func processQueries(bot *tgbotapi.BotAPI, gpt *GPT, userID int64, fileContent string) {
	answerChan := make(chan string)
	prevFileContent := fileContent
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	go func() {
		sendPrompt(bot, gpt, userID, fileContent, answerChan)
	}()

	var messageID int

	for {
		select {
		case currentAnswer, ok := <-answerChan:
			if !ok {
				return
			}

			fmt.Println("Received answer:", currentAnswer)

			if messageID == 0 {
				msg, err := bot.Send(tgbotapi.NewMessage(userID, currentAnswer))
				if err != nil {
					log.Print(err)
				}
				messageID = msg.MessageID
			} else {
				editedMsg := tgbotapi.NewEditMessageText(userID, messageID, currentAnswer)
				_, err := bot.Send(editedMsg)
				if err != nil {
					log.Print(err)
				}
			}

			timer.Reset(5 * time.Second)

		case <-timer.C:
			if fileContent != prevFileContent {
				prevFileContent = fileContent
				sendPrompt(bot, gpt, userID, fileContent, answerChan)
			}

			timer.Reset(5 * time.Second)
		}
	}
}

func sendPrompt(bot *tgbotapi.BotAPI, gpt *GPT, userID int64, fileContent string, answerChan chan<- string) {
	prompt := "Act as a radio host. Write me a short transitional intro (two sentences) from the track in the previous prompt to the track titled " + fileContent
	fmt.Println("Sending prompt:", prompt)

	contextTrimmed, err := gpt.SendMessage(userID, prompt, answerChan)
	if err != nil {
		log.Print(err)

		_, err = bot.Send(tgbotapi.NewMessage(userID, err.Error()))
		if err != nil {
			log.Print(err)
		}
	}

	if contextTrimmed {
		msg := tgbotapi.NewMessage(userID, "Context trimmed.")
		_, err = bot.Send(msg)
		if err != nil {
			log.Print(err)
		}
	}
}
