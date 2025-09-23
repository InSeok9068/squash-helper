package client

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"

	"github.com/go-rod/rod"
)

//go:embed web/*
var webClientFS embed.FS

func openBrowser(url string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}

func Run() {
	// 임베드 FS의 루트를 / 하위로 설정
	sub, err := fs.Sub(webClientFS, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/launch", Launch)
	mux.HandleFunc("/action", Action)

	mux.Handle("/", http.FileServer(http.FS(sub)))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	url := "http://" + ln.Addr().String() + "/index.html"

	go func() {
		log.Println("===========================================")
		log.Println(" 🚀 서버가 성공적으로 시작되었습니다!")
		log.Println(" 👉 브라우저에서 아래 주소로 접속하세요:")
		log.Printf("     %s\n", url)
		log.Println()
		log.Println(" ⚠️  이 터미널 창을 닫으면 프로그램이 종료됩니다.")
		log.Println("    종료하지 마시고, 사용을 마친 뒤에만 닫아주세요.")
		log.Println("===========================================")
		openBrowser(url)
		if err := http.Serve(ln, mux); err != nil {
			log.Fatal(err)
		}
	}()

	select {} // Ctrl+C로 종료
}

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
