package action

import (
	"encoding/json"
	"net/http"

	"github.com/go-rod/rod"
	stealth "github.com/go-rod/stealth"
)

func Action(w http.ResponseWriter, r *http.Request) {
	resp, _ := http.Get("http://127.0.0.1:9222/json/version")
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&v)

	browser := rod.New().ControlURL(v.WebSocketDebuggerURL).MustConnect()
	page := stealth.MustPage(browser)
	page.MustNavigate("https://www.wikipedia.org/")
	page.MustWaitStable().MustScreenshot("a.png")
	defer page.MustClose()
}
