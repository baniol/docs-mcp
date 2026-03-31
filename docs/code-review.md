# Code Review — docs-mcp

Data: 2026-03-30 (aktualizacja)
Poprzedni review: 2026-03-18

## Podsumowanie

Projekt jest dobrze zorganizowany — standardowy layout Go (`cmd/`, `internal/`), czytelna separacja odpowiedzialnosci, testy przy kodzie, graceful shutdown, pre-commit hook, CLAUDE.md. Od poprzedniego review naprawiono wszystkie P0 i P1, wiekszosc P2. Ponizej aktualna lista problemow.

---

## Status poprzedniego review (2026-03-18)

Wszystkie problemy P0/P1 naprawione:

| # | Problem | Status |
|---|---------|--------|
| P0 #1 | Race condition na BM25Index | FIXED — dodano `sync.RWMutex` |
| P0 #2 | Token w URL clone / wyciek w logach | FIXED — `BasicAuth` zamiast token w URL |
| P1 #3 | Webhook nie robi Sync() | FIXED — webhook wywoluje `SyncRepo()` |
| P1 #4 | readBody reczna implementacja | FIXED — zamienione na `io.ReadAll` |
| P1 #5 | Hardcoded `docs/current_docs` | FIXED — uzywa `cfg.DocsPath` |
| P1 #6 | Hardcoded `master` w linkach | FIXED — uzywa `cfg.GithubBranch` |
| P2 #7 | Niespojne auth w repo client | FIXED — oba uzyja `BasicAuth` |
| P2 #8 | envInt cicho ignoruje | FIXED — dodano `slog.Warn` |
| P2 #9 | Handler.repo to konkretny typ | FIXED — interfejs `RepoClient` |
| P2 #10 | Hardcoded tool name w MCP | FIXED — `CallTool` dispatch |
| P2 #11 | Custom min() | FIXED — uzywa builtin `min()` |
| P2 #12 | ValidateDocPath nieuzywany | FIXED — usunieto |
| P3 #13 | godotenv indirect | FIXED — poprawnie w require |
| P3 #14 | splitNonEmpty | OK — uproszczony do one-liner |
| P3 #15 | ListDocs unused pattern | FIXED — usunieto |
| P3 #16 | config_test.go t.Setenv | CZESCIOWO — patrz #8 ponizej |
| P3 #17 | Brak context propagation | OTWARTE — patrz #9 ponizej |
| P3 #18 | Cache brak eviction | OTWARTE — patrz #4 ponizej |

---

## Nowe i otwarte problemy

## P1 — Istotne

### 1. ~~Race condition: webhook sync vs background syncer~~ FIXED

**Plik:** `internal/repo/client.go`

Dodano `sync.Mutex` do `repo.Client`, lockowany w `Sync()`. Serializuje rownoczesne wywolania z webhook i background syncer.

### 2. ~~Brak limitu na webhook body~~ FIXED

**Plik:** `internal/server/server.go`

Dodano `http.MaxBytesReader(w, r.Body, 1<<20)` (1 MB limit). Przekroczenie zwraca 413.

### 3. ~~Chunki indeksowane ale nieuzywane w wyszukiwaniu~~ FIXED

**Plik:** `internal/search/bm25.go`

Usunieto pole `chunks` z `docEntry` oraz wywolanie `ChunkDocument()` z indeksowania. Usunieto `chunkSize`/`chunkOverlap` z `BM25Index` i uproszczono `NewBM25Index()`. Chunker (`chunker.go`) zachowany na przyszlosc.

---

## P2 — Do poprawy

### 4. Cache bez limitu rozmiaru i bez proaktywnej eviction

**Plik:** `internal/utils/cache.go`

Cache rosnie bez ograniczen. Kazdy unikalny query tworzy nowy wpis (`search_<query>_<max>`). Ekspirowane wpisy sa usuwane tylko przy `Get()` — jesli klucz nigdy nie jest ponownie odczytany, wpis zyje w pamieci wiecznie.

**Fix:** Dodac max entries (np. LRU) lub periodic eviction (goroutine z timerem).

### 5. ReadDoc — niespójna logika cache

**Plik:** `internal/handlers/handlers.go:193-199`

```go
if v, ok := h.cache.Get(cacheKey); ok {
    cached := v.(string)
    if len(cached) <= maxLength {
        return text(cached)
    }
}
```

Jesli cached string jest dluzszy niz `maxLength`, cache hit jest ignorowany, ale zawartosc jest czytana od nowa bez zapisu do cache z innym kluczem. Powoduje powtarzalne chybienia.

### 6. ~~Server start error nie jest propagowany~~ FIXED

**Plik:** `cmd/server/main.go`

Dodano kanal `errCh` — jesli `srv.Start()` zwroci error natychmiast, program loguje i wychodzi z kodem 1 zamiast wisiec na `<-quit`.

### 7. Hardcoded wersja serwera

**Plik:** `internal/server/server.go:116`

```go
"version": "1.0.0",
```

Wersja jest na sztywno. Powinna byc brana z tagu git / zmiennej buildowej (`-ldflags "-X main.version=..."`) i przekazywana przez config.

---

## P3 — Drobne

### 8. ~~config_test.go — os.Unsetenv bez cleanup~~ FIXED

**Plik:** `internal/config/config_test.go`

Zamieniono wszystkie `os.Unsetenv()` na `t.Setenv("VAR", "")` — cleanup automatyczny po tescie.

### 9. Brak context propagation w handlerach

**Plik:** `internal/server/server.go`, `internal/handlers/handlers.go`

Zaden handler nie przekazuje `r.Context()` w dol. Jesli klient zamknie polaczenie, serwer dalej przetwarza request (czyta pliki, odpytuje indeks).

### 10. ~~tokenize() alokuje stop words map przy kazdym wywolaniu~~ FIXED

**Plik:** `internal/search/bm25.go`

Przeniesiono `stopWords` na package-level `var`.

### 11. ~~Brak IdleTimeout na HTTP server~~ FIXED

**Plik:** `internal/server/server.go`

Dodano `IdleTimeout: 120 * time.Second`.

### 12. ~~Nieuzywane pole SHA w DocumentInfo~~ FIXED

**Plik:** `internal/repo/types.go`

Usunieto nieuzywane pole `SHA` z `DocumentInfo`.

### 13. ~~BasicAuth — username convention~~ FIXED

**Plik:** `internal/repo/client.go`

Zmieniono na `Username: "x-access-token"`, `Password: token` — dziala zarowno dla PAT jak i GitHub App installation tokens.

### 14. API key porownywanie — timing side-channel

**Plik:** `internal/server/middleware.go:26`

```go
if keySet[token] {
```

Map lookup nie jest constant-time. Niski risk przy API keys, ale `subtle.ConstantTimeCompare` byloby poprawniejsze.

### 15. ~~Markdown table separator detection~~ FIXED

**Plik:** `internal/docproc/processor.go`

Zamieniono `strings.HasPrefix(line, "|--")` na regex `^\|[\s:]*-+[\s:|-]*\|$` — lapie `| --- | --- |`, `|:---|:---|` itd.

### 16. ~~ChunkDocument — pos tracking off-by-error~~ FIXED

**Plik:** `internal/search/chunker.go`

Dodano warunek `if i < len(paragraphs)-1` przed dodaniem +2 — ostatni paragraf nie dodaje separatora.

### 17. ~~min8() zamiast builtin min~~ FIXED

**Plik:** `internal/syncer/syncer.go`

Zamieniono custom `min8()` na builtin `min(8, len(s))` i usunieto funkcje.

---

## Podsumowanie priorytetow

| Prio | # | Problem | Effort |
|------|---|---------|--------|
| ~~P1~~ | ~~1~~ | ~~Race condition webhook vs syncer~~ | FIXED |
| ~~P1~~ | ~~2~~ | ~~Brak limitu na webhook body~~ | FIXED |
| ~~P1~~ | ~~3~~ | ~~Chunki indeksowane ale nieuzywane~~ | FIXED |
| P2 | 4 | Cache bez limitu / eviction | Sredni |
| P2 | 5 | ReadDoc niespojny cache | Maly |
| ~~P2~~ | ~~6~~ | ~~Server start error nie propagowany~~ | FIXED |
| P2 | 7 | Hardcoded wersja serwera | Maly |
| ~~P3~~ | ~~8~~ | ~~config_test.go os.Unsetenv~~ | FIXED |
| P3 | 9 | Brak context propagation | OTWARTE |
| ~~P3~~ | ~~10~~ | ~~tokenize() stop words alokacja~~ | FIXED |
| ~~P3~~ | ~~11~~ | ~~Brak IdleTimeout~~ | FIXED |
| ~~P3~~ | ~~12~~ | ~~Nieuzywane pole SHA~~ | FIXED |
| ~~P3~~ | ~~13~~ | ~~BasicAuth username convention~~ | FIXED |
| P3 | 14 | API key timing side-channel | Maly |
| ~~P3~~ | ~~15~~ | ~~Markdown table separator~~ | FIXED |
| ~~P3~~ | ~~16~~ | ~~ChunkDocument pos off-by-error~~ | FIXED |
| ~~P3~~ | ~~17~~ | ~~min8() zamiast builtin min~~ | FIXED |
