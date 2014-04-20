package logo

import (
	"io"
	"testing"
)

func assert(t *testing.T, n Node, err error, expected string) {

	if err != nil {
		t.Errorf("Error \"%s\"", err)
		return
	}

	s := n.String()
	if s != expected {
		t.Errorf("Expected \"%s\" was \"%s\"", expected, s)
	}
}

func TestParseSingleWord(t *testing.T) {

	n, err := ParseString("Hello")
	assert(t, n, err, "\"Hello\"")
}

func TestParseMultipleWords(t *testing.T) {

	n, err := ParseString("Hello World")
	assert(t, n, err, "\"Hello\" \"World\"")
}

func TestParseList(t *testing.T) {

	n, err := ParseString("[Hello World]")
	assert(t, n, err, "[ \"Hello\" \"World\" ]")
}

func TestParseNestedList(t *testing.T) {

	n, err := ParseString("[[Hello] [ World] ]")
	assert(t, n, err, "[ [ \"Hello\" ] [ \"World\" ] ]")
}

func TestMixedWordAndLists(t *testing.T) {

	n, err := ParseString("Hello [ My Little ] Ponies")
	assert(t, n, err, "\"Hello\" [ \"My\" \"Little\" ] \"Ponies\"")
}

func TestNewLine(t *testing.T) {

	n, err := ParseString("Hello\nWorld")
	assert(t, n, err, "\"Hello\" \"World\"")
}

func TestEscape(t *testing.T) {
	n, err := ParseString("Hello\\ Sweet World")
	assert(t, n, err, "\"Hello Sweet\" \"World\"")
}

func TestUnclosedList(t *testing.T) {
	n, err := ParseString("[ Goodbye Cruel ")
	if err != nil && err != io.ErrUnexpectedEOF {
		t.Errorf("Expected ErrUnexpectedEOF, was \"%s\"", err)
	} else if n != nil {
		t.Errorf("Got \"%s\"", n)
	}
}