package desc_test

import (
	"testing"

	"github.com/jhump/protoreflect/desc"
	_ "github.com/jhump/protoreflect/internal/testprotos"
	"github.com/jhump/protoreflect/internal/testutil"
)

func TestResolveImport(t *testing.T) {
	desc.RegisterImportPath("desc_test1.proto", "foobar/desc_test1.proto")
	testutil.Eq(t, "desc_test1.proto", desc.ResolveImport("foobar/desc_test1.proto"))
	testutil.Eq(t, "foobar/snafu.proto", desc.ResolveImport("foobar/snafu.proto"))

	expectPanic(t, func() {
		desc.RegisterImportPath("", "foobar/desc_test1.proto")
	})
	expectPanic(t, func() {
		desc.RegisterImportPath("desc_test1.proto", "")
	})
	expectPanic(t, func() {
		// not a real registered path
		desc.RegisterImportPath("github.com/jhump/x/y/z/foobar.proto", "x/y/z/foobar.proto")
	})
}

func TestImportResolver(t *testing.T) {
	var r desc.ImportResolver

	expectPanic(t, func() {
		r.RegisterImportPath("", "a/b/c/d.proto")
	})
	expectPanic(t, func() {
		r.RegisterImportPath("d.proto", "")
	})

	// no source constraints
	r.RegisterImportPath("foo/bar.proto", "bar.proto")
	testutil.Eq(t, "foo/bar.proto", r.ResolveImport("test.proto", "bar.proto"))
	testutil.Eq(t, "foo/bar.proto", r.ResolveImport("some/other/source.proto", "bar.proto"))

	// with specific source file
	r.RegisterImportPathFrom("fubar/baz.proto", "baz.proto", "test/test.proto")
	// match
	testutil.Eq(t, "fubar/baz.proto", r.ResolveImport("test/test.proto", "baz.proto"))
	// no match
	testutil.Eq(t, "baz.proto", r.ResolveImport("test.proto", "baz.proto"))
	testutil.Eq(t, "baz.proto", r.ResolveImport("test/test2.proto", "baz.proto"))
	testutil.Eq(t, "baz.proto", r.ResolveImport("some/other/source.proto", "baz.proto"))

	// with specific source file with long path
	r.RegisterImportPathFrom("fubar/frobnitz/baz.proto", "baz.proto", "a/b/c/d/e/f/g/test/test.proto")
	// match
	testutil.Eq(t, "fubar/frobnitz/baz.proto", r.ResolveImport("a/b/c/d/e/f/g/test/test.proto", "baz.proto"))
	// no match
	testutil.Eq(t, "baz.proto", r.ResolveImport("test.proto", "baz.proto"))
	testutil.Eq(t, "baz.proto", r.ResolveImport("test/test2.proto", "baz.proto"))
	testutil.Eq(t, "baz.proto", r.ResolveImport("some/other/source.proto", "baz.proto"))

	// with source path
	r.RegisterImportPathFrom("fubar/frobnitz/snafu.proto", "frobnitz/snafu.proto", "a/b/c/d/e/f/g/h")
	// match
	testutil.Eq(t, "fubar/frobnitz/snafu.proto", r.ResolveImport("a/b/c/d/e/f/g/h/test/test.proto", "frobnitz/snafu.proto"))
	testutil.Eq(t, "fubar/frobnitz/snafu.proto", r.ResolveImport("a/b/c/d/e/f/g/h/abc.proto", "frobnitz/snafu.proto"))
	// no match
	testutil.Eq(t, "frobnitz/snafu.proto", r.ResolveImport("a/b/c/d/e/f/g/test/test.proto", "frobnitz/snafu.proto"))
	testutil.Eq(t, "frobnitz/snafu.proto", r.ResolveImport("test.proto", "frobnitz/snafu.proto"))
	testutil.Eq(t, "frobnitz/snafu.proto", r.ResolveImport("test/test2.proto", "frobnitz/snafu.proto"))
	testutil.Eq(t, "frobnitz/snafu.proto", r.ResolveImport("some/other/source.proto", "frobnitz/snafu.proto"))

	// falls back to global registered paths
	desc.RegisterImportPath("desc_test1.proto", "x/y/z/desc_test1.proto")
	testutil.Eq(t, "desc_test1.proto", r.ResolveImport("a/b/c/d/e/f/g/h/test/test.proto", "x/y/z/desc_test1.proto"))
}

func expectPanic(t *testing.T, fn func()) {
	defer func() {
		p := recover()
		testutil.Require(t, p != nil, "expecting panic")
	}()

	fn()
}
