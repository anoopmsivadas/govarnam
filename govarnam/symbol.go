package govarnam

import (
	"context"
	sql "database/sql"
	"fmt"
	"log"
)

// Symbol result from VST
type Symbol struct {
	id              int
	generalType     int
	matchType       int
	pattern         string
	value1          string
	value2          string
	value3          string
	tag             string
	weight          int
	priority        int
	acceptCondition int
	flags           int
}

// Token info for making a suggestion
type Token struct {
	tokenType int
	symbols   []Symbol // Will be empty for non language character
	position  int
	character *string // Non language character
}

func (varnam *Varnam) openVST(vstPath string) {
	var err error
	varnam.vstConn, err = sql.Open("sqlite3", vstPath)
	if err != nil {
		log.Fatal(err)
	}
}

// Checks if a symbol exist in VST
func (varnam *Varnam) symbolExist(ch string) bool {
	rows, err := varnam.vstConn.Query("SELECT COUNT(*) FROM symbols WHERE value1 = ?", ch)
	checkError(err)

	count := 0
	for rows.Next() {
		err := rows.Scan(&count)
		checkError(err)
	}
	return count != 0
}

func (varnam *Varnam) searchSymbol(ctx context.Context, ch string, matchType int) []Symbol {
	var (
		rows    *sql.Rows
		err     error
		results []Symbol
	)

	if matchType == VARNAM_MATCH_ALL {
		rows, err = varnam.vstConn.QueryContext(ctx, "SELECT id, type, match_type, pattern, value1, value2, value3, tag, weight, priority, accept_condition, flags from symbols WHERE pattern = ? ORDER BY match_type ASC, weight DESC", ch)
	} else {
		rows, err = varnam.vstConn.QueryContext(ctx, "SELECT id, type, match_type, pattern, value1, value2, value3, tag, weight, priority, accept_condition, flags from symbols WHERE pattern = ? AND match_type = ?", ch, matchType)
	}

	if err != nil {
		log.Print(err)
		return results
	}
	defer rows.Close()

	for rows.Next() {
		var item Symbol
		rows.Scan(&item.id, &item.generalType, &item.matchType, &item.pattern, &item.value1, &item.value2, &item.value3, &item.tag, &item.weight, &item.priority, &item.acceptCondition, &item.flags)
		results = append(results, item)
	}

	err = rows.Err()
	if err != nil {
		log.Print(err)
	}

	return results
}

// Convert a string into Tokens for later processing
func (varnam *Varnam) tokenizeWord(ctx context.Context, word string, matchType int) []Token {
	var results []Token

	var prevSequenceMatches []Symbol
	var sequence string

	i := 0
	for i < len(word) {
		ch := string(word[i])

		sequence += ch

		matches := varnam.searchSymbol(ctx, sequence, matchType)

		if varnam.Debug {
			fmt.Println(sequence, matches)
		}

		if len(matches) == 0 {
			// No more matches

			if len(sequence) == 1 {
				// No matches for a single char, add it
				token := Token{VARNAM_TOKEN_CHAR, matches, i, &ch}
				results = append(results, token)
			} else if len(prevSequenceMatches) > 0 {
				// Backtrack and add the previous sequence matches
				i--
				token := Token{VARNAM_TOKEN_SYMBOL, prevSequenceMatches, i, nil}
				results = append(results, token)
			}

			sequence = ""
		} else {
			if matches[0].generalType == VARNAM_SYMBOL_NUMBER && !varnam.LangRules.IndicDigits {
				// Skip numbers
				token := Token{VARNAM_TOKEN_CHAR, []Symbol{}, i, &ch}
				results = append(results, token)

				sequence = ""
			} else if i == len(word)-1 {
				// Last character
				token := Token{VARNAM_TOKEN_SYMBOL, matches, i, nil}
				results = append(results, token)
			} else {
				prevSequenceMatches = matches
			}
		}
		i++
	}
	return results
}

// Tokenize end part of a word and append it to results
func (varnam *Varnam) tokenizeRestOfWord(ctx context.Context, word string, results []Suggestion) []Suggestion {
	if varnam.Debug {
		fmt.Printf("Tokenizing %s\n", word)
	}

	restOfWordTokens := varnam.tokenizeWord(ctx, word, VARNAM_MATCH_ALL)
	restOfWordSugs := varnam.tokensToSuggestions(ctx, restOfWordTokens, true)

	if varnam.Debug {
		fmt.Println("Tokenized:", restOfWordSugs)
	}

	if len(restOfWordSugs) > 0 {
		for j, result := range results {
			till := varnam.removeLastVirama(result.Word)
			tillWeight := result.Weight

			firstSug := restOfWordSugs[0]
			results[j].Word = varnam.removeLastVirama(results[j].Word) + firstSug.Word
			results[j].Weight += firstSug.Weight

			for k, sug := range restOfWordSugs {
				if k == 0 {
					continue
				}
				sug := Suggestion{till + sug.Word, tillWeight + sug.Weight, sug.LearnedOn}
				results = append(results, sug)
			}
		}
	}

	return results
}

// Split a word into conjuncts
func (varnam *Varnam) splitWordByConjunct(input string) ([]string, error) {
	var results []string

	var prevSequenceMatch string
	var sequence string

	// Not using len() because it will be wrong for non ASCII characters
	var sequenceLength int

	word := []rune(input)

	i := 0
	for i < len(word) {
		ch := string(word[i])

		sequence += ch
		sequenceLength++

		if !varnam.symbolExist(sequence) {
			// No more matches

			if sequenceLength == 1 {
				// Has non language characters, give error
				return []string{}, fmt.Errorf("Has non language characters")
			} else if len(prevSequenceMatch) > 0 {
				// Backtrack and add the previous sequence matches
				i--
				results = append(results, prevSequenceMatch)
			}

			sequence = ""
			sequenceLength = 0
		} else {
			if i == len(word)-1 {
				// Last character
				results = append(results, sequence)
			} else {
				prevSequenceMatch = sequence
			}
		}
		i++
	}
	return results, nil
}

func getSymbolValue(symbol Symbol, position int) string {
	if symbol.tag == RENDER_VALUE2_TAG {
		// Specific rule to use value 2
		return symbol.value2
	} else if symbol.generalType == VARNAM_SYMBOL_VOWEL && position > 0 {
		// If in between word, we use the vowel and not the consonant
		return symbol.value2 // ാ
	} else {
		return symbol.value1 // ആ
	}
}

func getSymbolWeight(symbol Symbol) int {
	return symbol.weight + (VARNAM_MATCH_POSSIBILITY-symbol.matchType)*100
}

// Remove non-exact matching tokens
func removeNonExactTokens(tokens []Token) []Token {
	// Remove non-exact symbols
	for i, token := range tokens {
		var reducedSymbols []Symbol
		for _, symbol := range token.symbols {
			if symbol.matchType == VARNAM_MATCH_EXACT {
				reducedSymbols = append(reducedSymbols, symbol)
			} else {
				// If a possibility result, then rest of them will also be same
				// so save time by skipping rest
				break
			}
		}
		tokens[i].symbols = reducedSymbols
	}
	return tokens
}
