package event_consumer

import (
	"SteamSaleBot/events"
	"log"
	"time"
)

type Consumer struct {
	fetcher   events.Fetcher
	processor events.Processor
	batchSize int
}

func New(fetcher events.Fetcher, processor events.Processor, batchSize int) *Consumer {
	return &Consumer{
		fetcher:   fetcher,
		processor: processor,
		batchSize: batchSize,
	}
}

func (c *Consumer) Start() error {
	go c.processor.DiscNotif()
	go c.processor.WeekSaleNotif()
	go c.processor.SalesNotif()

	for {
		gotEvents, err := c.fetcher.Fetch(c.batchSize)
		if err != nil {
			log.Printf("Error fetching events: %v", err)

			success := false
			for i := 0; i < 3; i++ {
				log.Printf("Retry %d/3 fetching events...", i+1)
				time.Sleep(1 * time.Minute)

				gotEvents, err = c.fetcher.Fetch(c.batchSize)
				if err == nil {
					success = true
					break
				}
			}

			if !success {
				log.Println("All retries failed. Waiting before next attempt...")
				time.Sleep(3 * time.Minute) // ⛔ ключевое отличие!
				continue
			}
		}
		if len(gotEvents) == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		if err := c.handleEvents(gotEvents); err != nil {
			log.Printf("Error handling events: %v", err)
			continue
		}

	}
}

func (c *Consumer) handleEvents(events []events.Event) error {
	for _, event := range events {

		if err := c.processor.Process(event); err != nil {
			log.Printf("Error processing event: %v", err)
			continue
		}
	}
	return nil
}
