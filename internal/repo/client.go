package repo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/baniol/docs-mcp/internal/config"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gogithttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Client provides access to a locally cloned Git repository.
type Client struct {
	mu       sync.Mutex
	cfg      *config.Config
	repoPath string
	docsPath string // absolute path to docs within repo
	repo     *gogit.Repository
}

func NewClient(cfg *config.Config) *Client {
	docsPath := filepath.Join(cfg.RepoPath, cfg.DocsPath)
	return &Client{
		cfg:      cfg,
		repoPath: cfg.RepoPath,
		docsPath: docsPath,
	}
}

// Initialize clones the repo if not present, or pulls if already there.
func (c *Client) Initialize() error {
	repoURL := fmt.Sprintf("https://github.com/%s.git", c.cfg.GithubRepo)
	var auth *gogithttp.BasicAuth
	if c.cfg.GithubToken != "" {
		auth = &gogithttp.BasicAuth{
			Username: "x-access-token",
			Password: c.cfg.GithubToken,
		}
	}

	_, statErr := os.Stat(filepath.Join(c.repoPath, ".git"))
	if statErr == nil {
		// Already cloned — open and pull
		slog.Info("opening existing repository", "path", c.repoPath)
		r, err := gogit.PlainOpen(c.repoPath)
		if err != nil {
			return fmt.Errorf("open repo: %w", err)
		}
		c.repo = r

		w, err := r.Worktree()
		if err != nil {
			return fmt.Errorf("worktree: %w", err)
		}
		pullErr := w.Pull(&gogit.PullOptions{
			Auth:          auth,
			ReferenceName: plumbing.NewBranchReferenceName(c.cfg.GithubBranch),
			SingleBranch:  true,
		})
		if pullErr != nil && pullErr != gogit.NoErrAlreadyUpToDate {
			slog.Warn("pull failed, continuing with existing data", "err", pullErr)
		}
	} else {
		// Clone
		slog.Info("cloning repository", "repo", c.cfg.GithubRepo, "dest", c.repoPath)
		r, err := gogit.PlainClone(c.repoPath, false, &gogit.CloneOptions{
			URL:           repoURL,
			Auth:          auth,
			ReferenceName: plumbing.NewBranchReferenceName(c.cfg.GithubBranch),
			SingleBranch:  true,
			Depth:         1,
			Progress:      os.Stdout,
		})
		if err != nil {
			return fmt.Errorf("clone: %w", err)
		}
		c.repo = r
		slog.Info("clone complete")
	}

	if _, err := os.Stat(c.docsPath); err != nil {
		return fmt.Errorf("docs path %q not found in repo", c.docsPath)
	}
	return nil
}

// ListDocs returns all markdown files under the docs path.
func (c *Client) ListDocs(includeSubdirs bool) ([]DocumentInfo, error) {
	var docs []DocumentInfo
	err := filepath.WalkDir(c.docsPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if !includeSubdirs && path != c.docsPath {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		rel, _ := filepath.Rel(c.docsPath, path)
		docs = append(docs, DocumentInfo{
			Name: d.Name(),
			Path: rel,
			Size: info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk docs path: %w", err)
	}
	slog.Info("listed docs", "count", len(docs))
	return docs, nil
}

// GetDocContent reads the content of a document by its relative path.
// Prevents path traversal.
func (c *Client) GetDocContent(docPath string) (string, error) {
	// Resolve absolute path
	abs, err := filepath.Abs(filepath.Join(c.docsPath, docPath))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	// Ensure it's within the docs directory
	docsAbs, _ := filepath.Abs(c.docsPath)
	if !strings.HasPrefix(abs, docsAbs+string(os.PathSeparator)) && abs != docsAbs {
		return "", fmt.Errorf("invalid path: %q", docPath)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("document not found: %s", docPath)
		}
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}

// Sync pulls latest changes. Returns true if HEAD changed.
// Safe for concurrent calls — only one Pull runs at a time.
func (c *Client) Sync() (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.repo == nil {
		return false, fmt.Errorf("repository not initialized")
	}

	head, err := c.repo.Head()
	if err != nil {
		return false, fmt.Errorf("get HEAD: %w", err)
	}
	oldHash := head.Hash()

	w, err := c.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("worktree: %w", err)
	}

	var auth *gogithttp.BasicAuth
	if c.cfg.GithubToken != "" {
		auth = &gogithttp.BasicAuth{
			Username: "x-access-token",
			Password: c.cfg.GithubToken,
		}
	}

	pullErr := w.Pull(&gogit.PullOptions{
		Auth:          auth,
		ReferenceName: plumbing.NewBranchReferenceName(c.cfg.GithubBranch),
		SingleBranch:  true,
	})
	if pullErr == gogit.NoErrAlreadyUpToDate {
		return false, nil
	}
	if pullErr != nil {
		return false, pullErr
	}

	newHead, err := c.repo.Head()
	if err != nil {
		return false, err
	}
	return oldHash != newHead.Hash(), nil
}

// HeadHash returns the current HEAD commit hash.
func (c *Client) HeadHash() string {
	if c.repo == nil {
		return ""
	}
	h, err := c.repo.Head()
	if err != nil {
		return ""
	}
	return h.Hash().String()
}
