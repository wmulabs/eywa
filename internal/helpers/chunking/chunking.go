// Package chunking splits text into overlapping chunks for vector ingestion. It is a pure utility
// (no ports, no infrastructure). The recursive splitter prefers natural boundaries (paragraphs,
// lines, sentences, words) and is rune-safe — it never cuts a multi-byte character.
package chunking

import (
	"strings"
	"unicode/utf8"
)

// separators are tried in order: split on the coarsest boundary that keeps pieces under the size,
// falling back to finer ones, and finally a hard rune window ("").
var separators = []string{"\n\n", "\n", ". ", " ", ""}

// Recursive splits text into chunks of at most size runes, breaking on the coarsest natural boundary
// that fits and carrying overlap runes of context between consecutive chunks. size <= 0 or text that
// already fits returns the text as a single chunk; empty text returns nil.
func Recursive(text string, size, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if size <= 0 || utf8.RuneCountInString(text) <= size {
		return []string{text}
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= size {
		overlap = size / 2
	}

	atoms := splitToAtoms(text, size, overlap, separators)
	return mergeAtoms(atoms, size, overlap)
}

// splitToAtoms breaks text (always longer than size when called) into pieces each at most size runes,
// descending the separator hierarchy and hard-splitting (with overlap) only what no separator reduces.
func splitToAtoms(text string, size, overlap int, seps []string) []string {
	if len(seps) == 0 || seps[0] == "" {
		return hardSplit(text, size, overlap)
	}

	var atoms []string
	for _, part := range strings.Split(text, seps[0]) {
		if strings.TrimSpace(part) == "" {
			continue
		}
		if utf8.RuneCountInString(part) <= size {
			atoms = append(atoms, strings.TrimSpace(part))
		} else {
			atoms = append(atoms, splitToAtoms(part, size, overlap, seps[1:])...)
		}
	}
	return atoms
}

// mergeAtoms greedily joins atoms into chunks up to size, carrying the trailing overlap runes into the
// next chunk. Atoms are already each at most size runes.
func mergeAtoms(atoms []string, size, overlap int) []string {
	var chunks []string
	var cur []string
	curLen := 0

	flush := func() {
		if len(cur) > 0 {
			chunks = append(chunks, strings.Join(cur, " "))
		}
	}

	for _, a := range atoms {
		aLen := utf8.RuneCountInString(a)
		// +len(cur) approximates the spaces added when joining.
		if curLen+aLen+len(cur) > size && len(cur) > 0 {
			flush()
			for curLen > overlap && len(cur) > 0 {
				curLen -= utf8.RuneCountInString(cur[0])
				cur = cur[1:]
			}
		}
		cur = append(cur, a)
		curLen += aLen
	}
	flush()
	return chunks
}

// hardSplit cuts text that has no usable separator into overlapping rune windows of size, never
// splitting a multi-byte character. step = size - overlap carries context between windows.
func hardSplit(text string, size, overlap int) []string {
	runes := []rune(text)
	step := size - overlap
	if step <= 0 {
		step = size
	}
	var out []string
	for i := 0; i < len(runes); i += step {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		if chunk := strings.TrimSpace(string(runes[i:end])); chunk != "" {
			out = append(out, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return out
}
