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
		NoSandbox(true).
		HeadlessNew(true).
		// GPU 경로 제거
		Append("--disable-gpu").
		// 소프트웨어 GL까지 차단 → CPU 낭비↓
		Append("--disable-software-rasterizer").
		// 첫 실행 체크 제거
		Append("--no-first-run").
		Append("--no-default-browser-check").
		// 대기열 유지에 중요 (타이머/렌더러 절전 방지)
		Append("--disable-background-timer-throttling").
		Append("--disable-renderer-backgrounding").
		Append("--disable-backgrounding-occluded-windows").
		// 창 사이즈 설정
		Set("window-size", "1280,800").
		MustLaunch()

	browser := rod.New().ControlURL(u).MustConnect()
	page := stealth.MustPage(browser)
	page.MustNavigate("https://www.auc.or.kr/hogye/main/view")

	session := &userSession{
		browser: browser,
		page:    page,
	}
	sessionID, err := registerSession(session)
	if err != nil {
		_ = browser.Close()
		http.Error(w, "세션 생성 중 오류가 발생했습니다. 잠시 후 다시 시도해주세요.", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, sessionID)

	if f, ok := w.(http.Flusher); ok {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		f.Flush()
	}

	session.mu.Lock()

	defer session.mu.Unlock()

	page.MustWaitLoad()

	page.MustNavigate("https://www.auc.or.kr/sign/in/base/user")

	go handleLoginDialogs(page)

	page.MustWaitLoad()

	removeWaitPage(page)

	page.MustElement(".total-loginN__btn").MustClick()

	page.MustWaitLoad()

	removeWaitPage(page)

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

func Close(w http.ResponseWriter, r *http.Request) {
	if sessionID, session, ok := getSessionFromRequest(r); ok {
		if session != nil {
			session.pushInfo("사용자 요청으로 브라우저를 종료합니다.")
		}
		cleanupSession(sessionID)
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("브라우저 종료 완료"))
}

func Refresh(w http.ResponseWriter, r *http.Request) {
	session, _, ok := requireSession(w, r)
	if !ok {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	page := session.page
	session.pushInfo("브라우저 새로고침을 요청했습니다.")
	page.MustReload()
	// 페이지 진입 대기
	page.MustWaitLoad()
	session.pushInfo("브라우저 새로고침이 완료되었습니다.")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("브라우저 새로고침 완료"))
}

func removeWaitPage(page *rod.Page) {
	el, _ := page.Timeout(1 * time.Second).Element("#waitPage")
	if el != nil {
		_ = el.Remove()
	}
}
