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
