package search

import "strings"

// Chunk holds a text chunk with its position in the source document.
type Chunk struct {
	Text  string
	Start int
	End   int
}

// ChunkDocument splits content into overlapping chunks.
func ChunkDocument(content string, chunkSize, overlap int) []Chunk {
	if len(content) <= chunkSize {
		return []Chunk{{Text: content, Start: 0, End: len(content)}}
	}

	var chunks []Chunk
	paragraphs := strings.Split(content, "\n\n")

	current := ""
	currentStart := 0
	pos := 0

	for i, para := range paragraphs {
		paraLen := len(para)

		if current != "" && len(current)+paraLen+2 > chunkSize {
			// Save current chunk
			chunks = append(chunks, Chunk{
				Text:  strings.TrimSpace(current),
				Start: currentStart,
				End:   currentStart + len(current),
			})
			// Overlap: carry last `overlap` chars into next chunk
			overlapStart := len(current) - overlap
			if overlapStart < 0 {
				overlapStart = 0
			}
			overlapText := strings.TrimSpace(current[overlapStart:])
			currentStart = currentStart + overlapStart
			current = overlapText + "\n\n" + para
		} else {
			if current == "" {
				currentStart = pos
			}
			if current == "" {
				current = para
			} else {
				current += "\n\n" + para
			}
		}
		pos += paraLen
		if i < len(paragraphs)-1 {
			pos += 2 // +2 for "\n\n" separator between paragraphs
		}
	}

	if strings.TrimSpace(current) != "" {
		chunks = append(chunks, Chunk{
			Text:  strings.TrimSpace(current),
			Start: currentStart,
			End:   currentStart + len(current),
		})
	}

	if len(chunks) == 0 {
		return []Chunk{{Text: content, Start: 0, End: len(content)}}
	}
	return chunks
}
