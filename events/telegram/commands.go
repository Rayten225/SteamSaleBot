package telegram

import (
	"SteamSaleBot/lib/e"
	"SteamSaleBot/storage"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	StartCmd     = "/start"
	HelpCmd      = "/help"
	AddCmd       = "/add"
	SettingsCmd  = "/settings"
	CheckCmd     = "/check"
	DonateCmd    = "/donate"
	DeleteCmd    = "/delete"
	CheckMyGames = "/my_games"
)

var queueAdd = make(map[int]string)

func (p *Processor) doCmd(text string, chatId int, username string) error {
	text = strings.TrimSpace(text)

	log.Printf("got new command: %s from %s", text, username)

	b, err := p.InQueueCmd(chatId, text, username)
	if err != nil {
		return err
	}
	if b {
		return nil
	}
	switch text {
	case HelpCmd:
		return p.sendHelp(chatId)
	case StartCmd:
		queueAdd[chatId] = ""
		return p.sendStart(chatId, username)
	case AddCmd:
		if err := p.tg.SendMessage(chatId, msgSendID); err != nil {
			return err
		}
		queueAdd[chatId] = "add"
		return nil
	case SettingsCmd:
		if err := p.sendSettings(chatId, username); err != nil {
			return err
		}
		queueAdd[chatId] = "settings"
		return nil
	case DonateCmd:
		return p.sendDonate(chatId)
	case CheckCmd:
		if err := p.tg.SendMessage(chatId, msgSendID); err != nil {
			return err
		}
		queueAdd[chatId] = "check"
		return nil

	case DeleteCmd:
		if err := p.tg.SendMessage(chatId, msgSendID); err != nil {
			return err
		}
		queueAdd[chatId] = "delete"

	case CheckMyGames:
		return p.sendMyGames(chatId, username)
	}
	return nil
}

func (p *Processor) AddImport(chatId int, gameID string, username string) (err error) {
	defer func() { err = e.WrapIfErr("can't to command: add game", err) }()
	data, err := p.tg.Game(gameID)
	if err != nil {
		if err1 := p.tg.SendMessage(chatId, msgErrImport); err != nil {
			return err1
		}
		return err
	}
	user := storage.User{
		UserName: username,
		Game:     storage.Game{Name: data.Name, ID: gameID, Price: data.Price.Final},
	}
	if err := p.storage.Save(&user); err != nil {
		return err
	}

	msg := fmt.Sprintf(msgSuccessImport + data.Name)
	if err := p.tg.SendMessage(chatId, msg); err != nil {
		return err
	}
	return nil
}

func (p *Processor) sendDonate(chatId int) (err error) {
	return p.tg.SendMessage(chatId, msgDonate)
}

func (p *Processor) sendSettings(chatId int, username string) (err error) {
	user, err := p.storage.Settings(username)
	if err != nil {
		return err
	}
	var (
		sales, freeWeekend, discounts string
	)
	if user.UserSettings.Sales == true {
		sales = "Да"
	} else {
		sales = "Нет"
	}

	if user.UserSettings.FreeWeekend == true {
		freeWeekend = "Да"
	} else {
		freeWeekend = "Нет"
	}

	if user.UserSettings.Discounts == true {
		discounts = "Да"
	} else {
		discounts = "Нет"
	}
	msg := fmt.Sprintf(
		"*Настройки уведомлений:*\n"+
			"1. Распродажи: *%s* \n"+
			"2. Бесплатные выходные: *%s* \n"+
			"3.  Скидки ваших игр: *%s* \n\n"+
			"Чтобы изменить настроки напишите номера которые хотите отключить или включить через запятую\n\nЧтобы выйти без изменений напишите \"exit\" ", sales, freeWeekend, discounts)

	if err := p.tg.SendMessage(chatId, msg); err != nil {
	}
	return err
}

func (p *Processor) updSettings(chatId int, username, settings string) (err error) {
	defer func() { err = e.WrapIfErr("can't to command: UpdSettings", err) }()
	settings = strings.ReplaceAll(settings, " ", "")
	if settings == "exit" {
		return nil
	}
	parts := strings.Split(settings, ",")
	if len(parts) <= 0 || len(parts) > 2 {
		return p.tg.SendMessage(chatId, "Не правильное кол-во аргументов")
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return p.tg.SendMessage(chatId, "Не правильное кол-во аргументов")
	}

	if err := p.storage.UpdSettings(username, parts); err != nil {
		return err
	}
	return p.tg.SendMessage(chatId, msgSuccessEdit)
}

func (p *Processor) sendCheck(chatId int, gameID string) (err error) {
	defer func() { err = e.WrapIfErr("can't to command: send random", err) }()

	data, err := p.tg.Game(gameID)
	if err != nil {
		return err
	}

	re := regexp.MustCompile(`(?:<strong>\*</strong>)|(?:<br><strong>\*</strong>.*)`)
	data.Languages = re.ReplaceAllString(data.Languages, "")

	msg := fmt.Sprintf(
		"*Название:* %s \n\n"+
			"*Описание:* %s \n\n"+
			"*Цена без скидки:* %s \n\n"+
			"*Цена со скидкой:* %s \n\n"+
			"*Поддерживаемые языки:* %s", data.Name, data.Description, data.Price.Initial, data.Price.Final, data.Languages)

	return p.tg.SendMessage(chatId, msg)
}

func (p *Processor) sendMyGames(chatId int, username string) (err error) {
	defer func() { err = e.WrapIfErr("can't to command: send game", err) }()
	games, err := p.storage.CheckAllGame(username)
	if len(games) == 0 {
		return p.tg.SendMessage(chatId, msgNoSavedPages)
	}

	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	msg := ""
	for _, game := range games {
		data, err := p.tg.Game(game.ID)
		if err != nil {
			return err
		}
		msg += fmt.Sprintf(
			"*ID игры:* `%s` \n"+
				"*Название игры:* %s \n"+
				"*Актуальная цена:* %s \n\n", game.ID, game.Name, data.Price.Final)
	}
	return p.tg.SendMessage(chatId, msg)
}

func (p *Processor) sendHelp(chatId int) error {
	return p.tg.SendMessage(chatId, msgHelp)
}

func (p *Processor) sendStart(chatId int, name string) error {
	g := storage.User{UserName: name, UserSettings: storage.UserSettings{ChatId: chatId}}
	if err := p.storage.CreateSettings(&g); err != nil {
		return err
	}
	return p.tg.SendMessage(chatId, msgHello)
}

func (p *Processor) DeleteGame(chatId int, GameID string, username string) error {
	data, _ := p.tg.Game(GameID)
	user := storage.User{
		UserName: username,
		Game:     storage.Game{ID: GameID, Name: data.Name},
	}
	msg := fmt.Sprintf(msgDeleteGame + data.Name)
	if err := p.storage.Remove(&user); errors.Is(err, os.ErrNotExist) {
		return p.tg.SendMessage(chatId, msgNotExist)
	}
	if err := p.tg.SendMessage(chatId, msg); err != nil {
		return err
	}
	return nil
}

func (p *Processor) InQueueCmd(chatId int, text string, username string) (bool, error) {
	for _, f := range queueAdd {
		switch f {
		case "add":
			queueAdd[chatId] = ""
			if err := p.AddImport(chatId, text, username); err != nil {
				return true, err
			}
		case "delete":
			queueAdd[chatId] = ""
			if err := p.DeleteGame(chatId, text, username); err != nil {
				return true, err
			}
		case "check":
			queueAdd[chatId] = ""
			if err := p.sendCheck(chatId, text); err != nil {
				return true, err
			}
		case "settings":
			queueAdd[chatId] = ""
			if err := p.updSettings(chatId, username, text); err != nil {
				return true, err
			}
		}
	}
	return false, nil
}
