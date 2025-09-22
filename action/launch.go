//go:build windows
// +build windows

package action

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

// GET http://127.0.0.1:9222/json/version 이 열려있는지 체크
func debugPortAlive() bool {
	c := &http.Client{Timeout: 700 * time.Millisecond}
	resp, err := c.Get("http://127.0.0.1:9222/json/version")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func Launch(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS != "windows" {
		http.Error(w, "Windows만 지원", http.StatusBadRequest)
		return
	}

	// 이미 9222가 살아있으면 다시 켤 필요 없음
	if debugPortAlive() {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"msg":  "이미 디버깅 포트(9222)가 열려 있습니다.",
			"port": 9222,
		})
		return
	}

	chrome := `C:\Program Files\Google\Chrome\Application\chrome.exe`
	if _, err := os.Stat(chrome); err != nil {
		// Edge(Chromium)로 폴백
		chrome = `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`
		if _, err2 := os.Stat(chrome); err2 != nil {
			http.Error(w, "Chrome/Edge 실행 파일을 찾지 못했습니다.", 500)
			return
		}
	}

	// 사용자 실제 프로필 경로 (당신이 원하는 그대로)
	userDataDir := filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\User Data`)
	if _, err := os.Stat(userDataDir); err != nil {
		http.Error(w, fmt.Sprintf("프로필 경로 확인 실패: %v", err), 500)
		return
	}

	// ⚠️ 주의: 이 프로필을 쓰려면 모든 크롬을 완전히 종료해야 합니다.
	// 안 그러면 기존 인스턴스가 재사용되어 --remote-debugging-port가 무시됩니다.
	if !canLockProfile(userDataDir) {
		http.Error(w, "Chrome이 실행 중 같습니다. 모든 Chrome 창을 종료한 뒤 다시 시도하세요.", http.StatusConflict)
		return
	}

	args := []string{
		"--remote-debugging-port=9222",
		"--user-data-dir=" + "C:\\ChromeTEMP",
	}

	cmd := exec.Command(chrome, args...)
	// 콘솔창 숨기기
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	// 필요 시 로그 보고 싶으면:
	// cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	if err := cmd.Start(); err != nil {
		http.Error(w, "Chrome 실행 실패: "+err.Error(), 500)
		return
	}

	// 9222 포트가 열릴 때까지 잠깐 대기
	ok := waitUntil(3*time.Second, 120*time.Millisecond, debugPortAlive)
	if !ok {
		http.Error(w, "DevTools 포트(9222) 오픈 확인 실패", 500)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":   true,
		"msg":  "Chrome 디버깅 포트로 실행 완료",
		"port": 9222,
	})
}

func waitUntil(deadline time.Duration, step time.Duration, fn func() bool) bool {
	t0 := time.Now()
	for time.Since(t0) < deadline {
		if fn() {
			return true
		}
		time.Sleep(step)
	}
	return false
}

// 간단한 프로필 락 감지 (아주 러프함; 운영 환경에선 더 정교하게)
func canLockProfile(dir string) bool {
	testPath := filepath.Join(dir, ".__sqh_lock_test")
	if err := os.WriteFile(testPath, []byte("x"), 0o600); err != nil {
		// 쓰기 실패면 대개 다른 크롬이 해당 프로필을 사용 중
		return false
	}
	_ = os.Remove(testPath)
	return true
}
