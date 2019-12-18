// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package markers_test

import (
	"context"
	"errors"
	goErr "errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/markers"
	"github.com/cockroachdb/errors/testutils"
	pkgErr "github.com/pkg/errors"
)

// This test demonstrates that Is() returns true if passed the same
// error reference twice, and that errors that are structurally
// different appear different via Is().
func TestLocalErrorEquivalence(t *testing.T) {
	tt := testutils.T{T: t}

	err1 := errors.New("hello")
	err2 := errors.New("world")

	tt.Check(!markers.Is(err1, err2))
	tt.Check(markers.Is(err1, err1))
	tt.Check(markers.Is(err2, err2))
}

// This test demonstrates that Is() returns true if
// two errors are structurally equivalent.
func TestStructuralEquivalence(t *testing.T) {
	tt := testutils.T{T: t}

	err1 := errors.New("hello")
	err2 := errors.New("hello")

	tt.Check(markers.Is(err1, err2))
}

// This test demonstrates that both the error type and package path
// are used to ascertain equivalence.
func TestErrorTypeEquivalence(t *testing.T) {
	tt := testutils.T{T: t}

	err1 := errors.New("hello")
	err2 := pkgErr.New("hello")
	err3 := &fundamental{msg: "hello"}

	tt.Check(!markers.Is(err1, err2))
	tt.Check(!markers.Is(err2, err3))
}

// fundamental is a local error type, but it has the
// same name as the type in github.com/pkg/errors.
type fundamental struct {
	msg string
}

func (e *fundamental) Error() string { return e.msg }

func network(err error) error {
	enc := errbase.EncodeError(context.Background(), err)
	return errbase.DecodeError(context.Background(), enc)
}

// This test demonstrates that the equivalence
// of errors is preserved over the network.
func TestRemoteErrorEquivalence(t *testing.T) {
	tt := testutils.T{T: t}

	err1 := errors.New("hello")
	err2 := errors.New("world")

	newErr1 := network(err1)

	tt.Check(markers.Is(err1, newErr1))
	tt.Check(!markers.Is(err2, newErr1))
}

// This test demonstrates that it is possible to recognize standard
// errors that have been sent over the network.
func TestStandardErrorRemoteEquivalence(t *testing.T) {
	tt := testutils.T{T: t}

	err1 := io.EOF
	err2 := context.DeadlineExceeded

	newErr1 := network(err1)

	tt.Check(markers.Is(err1, newErr1))
	tt.Check(!markers.Is(err2, newErr1))
}

// This test demonstrates that when the error library does not know
// how to encode an error, it still knows that it is different from
// other errors of different types, even though the message may be the
// same.
func TestUnknownErrorTypeDifference(t *testing.T) {
	tt := testutils.T{T: t}

	err1 := &fundamental{msg: "hello"}
	err2 := &fundamental2{msg: "hello"}

	tt.Check(!markers.Is(err1, err2))

	newErr1 := network(err1)

	tt.Check(markers.Is(err1, newErr1))

	newErr2 := network(err2)

	tt.Check(!markers.Is(newErr1, newErr2))
}

// fundamental2 is a local error type, and
// like fundamental above it is not known to the
// library (no decoders registered, no proto encoding).
type fundamental2 struct {
	msg string
}

func (e *fundamental2) Error() string { return e.msg }

// This test demonstrates that the error library preserves
// the type difference between known errors of different types.
func TestKnownErrorTypeDifference(t *testing.T) {
	tt := testutils.T{T: t}

	err1 := errors.New("hello")
	err2 := pkgErr.New("hello")

	tt.Check(!markers.Is(err1, err2))

	newErr1 := network(err1)
	newErr2 := network(err2)

	tt.Check(markers.Is(err1, newErr1))
	tt.Check(markers.Is(err2, newErr2))

	tt.Check(!markers.Is(newErr1, newErr2))
}

// This test demonstrates that two errors that are structurally
// different can be made to become equivalent by using the same
// marker.
func TestMarkerDrivenEquivalence(t *testing.T) {
	tt := testutils.T{T: t}

	err1 := errors.New("hello")
	err2 := errors.New("world")

	tt.Check(!markers.Is(err1, err2))

	m := errors.New("mark")
	err1w := markers.Mark(err1, m)
	err2w := markers.Mark(err2, m)

	tt.Check(markers.Is(err1w, m))
	tt.Check(markers.Is(err2w, m))

	tt.Check(markers.Is(err1w, err2w))
}

// This test demonstrates that equivalence can be "peeked" through
// behind multiple layers of wrapping.
func TestWrappedEquivalence(t *testing.T) {
	tt := testutils.T{T: t}

	err1 := errors.New("hello")
	err2 := pkgErr.Wrap(errors.New("hello"), "world")

	tt.Check(markers.Is(err2, err1))

	m2 := errors.New("m2")
	err2w := markers.Mark(err2, m2)

	tt.Check(markers.Is(err2w, err1))
}

// This test demonstrates that two errors that are structurally
// equivalent can be made to become non-equivalent through markers.Is()
// by using markers.
func TestMarkerDrivenDifference(t *testing.T) {
	tt := testutils.T{T: t}

	err1 := errors.New("hello")
	err2 := errors.New("hello")

	tt.Check(markers.Is(err1, err2))

	m1 := errors.New("m1")
	m2 := errors.New("m2")

	err1w := markers.Mark(err1, m1)
	err2w := markers.Mark(err2, m2)

	tt.Check(markers.Is(err1w, m1))
	tt.Check(markers.Is(err2w, m2))

	tt.Check(!markers.Is(err1w, err2w))
}

// This test demonstrates that error differences introduced
// via Mark() are preserved across the network.
func TestRemoteMarkerEquivalence(t *testing.T) {
	tt := testutils.T{T: t}

	mark := errors.New("mark")

	err1 := errors.New("hello")
	err1w := markers.Mark(err1, mark)

	newErr1w := network(err1w)

	tt.Check(markers.Is(err1w, newErr1w))

	err2 := errors.New("world")
	err2w := markers.Mark(err2, mark)

	tt.Check(markers.Is(newErr1w, err2w))
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestIsType(t *testing.T) {
	tt := testutils.T{T: t}
	base := &testError{msg: "hmm"}
	wrapped := pkgErr.Wrap(base, "boom")

	tt.Check(!markers.IsType(base, nil))
	tt.Check(!markers.IsType(wrapped, nil))

	tt.Check(markers.IsType(base, (*testError)(nil)))
	tt.Check(markers.IsType(wrapped, (*testError)(nil)))

	// nil errors don't contain any types, not even nil.
	tt.Check(!markers.IsType(nil, nil))
}

// This test is used in the RFC.
func TestLocalLocalEquivalence(t *testing.T) {
	tt := testutils.T{T: t}

	err1 := errors.New("hello")
	err2 := errors.New("hello")
	err3 := errors.New("world")

	// Different errors are different via markers.Is().
	tt.Check(!markers.Is(err1, err3))

	// Errors are equivalent to themselves.
	tt.Check(markers.Is(err1, err1))
	tt.Check(markers.Is(err2, err2))
	tt.Check(markers.Is(err3, err3))

	m := errors.New("mark")
	err1w := markers.Mark(err1, m)
	err3w := markers.Mark(err3, m)

	// Shared marks introduce explicit equivalence.
	tt.Check(markers.Is(err1w, m))
	tt.Check(markers.Is(err3w, m))
	tt.Check(markers.Is(err3w, err1w))

	m2 := errors.New("m2")
	err2w := markers.Mark(err2, m2)

	// Different marks introduce explicit non-equivalence,
	// even when the underlying errors are equivalent.
	tt.Check(!markers.Is(err2w, err1w))
}

// This test is used in the RFC.
func TestLocalRemoteEquivalence(t *testing.T) {
	tt := testutils.T{T: t}

	err1 := errors.New("hello")
	err2 := errors.New("hello")
	err3 := errors.New("world")

	err1dec := network(err1)
	err2dec := network(err2)
	err3dec := network(err3)

	// Equivalence is preserved across the network.
	tt.Check(markers.Is(err1, err1dec) && markers.Is(err1dec, err1))
	tt.Check(markers.Is(err2, err2dec) && markers.Is(err2dec, err2))
	tt.Check(markers.Is(err3, err3dec) && markers.Is(err3dec, err3))

	// Non-equivalence is preserved across the network.
	tt.Check(!markers.Is(err1dec, err3))
	tt.Check(!markers.Is(err2dec, err3))

	// "m" makes err1w and err3w equivalent.
	m := errors.New("mark")
	err1w := markers.Mark(err1, m)
	err3w := markers.Mark(err3, m)
	// "m2" makes err1w and err2w non-equivalent even though err2 and err1 are.
	m2 := errors.New("m2")
	err2w := markers.Mark(err2, m2)

	err1decw := network(err1w)
	err2decw := network(err2w)
	err3decw := network(err3w)

	// Equivalence is preserved across the network.
	tt.Check(markers.Is(err1decw, err1w) && markers.Is(err1w, err1decw))
	tt.Check(markers.Is(err2decw, err2w) && markers.Is(err2w, err2decw))
	tt.Check(markers.Is(err3decw, err3w) && markers.Is(err3w, err3decw))
	tt.Check(markers.Is(err1decw, err3w) && markers.Is(err3decw, err1w))

	// Non-equivalence is preserved across the network.
	tt.Check(!markers.Is(err1w, err2decw) && !markers.Is(err2w, err1decw))
}

// This test is used in the RFC.
func TestRemoteRemoteEquivalence(t *testing.T) {
	tt := testutils.T{T: t}

	err1 := errors.New("hello")
	err2 := errors.New("hello")
	err3 := errors.New("world")

	err1dec := network(err1)
	err2dec := network(err2)
	err3dec := network(err3)
	err1decOther := network(err1)
	err2decOther := network(err2)
	err3decOther := network(err3)

	// Equivalence is preserved across the network.
	tt.Check(markers.Is(err1dec, err1decOther) &&
		markers.Is(err2dec, err2decOther) &&
		markers.Is(err3dec, err3decOther))
	tt.Check(markers.Is(err1dec, err2decOther))

	// Non-equivalence is preserved across the network.
	tt.Check(!markers.Is(err1dec, err3decOther) && !markers.Is(err2dec, err3dec))

	// "m" makes err1w and err3w equivalent.
	m := errors.New("mark")
	err1w := markers.Mark(err1, m)
	err3w := markers.Mark(err3, m)
	// "m2" makes err1w and err2w non-equivalent even though err2 and err1 are.
	m2 := errors.New("m2")
	err2w := markers.Mark(err2, m2)

	err1decw := network(err1w)
	err2decw := network(err2w)
	err3decw := network(err3w)
	err1decwOther := network(err1w)
	err2decwOther := network(err2w)
	err3decwOther := network(err3w)

	// Equivalence is preserved across the network.
	tt.Check(markers.Is(err1decw, err1decwOther) && markers.Is(err1decwOther, err1decw))
	tt.Check(markers.Is(err2decw, err2decwOther) && markers.Is(err2decwOther, err2decw))
	tt.Check(markers.Is(err3decw, err3decwOther) && markers.Is(err3decwOther, err3decw))

	tt.Check(markers.Is(err1decw, err3decwOther) && markers.Is(err3decw, err1decwOther))

	// Non-equivalence is preserved across the network.
	tt.Check(!markers.Is(err1decw, err2decwOther) && !markers.Is(err2decw, err1decwOther))
}

// This test demonstrates why it is important to use all the types of the
// causes and not just the type of the first layer of wrapper.
func TestMaskedErrorEquivalence(t *testing.T) {
	tt := testutils.T{T: t}

	// The reference error in some library is constructed using errors.Wrap around some reference
	// error with a simple message and a given type.
	refErr := pkgErr.Wrap(&myErrType1{msg: "world"}, "hello")

	// Somewhere else another error gets wrapped, the error has actually
	// a different type, but it happens to have the same message.
	someErr := pkgErr.WithStack(&myErrType2{msg: "hello: world"})

	// because `Wrap` wraps with the same Go type as `WithStack`, we would have a problem
	// if we only compared the outer type.

	// However, the library does the right thing.
	tt.Check(!markers.Is(someErr, refErr))

	// Even so across the network.
	otherErr := network(someErr)
	tt.Check(!markers.Is(otherErr, refErr))
}

type myErrType1 struct{ msg string }

func (e *myErrType1) Error() string { return e.msg }

type myErrType2 struct{ msg string }

func (e *myErrType2) Error() string { return e.msg }

func TestFormat(t *testing.T) {
	tt := testutils.T{t}

	refErr := goErr.New("foo")
	const woo = `woo`
	const waawoo = `waa: woo`
	testCases := []struct {
		name          string
		err           error
		expFmtSimple  string
		expFmtVerbose string
	}{
		{"marked",
			markers.Mark(goErr.New("woo"), refErr),
			woo, `
error with mark override:
    "foo"
    errors/*errors.errorString::
  - woo`},

		{"marked + wrapper",
			markers.Mark(&werrFmt{goErr.New("woo"), "waa"}, refErr),
			waawoo, `
error with mark override:
    "foo"
    errors/*errors.errorString::
  - waa:
    -- verbose wrapper:
    waa
  - woo`},

		{"wrapper + marked",
			&werrFmt{markers.Mark(goErr.New("woo"), refErr), "waa"},
			waawoo, `
waa:
    -- verbose wrapper:
    waa
  - error with mark override:
    "foo"
    errors/*errors.errorString::
  - woo`},
	}

	for _, test := range testCases {
		tt.Run(test.name, func(tt testutils.T) {
			err := test.err

			// %s is simple formatting
			tt.CheckEqual(fmt.Sprintf("%s", err), test.expFmtSimple)

			// %v is simple formatting too, for compatibility with the past.
			tt.CheckEqual(fmt.Sprintf("%v", err), test.expFmtSimple)

			// %q is always like %s but quotes the result.
			ref := fmt.Sprintf("%q", test.expFmtSimple)
			tt.CheckEqual(fmt.Sprintf("%q", err), ref)

			// %+v is the verbose mode.
			refV := strings.TrimPrefix(test.expFmtVerbose, "\n")
			spv := fmt.Sprintf("%+v", err)
			tt.CheckEqual(spv, refV)
		})
	}
}

type werrFmt struct {
	cause error
	msg   string
}

var _ errbase.Formatter = (*werrFmt)(nil)

func (e *werrFmt) Error() string                 { return fmt.Sprintf("%s: %v", e.msg, e.cause) }
func (e *werrFmt) Unwrap() error                 { return e.cause }
func (e *werrFmt) Format(s fmt.State, verb rune) { errbase.FormatError(e, s, verb) }
func (e *werrFmt) FormatError(p errbase.Printer) error {
	p.Print(e.msg)
	if p.Detail() {
		p.Printf("-- verbose wrapper:\n%s", e.msg)
	}
	return e.cause
}
