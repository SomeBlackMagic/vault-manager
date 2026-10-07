package cmd_test

import (
	"os"

	"github.com/jhunt/go-cli"
	env "github.com/jhunt/go-envirotron"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/SomeBlackMagic/vault-manager/cmd"
)

// parse mirrors the option handling in main: environment first, then flags.
func parse(args ...string) *cmd.Options {
	opt := cmd.NewOptions()
	env.Override(opt)
	p, err := cli.NewParser(opt, cmd.ExpandFlagAssignments(args))
	Expect(err).ToNot(HaveOccurred())
	Expect(p.Next()).To(BeTrue())
	Expect(p.Error()).ToNot(HaveOccurred())
	return opt
}

var _ = Describe("Logging options", func() {
	keys := []string{"VAULT_MANAGER_LOG_LEVEL", "VAULT_MANAGER_LOG_FORMAT"}
	saved := map[string]*string{}

	BeforeEach(func() {
		for _, k := range keys {
			saved[k] = nil
			if old, ok := os.LookupEnv(k); ok {
				saved[k] = &old
			}
			os.Unsetenv(k)
		}
	})

	AfterEach(func() {
		for _, k := range keys {
			if saved[k] != nil {
				os.Setenv(k, *saved[k])
			} else {
				os.Unsetenv(k)
			}
		}
	})

	It("is empty by default so the logging package applies its defaults", func() {
		opt := parse("sync", "plan", "secret/app", "./dir")
		Expect(opt.LogLevel).To(Equal(""))
		Expect(opt.LogFormat).To(Equal(""))
	})

	It("reads the environment variables", func() {
		os.Setenv("VAULT_MANAGER_LOG_LEVEL", "debug")
		os.Setenv("VAULT_MANAGER_LOG_FORMAT", "json")
		opt := parse("sync", "plan", "secret/app", "./dir")
		Expect(opt.LogLevel).To(Equal("debug"))
		Expect(opt.LogFormat).To(Equal("json"))
	})

	It("lets command-line flags override the environment", func() {
		os.Setenv("VAULT_MANAGER_LOG_LEVEL", "debug")
		os.Setenv("VAULT_MANAGER_LOG_FORMAT", "json")
		opt := parse("--log-level=trace", "--log-format", "text", "sync", "plan", "secret/app", "./dir")
		Expect(opt.LogLevel).To(Equal("trace"))
		Expect(opt.LogFormat).To(Equal("text"))
	})

	It("accepts the flags after the command name", func() {
		opt := parse("sync", "plan", "--log-level", "warn", "secret/app", "./dir")
		Expect(opt.LogLevel).To(Equal("warn"))
	})
})

var _ = Describe("ExpandFlagAssignments", func() {
	It("splits --log-level=value and --log-format=value", func() {
		Expect(cmd.ExpandFlagAssignments([]string{"--log-level=debug", "--log-format=json", "get", "secret/x"})).
			To(Equal([]string{"--log-level", "debug", "--log-format", "json", "get", "secret/x"}))
	})

	It("leaves every other argument untouched", func() {
		args := []string{"--target", "prod", "set", "secret/x", "key=--value", "--log-levelx=1"}
		Expect(cmd.ExpandFlagAssignments(args)).To(Equal(args))
	})
})
