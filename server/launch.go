package server

import (
	"net/http"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

func Launch(w http.ResponseWriter, r *http.Request) {
	if sessionID, _, ok := getSessionFromRequest(r); ok {
		cleanupSession(sessionID)
	}

	u := launcher.New().
		Leakless(false).
		Headless(false).
		Set("window-size", "1280,800"). // 크롬 런치 인자
		MustLaunch()

	browser := rod.New().ControlURL(u).MustConnect()
	page := stealth.MustPage(browser)
	page.MustNavigate("https://www.auc.or.kr/hogye/main/view")

	session := &userSession{browser: browser, page: page}
	sessionID, err := registerSession(session)
	if err != nil {
		_ = browser.Close()
		http.Error(w, "세션 생성 중 오류가 발생했습니다. 잠시 후 다시 시도해주세요.", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, sessionID)

	session.mu.Lock()
	defer session.mu.Unlock()

	// 호계 복합청사 페이지 진입 후 대기
	time.Sleep(2 * time.Second)

	// 로그인 페이지 진입
	page.MustNavigate("https://www.auc.or.kr/sign/in/base/user")

	go handleLoginDialogs(page)

	// 2초 대기
	time.Sleep(2 * time.Second)

	// 통합 로그인 클릭
	page.MustElement(".total-loginN__btn").MustClick()

	// 1초 대기
	time.Sleep(1 * time.Second)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("로그인 페이지 진입 완료"))
}

func handleLoginDialogs(page *rod.Page) {
	// 첫 번째 confirm → 취소
	w1, h1 := page.HandleDialog()
	w1()
	_ = h1(&proto.PageHandleJavaScriptDialog{Accept: false})

	// 두 번째 confirm → 확인
	w2, h2 := page.HandleDialog()
	w2()
	_ = h2(&proto.PageHandleJavaScriptDialog{Accept: true})
}
