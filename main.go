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
		log.Println("listening:", url)
		log.Println("Ctrl+C to exit")
		openBrowser(url)
		if err := http.Serve(ln, mux); err != nil {
			log.Fatal(err)
		}
	}()

	select {} // Ctrl+C로 종료
}
