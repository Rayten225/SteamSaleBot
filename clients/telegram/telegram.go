package telegram

import (
	"SteamSaleBot/lib/e"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type Client struct {
	host     string
	basePath string
	client   http.Client
}

const (
	getUpdatesMethod  = "getUpdates"
	sendMessageMethod = "sendMessage"
)

func New(host string, token string) *Client {
	return &Client{
		host:     host,
		basePath: newBasePath(token),
		client:   http.Client{},
	}
}

func newBasePath(token string) string {
	return "bot" + token
}

func (c *Client) Updates(offset int, limit int) (updates []Update, err error) {
	defer func() { err = e.WrapIfErr("can't get updates", err) }()

	q := url.Values{}
	q.Add("offset", strconv.Itoa(offset))
	q.Add("limit", strconv.Itoa(limit))

	data, err := c.doTgRequest(getUpdatesMethod, q)
	if err != nil {
		return nil, err
	}

	var res UpdatesResponse

	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}

	return res.Result, nil
}

func (c *Client) SendMessage(chatID int, text string) error {
	q := url.Values{}
	q.Add("chat_id", strconv.Itoa(chatID))
	q.Add("text", text)
	q.Add("parse_mode", "Markdown")

	_, err := c.doTgRequest(sendMessageMethod, q)
	if err != nil {
		return e.Warp("can't send message", err)
	}

	return nil
}

func (c *Client) Game(gameId string) (g GameData, err error) {
	link := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%s&cc=ru&l=ru", gameId)
	body, err := c.doSteamReq(link)
	if err != nil {
		return g, e.Warp("can't import game", err)
	}
	var result map[string]GameResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return g, err
	}

	gameResp, ok := result[gameId]
	if !ok || !gameResp.Success {
		return GameData{}, fmt.Errorf("game not found or unsuccessful response")
	}
	if gameResp.Data.Price.Final == "" {
		gameResp.Data.Price.Final = "бесплатно"
	}
	if gameResp.Data.Price.Initial == "" {
		gameResp.Data.Price.Initial = gameResp.Data.Price.Final
	}
	return gameResp.Data, nil
}

func (c *Client) Sale() (g []GameInfo, err error) {
	link := fmt.Sprintf("https://store.steampowered.com/search/?filter=weeklongdeals")
	body, err := c.doSteamReq(link)
	if err != nil {
		return g, e.Warp("can't import game", err)
	}
	games, err := c.parseGamesSale(body)
	return games, nil
}

func (c *Client) doSteamReq(link string) (data []byte, err error) {
	defer func() { err = e.WrapIfErr("can't do request", err) }()

	req, err := http.NewRequest(http.MethodGet, link, nil)
	req.Header.Set("X-Forwarded-For", "213.180.204.3")
	req.Header.Add("Accept-Language", "ru")
	if err != nil {
		return data, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return data, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return data, err
	}
	return body, nil
}

func (c *Client) doTgRequest(method string, query url.Values) (data []byte, err error) {
	defer func() { err = e.WrapIfErr("can't do request", err) }()

	u := url.URL{
		Scheme: "https",
		Host:   c.host,
		Path:   path.Join(c.basePath, method),
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.URL.RawQuery = query.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (c *Client) parseGamesSale(body []byte) ([]GameInfo, error) {
	var games []GameInfo

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	doc.Find(".search_result_row").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find(".title").Text())
		href, _ := s.Attr("href")
		
		// Цены
		finalPrice := strings.TrimSpace(s.Find(".discount_final_price").Text())
		oldPrice := strings.TrimSpace(s.Find(".discount_original_price").Text())

		if finalPrice == "" {
			// Если нет скидки, пробуем просто .search_price
			raw := strings.TrimSpace(s.Find(".search_price").Text())
			finalPrice = strings.Join(strings.Fields(raw), " ")
		}

		game := GameInfo{
			Title:      title,
			OldPrice:   oldPrice,
			FinalPrice: finalPrice,
			URL:        href,
		}

		games = append(games, game)
	})

	return games, nil
}
