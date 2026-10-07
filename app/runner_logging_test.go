package app

import (
	"bytes"
	"errors"
	"log/slog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/SomeBlackMagic/vault-manager/logging"
)

var _ = Describe("Runner logging", func() {
	var (
		r   *Runner
		buf *bytes.Buffer
	)

	newLogger := func(level string) *slog.Logger {
		l, err := logging.New(logging.Config{Level: level, Writer: buf})
		Expect(err).ToNot(HaveOccurred())
		return l
	}

	BeforeEach(func() {
		r = NewRunner()
		buf = &bytes.Buffer{}
	})

	It("has a non-nil logger by default", func() {
		Expect(r.Log()).ToNot(BeNil())
		r.Logger = nil
		Expect(r.Log()).ToNot(BeNil())
	})

	It("logs command lifecycle at debug without argument values", func() {
		r.Logger = newLogger("debug")
		r.Dispatch("set", nil, func(cmd string, args ...string) error { return nil })

		Expect(r.Execute("set", "secret/app", "password=hunter2")).To(Succeed())

		out := buf.String()
		Expect(out).To(ContainSubstring(`msg="command started" command=set args=2`))
		Expect(out).To(ContainSubstring(`msg="command completed" command=set duration=`))
		Expect(out).ToNot(ContainSubstring("hunter2"))
		Expect(out).ToNot(ContainSubstring("secret/app"))
	})

	It("stays silent at info level", func() {
		r.Logger = newLogger("info")
		r.Dispatch("ok", nil, func(cmd string, args ...string) error { return nil })
		r.Dispatch("fail", nil, func(cmd string, args ...string) error { return errors.New("boom") })

		Expect(r.Execute("ok")).To(Succeed())
		Expect(r.Execute("fail")).To(HaveOccurred())
		Expect(buf.String()).To(BeEmpty())
	})

	It("logs failures with their type and returns the original error", func() {
		r.Logger = newLogger("debug")
		usage := NewUsageError("missing argument")
		r.Dispatch("u", nil, func(cmd string, args ...string) error { return usage })
		r.Dispatch("e", nil, func(cmd string, args ...string) error { return errors.New("boom") })

		Expect(r.Execute("u")).To(BeIdenticalTo(usage))
		Expect(r.Execute("e")).To(MatchError("boom"))

		out := buf.String()
		Expect(out).To(ContainSubstring(`msg="command failed" command=u error_type=usage error="missing argument"`))
		Expect(out).To(ContainSubstring(`msg="command failed" command=e error_type=runtime error=boom`))
	})
})
