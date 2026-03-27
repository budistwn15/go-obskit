package gormx

import (
	"encoding/hex"
	"hash/fnv"
	"strings"
	"unicode"
)

func fingerprintSQL(statement string) string {
	norm := normalizeSQL(statement)
	if norm == "" {
		return ""
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(norm))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

func normalizeSQL(in string) string {
	s := strings.TrimSpace(strings.ToLower(in))
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := false
	lastByte := byte(0)
	inSingle := false
	inDouble := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inSingle {
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if ch == '\'' {
			inSingle = true
			if lastByte != '?' {
				b.WriteByte('?')
				lastByte = '?'
			}
			lastSpace = false
			continue
		}
		if ch == '"' {
			inDouble = true
			if lastByte != '?' {
				b.WriteByte('?')
				lastByte = '?'
			}
			lastSpace = false
			continue
		}
		if isDigit(ch) {
			if lastByte != '?' {
				b.WriteByte('?')
				lastByte = '?'
			}
			lastSpace = false
			continue
		}
		if unicode.IsSpace(rune(ch)) {
			if !lastSpace && b.Len() > 0 {
				b.WriteByte(' ')
				lastByte = ' '
				lastSpace = true
			}
			continue
		}
		b.WriteByte(ch)
		lastByte = ch
		lastSpace = false
	}
	return strings.TrimSpace(b.String())
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}
