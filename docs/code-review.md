# Code Review — docs-mcp

Data: 2026-03-18

## Podsumowanie

Projekt jest dobrze zorganizowany — standardowy layout Go (`cmd/`, `internal/`), czytelna separacja odpowiedzialności, testy przy kodzie, graceful shutdown, pre-commit hook. Poniżej lista znalezionych problemów.

---

## P0 — Krytyczne

### 1. Race condition na BM25Index

**Plik:** `internal/search/bm25.go`

`BM25Index` nie ma żadnej synchronizacji. `Search()` jest wywoływany z HTTP handlerów (wiele goroutines), a `Rebuild()` z syncera w osobnej goroutine. To jest data race — `Rebuild()` zeruje `docs` i `df` w trakcie gdy `Search()` może je iterować.

**Fix:** Dodać `sync.RWMutex` — `RLock` w `Search()`, `Lock` w `Index()`/`Rebuild()`.

### 2. Token w URL clone — wyciek w logach

**Plik:** `internal/repo/client.go:36,61`

`repoURL` zawiera token bezpośrednio w URL. Linia 61 loguje ten URL: `slog.Info("cloning repository", "url", repoURL, ...)`. Token trafi do logów.

**Fix:** Użyć `gogithttp.BasicAuth` (tak jak w `Sync()`) zamiast embeddowania tokena w URL, lub zamaskować URL w logach.

---

## P1 — Istotne

### 3. Webhook nie robi Sync() przed BuildIndex()

**Plik:** `internal/server/server.go:203-208`

Webhook triggeruje `go func() { handler.BuildIndex() }()` ale nigdy nie wywołuje `repo.Sync()` (pull). `BuildIndex()` czyta pliki z dysku, ale repo nie zostało zaktualizowane. Cały sens webhooka to natychmiastowa reakcja na push.

### 4. readBody — ręczna implementacja zamiast stdlib

**Plik:** `internal/server/server.go:236-247`

`readBody()` ręcznie czyta body i łamie pętlę na każdym błędzie, ale zawsze zwraca `nil` error. Jeśli read się wywali na prawdziwym I/O error, dane będą niekompletne a webhook to zaakceptuje.

**Fix:** Zamienić na `io.ReadAll(r.Body)`.

### 5. Hardcoded `docs/current_docs` w webhooku i formatowaniu

**Pliki:**
- `internal/server/server.go:190` — `strings.Contains(f, "docs/current_docs")`
- `internal/utils/format.go:42,68` — `blob/master/docs/current_docs/`

Te ścieżki powinny pochodzić z `Config.DocsPath`, nie być hardcoded. Jeśli ktoś zmieni `DOCS_PATH`, webhook przestanie reagować a linki będą błędne.

### 6. Hardcoded `master` w linkach GitHub

**Pliki:** `internal/handlers/handlers.go:139`, `internal/utils/format.go:42,68`

`blob/master/...` powinno być `blob/{cfg.GithubBranch}/...`. Default branch to `master` w configu, ale jeśli ktoś ustawi `main`, linki będą złe.

---

## P2 — Do poprawy

### 7. Niespójne auth w repo client

**Plik:** `internal/repo/client.go`

`Initialize()` embedduje token w URL (linia 36), ale `Sync()` używa `BasicAuth` (linia 166). Dwa różne mechanizmy auth dla tego samego repo — ujednolicić na `BasicAuth`.

### 8. envInt cicho ignoruje nieprawidłowe wartości

**Plik:** `internal/config/config.go:95-102`

Jeśli ktoś ustawi `PORT=abc`, dostanie domyślny port bez żadnego ostrzeżenia. To prawdopodobnie błąd konfiguracji i powinien być zgłoszony.

### 9. Handler.repo to konkretny typ, nie interfejs

**Plik:** `internal/handlers/handlers.go:18`

`repo *repo.Client` — zgodnie z Go conventions i CLAUDE.md ("interfaces defined at consumer side"), `Handler` powinien zależeć od interfejsu. Blokuje to łatwe testowanie — w `handlers_test.go` pole `repo` jest `nil`, co ogranicza pokrycie testowe.

### 10. MCP handler — hardcoded tool name

**Plik:** `internal/server/server.go:130`

`params.Name != "query_infrastructure_docs"` — jeśli dojdzie nowy tool, trzeba pamiętać o zmianie warunku. Lepiej oprzeć dispatching na mapie tool name -> handler function.

### 11. Custom min() w dwóch pakietach

**Pliki:** `internal/handlers/handlers.go:332`, `internal/docproc/processor.go:294`

Go 1.21+ ma wbudowane `min()`. Przy Go 1.26 te custom funkcje są zbędne i mogą kolidować z builtin.

### 12. Brak ValidateDocPath w ścieżce request

`utils/validate.go` definiuje `ValidateDocPath()` ale nigdzie nie jest wywoływana. `ReadDoc()` i `GetDocContent()` z niej nie korzystają. Albo użyć, albo usunąć martwy kod.

---

## P3 — Drobne

### 13. godotenv oznaczony jako indirect

`go.mod:23` — `godotenv` jest importowany bezpośrednio w `main.go` ale oznaczony `// indirect`. Uruchomić `go mod tidy`.

### 14. splitNonEmpty — niepotrzebna custom implementacja

`config.go:113-126` — zastąpić przez `strings.FieldsFunc(s, func(r rune) bool { return r == ',' })`.

### 15. ListDocs — nieużywany pattern

`repo/client.go:84-100` — zmienna `pattern` jest ustawiana ale nigdy nie używana (linia 100: `_ = pattern`).

### 16. config_test.go — ręczny cleanup env vars

Testy configu ustawiają zmienne env przez `os.Setenv`. Lepiej użyć `t.Setenv()` (Go 1.17+) — automatycznie cofa zmiany.

### 17. Brak context propagation w handlerach

Żaden handler nie przekazuje `r.Context()` w dół. Jeśli klient zamknie połączenie, serwer dalej przetwarza request.

### 18. Cache — brak eviction

Ekspirowane wpisy są usuwane tylko przy `Get()`. Jeśli klucz nigdy nie jest ponownie odczytany, wpis żyje w pamięci wiecznie.

---

## Podsumowanie priorytetów

| Prio | Issue | Effort |
|------|-------|--------|
| P0 | #1 Race condition na BM25Index | Mały |
| P0 | #2 Token w logach | Mały |
| P1 | #3 Webhook nie robi Sync | Mały |
| P1 | #4 readBody → io.ReadAll | Mały |
| P1 | #5-6 Hardcoded paths/branch | Mały |
| P2 | #7 Niespójne auth w repo | Mały |
| P2 | #9 Interfejs zamiast konkretu | Średni |
| P2 | #10-12 Reszta P2 | Mały |
| P3 | #13-18 Drobne | Mały |

## Ocena jako template

Projekt jest solidną bazą na template — layout, konwencje, testy, Makefile, graceful shutdown, pre-commit hook, CLAUDE.md. Po fixach P0/P1 gotowy do użycia jako wzorzec.
