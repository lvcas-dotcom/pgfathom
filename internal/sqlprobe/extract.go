package sqlprobe

import "strings"

// rawJoin is an equality between two column references as written in the SQL,
// resolved against the statement's aliases but not yet against the catalog.
type rawJoin struct {
	left, right rawRef
}

// rawRef is a table-qualified column. Schema is empty when the SQL did not
// qualify it.
type rawRef struct {
	schema, table, column string
}

// predOp is the extractor's own classification of a predicate operator,
// mapped to model.OperatorClass once the reference is resolved against the
// catalog in probe.go. Keeping it local means extract.go stays free of the
// model import, same as rawJoin and rawRef.
type predOp int

const (
	predEquality predOp = iota
	predRange
	predLikePrefix
	predLikeInfix
	predContainment
	predFullText
	predVectorDistance
)

// rawPredicate is one predicate on a reference as written in the SQL, not yet
// resolved against the catalog.
type rawPredicate struct {
	ref rawRef
	op  predOp
}

// comparisonOperators are order comparisons: btree serves all of them.
var comparisonOperators = map[string]bool{
	"<": true, ">": true, "<=": true, ">=": true, "<>": true, "!=": true,
}

// containmentOperators are jsonb/array containment and key existence: GIN
// serves them, btree does not.
var containmentOperators = map[string]bool{
	"@>": true, "<@": true, "?": true, "?|": true, "?&": true,
}

// vectorDistanceOperators are pgvector's nearest-neighbor operators.
var vectorDistanceOperators = map[string]bool{
	"<->": true, "<=>": true, "<#>": true,
}

// clauseStoppers end the collection of one FROM item. Anything else after a
// table name is read as its alias.
var clauseStoppers = map[string]bool{
	"on": true, "using": true, "where": true, "group": true, "order": true,
	"limit": true, "offset": true, "having": true, "union": true, "intersect": true,
	"except": true, "returning": true, "join": true, "inner": true, "left": true,
	"right": true, "full": true, "cross": true, "outer": true, "natural": true,
	"lateral": true, "as": true, "into": true, "for": true, "window": true,
	"tablesample": true, "set": true, "values": true, "select": true, "from": true,
}

// extract mines every recognizable join predicate and column predicate from
// one piece of SQL. Join predicates feed relationship inference; column
// predicates feed index method recommendation.
//
// The alias map is flat per statement: subqueries have their own scopes, and
// modelling them is half a parser. On an alias clash across scopes the
// resolution can be wrong — and the wrong candidate dies in validation, which
// is the trade the package documentation defends.
func extract(sql string) ([]rawJoin, []rawPredicate) {
	// A body handed over wrapped in one dollar quote — the CREATE FUNCTION
	// form — is code, not a string. Embedded dollar quotes deeper in remain
	// strings: extracting from dynamically assembled SQL would be guessing.
	sql = unwrapDollarBody(sql)

	var joins []rawJoin
	var preds []rawPredicate
	tokens := tokenize(sql)

	aliases := make(map[string]rawRef)

	for i := 0; i < len(tokens); i++ {
		t := tokens[i]

		switch {
		case t.kind == tokSymbol && t.text == ";":
			aliases = make(map[string]rawRef)

		case t.kind == tokIdent && (t.text == "from" || t.text == "join"):
			i = collectAliases(tokens, i+1, aliases)

		case t.kind == tokSymbol && t.text == "=":
			left, okL := refEndingAt(tokens, i-1)
			right, okR := refStartingAt(tokens, i+1)
			switch {
			case okL && okR:
				joins = append(joins, rawJoin{
					left:  resolve(left, aliases),
					right: resolve(right, aliases),
				})
			case okL:
				preds = append(preds, rawPredicate{ref: resolve(left, aliases), op: predEquality})
			}

		case t.kind == tokSymbol && comparisonOperators[t.text]:
			if left, ok := refEndingAt(tokens, i-1); ok {
				preds = append(preds, rawPredicate{ref: resolve(left, aliases), op: predRange})
			}

		case t.kind == tokSymbol && containmentOperators[t.text]:
			if left, ok := refEndingAt(tokens, i-1); ok {
				preds = append(preds, rawPredicate{ref: resolve(left, aliases), op: predContainment})
			}

		case t.kind == tokSymbol && t.text == "@@":
			if left, ok := refEndingAt(tokens, i-1); ok {
				preds = append(preds, rawPredicate{ref: resolve(left, aliases), op: predFullText})
			}

		case t.kind == tokSymbol && vectorDistanceOperators[t.text]:
			if left, ok := refEndingAt(tokens, i-1); ok {
				preds = append(preds, rawPredicate{ref: resolve(left, aliases), op: predVectorDistance})
			}

		case t.kind == tokIdent && (t.text == "like" || t.text == "ilike"):
			if left, ok := refEndingAt(tokens, i-1); ok {
				preds = append(preds, rawPredicate{ref: resolve(left, aliases), op: likeOperator(tokens, i+1)})
			}
		}
	}

	return joins, preds
}

// likeOperator classifies a LIKE/ILIKE pattern by its leading character. A
// pattern that starts with a wildcard defeats a leading-anchored btree scan
// and wants a trigram index; anything else, including a pattern the extractor
// cannot see (a parameter, a column, a function call), is read as a prefix —
// the conservative choice, since btree is never the wrong recommendation for
// it.
func likeOperator(tokens []token, i int) predOp {
	if i < len(tokens) && tokens[i].kind == tokString && strings.HasPrefix(tokens[i].text, "%") {
		return predLikeInfix
	}
	return predLikePrefix
}

// collectAliases reads the table references after a FROM or JOIN keyword and
// returns the index to resume scanning from.
func collectAliases(tokens []token, i int, aliases map[string]rawRef) int {
	for {
		// A parenthesis after FROM is either a subquery — skipped, since there
		// is no table to resolve to — or plain grouping, which is how the
		// server reconstructs a multi-way join in pg_views and must be
		// descended into rather than jumped over.
		if i < len(tokens) && tokens[i].kind == tokSymbol && tokens[i].text == "(" {
			if opensSubquery(tokens, i) {
				i = skipParens(tokens, i)
				i = skipItemAlias(tokens, i)
			} else {
				i++
				continue
			}
		} else {
			parts, next := readDottedName(tokens, i)
			if len(parts) == 0 {
				return i - 1
			}
			i = next

			ref := tableRef(parts)
			name, i2 := readAlias(tokens, i)
			i = i2
			if name != "" {
				aliases[strings.ToLower(name)] = ref
			}
			// The bare table name refers to itself when no alias overrides it.
			if _, taken := aliases[strings.ToLower(ref.table)]; !taken {
				aliases[strings.ToLower(ref.table)] = ref
			}
		}

		// A comma continues the FROM list; anything else ends it.
		if i < len(tokens) && tokens[i].kind == tokSymbol && tokens[i].text == "," {
			i++
			continue
		}
		return i - 1
	}
}

func tableRef(parts []string) rawRef {
	if len(parts) >= 2 {
		return rawRef{schema: parts[len(parts)-2], table: parts[len(parts)-1]}
	}
	return rawRef{table: parts[0]}
}

func readAlias(tokens []token, i int) (string, int) {
	if i < len(tokens) && tokens[i].kind == tokIdent && tokens[i].text == "as" {
		i++
	}
	if i < len(tokens) && tokens[i].kind == tokIdent && !clauseStoppers[tokens[i].text] {
		return tokens[i].text, i + 1
	}
	return "", i
}

func skipItemAlias(tokens []token, i int) int {
	_, next := readAlias(tokens, i)
	return next
}

func opensSubquery(tokens []token, i int) bool {
	next := i + 1
	if next >= len(tokens) || tokens[next].kind != tokIdent {
		return false
	}
	return tokens[next].text == "select" || tokens[next].text == "values" ||
		tokens[next].text == "with"
}

func skipParens(tokens []token, i int) int {
	depth := 0
	for ; i < len(tokens); i++ {
		if tokens[i].kind != tokSymbol {
			continue
		}
		switch tokens[i].text {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return i
}

// readDottedName reads ident[.ident[.ident]] starting at i.
func readDottedName(tokens []token, i int) ([]string, int) {
	var parts []string
	for i < len(tokens) && tokens[i].kind == tokIdent {
		if clauseStoppers[tokens[i].text] && len(parts) == 0 {
			return nil, i
		}
		parts = append(parts, tokens[i].text)
		if i+1 < len(tokens) && tokens[i+1].kind == tokSymbol && tokens[i+1].text == "." {
			i += 2
			continue
		}
		return parts, i + 1
	}
	return parts, i
}

// refEndingAt reads a qualified reference whose last token sits at i, walking
// backwards. A bare column is rejected: without a qualifier there is no table
// to resolve, and guessing would manufacture evidence.
func refEndingAt(tokens []token, i int) (rawRef, bool) {
	var parts []string
	for i >= 0 && tokens[i].kind == tokIdent {
		parts = append([]string{tokens[i].text}, parts...)
		if i-1 >= 0 && tokens[i-1].kind == tokSymbol && tokens[i-1].text == "." {
			i -= 2
			continue
		}
		break
	}
	return refFromParts(parts)
}

func refStartingAt(tokens []token, i int) (rawRef, bool) {
	parts, _ := readDottedName(tokens, i)
	return refFromParts(parts)
}

func refFromParts(parts []string) (rawRef, bool) {
	switch len(parts) {
	case 2:
		return rawRef{table: parts[0], column: parts[1]}, true
	case 3:
		return rawRef{schema: parts[0], table: parts[1], column: parts[2]}, true
	default:
		return rawRef{}, false
	}
}

// resolve swaps an alias qualifier for the table it names. A qualifier that is
// no known alias is kept as a table name and left for catalog resolution.
func resolve(ref rawRef, aliases map[string]rawRef) rawRef {
	if ref.schema != "" {
		return ref
	}
	if target, ok := aliases[strings.ToLower(ref.table)]; ok {
		return rawRef{schema: target.schema, table: target.table, column: ref.column}
	}
	return ref
}

// unwrapDollarBody strips one dollar quote wrapping the entire input.
func unwrapDollarBody(sql string) string {
	s := strings.TrimSpace(sql)
	if !strings.HasPrefix(s, "$") {
		return sql
	}
	end := strings.Index(s[1:], "$")
	if end < 0 {
		return sql
	}
	delim := s[:end+2]
	if !strings.HasSuffix(s, delim) || len(s) < 2*len(delim) {
		return sql
	}
	return s[len(delim) : len(s)-len(delim)]
}
