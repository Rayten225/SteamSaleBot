package telegram

import (
	"SteamSaleBot/clients/telegram"
	"SteamSaleBot/events"
	"SteamSaleBot/lib/e"
	"SteamSaleBot/storage"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"
)

type Processor struct {
	tg      *telegram.Client
	offset  int
	storage storage.Storage
}

type Meta struct {
	ChatID   int
	Username string
}

type rawSale struct {
	Name  string `json:"name"`
	Start string `json:"start"` // "2006-01-02 15:04"
	End   string `json:"end"`
}

type Sale struct {
	Name  string
	Start time.Time
	End   time.Time
}

var (
	ErrUnknownEventType = errors.New("unknown event type")
	ErrUnknownMetaType  = errors.New("unknown meta type")
)

func New(client *telegram.Client, storage storage.Storage) *Processor {
	return &Processor{
		tg:      client,
		storage: storage,
	}
}

func (p *Processor) Fetch(limit int) ([]events.Event, error) {
	updates, err := p.tg.Updates(p.offset, limit)
	if err != nil {
		return nil, e.Warp("can't get events", err)
	}

	if len(updates) == 0 {
		return nil, nil
	}
	res := make([]events.Event, 0, len(updates))

	for _, u := range updates {
		res = append(res, event(u))
	}
	p.offset = updates[len(updates)-1].ID + 1

	return res, nil
}

func (p *Processor) DiscNotif() {
	for {
		users, err := p.storage.Users()
		if err != nil {
			log.Println("can't get users from storage", err)
		}
		for u, games := range users {
			time.Sleep(30 * time.Second)
			log.Println(u.UserName, u.UserSettings.ChatId)
			msg := ""
			for _, g := range games {
				game, err := p.tg.Game(g.ID)
				if err != nil {
					log.Println("can't get game", err)
					time.Sleep(5 * time.Minute)
					game, err = p.tg.Game(g.ID)
				}
				re := regexp.MustCompile(`\d+`)
				final, err := strconv.Atoi(re.FindString(game.Price.Final))
				now, err := strconv.Atoi(re.FindString(g.Price))
				if game.Price.Final != "" {
					continue
				}
				u.Game.ID = g.ID
				u.Game.Name = g.Name
				u.Game.Price = game.Price.Final
				if err := p.storage.Save(u); err != nil {
					log.Println("Ошибка сохранения DiscNotif: ", err)
				}
				if final < now {
					msg += fmt.Sprintf("Скидка на игру %s: %s \n", g.Name, game.Price.Final)
				}

			}
			if msg != "" && u.UserSettings.Discounts != false {
				log.Println("Отправлено сообщение о скидке", u.UserName, msg)
				if err := p.tg.SendMessage(u.UserSettings.ChatId, msg); err != nil {
					log.Println("can't send message", err)
				}
			}
		}
		time.Sleep(30 * time.Minute)
	}
}

func (p *Processor) WeekSaleNotif() {
	for {
		loc, _ := time.LoadLocation("Europe/Moscow")
		now := time.Now().In(loc)
		target := time.Date(now.Year(), now.Month(), now.Day(), 10, 00, 0, 0, loc)
		if now.After(target) {
			target = target.Add(24 * time.Hour)

			users, err := p.storage.Users()
			if err != nil {
				log.Println("can't get users from storage", err)
			}
			games, err := p.tg.Sale()
			if err != nil {
				log.Println("can't get WeekSale", err)
			}
			for u, _ := range users {
				go p.weekSaleSend(games, u)
			}
		}
		time.Sleep(time.Until(target))
	}
}

func (p *Processor) SalesNotif() {
	for {
		loc, err := time.LoadLocation("Europe/Moscow")
		if err != nil {
			log.Fatal("SalesNotif: can't load location:", err)
		}

		// Вспомогательная структура для исходных данных
		type rawSale struct {
			Name  string `json:"name"`
			Start string `json:"start"` // "2006-01-02 15:04"
			End   string `json:"end"`
		}

		// Читаем и конвертим в события
		file, err := os.Open("sales.json")
		if err != nil {
			log.Fatal("SalesNotif: can't open sales.json:", err)
		}
		defer file.Close()

		var raws []rawSale
		if err := json.NewDecoder(file).Decode(&raws); err != nil {
			log.Fatal("SalesNotif: can't decode sales.json:", err)
		}

		type event struct {
			when time.Time
			kind string // "before-start", "on-start", "before-end"
			name string
			end  time.Time
		}

		var events []event
		now := time.Now().UTC()
		for _, r := range raws {
			startLocal, err := time.ParseInLocation("2006-01-02 15:04", r.Start, loc)
			if err != nil {
				log.Printf("SalesNotif: parse start %q: %v", r.Start, err)
				continue
			}
			endLocal, err := time.ParseInLocation("2006-01-02 15:04", r.End, loc)
			if err != nil {
				log.Printf("SalesNotif: parse end %q: %v", r.End, err)
				continue
			}
			startUTC := startLocal.UTC()
			endUTC := endLocal.UTC()

			// формируем 3 события
			events = append(events,
				event{startUTC.Add(-24 * time.Hour), "before-start", r.Name, endUTC},
				event{startUTC, "on-start", r.Name, endUTC},
				event{endUTC.Add(-24 * time.Hour), "before-end", r.Name, endUTC},
			)
		}

		// оставляем только будущие события
		var future []event
		for _, e := range events {
			if e.when.After(now) {
				future = append(future, e)
			}
		}

		// 3. Запускаем цикл «спать → уведомить → вычеркнуть»
		for len(future) > 0 {
			// находим ближайшее
			next := future[0]
			for _, e := range future {
				if e.when.Before(next.when) {
					next = e
				}
			}

			// спим до нужного момента
			sleepDur := time.Until(next.when)
			log.Printf("SalesNotif: sleeping %v until %s of %s", sleepDur, next.kind, next.name)
			time.Sleep(sleepDur)

			// формируем текст
			tMsk := next.when.In(loc).Format("02 Jan 15:04")
			var msg string
			switch next.kind {
			case "before-start":
				msg = fmt.Sprintf("🟡 Завтра начнётся %s (%s МСК)", next.name, tMsk)
			case "on-start":
				msg = fmt.Sprintf("🟢 Началась %s! Идёт до %s (МСК)", next.name, next.end.In(loc).Format("02 Jan 15:04"))
			case "before-end":
				msg = fmt.Sprintf("🔴 Завтра закончится %s (%s МСК)", next.name, tMsk)
			}

			users, err := p.storage.Users()
			if err != nil {
				log.Fatal("SalesNotif: can't load users:", err)
			}
			for u, _ := range users {
				go func() {
					log.Println("Отправлено сообщение о распродаже", u.UserName, u.UserSettings.ChatId)
					if err := p.tg.SendMessage(u.UserSettings.ChatId, msg); err != nil {
						log.Printf("SalesNotif: can't send to %d: %v", u.UserSettings.ChatId, err)
					}
				}()
			}

			// убираем событие из списка
			remaining := future[:0]
			for _, e := range future {
				if !(e.when.Equal(next.when) && e.kind == next.kind && e.name == next.name) {
					remaining = append(remaining, e)
				}
			}
			future = remaining
		}

		if err := p.tg.SendMessage(2134561992, "SalesNotif: все уведомления отправлены, обновите распродажи и перезапустите бота"); err != nil {
			log.Printf("SalesNotif: can't send to admin")
		}
		time.Sleep(720 * time.Hour)
	}
}

func (p *Processor) Process(event events.Event) error {
	switch event.Type {
	case events.Message:
		return p.processMessage(event)
	default:
		return e.Warp("can't process message", ErrUnknownEventType)

	}
}

func (p *Processor) weekSaleSend(games []telegram.GameInfo, u *storage.User) {
	msg := "Ежедневные скидки:"
	for _, g := range games {
		msg += fmt.Sprintf("\n\nНазвание: "+g.Title+
			"\nЦена до: "+g.OldPrice+
			"\nЦена после: "+g.FinalPrice+
			"\n[Открыть steam](%s)", g.URL)
	}
	if msg != "" && u.UserSettings.FreeWeekend != false {
		log.Println("Отправлено сообщение о скидках недели", u.UserName, u.UserSettings.ChatId)
		if err := p.tg.SendMessage(u.UserSettings.ChatId, msg); err != nil {
			log.Println("can't send message", err)
		}
	}
}

func (p *Processor) processMessage(event events.Event) error {
	meta, err := meta(event)
	if err != nil {
		return e.Warp("can't process message", err)
	}

	if err := p.doCmd(event.Text, meta.ChatID, meta.Username); err != nil {
		return e.Warp("can't process message", err)
	}

	return nil
}

func meta(event events.Event) (Meta, error) {
	res, ok := event.Meta.(Meta)
	if !ok {
		return Meta{}, e.Warp("can't get meta", ErrUnknownMetaType)
	}
	return res, nil
}

func event(upd telegram.Update) events.Event {
	updType := fetchType(upd)

	res := events.Event{
		Type: updType,
		Text: fetchText(upd),
	}

	if upd.Message != nil {
		res.Meta = Meta{
			ChatID:   upd.Message.Chat.ID,
			Username: upd.Message.From.Username,
		}
	}
	return res
}

func fetchText(upd telegram.Update) string {
	if upd.Message != nil {
		return upd.Message.Text
	}
	return ""
}

func fetchType(upd telegram.Update) events.Type {
	if upd.Message != nil {
		return events.Message
	}
	return events.Message
}
