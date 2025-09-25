//go:build !windows

package client

import "net/http"

// 리눅스/맥 등 비윈도우 환경에서 컴파일만 되게 하는 스텁
func Launch(w http.ResponseWriter, r *http.Request) {
	// 아무 것도 안 함 (필요시 400 반환해도 됨)
	// http.Error(w, "Launch is not supported on this OS", http.StatusBadRequest)
}
