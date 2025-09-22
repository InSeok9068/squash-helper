package action

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-rod/rod"
)

func Action(w http.ResponseWriter, r *http.Request) {
	// page := stealth.MustPage(browser)
	code := r.URL.Query().Get("code")
	resp, _ := http.Get("http://127.0.0.1:9222/json/version")
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&v)

	browser := rod.New().ControlURL(v.WebSocketDebuggerURL).MustConnect()
	pages := browser.MustPages()

	if len(pages) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("탭이 존재하지 않습니다."))
		return
	}

	page := pages[0]

	url := page.MustInfo().URL

	if !strings.HasPrefix(url, "https://www.auc.or.kr/reservation/program/lesson/list") {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("강습 신청 페이지로 진입해주세요!"))
		return
	}

	switch code {
	case "1":
		{
			sel := page.MustElement("#areaGbn")
			sel.MustClick() // 드롭다운 열기
			sel.MustSelect("호계스쿼시")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("강습 구분 선택 완료"))
		}
	case "2":
		{
			entranceType := page.MustElement("#entranceType")
			entranceType.MustClick() // 드롭다운 열기
			entranceType.MustSelect("화목(강습)")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("강습 과정 선택 완료"))
		}
	case "3":
		{
			// <a href="#" onclick="insertOrderSeq('11','218','주2일(화,목)','03','주2일(화,목)','11:00 - 12:30','배드민턴','임미정');" class="common_btn regist">신청</a>
			btns := page.MustElements("a.common_btn.regist")
			for _, btn := range btns {
				html := btn.MustProperty("outerHTML").String()

				// if strings.Contains(html, "주2일(화,목)") &&
				// 	strings.Contains(html, "11:00 - 12:30") &&
				// 	strings.Contains(html, "신청") {
				if strings.Contains(html, "화목(강습)") &&
					strings.Contains(html, "20:00 - 21:00") &&
					strings.Contains(html, "신청") {
					btn.MustClick()
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("강습 시간 선택 완료"))
					break // 하나만 클릭하고 종료
				}
			}
		}
	}
}
