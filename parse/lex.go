// Derived from ideas in
// github.com/golang/go/src/text/template/parse

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Pos int

// item represents a token or text string returned from the scanner.
type item struct {
	typ itemType // The type of this item.
	pos Pos      // The starting position, in bytes, of this item in the input string.
	val string   // The value of this item.
}

// itemType identifies the type of lex items.
type itemType int

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*lexer) stateFn

// lexer holds the state of the scanner.
type lexer struct {
	name    string    // the name of the input; used only for error reports
	input   string    // the string being scanned
	state   stateFn   // the next lexing function to enter
	pos     Pos       // current position in the input
	start   Pos       // start position of this item
	width   Pos       // width of last rune read from input
	lastPos Pos       // position of most recent item returned by nextItem
	items   chan item // channel of scanned items
}

const (
	itemEOF          itemType = iota
	itemError                 // error occurred; value is text of error
	itemLeftParen             // '('
	itemRightParen            // ')'
	itemSemiColon             // ';'
	itemLeftBracket           // '['
	itemRightBracket          // ']'
	itemPropertyName
	itemPropertyValue
)

const eof = -1

var itemName = map[itemType]string{
	itemLeftParen:    "(",
	itemRightParen:   ")",
	itemLeftBracket:  "[",
	itemRightBracket: "]",
	itemSemiColon:    ";",
}

func succinct(s string) string {
	if len(s) > 10 {
		return fmt.Sprintf("%.10q...", s)
	} else {
		return fmt.Sprintf("%.10q", s)
	}
}

func strip_newlines(s string) string {
	result := strings.Replace(s, "\n", "", -1)
	result = strings.Replace(result, "\r", "", -1)
	return result
}

func (i item) String() string {
	switch i.typ {
	case itemEOF:
		return "EOF"
	case itemError:
		return i.val
	case itemLeftParen:
		return "("
	case itemRightParen:
		return ")"
	case itemSemiColon:
		return ";"
	case itemLeftBracket:
		return "["
	case itemRightBracket:
		return "]"
	case itemPropertyName:
		return succinct(i.val)
	case itemPropertyValue:
		return succinct(i.val)
	default:
		return succinct(i.val)
	}
}

func (i itemType) String() string {
	s := itemName[i]
	if s == "" {
		return fmt.Sprintf("unknown item: %d", int(i))
	}
	return s
}

// next returns the next rune in the input.
func (l *lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = Pos(w)
	l.pos += l.width
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *lexer) backup() {
	l.pos -= l.width
}

// emit passes an item back to the client.
func (l *lexer) emit(t itemType) {
	i := item{t, l.start, l.input[l.start:l.pos]}
	if i.typ == itemPropertyName {
		i.val = strings.ToUpper(i.val)
	}
	l.items <- i
	l.start = l.pos
}

// ignore skips over the pending input before this point.
func (l *lexer) ignore() {
	l.start = l.pos
}

// drop the next rune
func (l *lexer) advance() {
	l.next()
	l.ignore()
}

func (l *lexer) quoteContext() string {
	start := l.pos - 6
	if start < 0 {
		start = 0
	}
	end := int(l.pos + 6)
	if end >= len(l.input) {
		end = len(l.input) - 1
	}
	return l.input[start:l.pos] + "|" + l.input[l.pos:end]
}

func (l *lexer) QuoteErrorContext(message string) string {
	return fmt.Sprintf("%s, position %d, %q", message, l.pos, l.quoteContext())
}

// acceptAlphaRun consumes a run of alphabeticals from the valid set.
func (l *lexer) acceptAlphaRun() {
	for isAlpha(l.next()) {
	}
	l.backup()
}

// acceptPropertyValue consumes a run of alphabeticals from the valid set.
func (l *lexer) acceptPropertyValueRun() {
	for isPropertyValueChar(l.next()) {
	}
	l.backup()
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{itemError, l.start, fmt.Sprintf(format, args...)}
	return nil
}

// nextItem returns the next item from the input.
func (l *lexer) nextItem() item {
	item := <-l.items
	l.lastPos = item.pos
	return item
}

// run the state machine for the lexer.
func (l *lexer) run() {
	for l.state = lexBegin; l.state != nil; {
		l.state = l.state(l)
	}
}

// lex creates a new scanner for the input string.
func lex(input string) *lexer {
	l := &lexer{
		input: strip_newlines(input),
		items: make(chan item),
	}
	go l.run()
	return l
}

// lexBegin scans until an opening left parenthesis "(".
func lexBegin(l *lexer) stateFn {
	for {
		if strings.HasPrefix(l.input[l.pos:], "(") {
			return lexLeftParen
		}
		if l.next() == eof {
			break
		}
	}
	// Correctly reached EOF.
	l.emit(itemEOF)
	return nil
}

func lexLeftParen(l *lexer) stateFn {
	l.pos += Pos(len("("))
	l.emit(itemLeftParen)
	if l.peek() != ';' {
		return l.errorf(l.QuoteErrorContext("semi-colon expected here"))
	}
	return lexSemiColon
}

func lexRightParen(l *lexer) stateFn {
	l.pos += Pos(len(")"))
	l.emit(itemRightParen)

	switch {
	case l.peek() == '(':
		return lexLeftParen
	case isAlpha(l.peek()):
		return lexPropertyName
	case l.peek() == ')':
		return lexRightParen
	case l.peek() == ';':
		return lexSemiColon
	default:
		l.emit(itemEOF)
		return nil
	}
}

func lexSemiColon(l *lexer) stateFn {
	l.pos += Pos(len(";"))
	l.emit(itemSemiColon)
	if l.peek() == ';' {
		l.advance()
	}
	if !isAlpha(l.peek()) {
		return l.errorf(l.QuoteErrorContext("property expected here"))
	}
	return lexPropertyName
}

func lexPropertyName(l *lexer) stateFn {
	l.acceptAlphaRun()
	l.emit(itemPropertyName)
	if (l.peek()) != '[' {
		return l.errorf(l.QuoteErrorContext("left bracket '[' expected here"))
	}
	return lexLeftBracket
}

func lexLeftBracket(l *lexer) stateFn {
	l.advance()
	l.acceptPropertyValueRun()
	l.emit(itemPropertyValue)

	if l.peek() != ']' {
		return l.errorf("right bracket ']' expected here (position: %d)", l.pos)
	}
	l.advance()

	for isWhiteSpace(l.peek()) {
		l.next()
	}

	switch l.peek() {
	case '[':
		return lexLeftBracket
	case ';':
		return lexSemiColon
	case '(':
		return lexLeftParen
	case ')':
		return lexRightParen
	}

	if isAlpha(l.peek()) {
		return lexPropertyName
	}

	return l.errorf("property or node or parenthesis expected here (position: %d). Found: %q", l.pos, l.peek())
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

func isEndOfLine(r rune) bool {
	return r == '\r' || r == '\n'
}

func isWhiteSpace(r rune) bool {
	return isSpace(r) || isEndOfLine(r)
}

func isAlpha(r rune) bool {
	return unicode.IsLetter(r)
}

func isPropertyValueChar(r rune) bool {
	return unicode.IsPrint(r) && r != ']'
}
