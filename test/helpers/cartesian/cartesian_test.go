package cartesian_test

import (
	"testing"

	"github.com/rwool/ex/test/helpers/goroutinechecker"

	"github.com/stretchr/testify/assert"

	"github.com/rwool/ex/test/helpers/cartesian"
)

func TestCartesian(t *testing.T) {
	defer goroutinechecker.New(t, false)()

	tcs := []struct {
		Name   string
		Input  [][]interface{}
		Output [][]interface{}
	}{
		{
			Name:   "Zero",
			Input:  [][]interface{}{},
			Output: nil,
		},
		{
			Name: "One Value",
			Input: [][]interface{}{
				{1},
			},
			Output: [][]interface{}{
				{1},
			},
		},
		{
			Name: "2x3",
			Input: [][]interface{}{
				{1, 2},
				{3, 4, 5},
			},
			Output: [][]interface{}{
				{1, 3},
				{1, 4},
				{1, 5},
				{2, 3},
				{2, 4},
				{2, 5},
			},
		},
		{
			Name: "2x2x2",
			Input: [][]interface{}{
				{1, 2},
				{3, 4},
				{5, 6},
			},
			Output: [][]interface{}{
				{1, 3, 5},
				{1, 3, 6},
				{1, 4, 5},
				{1, 4, 6},
				{2, 3, 5},
				{2, 3, 6},
				{2, 4, 5},
				{2, 4, 6},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t2 *testing.T) {
			defer goroutinechecker.New(t2, true)()

			var out [][]interface{}
			c := cartesian.New(tc.Input...)
			for c.Next() {
				out = append(out, c.Slice())
			}

			assert.Equal(t2, out, tc.Output)
		})
	}
}
