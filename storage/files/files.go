package files

import (
	"SteamSaleBot/lib/e"
	"SteamSaleBot/storage"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Storage struct {
	basePath string
}

const defaultPerm = 0774

func New(basePath string) *Storage {
	return &Storage{basePath: basePath}
}

func (s Storage) Save(u *storage.User) (err error) {
	defer func() { err = e.WrapIfErr("can't save game", err) }()
	fPath := filepath.Join(s.basePath, u.UserName, "games")

	if err := os.MkdirAll(fPath, defaultPerm); err != nil {
		return err
	}

	fName, err := fileName(u)
	if err != nil {
		return err
	}

	fPath = filepath.Join(fPath, fName)
	file, err := os.Create(fPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close }()
	g := storage.Game{Name: u.Game.Name, ID: u.Game.ID, Price: u.Game.Price}
	if err = gob.NewEncoder(file).Encode(g); err != nil {
		return err
	}
	return nil
}

func (s Storage) CheckAllGame(userName string) (Game []*storage.Game, err error) {
	defer func() { err = e.WrapIfErr("can't check games", err) }()

	fPath := filepath.Join(s.basePath, userName, "games")
	files, err := ioutil.ReadDir(fPath)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, storage.ErrNotSavedGame
	}
	result := make([]*storage.Game, 0)
	for _, file := range files {
		game, err := s.decodeGame(filepath.Join(fPath, file.Name()))
		if err != nil {
			return nil, err
		}
		result = append(result, game)
	}
	return result, nil
}

func (s Storage) Remove(u *storage.User) error {
	fileName, err := fileName(u)
	if err != nil {
		return e.Warp("can't remove file", err)
	}

	path := filepath.Join(s.basePath, u.UserName, "games", fileName)

	switch _, err = os.Stat(path); {
	case errors.Is(err, os.ErrNotExist):
		return os.ErrNotExist
	case err != nil:
		msg := fmt.Sprintf("can't check if file %s exists", path)
		return e.Warp(msg, err)
	}

	if err := os.Remove(path); err != nil {
		msg := fmt.Sprintf("can't remove file %s", path)

		return e.Warp(msg, err)
	}

	return nil
}

func (s Storage) CreateSettings(u *storage.User) (err error) {
	defer func() { err = e.WrapIfErr("can't save game", err) }()
	fPath := filepath.Join(s.basePath, u.UserName)

	if err := os.MkdirAll(fPath, defaultPerm); err != nil {
		return err
	}
	fPath = filepath.Join(fPath, "settings")
	if _, err := os.Stat(fPath); err == nil {
		return nil
	}
	file, err := os.Create(fPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close }()

	u.UserSettings.Sales = true
	u.UserSettings.FreeWeekend = true
	u.UserSettings.Discounts = true

	if err = gob.NewEncoder(file).Encode(u); err != nil {
		return err
	}
	return nil
}

func (s Storage) Users() (usersFile map[*storage.User][]*storage.Game, err error) {
	defer func() { err = e.WrapIfErr("can't get users", err) }()

	usersFile = make(map[*storage.User][]*storage.Game)
	fPath := filepath.Join(s.basePath)
	users, err := ioutil.ReadDir(fPath)

	for _, user := range users {
		os.MkdirAll(filepath.Join(fPath, user.Name(), "games"), defaultPerm)
		files, err := ioutil.ReadDir(filepath.Join(fPath, user.Name(), "games"))
		if err != nil {
			return nil, err
		}

		userSet, err := s.decodeSettings(filepath.Join(fPath, user.Name(), "settings"))
		if err != nil {
			return nil, err
		}

		games := make([]*storage.Game, 0)
		for _, file := range files {

			game, err := s.decodeGame(filepath.Join(fPath, user.Name(), "games", file.Name()))
			if err != nil {
				return nil, err
			}
			games = append(games, game)
		}
		usersFile[userSet] = games
	}

	return usersFile, nil
}

func (s Storage) UpdSettings(userName string, change []string) (err error) {
	defer func() { err = e.WrapIfErr("can't upd settings", err) }()

	fPath := filepath.Join(s.basePath, userName, "settings")
	user, err := s.decodeSettings(fPath)
	if err != nil {
		return err
	}
	for _, i := range change {
		i = strings.TrimSpace(i)
		switch i {
		case "1":
			if user.UserSettings.Sales {
				user.UserSettings.Sales = false
				continue
			}
			user.UserSettings.Sales = true
		case "2":
			if user.UserSettings.FreeWeekend {
				user.UserSettings.FreeWeekend = false
				continue
			}
			user.UserSettings.FreeWeekend = true
		case "3":
			if user.UserSettings.Discounts {
				user.UserSettings.Discounts = false
				continue
			}
			user.UserSettings.Discounts = true
		}
	}
	file, err := os.Create(fPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close }()
	if err = gob.NewEncoder(file).Encode(user); err != nil {
		return err
	}

	return nil
}

func (s Storage) Settings(userName string) (*storage.User, error) {
	fPath := filepath.Join(s.basePath, userName)
	set, err := s.decodeSettings(filepath.Join(fPath, "settings"))

	if err != nil {
		return &storage.User{}, err
	}

	return set, nil
}

func (s Storage) decodeGame(filePath string) (*storage.Game, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, e.Warp("can't open game", err)
	}
	defer func() { _ = f.Close() }()
	var p storage.Game
	if err := gob.NewDecoder(f).Decode(&p); err != nil {
		return nil, e.WrapIfErr("can't decode Game", err)
	}
	return &p, nil
}

func (s Storage) decodeSettings(filePath string) (*storage.User, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, e.Warp("can't open settings", err)
	}
	defer func() { _ = f.Close() }()
	var p storage.User
	if err := gob.NewDecoder(f).Decode(&p); err != nil {
		return nil, e.WrapIfErr("can't decode settings", err)
	}
	return &p, nil
}

func fileName(p *storage.User) (string, error) {
	return p.Hash()
}
