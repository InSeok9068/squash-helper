package main

import (
	"embed"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"squash-helper/action"
)

//go:embed web/*
var webFS embed.FS

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

func main() {
	// 임베드 FS의 루트를 / 하위로 설정
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/launch", action.Launch)
	mux.HandleFunc("/action", action.Action)

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
