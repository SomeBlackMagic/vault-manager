package vaultsync_test

import (
	"bytes"
	"log/slog"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/SomeBlackMagic/vault-manager/logging"
	"github.com/SomeBlackMagic/vault-manager/vaultsync"
)

var _ = Describe("Sync logging", func() {
	const secretValue = "hunter2-do-not-log"

	var (
		mv     *mockVault
		tmpDir string
		buf    *bytes.Buffer
	)

	newLogger := func(level string) *slog.Logger {
		l, err := logging.New(logging.Config{Level: level, Writer: buf})
		Expect(err).ToNot(HaveOccurred())
		return l
	}

	// withStdin feeds input to the prompt package and keeps stdin non-TTY.
	withStdin := func(input string, fn func()) {
		r, w, err := os.Pipe()
		Expect(err).ToNot(HaveOccurred())
		_, err = w.WriteString(input)
		Expect(err).ToNot(HaveOccurred())
		w.Close()

		orig := os.Stdin
		os.Stdin = r
		defer func() { os.Stdin = orig }()
		fn()
	}

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "vaultsync-logging-*")
		Expect(err).ToNot(HaveOccurred())
		buf = &bytes.Buffer{}

		mv = newMockVault()
		mv.addSecret("secret/same", map[string]string{"k": secretValue})
		mv.addSecret("secret/changed", map[string]string{"k": "remote-" + secretValue})
		mv.addSecret("secret/removed", map[string]string{"k": secretValue})

		Expect(vaultsync.WriteLocalSecret(tmpDir, "secret/same", map[string]interface{}{"k": secretValue})).To(Succeed())
		Expect(vaultsync.WriteLocalSecret(tmpDir, "secret/changed", map[string]interface{}{"k": "local-" + secretValue})).To(Succeed())
		Expect(vaultsync.WriteLocalSecret(tmpDir, "secret/added", map[string]interface{}{"k": secretValue})).To(Succeed())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("Plan", func() {
		It("logs nothing at info level", func() {
			_, err := vaultsync.Plan(newLogger("info"), mv, "secret", tmpDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(buf.String()).To(BeEmpty())
		})

		It("logs stages, paths and counts at debug level without values", func() {
			_, err := vaultsync.Plan(newLogger("debug"), mv, "secret", tmpDir)
			Expect(err).ToNot(HaveOccurred())

			out := buf.String()
			Expect(out).To(ContainSubstring(`msg="sync plan started" vault_path=secret`))
			Expect(out).To(ContainSubstring(`msg="read local secrets" count=3`))
			Expect(out).To(ContainSubstring(`msg="fetched remote secrets" count=3`))
			Expect(out).To(ContainSubstring(`msg="compared secret" path=secret/same change=none`))
			Expect(out).To(ContainSubstring(`msg="compared secret" path=secret/added change=add`))
			Expect(out).To(ContainSubstring(`msg="compared secret" path=secret/changed change=modify`))
			Expect(out).To(ContainSubstring(`msg="compared secret" path=secret/removed change=delete`))
			Expect(out).To(ContainSubstring(`msg="sync plan completed" adds=1 changes=1 deletes=1`))
			Expect(out).ToNot(ContainSubstring(secretValue))
		})

		It("accepts a nil logger", func() {
			_, err := vaultsync.Plan(nil, mv, "secret", tmpDir)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Pull", func() {
		It("logs nothing at info level", func() {
			withStdin("", func() {
				Expect(vaultsync.Pull(newLogger("info"), mv, "secret", tmpDir)).To(Succeed())
			})
			Expect(buf.String()).To(BeEmpty())
		})

		It("logs per-secret decisions at debug level without values", func() {
			withStdin("", func() {
				Expect(vaultsync.Pull(newLogger("debug"), mv, "secret", tmpDir)).To(Succeed())
			})

			out := buf.String()
			Expect(out).To(ContainSubstring(`msg="sync pull started" vault_path=secret`))
			Expect(out).To(ContainSubstring(`msg="local secret up to date" path=secret/same`))
			Expect(out).To(ContainSubstring(`msg="local secret differs from remote" path=secret/changed interactive=false`))
			Expect(out).To(ContainSubstring(`msg="wrote new local secret" path=secret/removed`))
			Expect(out).To(ContainSubstring(`msg="sync pull completed" written=2 unchanged=1 kept_local=0`))
			Expect(out).ToNot(ContainSubstring(secretValue))
		})
	})

	Describe("Apply", func() {
		It("logs nothing at info level", func() {
			// Remote matches local, so Apply finishes without prompting.
			synced := newMockVault()
			synced.addSecret("secret/same", map[string]string{"k": secretValue})
			synced.addSecret("secret/changed", map[string]string{"k": "local-" + secretValue})
			synced.addSecret("secret/added", map[string]string{"k": secretValue})

			Expect(vaultsync.Apply(newLogger("info"), synced, "secret", tmpDir)).To(Succeed())
			Expect(buf.String()).To(BeEmpty())
			Expect(synced.written).To(BeEmpty())
		})

		It("logs applied changes at debug level without values", func() {
			withStdin("y\n", func() {
				Expect(vaultsync.Apply(newLogger("debug"), mv, "secret", tmpDir)).To(Succeed())
			})

			out := buf.String()
			Expect(out).To(ContainSubstring(`msg="sync apply started" vault_path=secret`))
			Expect(out).To(ContainSubstring(`msg="applying change" path=secret/added change=add`))
			Expect(out).To(ContainSubstring(`msg="applying change" path=secret/changed change=modify`))
			Expect(out).To(ContainSubstring(`msg="applying change" path=secret/removed change=delete`))
			Expect(out).To(ContainSubstring(`msg="sync apply completed" adds=1 changes=1 deletes=1`))
			Expect(out).ToNot(ContainSubstring(secretValue))

			Expect(mv.written).To(HaveKey("secret/added"))
			Expect(mv.written).To(HaveKey("secret/changed"))
			Expect(mv.deleted).To(ConsistOf("secret/removed"))
		})
	})
})
