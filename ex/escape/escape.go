// Package escape provides support for handling escape sequences from input
// streams.
package escape

import (
	"io"
	"sync"
)

// Processor handles processing escape sequences from an input stream.
type Processor struct {
	// TODO Use a trie here?
	// escapes is a map of the current escape sequences and how many characters
	// of each escape hve been found.
	escapes   map[string]int
	escapeFns map[string]func()
	mu        *sync.Mutex
}

// NewProcessor creates a new Processor.
func NewProcessor(seqs ...interface{}) *Processor {
	ep := &Processor{
		escapes:   make(map[string]int),
		escapeFns: make(map[string]func()),
		mu:        &sync.Mutex{},
	}

	ep.AddSequencePairs(seqs...)

	return ep
}

// InsertByte checks if the given byte complete a sequence.
//
// Note that this is greedy and has no support for sequences that have a prefix
// of another sequence.
func (ep *Processor) InsertByte(b byte) (sequenceFound []byte) {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	var seq []byte
	var seqStr string
	var k string
	var v int
	// Do not return in this for loop as all keys must be processed.
	for k, v = range ep.escapes {
		kB := []byte(k)
		if kB[v] == b {
			newVal := v + 1
			if newVal == len(kB) {
				ep.escapes[k] = 0
				seq = kB
				seqStr = k
			} else {
				ep.escapes[k] = newVal
			}
		} else {
			if kB[0] == b {
				ep.escapes[k] = 1
			} else {
				ep.escapes[k] = 0
			}
		}
	}

	if seq != nil {
		ep.escapeFns[seqStr]()
	}
	return seq
}

// AddSequencePairs adds new pairs of escape sequences and functions to call
// when the escape sequences are found.
func (ep *Processor) AddSequencePairs(seqs ...interface{}) {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if len(seqs)%2 == 1 {
		panic("odd number of arguments given")
	}

	tmpSeqToFunc := make(map[string]func(), len(seqs)/2)

	var currentSeq []byte
	var isOdd bool
	for i := range seqs {
		if isOdd {
			// Function.
			if v, ok := seqs[i].(func()); ok {
				tmpSeqToFunc[string(currentSeq)] = v
			} else {
				panic("not a function for odd argument")
			}
		} else {
			// Sequence.
			if v, ok := seqs[i].([]byte); ok {
				currentSeq = v
			} else {
				panic("not a byte slice for even argument")
			}
		}
		isOdd = !isOdd
	}

	for k, v := range tmpSeqToFunc {
		if _, ok := ep.escapes[k]; !ok {
			ep.escapes[k] = 0
		}

		ep.escapeFns[k] = v
	}
}

// Reader handles reading escapes from an io.Reader.
type Reader struct {
	source  io.Reader
	ep      *Processor
	scratch [64]byte
}

// NewReader returns an escape reader that reads from the given source.
//
// The Reader returned from this function should be used in place of the
// source reader.
func NewReader(source io.Reader, seqs ...interface{}) *Reader {
	er := &Reader{
		source: source,
	}

	er.ep = NewProcessor(seqs...)

	return er
}

// Read reads from the underlying io.Reader, attempting to find escape
// sequences.
func (er *Reader) Read(p []byte) (int, error) {
	read, err := er.source.Read(p)
	// Do processing before error is handled as an escape sequence may be in
	// the part that was read.
	for _, b := range p[:read] {
		er.ep.InsertByte(b)
	}
	if err != nil {
		return read, err
	}
	return read, nil
}
