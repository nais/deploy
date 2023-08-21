package deployclient

import (
	"fmt"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/stretchr/testify/assert"
	"testing"
)

var notEmpty = "whatever"
var empty = ""

func TestOneIsEmpty(t *testing.T) {
	onlyOne := xor(notEmpty, empty)
	if !onlyOne {
		t.Errorf("Only one value is set, should be true")
	}
}

func TestOtherOneIsEmpty(t *testing.T) {
	onlyOne := xor(empty, notEmpty)
	if !onlyOne {
		t.Errorf("Only one value is set, should be true")
	}
}

func TestBothAreEmpty(t *testing.T) {
	onlyOne := xor(empty, empty)
	if onlyOne {
		t.Errorf("None of the values are set, should be false")
	}
}

func TestBothAreNonEmpty(t *testing.T) {
	onlyOne := xor(empty, empty)
	if onlyOne {
		t.Errorf("Both values are set, should be false")
	}
}

func TestTheStuff(t *testing.T) {
	a, err := jwt.Parse([]byte("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJ0aGUgdGVzdGluZyBnb2RzIiwic3ViIjoiMTIzNDU2Nzg5MCIsIm5hbWUiOiJKb2huIERvZSIsImlhdCI6MTUxNjIzOTAyMiwiZXhwIjoxNTE2MjM5MTIyfQ.2RhUnLPOu_FygCK5Y6UznvKGn-NBD0Nku2usPq-qwOE"))
	if err != nil {
		t.Errorf("%v", err)
	}
	assert.NotNil(t, a)
	fmt.Printf("---- %v\n", a.Expiration())
}
