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
	browser    *rod.Browser
	page       *rod.Page
	mu         sync.Mutex
	createdAt  time.Time
	lastActive time.Time

	statusMu      sync.Mutex
	statusCh      chan statusEvent
	lastStatus    statusEvent
	hasLastStatus bool
}

type statusEvent struct {
	Level   string    `json:"level"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

const (
	sessionCookieName = "squash-helper-session"
	sessionTTL        = time.Hour
)

var (
	sessionMu sync.RWMutex
	sessions  = make(map[string]*userSession)
)

func (s *userSession) pushStatus(level, message string) {
	if s == nil {
		return
	}

	level = strings.TrimSpace(level)
	if level == "" {
		level = "info"
	}

	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	ev := statusEvent{
		Level:   level,
		Message: message,
		At:      time.Now(),
	}

	s.statusMu.Lock()
	s.lastStatus = ev
	s.hasLastStatus = true
	if s.statusCh != nil {
		select {
		case s.statusCh <- ev:
		default:
		}
	}
	s.statusMu.Unlock()
}

func (s *userSession) pushInfo(message string) {
	s.pushStatus("info", message)
}

func (s *userSession) pushError(message string) {
	s.pushStatus("error", message)
}

func (s *userSession) subscribeStatus() (chan statusEvent, []statusEvent, func()) {
	if s == nil {
		return nil, nil, func() {}
	}

	s.statusMu.Lock()
	prev := s.statusCh
	if prev != nil {
		close(prev)
	}
	ch := make(chan statusEvent, 16)
	s.statusCh = ch
	var history []statusEvent
	if s.hasLastStatus {
		history = append(history, s.lastStatus)
	}
	s.statusMu.Unlock()

	cleanup := func() {
		s.statusMu.Lock()
		if s.statusCh == ch {
			close(ch)
			s.statusCh = nil
		}
		s.statusMu.Unlock()
	}

	return ch, history, cleanup
}

func (s *userSession) closeStatusChannel() {
	if s == nil {
		return
	}

	s.statusMu.Lock()
	if s.statusCh != nil {
		close(s.statusCh)
		s.statusCh = nil
	}
	s.statusMu.Unlock()
}

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
	mux.HandleFunc("/refresh", Refresh)
	mux.HandleFunc("/close", Close)
	mux.HandleFunc("/status/stream", StatusStream)
	mux.Handle("/", http.FileServer(http.FS(sub)))

	go startSessionReaper()

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
	now := time.Now()
	if session != nil {
		if session.createdAt.IsZero() {
			session.createdAt = now
		}
		session.mu.Lock()
		session.lastActive = now
		session.mu.Unlock()
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
	if session.browser != nil {
		if err := session.browser.Close(); err != nil {
			log.Printf("세션 %s 브라우저 종료 실패: %v", sessionID, err)
		}
	}
	session.browser = nil
	session.page = nil
	session.mu.Unlock()

	session.pushInfo("세션이 종료되었습니다.")
	session.closeStatusChannel()
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

	sessionMu.Lock()
	session, ok := sessions[cookie.Value]
	sessionMu.Unlock()

	if ok && session != nil {
		session.mu.Lock()
		session.lastActive = time.Now()
		session.mu.Unlock()
	}

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

func startSessionReaper() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for now := range ticker.C {
		var expired []string

		sessionMu.RLock()
		for id, session := range sessions {
			if session == nil {
				expired = append(expired, id)
				continue
			}

			session.mu.Lock()
			lastActive := session.lastActive
			session.mu.Unlock()

			if now.Sub(lastActive) > sessionTTL {
				expired = append(expired, id)
			}
		}
		sessionMu.RUnlock()

		for _, id := range expired {
			log.Printf("세션 %s이(가) 비활성 상태로 만료되어 종료합니다.", id)
			cleanupSession(id)
		}
	}
}

func Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST 메서드만 허용됩니다.", http.StatusMethodNotAllowed)
		return
	}

	session, _, ok := requireSession(w, r)
	if !ok {
		return
	}

	session.pushInfo("로그인 요청을 처리합니다.")

	defer r.Body.Close()
	var payload struct {
		ID       string `json:"id"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "요청 본문 파싱에 실패했습니다.", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(payload.ID) == "" || strings.TrimSpace(payload.Password) == "" {
		session.pushError("아이디와 비밀번호를 모두 입력해 주세요.")
		http.Error(w, "아이디와 비밀번호를 모두 입력해주세요.", http.StatusBadRequest)
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	page := session.page

	// 아이디 입력
	session.pushInfo("아이디 입력 필드를 찾습니다.")
	login_id := page.MustElement("#login_id")
	login_id.MustInput(payload.ID)
	session.pushInfo("아이디 입력을 완료했습니다.")
	time.Sleep(1 * time.Second)

	// 비밀번호 입력
	session.pushInfo("비밀번호 입력 필드를 찾습니다.")
	login_password := page.MustElement("#login_pwd")
	login_password.MustInput(payload.Password)
	session.pushInfo("비밀번호 입력을 완료했습니다.")
	time.Sleep(1 * time.Second)

	// 로그인 버튼 클릭
	session.pushInfo("로그인 버튼을 클릭합니다.")
	buttons := page.MustElements("button")
	for _, button := range buttons {
		if button.MustText() == "로그인" {
			button.MustClick()
			session.pushInfo("로그인 버튼을 클릭했습니다.")
			break
		}
	}
	// 페이지 진입 대기
	session.pushInfo("로그인 결과를 확인 중입니다.")
	page.MustWaitLoad()
	time.Sleep(3 * time.Second)

	url := page.MustInfo().URL
	if strings.HasPrefix(url, "https://newsso.anyang.go.kr/") {
		session.pushError("로그인에 실패했습니다. 아이디와 비밀번호를 확인해 주세요.")
		http.Error(w, "로그인 실패하였습니다. 아이디와 비밀번호를 확인해주세요.", http.StatusForbidden)
		return
	}

	session.pushInfo("로그인에 성공했습니다.")
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

	page := session.page
	session.pushInfo("강습 신청 페이지로 이동합니다.")
	page.MustNavigate("https://www.auc.or.kr/reservation/program/lesson/list")
	session.pushInfo("강습 신청 페이지를 불러오는 중입니다.")
	// 페이지 진입 대기
	page.MustWaitLoad()
	removeWaitPage(page)
	session.pushInfo("강습 신청 페이지 진입을 완료했습니다.")
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
	if code != "" {
		session.pushInfo(fmt.Sprintf("요청 코드 %s 작업을 시작합니다.", code))
	}

	switch code {
	case "1":
		session.pushInfo("강습 구분을 선택합니다.")
		if forceSelect(page, "#areaGbn", "호계스쿼시") {
			// 페이지 진입 대기
			page.MustWaitLoad()
			removeWaitPage(page)
			session.pushInfo("강습 구분 선택을 완료했습니다.")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("강습 구분 선택 완료"))
		} else {
			session.pushError("강습 구분 선택에 실패했습니다.")
			http.Error(w, "강습 구분 선택 실패", http.StatusNotFound)
		}
	case "2":
		session.pushInfo("강습 과정을 선택합니다.")
		if forceSelect(page, "#entranceType", "화목(강습)") {
			// 페이지 진입 대기
			page.MustWaitLoad()
			removeWaitPage(page)
			session.pushInfo("강습 과정 선택을 완료했습니다.")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("강습 과정 선택 완료"))
		} else {
			session.pushError("강습 과정 선택에 실패했습니다.")
			http.Error(w, "강습 과정 선택 실패", http.StatusNotFound)
		}
	case "3":
		session.pushInfo("조건에 맞는 강습 시간을 찾는 중입니다.")
		btns := page.MustElements("a.common_btn.regist")
		clicked := false
		for _, btn := range btns {
			html := btn.MustProperty("outerHTML").String()

			if strings.Contains(html, "화목(강습)") &&
				strings.Contains(html, "20:00 - 21:00") &&
				strings.Contains(html, "신청") {
				btn.MustEval(`() => this.click()`)
				clicked = true
				// 페이지 진입 대기
				page.MustWaitLoad()
				session.pushInfo("강습 시간 선택을 완료했습니다.")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("강습 시간 선택 완료"))
				break
			}
		}

		if !clicked {
			session.pushError("조건에 맞는 강습 시간을 찾지 못했습니다.")
			http.Error(w, "조건에 맞는 강습 시간 버튼을 찾지 못했습니다.", http.StatusNotFound)
		}
	case "4":
		session.pushInfo("강습 목록 페이지로 이동합니다.")
		page.MustNavigate("https://www.auc.or.kr/reservation/program/lesson/list")
		// 페이지 진입 대기
		page.MustWaitLoad()
		session.pushInfo("강습 목록 페이지 로딩이 완료되었습니다.")
		time.Sleep(500 * time.Millisecond)

		session.pushInfo("강습 구분을 선택합니다.")
		if forceSelect(page, "#areaGbn", "호계스쿼시") {
			// 페이지 진입 대기
			page.MustWaitLoad()
			removeWaitPage(page)
			session.pushInfo("강습 구분 선택을 완료했습니다.")
			time.Sleep(500 * time.Millisecond)
		} else {
			session.pushError("강습 구분 선택에 실패했습니다.")
			http.Error(w, "강습 구분 선택 실패", http.StatusNotFound)
			return
		}

		session.pushInfo("강습 과정을 선택합니다.")
		if forceSelect(page, "#entranceType", "화목(강습)") {
			// 페이지 진입 대기
			page.MustWaitLoad()
			removeWaitPage(page)
			session.pushInfo("강습 과정 선택을 완료했습니다.")
			time.Sleep(500 * time.Millisecond)
		} else {
			session.pushError("강습 과정 선택에 실패했습니다.")
			http.Error(w, "강습 과정 선택 실패", http.StatusNotFound)
			return
		}

		session.pushInfo("조건에 맞는 정기 강습 시간을 찾는 중입니다.")
		btns := page.MustElements("a.common_btn.regist")
		clicked := false
		for _, btn := range btns {
			html := btn.MustProperty("outerHTML").String()

			if strings.Contains(html, "화목(강습)") &&
				strings.Contains(html, "20:00 - 21:00") &&
				strings.Contains(html, "신청") {
				btn.MustEval(`() => this.click()`)
				clicked = true
				// 페이지 진입 대기
				page.MustWaitLoad()
				removeWaitPage(page)
				session.pushInfo("강습 시간 선택을 완료했습니다.")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("강습 시간 선택 완료"))
				break
			}
		}

		if !clicked {
			session.pushError("조건에 맞는 강습 시간을 찾지 못했습니다.")
			http.Error(w, "조건에 맞는 강습 시간 버튼을 찾지 못했습니다.", http.StatusNotFound)
		}
	default:
		http.Error(w, "알 수 없는 작업 코드입니다.", http.StatusBadRequest)
	}
}

func StatusStream(w http.ResponseWriter, r *http.Request) {
	session, _, ok := requireSession(w, r)
	if !ok {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE를 지원하지 않는 환경입니다.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, history, cleanup := session.subscribeStatus()
	defer cleanup()

	sendEvent := func(ev statusEvent) bool {
		payload, err := json.Marshal(ev)
		if err != nil {
			log.Printf("상태 이벤트 직렬화 실패: %v", err)
			return true
		}

		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return false
		}

		flusher.Flush()
		return true
	}

	for _, ev := range history {
		if !sendEvent(ev) {
			return
		}
	}

	if len(history) == 0 {
		session.pushInfo("상태 모니터링이 연결되었습니다.")
	}

	ctx := r.Context()
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if !sendEvent(ev) {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func Screenshot(w http.ResponseWriter, r *http.Request) {
	session, _, ok := requireSession(w, r)
	if !ok {
		return
	}

	session.pushInfo("스크린샷을 요청했습니다.")

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.page == nil {
		http.Error(w, "활성화된 페이지가 없습니다.", http.StatusBadRequest)
		return
	}

	data, err := session.page.Screenshot(true, nil)
	if err != nil {
		log.Printf("세션 화면 캡처 실패: %v", err)
		session.pushError("스크린샷 캡처에 실패했습니다.")
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

	session.pushInfo("스크린샷 데이터를 준비했습니다.")
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
