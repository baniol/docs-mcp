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

### 3. Chunki indeksowane ale nieuzywane w wyszukiwaniu

**Plik:** `internal/search/bm25.go:70`

```go
chunks: ChunkDocument(content, b.chunkSize, b.chunkOverlap),
```

Chunki sa tworzone i przechowywane w kazdym `docEntry`, ale scoring operuje na pelnodokumentowym `termFreq` (z calego `content`). Chunki nigdzie nie sa czytane po zapisaniu. To niepotrzebna alokacja pamieci — kazdy dokument trzyma zduplikowana tresc (raz jako `content`, raz jako chunki).

**Fix:** Albo usunac chunking z indeksu (jesli BM25 ma dzialac na poziomie dokumentu), albo zaimplementowac chunk-level scoring (lepsze wyniki dla duzych dokumentow — snippet extraction moze tez korzystac z chunkow).

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

### 6. Server start error nie jest propagowany

**Plik:** `cmd/server/main.go:93-97`

```go
go func() {
    if err := srv.Start(); err != nil {
        slog.Error("server error", "err", err)
    }
}()
```

Jesli `ListenAndServe` zwroci error natychmiast (port zajety), program loguje blad ale wisi czekajac na signal (linia 99: `<-quit`). Powinien wyjsc z kodem 1.

**Fix:** Uzyc kanalu error:
```go
errCh := make(chan error, 1)
go func() { errCh <- srv.Start() }()
select {
case <-quit: // graceful shutdown
case err := <-errCh: // startup failure
    slog.Error("server failed", "err", err)
    os.Exit(1)
}
```

### 7. Hardcoded wersja serwera

**Plik:** `internal/server/server.go:116`

```go
"version": "1.0.0",
```

Wersja jest na sztywno. Powinna byc brana z tagu git / zmiennej buildowej (`-ldflags "-X main.version=..."`) i przekazywana przez config.

---

## P3 — Drobne

### 8. config_test.go — os.Unsetenv bez cleanup

**Plik:** `internal/config/config_test.go:10-12, 24`

`TestLoad_MissingToken_DefaultRepo` uzywa `os.Unsetenv()` bezposrednio — nie `t.Setenv()`. Te zmiany nie sa cofane po tescie, co moze wplynac na inne testy przy rownoczesnym uruchamianiu.

**Fix:** Zamienic na `t.Setenv("GITHUB_TOKEN", "")` + `t.Setenv("GITHUB_REPO", "")` itd. (lub dedykowany helper).

### 9. Brak context propagation w handlerach

**Plik:** `internal/server/server.go`, `internal/handlers/handlers.go`

Zaden handler nie przekazuje `r.Context()` w dol. Jesli klient zamknie polaczenie, serwer dalej przetwarza request (czyta pliki, odpytuje indeks).

### 10. tokenize() alokuje stop words map przy kazdym wywolaniu

**Plik:** `internal/search/bm25.go:181`

```go
func tokenize(text string) []string {
    stopWords := map[string]bool{...}
```

Mapa stop words jest tworzona od nowa przy kazdym wywolaniu `tokenize()` — a ta funkcja jest wolana dla kazdego dokumentu przy indeksowaniu i dla kazdego query przy wyszukiwaniu. Powinna byc package-level `var`.

### 11. Brak IdleTimeout na HTTP server

**Plik:** `internal/server/server.go:31-36`

`ReadTimeout` i `WriteTimeout` sa ustawione, ale brakuje `IdleTimeout`. Przy keep-alive connections idle polaczenia moga akumulowac sie.

### 12. Nieuzywane pole SHA w DocumentInfo

**Plik:** `internal/repo/types.go:8`

`DocumentInfo` ma pole `SHA` ale nigdzie nie jest wypelniane w `ListDocs`. Albo uzywac, albo usunac.

### 13. BasicAuth — username convention

**Plik:** `internal/repo/client.go:38-41`

```go
auth = &gogithttp.BasicAuth{
    Username: c.cfg.GithubToken,
    Password: "",
}
```

Standardowa konwencja GitHub API to `Username: "x-access-token"`, `Password: token`. Obecna forma dziala dla PAT-ow, ale nie dla GitHub App installation tokens.

### 14. API key porownywanie — timing side-channel

**Plik:** `internal/server/middleware.go:26`

```go
if keySet[token] {
```

Map lookup nie jest constant-time. Niski risk przy API keys, ale `subtle.ConstantTimeCompare` byloby poprawniejsze.

### 15. Markdown table separator detection

**Plik:** `internal/docproc/processor.go:51`

```go
!strings.HasPrefix(line, "|--")
```

Nie lapie typowych separatorow `| --- | --- |` ani `|:---|:---|`. Moze blednie traktowac separator jako dane.

### 16. ChunkDocument — pos tracking off-by-error

**Plik:** `internal/search/chunker.go:53`

```go
pos += paraLen + 2 // +2 for "\n\n"
```

Przy ostatnim paragrafie (brak trailing `\n\n`) `pos` liczy dodatkowe 2 znaki. Chunk `End` moze przekraczac dlugosc content.

### 17. min8() zamiast builtin min

**Plik:** `internal/syncer/syncer.go:71-76`

Custom `min8()` moze byc zastapione przez `min(8, len(s))` z Go 1.21+ builtin.

---

## Podsumowanie priorytetow

| Prio | # | Problem | Effort |
|------|---|---------|--------|
| ~~P1~~ | ~~1~~ | ~~Race condition webhook vs syncer~~ | FIXED |
| ~~P1~~ | ~~2~~ | ~~Brak limitu na webhook body~~ | FIXED |
| P1 | 3 | Chunki indeksowane ale nieuzywane | Sredni |
| P2 | 4 | Cache bez limitu / eviction | Sredni |
| P2 | 5 | ReadDoc niespojny cache | Maly |
| P2 | 6 | Server start error nie propagowany | Maly |
| P2 | 7 | Hardcoded wersja serwera | Maly |
| P3 | 8-17 | Drobne | Maly |
