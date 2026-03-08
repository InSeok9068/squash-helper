# 스쿼시 강습 신청

## 로그 확인

```shell
docker compose logs -f squash-helper
```

## 재시작

```shell
docker compose restart squash-helper 
```

## 이미지 갱신 후 재시작

```shell
docker compose pull squash-helper && docker compose up -d squash-helper
```

## 브라우저 베이스 이미지

Chromium, 폰트, 인증서, tzdata는 `browser-base` 이미지로 분리해 재사용합니다.
평소 `release` 배포는 이 베이스 이미지를 기반으로 Go 바이너리만 다시 빌드합니다.

- 최초 1회: GitHub Actions의 `Build Browser Base Image` 워크플로를 수동 실행
- 이후 갱신: Chromium/폰트 패키지를 바꿨을 때만 수동 실행
- Chromium/폰트 패키지 구성을 바꾸면 workflow의 `BROWSER_BASE_TAG`도 함께 올려 새 태그를 발행
- 로컬 단일 빌드: `docker build -t squash-helper .`
