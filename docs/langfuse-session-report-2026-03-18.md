# Raport analizy sesji Langfuse — 2026-03-18

**Model:** `qwen3-coder-next` (via LiteLLM → Zed)
**Czas sesji:** 18:35 – 18:56 (~21 min)
**Liczba wywołań LLM:** 50
**Tokeny łącznie:** 1 568 545 (input: 1 562 150, output: 6 395)
**Zadanie:** Naprawienie błędu `GITHUB_TOKEN is required` dla publicznych repozytoriów

---

## 1. Ogromne marnotrawstwo tokenów przez akumulację kontekstu

To największy problem. Konwersacja rozrosła się do **196 wiadomości** w ostatnim wywołaniu. Ostatnie wywołanie miało 40 391 tokenów wejściowych — za 379 tokenów outputu. Cała sesja: **1.56M tokenów wejściowych** na zadanie, które modyfikuje kilka plików.

**Przyczyna:** Zed wysyła pełną historię konwersacji przy każdym wywołaniu. Brak kompresji/summaryzacji.

**Rekomendacja:**
- Rozważ tryb "context pruning" — po każdej akcji narzędziowej stare `tool_result` można skracać do podsumowania
- Podziel złożone zadania na osobne, krótkie sesje zamiast jednej długiej

---

## 2. Wielokrotne odczyty tych samych plików

Model czytał `config.go` co najmniej 4-5 razy, `client.go` 2-3 razy. Każdy odczyt przepycha do kontekstu i do kolejnych wywołań.

**Rekomendacja:** W prompcie systemowym: *"Read each file only once. Reuse previously seen content from the conversation."*

---

## 3. Zbyt ogólne zadanie → błędna implementacja

User napisał: *"a przecież miało działać dla publicznych repo"* — bez precyzowania jakich publicznych repo. Model zinterpretował to jako "publicznych = tylko domyślne `tldr-pages/tldr`" i zakodował to dosłownie:

```go
const defaultRepo = "tldr-pages/tldr"
if c.GithubRepo != defaultRepo && c.GithubToken == "" {
    return nil, fmt.Errorf("GITHUB_TOKEN is required for private repositories")
}
```

Inne publiczne repozytoria (np. `golang/go`) dalej wymagają tokenu.

**Brakujące wymaganie w prompcie:** *"token powinien być opcjonalny dla wszystkich publicznych repozytoriów, nie tylko domyślnego."*

**Rekomendacja:** Precyzyjniejsze zadanie: *"make GITHUB_TOKEN fully optional — go-git supports nil auth for public repos, use that pattern"*

---

## 4. Model nie przetestował kluczowego scenariusza — bug w repo/client.go

Model zauważył problem z `Pull` bez tokenu, ale zamiast zweryfikować że go-git obsługuje `nil` auth dla publicznych repo, pominął sync całkowicie:

```go
// Initialize() — skips pull when no token
if auth != nil {
    pullErr := w.Pull(...)
} else {
    slog.Info("skipping git pull (no token, public repo)")
}

// Sync() — returns immediately when no token
if auth == nil {
    slog.Info("skipping git sync (no token, public repo)")
    return false, nil
}
```

Efekt: serwer nigdy nie zaktualizuje publicznego repo po starcie. Model zakończył zadanie nie testując tego scenariusza end-to-end.

**Prawidłowe rozwiązanie:** go-git obsługuje pull bez auth — wystarczy przekazać `nil`:

```go
pullErr := w.Pull(&gogit.PullOptions{
    Auth:          auth, // nil is valid for public repos
    ReferenceName: plumbing.NewBranchReferenceName(c.cfg.GithubBranch),
    SingleBranch:  true,
})
```

**Rekomendacja w prompcie:** *"verify that git pull works for public repos by passing nil auth to go-git PullOptions, then test with an actual public repo URL"*

---

## 5. Drobne błędy niedokończone po iteracjach

- **Literówka w README:** `tldr-pages/tlbl` zamiast `tldr-pages/tldr`
- **Zepsuty blok markdown** — otwarto nowy ` ```bash ` w środku istniejącego kodu (sekcja Development)
- **Brak commitu** po zakończeniu sesji

**Rekomendacja:** W system prompcie lub na końcu zadania: *"before finishing: check for typos in any modified documentation, then run `make check-all`"*

---

## 6. Bezpieczeństwo — dobra decyzja

Model poprawnie usunął hardcoded token z `docker-compose.yaml`. To była zmiana nieoczekiwana przez użytkownika, ale właściwa. Token `github_pat_11AABJXAQ0z6...` widoczny w git diff powinien zostać zrotowany jeśli jeszcze nie został.

---

## Podsumowanie rekomendacji

| Problem | Priorytet | Rozwiązanie |
|---|---|---|
| Akumulacja kontekstu 1.5M tokenów | Wysoki | Context pruning / krótsze sesje |
| Zbyt ogólny prompt → hardcoded fix | Wysoki | Precyzuj wymagania (nil auth, nie whitelist) |
| Brak testu end-to-end po zmianie | Wysoki | Wymagaj weryfikacji w prompcie |
| Wielokrotne odczyty tych samych plików | Średni | Instrukcja "read each file only once" |
| Brak quality check na końcu | Niski | `make check-all` jako ostatni krok zadania |
