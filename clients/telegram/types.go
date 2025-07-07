package telegram

type UpdatesResponse struct {
	Ok     bool     `json:"ok"`
	Result []Update `json:"result"`
}
type Update struct {
	ID      int              `json:"update_id"`
	Message *IncomingMessage `json:"message"`
}

type IncomingMessage struct {
	Text string `json:"text"`
	From From   `json:"from"`
	Chat Chat   `json:"chat"`
}

type GameResponse struct {
	Success bool     `json:"success"`
	Data    GameData `json:"data"`
}

type GameData struct {
	Name        string    `json:"name"`
	IsFree      bool      `json:"is_free"`
	Description string    `json:"short_description"`
	Languages   string    `json:"supported_languages"`
	Price       GamePrice `json:"price_overview"`
}

type GameInfo struct {
	Title      string
	OldPrice   string
	FinalPrice string
	URL        string
}

type GamePrice struct {
	Initial string `json:"initial_formatted"`
	Final   string `json:"final_formatted"`
}

type From struct {
	Username string `json:"username"`
}

type Chat struct {
	ID int `json:"id"`
}
