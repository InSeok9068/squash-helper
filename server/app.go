package server

import (
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
)

//go:embed web/*
var webServerFS embed.FS

type userSession struct {
	browser *rod.Browser
	page    *rod.Page
	mu      sync.Mutex
}

const sessionCookieName = "squash-helper-session"

var (
	sessionMu sync.RWMutex
	sessions  = make(map[string]*userSession)
)

func Run() {
	// 임베드 FS의 루트를 / 하위로 설정
	sub, err := fs.Sub(webServerFS, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/launch", Launch)
	mux.HandleFunc("/login", Login)
	mux.HandleFunc("/move", Move)
	mux.HandleFunc("/action", Action)
	mux.HandleFunc("/screenshot", Screenshot)
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// 서버 실행
	fmt.Println("서버 실행 중... http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}
}

func generateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func registerSession(session *userSession) (string, error) {
	id, err := generateSessionID()
	if err != nil {
		return "", err
	}
	sessionMu.Lock()
	sessions[id] = session
	sessionMu.Unlock()
	return id, nil
}

func cleanupSession(sessionID string) {
	sessionMu.Lock()
	session, ok := sessions[sessionID]
	if ok {
		delete(sessions, sessionID)
	}
	sessionMu.Unlock()

	if !ok || session == nil {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.browser != nil {
		if err := session.browser.Close(); err != nil {
			log.Printf("세션 %s 브라우저 종료 실패: %v", sessionID, err)
		}
	}
}

func setSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func getSessionFromRequest(r *http.Request) (string, *userSession, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", nil, false
	}

	sessionMu.RLock()
	session, ok := sessions[cookie.Value]
	sessionMu.RUnlock()

	if !ok || session == nil {
		return cookie.Value, nil, false
	}

	return cookie.Value, session, true
}

func requireSession(w http.ResponseWriter, r *http.Request) (*userSession, string, bool) {
	sessionID, session, ok := getSessionFromRequest(r)
	if !ok {
		http.Error(w, "활성화된 브라우저 세션이 없습니다. 먼저 브라우저 실행을 진행해주세요.", http.StatusBadRequest)
		return nil, "", false
	}
	return session, sessionID, true
}

func Login(w http.ResponseWriter, r *http.Request) {
	session, _, ok := requireSession(w, r)
	if !ok {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	page := session.page

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
	time.Sleep(1 * time.Second)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("로그인 완료"))
}

func Move(w http.ResponseWriter, r *http.Request) {
	session, _, ok := requireSession(w, r)
	if !ok {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	session.page.MustNavigate("https://www.auc.or.kr/reservation/program/lesson/list")
	time.Sleep(1 * time.Second)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("강습 신청 페이지 진입 완료"))
}

func Action(w http.ResponseWriter, r *http.Request) {
	session, _, ok := requireSession(w, r)
	if !ok {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	page := session.page

	code := r.URL.Query().Get("code")
	switch code {
	case "1":
		{
			// [구버전
			// sel := page.MustElement("#areaGbn")
			// sel.MustClick() // 드롭다운 열기
			// sel.MustSelect("호계스쿼시")
			// w.WriteHeader(http.StatusOK)
			// w.Write([]byte("강습 구분 선택 완료"))
			if forceSelect(page, "#areaGbn", "호계스쿼시") {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("강습 구분 선택 완료"))
			} else {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("강습 구분 선택 실패"))
			}
		}
	case "2":
		{
			// [구버전]
			// entranceType := page.MustElement("#entranceType")
			// entranceType.MustClick() // 드롭다운 열기
			// entranceType.MustSelect("화목(강습)")
			// w.WriteHeader(http.StatusOK)
			// w.Write([]byte("강습 과정 선택 완료"))
			if forceSelect(page, "#entranceType", "화목(강습)") {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("강습 과정 선택 완료"))
			} else {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("강습 과정 선택 실패"))
			}
		}
	case "3":
		{
			// <a href="#" onclick="insertOrderSeq('11','218','주2일(화,목)','03','주2일(화,목)','11:00 - 12:30','배드민턴','임미정');" class="common_btn regist">신청</a>
			btns := page.MustElements("a.common_btn.regist")
			clicked := false
			for _, btn := range btns {
				html := btn.MustProperty("outerHTML").String()

				// if strings.Contains(html, "주2일(화,목)") &&
				// 	strings.Contains(html, "11:00 - 12:30") &&
				// 	strings.Contains(html, "신청") {
				if strings.Contains(html, "화목(강습)") &&
					strings.Contains(html, "20:00 - 21:00") &&
					strings.Contains(html, "신청") {

					// [구버전]
					// btn.MustClick()

					btn.MustEval(`() => this.click()`)

					clicked = true
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("강습 시간 선택 완료"))
					break // 하나만 클릭하고 종료
				}
			}

			if !clicked {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("조건에 맞는 강습 시간 버튼을 찾지 못했습니다."))
			}
		}
	}
}

func Screenshot(w http.ResponseWriter, r *http.Request) {
	session, _, ok := requireSession(w, r)
	if !ok {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.page == nil {
		http.Error(w, "활성화된 페이지가 없습니다.", http.StatusBadRequest)
		return
	}

	data, err := session.page.Screenshot(true, nil)
	if err != nil {
		log.Printf("세션 화면 캡처 실패: %v", err)
		http.Error(w, "화면 캡처에 실패했습니다. 잠시 후 다시 시도해주세요.", http.StatusInternalServerError)
		return
	}

	resp := struct {
		Image      string    `json:"image"`
		CapturedAt time.Time `json:"capturedAt"`
	}{
		Image:      "data:image/png;base64," + base64.StdEncoding.EncodeToString(data),
		CapturedAt: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("스크린샷 응답 인코딩 실패: %v", err)
	}
}

// forceSelect 은 오버레이와 상관없이 <select>를 강제로 선택하고
// input/change 이벤트까지 발생시킵니다.
func forceSelect(page *rod.Page, sel string, want string) bool {
	return page.MustEval(`(sel, want) => {  
		const s = document.querySelector(sel);
		if (!s) return false;

		// 1) value로 매칭
		const byValue = s.querySelector('option[value="' + CSS.escape(want) + '"]');
		if (byValue) {
			s.value = byValue.value;
			s.dispatchEvent(new Event('input',  { bubbles:true }));
			s.dispatchEvent(new Event('change', { bubbles:true }));
			return true;
		}

		// 2) 표시 텍스트로 매칭
		const opts = Array.from(s.options);
		const found = opts.find(o => (o.textContent || '').trim() === want.trim());
		if (found) {
			s.value = found.value;
			s.dispatchEvent(new Event('input',  { bubbles:true }));
			s.dispatchEvent(new Event('change', { bubbles:true }));
			return true;
		}

		return false;
	}`, sel, want).Bool()
}
