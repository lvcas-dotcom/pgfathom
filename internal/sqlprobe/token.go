// Package sqlprobe mines join predicates from SQL the database itself stores:
// view definitions, function bodies and, when available, the normalized query
// log. A join there is proof the real code treats two columns as related,
// whatever they are named.
//
// It is deliberately not a parser. It recognizes exactly one syntactic form —
// an equality between two qualified column references — and has explicit
// permission to ignore everything else. A missed join is a missed signal; the
// candidate falls back to name inference. A wrongly extracted join produces a
// bad candidate that dies in validation. Neither is a wrong answer, which is
// what makes going without a real parser (and without cgo) safe.
package sqlprobe

import "strings"

type tokenKind int

const (
	// tokIdent is a name: bare identifiers arrive lowercased, as the server
	// folds them; quoted identifiers keep case and spaces.
	tokIdent tokenKind = iota

	// tokSymbol is punctuation or an operator run, verbatim.
	tokSymbol

	// tokOther is anything the extractor must see as "not a name": numbers,
	// parameters, unrecognized bytes.
	tokOther
)

type token struct {
	kind tokenKind
	text string
}

// tokenize walks the SQL and emits only what extraction needs. Comments,
// strings and dollar-quoted blocks are consumed and never emitted: an "="
// inside any of them would otherwise become a phantom predicate.
func tokenize(sql string) []token {
	var out []token
	i := 0

	for i < len(sql) {
		c := sql[i]

		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++

		case c == '-' && hasAt(sql, i+1, '-'):
			i = skipLineComment(sql, i+2)

		case c == '/' && hasAt(sql, i+1, '*'):
			i = skipBlockComment(sql, i+2)

		case c == '\'':
			i = skipString(sql, i+1, isEscapePrefixed(sql, i))

		case c == '$':
			next, ok := skipDollarQuoted(sql, i)
			if ok {
				i = next
			} else {
				// A parameter like $1, or a stray dollar.
				out = append(out, token{kind: tokOther, text: "$"})
				i++
			}

		case c == '"':
			text, next := readQuotedIdent(sql, i+1)
			out = append(out, token{kind: tokIdent, text: text})
			i = next

		case isIdentStart(c):
			start := i
			for i < len(sql) && isIdentPart(sql[i]) {
				i++
			}
			out = append(out, token{kind: tokIdent, text: strings.ToLower(sql[start:i])})

		case c >= '0' && c <= '9':
			for i < len(sql) && (isIdentPart(sql[i]) || sql[i] == '.') {
				i++
			}
			out = append(out, token{kind: tokOther, text: "number"})

		case isOperatorChar(c):
			start := i
			for i < len(sql) && isOperatorChar(sql[i]) {
				i++
			}
			out = append(out, token{kind: tokSymbol, text: sql[start:i]})

		default:
			out = append(out, token{kind: tokSymbol, text: string(c)})
			i++
		}
	}
	return out
}

func hasAt(s string, i int, c byte) bool { return i < len(s) && s[i] == c }

func skipLineComment(s string, i int) int {
	for i < len(s) && s[i] != '\n' {
		i++
	}
	return i
}

// skipBlockComment honours nesting, which PostgreSQL allows.
func skipBlockComment(s string, i int) int {
	depth := 1
	for i < len(s) && depth > 0 {
		switch {
		case s[i] == '/' && hasAt(s, i+1, '*'):
			depth++
			i += 2
		case s[i] == '*' && hasAt(s, i+1, '/'):
			depth--
			i += 2
		default:
			i++
		}
	}
	return i
}

// isEscapePrefixed reports whether the string opening at i carries the E
// prefix, where backslash escapes a quote.
func isEscapePrefixed(s string, i int) bool {
	return i > 0 && (s[i-1] == 'e' || s[i-1] == 'E') &&
		(i < 2 || !isIdentPart(s[i-2]))
}

func skipString(s string, i int, backslashEscapes bool) int {
	for i < len(s) {
		switch {
		case backslashEscapes && s[i] == '\\':
			i += 2
		case s[i] == '\'':
			// '' is a literal quote, not the end.
			if hasAt(s, i+1, '\'') {
				i += 2
				continue
			}
			return i + 1
		default:
			i++
		}
	}
	return i
}

// skipDollarQuoted consumes a $tag$...$tag$ block starting at i. It reports
// false when what follows is not a dollar quote — a $1 parameter, typically.
func skipDollarQuoted(s string, i int) (int, bool) {
	end := i + 1
	// A digit right after the dollar means a parameter placeholder, not a tag.
	for end < len(s) && isIdentPart(s[end]) &&
		(end != i+1 || s[end] < '0' || s[end] > '9') {
		end++
	}
	if end >= len(s) || s[end] != '$' {
		return 0, false
	}

	delim := s[i : end+1]
	close := strings.Index(s[end+1:], delim)
	if close < 0 {
		// Unterminated: consume to the end rather than misread the remainder.
		return len(s), true
	}
	return end + 1 + close + len(delim), true
}

func readQuotedIdent(s string, i int) (string, int) {
	var b strings.Builder
	for i < len(s) {
		if s[i] == '"' {
			if hasAt(s, i+1, '"') {
				b.WriteByte('"')
				i += 2
				continue
			}
			return b.String(), i + 1
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String(), i
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func isOperatorChar(c byte) bool {
	switch c {
	case '=', '<', '>', '!', '~', '+', '-', '*', '/', '%', '^', '&', '|', '@', '#', '?', ':':
		return true
	}
	return false
}
