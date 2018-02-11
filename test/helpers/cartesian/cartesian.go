// Package cartesian implements the calculation of cartesian products of
// slices.
package cartesian

// Cartesian contains the current state of the iteration over the cartesian
// products.
type Cartesian struct {
	// ss is the slices.
	ss [][]interface{}
	// ndx is the current index within each slice.
	ndx []int
	// j is the slice index.
	j int
	// firstDone indicates if the first loop was completed.
	firstDone bool
	// done indicates if the all of the products have been output.
	done bool
	// Next slice to output.
	next []interface{}
}

// New creates a new Cartesian product generator.
func New(slices ...[]interface{}) *Cartesian {
	return &Cartesian{
		ss:  slices,
		ndx: make([]int, len(slices)),
		j:   len(slices) - 1,
	}
}

func (c *Cartesian) atLastInSlice() bool {
	if len(c.ss[c.j])-1 == c.ndx[c.j] {
		return true
	}
	return false
}

func (c *Cartesian) zeroToRight() {
	if c.j == len(c.ndx)-1 {
		return
	}
	for i := c.j + 1; i < len(c.ndx); i++ {
		c.ndx[i] = 0
	}
	c.j = len(c.ndx) - 1
}

func (c *Cartesian) output() []interface{} {
	out := make([]interface{}, len(c.ss))
	for i, v := range c.ndx {
		out[i] = c.ss[i][v]
	}
	return out
}

// Next returns the next cartesian product. If there are no more products, nil
// will be returned.
func (c *Cartesian) Next() bool {
	if len(c.ss) == 0 {
		c.next = nil
		return false
	}
	if !c.firstDone {
		c.firstDone = true
		c.next = c.output()
		return true
	}
	if c.done {
		c.next = nil
		return false
	}

	for {
		// If the current index can be incremented, do so and zero all indexes
		// to the right.
		if !c.atLastInSlice() {
			c.ndx[c.j]++
			c.zeroToRight()
			c.next = c.output()
			return true
		}
		// If the index cannot be incremented, then move to the previous
		// slice.
		// If already at the first slice, then there is nothing left to do.
		if c.j == 0 {
			c.done = true
			c.next = nil
			return false
		}
		c.j--
		continue
	}
}

// Slice returns the slice that was generated from a call to Next.
//
// If Next has not been called or has returned false with the last call to it,
// then this function will return nil.
func (c *Cartesian) Slice() []interface{} {
	return c.next
}
