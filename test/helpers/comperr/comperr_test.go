package comperr_test

import (
	"errors"
	"testing"

	"github.com/rwool/ex/test/helpers/comperr"
)

func TestAssertEqualErr(t *testing.T) {
	comperr.AssertEqualErr(t, nil, nil)
	e := errors.New("Something")
	comperr.AssertEqualErr(t, errors.New("Something"), e)
}

func TestRequireEqualErr(t *testing.T) {
	comperr.RequireEqualErr(t, nil, nil)
	e := errors.New("Something")
	comperr.RequireEqualErr(t, errors.New("Something"), e)
}
