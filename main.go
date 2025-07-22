package main

import (
	tgClient "SteamSaleBot/clients/telegram"
	event_consumer "SteamSaleBot/consumer/event-consumer"
	"SteamSaleBot/events/telegram"
	"SteamSaleBot/storage/files"
	"flag"
	"time"
	"log"
)

const (
	tgBotHost   = "api.telegram.org"
	storagePath = "storage/db"
	bathSize    = 100
)

func main() {
loc, err := time.LoadLocation("Asia/Yekaterinburg") // или твой часовой пояс
  if err != nil {
    panic(err)
  }
  time.Local = loc
	eventsProcessor := telegram.New(
		tgClient.New(tgBotHost, mustToken()),
		files.New(storagePath),
	)
	log.Println("Starting telegram bot")

	consumer := event_consumer.New(eventsProcessor, eventsProcessor, bathSize)
	if err := consumer.Start(); err != nil {
		log.Fatal(err)
	}
}

func mustToken() string {
	token := flag.String("token", "", "The token to use")
	flag.Parse()

	if *token == "" {
		log.Fatal("You must provide a token")
	}
	return *token
}
