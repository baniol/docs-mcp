package repo

// DocumentInfo holds metadata about a documentation file.
type DocumentInfo struct {
	Name        string
	Path        string // relative to docs path
	Size        int64
	Description string
}
