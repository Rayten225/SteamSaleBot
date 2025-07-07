package storage

import (
	"SteamSaleBot/lib/e"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
)

var ErrNotSavedGame = errors.New("no save Game")

type Storage interface {
	Save(g *User) error
	CheckAllGame(userName string) ([]*Game, error)
	Remove(g *User) error
	CreateSettings(g *User) error
	UpdSettings(userName string, settings []string) (err error)
	Settings(userName string) (*User, error)
	Users() (map[*User][]*Game, error)
}

type User struct {
	UserName     string
	UserSettings UserSettings
	Game         Game
}
type UserSettings struct {
	ChatId      int
	Discounts   bool
	FreeWeekend bool
	Sales       bool
}
type Game struct {
	Name  string
	ID    string
	Price string
}

func (u *User) Hash() (string, error) {
	h := sha1.New()

	if _, err := io.WriteString(h, u.Game.ID); err != nil {
		return "", e.Warp("can't calculate hash", err)
	}

	if _, err := io.WriteString(h, u.UserName); err != nil {
		return "", e.Warp("can't calculate hash", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
