package server

import (
	"net/http"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

var browser *rod.Browser
var page *rod.Page

func Launch(w http.ResponseWriter, r *http.Request) {
	u := launcher.New().
		Leakless(false).
		Headless(false).
		MustLaunch()

	browser = rod.New().ControlURL(u).MustConnect()
	// 호계 복합청사 페이지 진입
	page = browser.MustPage("https://www.auc.or.kr/hogye/main/view")
	// 3초 대기
	time.Sleep(3 * time.Second)
	// 로그인 페이지 진입
	page.MustNavigate("https://www.auc.or.kr/sign/in/base/user")
	go func() {
		// 첫 번째 confirm → 취소
		w1, h1 := page.HandleDialog()
		w1()
		_ = h1(&proto.PageHandleJavaScriptDialog{Accept: false})

		// 두 번째 confirm → 확인
		w2, h2 := page.HandleDialog()
		w2()
		_ = h2(&proto.PageHandleJavaScriptDialog{Accept: true})
	}()
	// 3초 대기
	time.Sleep(2 * time.Second)
	page.MustElement(".total-loginN__btn").MustClick()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("로그인 페이지 진입 완료"))
}

func Login(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	password := r.URL.Query().Get("password")

	// 아이디 입력
	login_id := page.MustElement("#login_id")
	login_id.MustInput(id)
	time.Sleep(1 * time.Second)

	// 비밀번호 입력
	login_password := page.MustElement("#login_pwd")
	login_password.MustInput(password)
	time.Sleep(1 * time.Second)

	// 로그인 버튼 클릭
	buttons := page.MustElements("button")
	for _, button := range buttons {
		if button.MustText() == "로그인" {
			button.MustClick()
			break
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("로그인 완료"))
}
