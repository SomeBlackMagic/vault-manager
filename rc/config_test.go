package rc_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/SomeBlackMagic/vault-manager/rc"
)

var _ = Describe("Config", func() {
	var savedEnv map[string]string
	var tmpHome string

	envKeys := []string{"HOME", "VAULT_ADDR", "VAULT_TOKEN", "VAULT_SKIP_VERIFY", "VAULT_NAMESPACE", "VAULT_CACERT"}

	BeforeEach(func() {
		savedEnv = make(map[string]string)
		for _, key := range envKeys {
			savedEnv[key] = os.Getenv(key)
		}
		var err error
		tmpHome, err = ioutil.TempDir("", "rc-test")
		Expect(err).ToNot(HaveOccurred())
		os.Setenv("HOME", tmpHome)
	})

	AfterEach(func() {
		for _, key := range envKeys {
			if val, ok := savedEnv[key]; ok && val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
		os.RemoveAll(tmpHome)
	})

	Describe("SetTarget", func() {
		It("adds a new target and sets it as current", func() {
			c := rc.Config{Version: 1}
			err := c.SetTarget("myvault", rc.Vault{URL: "https://vault.example.com"})
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Current).To(Equal("myvault"))
			v, ok, _ := c.Find("myvault")
			Expect(ok).To(BeTrue())
			Expect(v.URL).To(Equal("https://vault.example.com"))
		})

		It("initializes the Vaults map if nil", func() {
			c := rc.Config{Version: 1}
			err := c.SetTarget("v", rc.Vault{URL: "https://v.example.com"})
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Vaults).ToNot(BeNil())
		})

		It("preserves token when re-targeting the same URL with same alias", func() {
			c := rc.Config{Version: 1, Vaults: map[string]*rc.Vault{
				"v1": {URL: "https://vault.example.com", Token: "old-token"},
			}}
			err := c.SetTarget("v1", rc.Vault{URL: "https://vault.example.com"})
			Expect(err).ToNot(HaveOccurred())
			v, _, _ := c.Find("v1")
			Expect(v.Token).To(Equal("old-token"))
		})

		It("does not preserve token when URL differs for same alias", func() {
			c := rc.Config{Version: 1, Vaults: map[string]*rc.Vault{
				"v1": {URL: "https://old.example.com", Token: "old-token"},
			}}
			err := c.SetTarget("v1", rc.Vault{URL: "https://new.example.com"})
			Expect(err).ToNot(HaveOccurred())
			v, _, _ := c.Find("v1")
			Expect(v.Token).To(Equal(""))
		})
	})

	Describe("SetCurrent", func() {
		It("sets the current target to the specified alias", func() {
			c := rc.Config{Version: 1, Vaults: map[string]*rc.Vault{
				"v1": {URL: "https://vault.example.com"},
			}}
			err := c.SetCurrent("v1", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Current).To(Equal("v1"))
		})

		It("returns error for unknown alias", func() {
			c := rc.Config{Version: 1, Vaults: map[string]*rc.Vault{}}
			err := c.SetCurrent("nonexistent", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unknown target"))
		})

		It("sets SkipVerify when reskip is true", func() {
			c := rc.Config{Version: 1, Vaults: map[string]*rc.Vault{
				"v1": {URL: "https://vault.example.com", SkipVerify: false},
			}}
			err := c.SetCurrent("v1", true)
			Expect(err).ToNot(HaveOccurred())
			v, _, _ := c.Find("v1")
			Expect(v.SkipVerify).To(BeTrue())
		})

		It("does not set SkipVerify when reskip is false", func() {
			c := rc.Config{Version: 1, Vaults: map[string]*rc.Vault{
				"v1": {URL: "https://vault.example.com", SkipVerify: false},
			}}
			err := c.SetCurrent("v1", false)
			Expect(err).ToNot(HaveOccurred())
			v, _, _ := c.Find("v1")
			Expect(v.SkipVerify).To(BeFalse())
		})
	})

	Describe("SetToken", func() {
		It("sets the token on the current target", func() {
			c := rc.Config{Version: 1, Current: "v1", Vaults: map[string]*rc.Vault{
				"v1": {URL: "https://vault.example.com"},
			}}
			err := c.SetToken("my-new-token")
			Expect(err).ToNot(HaveOccurred())
			v, _, _ := c.Find("v1")
			Expect(v.Token).To(Equal("my-new-token"))
		})

		It("returns error when no current target is set", func() {
			c := rc.Config{Version: 1}
			err := c.SetToken("token")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("No target selected"))
		})

		It("returns error when current target is not found", func() {
			c := rc.Config{Version: 1, Current: "ghost", Vaults: map[string]*rc.Vault{}}
			err := c.SetToken("token")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unknown target"))
		})
	})

	Describe("Find", func() {
		var c rc.Config

		BeforeEach(func() {
			c = rc.Config{Version: 1, Vaults: map[string]*rc.Vault{
				"alias1": {URL: "https://v1.example.com", Token: "t1"},
				"alias2": {URL: "https://v2.example.com", Token: "t2"},
			}}
		})

		It("finds by alias key", func() {
			v, ok, err := c.Find("alias1")
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(v.URL).To(Equal("https://v1.example.com"))
		})

		It("finds by URL when no alias matches", func() {
			v, ok, err := c.Find("https://v1.example.com")
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(v.Token).To(Equal("t1"))
		})

		It("finds by URL with trailing slash trimmed", func() {
			v, ok, err := c.Find("https://v1.example.com/")
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(v.Token).To(Equal("t1"))
		})

		It("returns false for a completely unknown alias and URL", func() {
			_, ok, err := c.Find("nonexistent")
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("returns error when multiple vaults match the same URL", func() {
			c.Vaults["alias3"] = &rc.Vault{URL: "https://v1.example.com", Token: "t3"}
			_, _, err := c.Find("https://v1.example.com")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("More than one target"))
		})
	})

	Describe("Vault", func() {
		It("returns nil with no error when no current target is set", func() {
			c := rc.Config{Version: 1}
			v, err := c.Vault("")
			Expect(err).ToNot(HaveOccurred())
			Expect(v).To(BeNil())
		})

		It("returns the current vault when which is empty", func() {
			c := rc.Config{Version: 1, Current: "v1", Vaults: map[string]*rc.Vault{
				"v1": {URL: "https://v.example.com"},
			}}
			v, err := c.Vault("")
			Expect(err).ToNot(HaveOccurred())
			Expect(v.URL).To(Equal("https://v.example.com"))
		})

		It("returns a specific vault when which is provided", func() {
			c := rc.Config{Version: 1, Current: "v1", Vaults: map[string]*rc.Vault{
				"v1": {URL: "https://v1.example.com"},
				"v2": {URL: "https://v2.example.com"},
			}}
			v, err := c.Vault("v2")
			Expect(err).ToNot(HaveOccurred())
			Expect(v.URL).To(Equal("https://v2.example.com"))
		})

		It("returns error when target not found", func() {
			c := rc.Config{Version: 1, Current: "ghost", Vaults: map[string]*rc.Vault{}}
			_, err := c.Vault("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Describe("URL", func() {
		It("returns the URL of the current vault", func() {
			c := rc.Config{Version: 1, Current: "v1", Vaults: map[string]*rc.Vault{
				"v1": {URL: "https://v1.example.com"},
			}}
			Expect(c.URL()).To(Equal("https://v1.example.com"))
		})

		It("returns empty string when no current vault exists", func() {
			c := rc.Config{Version: 1}
			Expect(c.URL()).To(Equal(""))
		})
	})

	Describe("Apply (method)", func() {
		It("sets VAULT_ADDR and VAULT_TOKEN environment variables", func() {
			c := rc.Config{Version: 1, Current: "v1", Vaults: map[string]*rc.Vault{
				"v1": {URL: "https://v1.example.com", Token: "mytoken"},
			}}
			err := c.Apply("")
			Expect(err).ToNot(HaveOccurred())
			Expect(os.Getenv("VAULT_ADDR")).To(Equal("https://v1.example.com"))
			Expect(os.Getenv("VAULT_TOKEN")).To(Equal("mytoken"))
		})

		It("sets VAULT_SKIP_VERIFY when SkipVerify is true", func() {
			c := rc.Config{Version: 1, Current: "v1", Vaults: map[string]*rc.Vault{
				"v1": {URL: "https://v1.example.com", Token: "t", SkipVerify: true},
			}}
			err := c.Apply("")
			Expect(err).ToNot(HaveOccurred())
			Expect(os.Getenv("VAULT_SKIP_VERIFY")).To(Equal("1"))
		})

		It("sets VAULT_NAMESPACE when Namespace is non-empty", func() {
			c := rc.Config{Version: 1, Current: "v1", Vaults: map[string]*rc.Vault{
				"v1": {URL: "https://v1.example.com", Token: "t", Namespace: "team1"},
			}}
			err := c.Apply("")
			Expect(err).ToNot(HaveOccurred())
			Expect(os.Getenv("VAULT_NAMESPACE")).To(Equal("team1"))
		})

		It("does not error when no vault is configured", func() {
			c := rc.Config{Version: 1}
			err := c.Apply("")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Read and Write round-trip", func() {
		It("writes and reads back a config from the vault-managerrc file", func() {
			c := rc.Config{Version: 1, Current: "v1", Vaults: map[string]*rc.Vault{
				"v1": {URL: "https://v.example.com", Token: "tok"},
			}}
			err := c.Write()
			Expect(err).ToNot(HaveOccurred())

			safeRcPath := filepath.Join(tmpHome, ".vault-managerrc")
			_, statErr := os.Stat(safeRcPath)
			Expect(statErr).ToNot(HaveOccurred())

			c2 := rc.Read()
			Expect(c2.Current).To(Equal("v1"))
			v, ok, _ := c2.Find("v1")
			Expect(ok).To(BeTrue())
			Expect(v.URL).To(Equal("https://v.example.com"))
		})

		It("returns a default config when vault-managerrc does not exist", func() {
			c := rc.Read()
			Expect(c.Version).To(Equal(1))
			Expect(c.Vaults).To(BeNil())
		})
	})
})
