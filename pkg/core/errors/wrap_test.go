package errors_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/skygeario/skygear-server/pkg/core/errors"
)

func TestErrorf(t *testing.T) {
	Convey("Wrap", t, func() {
		inner := errors.New("inner")
		err := errors.Wrap(inner, "test")
		So(err, ShouldBeError, "test")
		So(errors.Is(err, inner), ShouldBeTrue)
	})
	Convey("Wrapf", t, func() {
		inner := errors.New("inner")
		err := errors.Wrapf(inner, "err %d", 1)
		So(err, ShouldBeError, "err 1")
		So(errors.Is(err, inner), ShouldBeTrue)
	})
}
