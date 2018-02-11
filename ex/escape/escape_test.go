package escape_test

import (
	"testing"

	"github.com/rwool/ex/test/helpers/goroutinechecker"

	"github.com/stretchr/testify/assert"

	"github.com/rwool/ex/ex/escape"
)

func TestFindSequence(t *testing.T) {
	defer goroutinechecker.New(t, false)()

	tests := []struct {
		Name           string
		Input          []byte
		Escapes        [][]byte
		EscapesCounter []int
	}{
		{
			Name:           "Single Byte Match",
			Input:          []byte{'a'},
			Escapes:        [][]byte{{'a'}},
			EscapesCounter: []int{1},
		},
		{
			Name:           "Double Byte Match",
			Input:          []byte{'a', 'b'},
			Escapes:        [][]byte{{'a', 'b'}},
			EscapesCounter: []int{1},
		},
		{
			Name:           "Double Same Byte Match",
			Input:          []byte{'a', 'a'},
			Escapes:        [][]byte{{'a', 'a'}},
			EscapesCounter: []int{1},
		},
		{
			Name:           "Single Byte Match with Nonmatch Byte",
			Input:          []byte{'b', 'a'},
			Escapes:        [][]byte{{'a'}},
			EscapesCounter: []int{1},
		},
		{
			Name:           "Double Byte Match with Nonmatch Byte",
			Input:          []byte{'b', 'a', 'a'},
			Escapes:        [][]byte{{'a', 'a'}},
			EscapesCounter: []int{1},
		},
		{
			Name:           "Double Byte Match with Match Byte",
			Input:          []byte{'a', 'a', 'a'},
			Escapes:        [][]byte{{'a', 'a'}},
			EscapesCounter: []int{1},
		},
		{
			Name:           "Double Double Match",
			Input:          []byte{'a', 'a', 'a', 'a'},
			Escapes:        [][]byte{{'a', 'a'}},
			EscapesCounter: []int{2},
		},
		{
			Name:           "SSH Escape",
			Input:          []byte("\n~."),
			Escapes:        [][]byte{[]byte("\n~.")},
			EscapesCounter: []int{1},
		},
		{
			Name:           "SSH Escape With Preceeding Characters",
			Input:          []byte("abc  123\t\n~."),
			Escapes:        [][]byte{[]byte("\n~.")},
			EscapesCounter: []int{1},
		},
		{
			Name:           "Double SSH Escape with Surrounding Characters",
			Input:          []byte("abc  123\t\n~. \n qwerty \n~. Hello"),
			Escapes:        [][]byte{[]byte("\n~.")},
			EscapesCounter: []int{2},
		},
		{
			Name:           "Double Escape with Common Prefix",
			Input:          []byte("\n~.\n~!"),
			Escapes:        [][]byte{[]byte("\n~."), []byte("\n~!")},
			EscapesCounter: []int{1, 1},
		},
		{
			Name:           "Repeated SSH Escape with Other Escape Registered",
			Input:          []byte("\n~.\n~.\n~.\n~.\n~."),
			Escapes:        [][]byte{[]byte("\n~."), []byte("\n~!")},
			EscapesCounter: []int{5, 0},
		},
		{
			Name:           "Repeated SSH Escape and Prefix Escape",
			Input:          []byte("\n~.\n~.\n~!\n~.\n~.\n~!"),
			Escapes:        [][]byte{[]byte("\n~."), []byte("\n~!")},
			EscapesCounter: []int{4, 2},
		},
		{
			Name:           "Repeated SSH Escape and Prefix Escape with Other Characters",
			Input:          []byte("\n\n\n\n~.Hello\n~.\t\nText\r\n~!   \n~. @@@\n~.qwerty\n~!\n~"),
			Escapes:        [][]byte{[]byte("\n~."), []byte("\n~!")},
			EscapesCounter: []int{4, 2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t2 *testing.T) {
			defer goroutinechecker.New(t2, true)()

			if len(tc.Escapes) != len(tc.EscapesCounter) {
				panic("counters for escapes must match number of escapes")
			}

			counts := make([]int, len(tc.EscapesCounter))
			fnGen := func(index int) func() {
				return func() { counts[index]++ }
			}
			tmp := make([]interface{}, 0, len(tc.Escapes))
			for i, v := range tc.Escapes {
				tmp = append(tmp, v)
				tmp = append(tmp, fnGen(i))
			}

			ep := escape.NewProcessor(tmp...)
			var numRead int
			for _, b := range tc.Input {
				numRead++
				ep.InsertByte(b)
			}

			assert.Equal(t2, tc.EscapesCounter, counts)
		})
	}
}
