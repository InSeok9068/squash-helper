package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"squash-helper/client"
	"squash-helper/server"
)

//go:embed web_client/*
var webClientFS embed.FS

//go:embed web_server/*
var webServerFS embed.FS

func main() {
	flag.Parse()
	args := flag.Args()

	fmt.Println(args)

	// mod := "server"
	mod := "client"

	if len(args) > 0 {
		mod = args[0]
	}

	if mod == "client" {
		Client()
	} else {
		Server()
	}
}

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

func Client() {
	// 임베드 FS의 루트를 / 하위로 설정
	sub, err := fs.Sub(webClientFS, "web_client")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/launch", client.Launch)
	mux.HandleFunc("/action", client.Action)

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

func Server() {
	// 임베드 FS의 루트를 / 하위로 설정
	sub, err := fs.Sub(webServerFS, "web_server")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/launch", server.Launch)
	mux.HandleFunc("/login", server.Login)
	mux.HandleFunc("/move", server.Move)
	mux.HandleFunc("/action", server.Action)
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// 서버 실행
	fmt.Println("서버 실행 중... http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}
}
