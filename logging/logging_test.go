package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/SomeBlackMagic/vault-manager/logging"
)

var ctx = context.Background()

var _ = Describe("ParseLevel", func() {
	expectLevel := func(in string, want slog.Level) {
		lvl, err := logging.ParseLevel(in)
		Expect(err).ToNot(HaveOccurred())
		Expect(lvl).To(Equal(want))
	}

	It("parses every supported level", func() {
		expectLevel("error", slog.LevelError)
		expectLevel("warn", slog.LevelWarn)
		expectLevel("info", slog.LevelInfo)
		expectLevel("debug", slog.LevelDebug)
		expectLevel("trace", logging.LevelTrace)
	})

	It("is case-insensitive and trims whitespace", func() {
		expectLevel(" DEBUG ", slog.LevelDebug)
	})

	It("defaults to info for an empty value", func() {
		expectLevel("", slog.LevelInfo)
	})

	It("orders trace below debug", func() {
		Expect(logging.LevelTrace < slog.LevelDebug).To(BeTrue())
	})

	It("rejects unknown levels with a helpful message", func() {
		_, err := logging.ParseLevel("verbose")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unknown log level 'verbose'"))
		Expect(err.Error()).To(ContainSubstring("error, warn, info, debug, trace"))
	})
})

var _ = Describe("ParseFormat", func() {
	It("accepts text and json", func() {
		for _, f := range []string{"text", "json", "JSON"} {
			got, err := logging.ParseFormat(f)
			Expect(err).ToNot(HaveOccurred())
			Expect(got).To(Equal(strings.ToLower(f)))
		}
	})

	It("defaults to text for an empty value", func() {
		got, err := logging.ParseFormat("")
		Expect(err).ToNot(HaveOccurred())
		Expect(got).To(Equal("text"))
	})

	It("rejects unknown formats", func() {
		_, err := logging.ParseFormat("yaml")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unknown log format 'yaml'"))
	})
})

var _ = Describe("New", func() {
	var buf *bytes.Buffer

	BeforeEach(func() {
		buf = &bytes.Buffer{}
	})

	It("returns an error for an invalid level or format", func() {
		_, err := logging.New(logging.Config{Level: "loud", Writer: buf})
		Expect(err).To(HaveOccurred())
		_, err = logging.New(logging.Config{Format: "xml", Writer: buf})
		Expect(err).To(HaveOccurred())
	})

	It("defaults to info level", func() {
		l, err := logging.New(logging.Config{Writer: buf})
		Expect(err).ToNot(HaveOccurred())
		l.Debug("hidden")
		l.Info("shown")
		Expect(buf.String()).ToNot(ContainSubstring("hidden"))
		Expect(buf.String()).To(ContainSubstring("shown"))
	})

	It("writes text records with stable keys", func() {
		l, err := logging.New(logging.Config{Level: "info", Format: "text", Writer: buf})
		Expect(err).ToNot(HaveOccurred())
		l.Info("sync plan completed", logging.KeyPath, "secret/app", logging.KeyCount, 3)
		Expect(buf.String()).To(ContainSubstring("level=INFO"))
		Expect(buf.String()).To(ContainSubstring(`msg="sync plan completed"`))
		Expect(buf.String()).To(ContainSubstring("path=secret/app"))
		Expect(buf.String()).To(ContainSubstring("count=3"))
	})

	It("writes one JSON object per record", func() {
		l, err := logging.New(logging.Config{Level: "info", Format: "json", Writer: buf})
		Expect(err).ToNot(HaveOccurred())
		l.Warn("careful", logging.KeyCommand, "sync plan")

		var rec map[string]interface{}
		Expect(json.Unmarshal(buf.Bytes(), &rec)).To(Succeed())
		Expect(rec["level"]).To(Equal("WARN"))
		Expect(rec["msg"]).To(Equal("careful"))
		Expect(rec["command"]).To(Equal("sync plan"))
	})

	It("filters records below the configured level", func() {
		l, err := logging.New(logging.Config{Level: "warn", Writer: buf})
		Expect(err).ToNot(HaveOccurred())
		l.Info("info-msg")
		l.Warn("warn-msg")
		l.Error("error-msg")
		Expect(buf.String()).ToNot(ContainSubstring("info-msg"))
		Expect(buf.String()).To(ContainSubstring("warn-msg"))
		Expect(buf.String()).To(ContainSubstring("error-msg"))
	})

	It("emits trace records only at trace level and names them TRACE", func() {
		l, err := logging.New(logging.Config{Level: "debug", Writer: buf})
		Expect(err).ToNot(HaveOccurred())
		l.Log(ctx, logging.LevelTrace, "too-verbose")
		Expect(buf.String()).To(BeEmpty())

		l, err = logging.New(logging.Config{Level: "trace", Format: "json", Writer: buf})
		Expect(err).ToNot(HaveOccurred())
		l.Log(ctx, logging.LevelTrace, "very-verbose")
		var rec map[string]interface{}
		Expect(json.Unmarshal(buf.Bytes(), &rec)).To(Succeed())
		Expect(rec["level"]).To(Equal("TRACE"))
	})

	It("does not replace slog's default logger", func() {
		before := slog.Default()
		_, err := logging.New(logging.Config{Level: "trace", Writer: buf})
		Expect(err).ToNot(HaveOccurred())
		Expect(slog.Default()).To(BeIdenticalTo(before))
	})

	It("writes to stderr, never stdout, when no writer is given", func() {
		origOut, origErr := os.Stdout, os.Stderr
		outR, outW, err := os.Pipe()
		Expect(err).ToNot(HaveOccurred())
		errR, errW, err := os.Pipe()
		Expect(err).ToNot(HaveOccurred())
		os.Stdout, os.Stderr = outW, errW
		defer func() { os.Stdout, os.Stderr = origOut, origErr }()

		l, err := logging.New(logging.Config{Level: "info"})
		Expect(err).ToNot(HaveOccurred())
		l.Info("to-stderr")
		outW.Close()
		errW.Close()

		stdout, _ := io.ReadAll(outR)
		stderr, _ := io.ReadAll(errR)
		Expect(string(stdout)).To(BeEmpty())
		Expect(string(stderr)).To(ContainSubstring("to-stderr"))
	})
})

var _ = Describe("Setup", func() {
	var buf *bytes.Buffer

	BeforeEach(func() {
		buf = &bytes.Buffer{}
	})

	It("uses info level when nothing is configured", func() {
		l, err := logging.Setup("", "", "", buf)
		Expect(err).ToNot(HaveOccurred())
		Expect(l.Enabled(ctx, slog.LevelInfo)).To(BeTrue())
		Expect(l.Enabled(ctx, slog.LevelDebug)).To(BeFalse())
		Expect(buf.String()).To(BeEmpty())
	})

	It("maps the legacy DEBUG variable to trace and warns about it", func() {
		l, err := logging.Setup("", "", "1", buf)
		Expect(err).ToNot(HaveOccurred())
		Expect(l.Enabled(ctx, logging.LevelTrace)).To(BeTrue())
		Expect(buf.String()).To(ContainSubstring("level=WARN"))
		Expect(buf.String()).To(ContainSubstring("DEBUG environment variable is deprecated"))
	})

	It("ignores DEBUG values that mean off", func() {
		for _, v := range []string{"0", "false", "no", "off", "OFF"} {
			l, err := logging.Setup("", "", v, buf)
			Expect(err).ToNot(HaveOccurred())
			Expect(l.Enabled(ctx, slog.LevelDebug)).To(BeFalse())
		}
		Expect(buf.String()).To(BeEmpty())
	})

	It("prefers an explicit level over DEBUG", func() {
		l, err := logging.Setup("warn", "", "1", buf)
		Expect(err).ToNot(HaveOccurred())
		Expect(l.Enabled(ctx, slog.LevelInfo)).To(BeFalse())
		Expect(buf.String()).To(BeEmpty())
	})

	It("returns usage errors for invalid settings", func() {
		_, err := logging.Setup("chatty", "", "", buf)
		Expect(err).To(HaveOccurred())
		_, err = logging.Setup("info", "csv", "", buf)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("OrDiscard", func() {
	It("returns a usable logger for nil", func() {
		l := logging.OrDiscard(nil)
		Expect(l).ToNot(BeNil())
		l.Error("goes nowhere")
	})
})
