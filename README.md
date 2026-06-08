# Personal Finance Manager

Aplikacja do zarządzania finansami osobistymi napisana w Go. Projekt na zaliczenie przedmiotu PAW.

Umożliwia rejestrację użytkowników, dodawanie przychodów i wydatków, kategoryzowanie transakcji oraz śledzenie salda w czasie rzeczywistym przez WebSocket. Dodatkowo obsługuje upload paragonów i przeliczanie walut przez mikroserwis gRPC.

## Uruchomienie

```bash
docker compose up --build
```

- UI: http://localhost:3000
- API: http://localhost:8080
- Swagger: http://localhost:8081

## Testy

```bash
go test ./tests/... -v
```
