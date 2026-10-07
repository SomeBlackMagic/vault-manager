package vault_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/SomeBlackMagic/vault-manager/logging"
	"github.com/SomeBlackMagic/vault-manager/vault"
)

var _ = Describe("RedactURL", func() {
	It("drops user info and masks sensitive query values", func() {
		u, err := url.Parse("https://user:pass@vault.example.com:8200/v1/secret/app?version=2&token=abc&X-Client-Secret=zzz")
		Expect(err).ToNot(HaveOccurred())
		got := vault.RedactURL(u)
		Expect(got).ToNot(ContainSubstring("user"))
		Expect(got).ToNot(ContainSubstring("pass"))
		Expect(got).ToNot(ContainSubstring("abc"))
		Expect(got).ToNot(ContainSubstring("zzz"))
		Expect(got).To(ContainSubstring("version=2"))
		Expect(got).To(ContainSubstring("token=REDACTED"))
		Expect(got).To(ContainSubstring("/v1/secret/app"))
	})

	It("handles nil", func() {
		Expect(vault.RedactURL(nil)).To(Equal(""))
	})
})

var _ = Describe("Vault HTTP tracing", func() {
	const (
		token       = "s.SUPER-SECRET-TOKEN"
		secretValue = "hunter2-do-not-log"
	)

	var (
		server *httptest.Server
		buf    *bytes.Buffer
	)

	BeforeEach(func() {
		buf = &bytes.Buffer{}
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/v1/sys/internal/ui/mounts":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"secret": map[string]interface{}{
							"secret/": map[string]interface{}{
								"type":    "kv",
								"options": map[string]interface{}{"version": "1"},
							},
						},
					},
				})
			case "/v1/secret/app":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{"password": secretValue},
				})
			default:
				w.WriteHeader(404)
				json.NewEncoder(w).Encode(map[string]interface{}{"errors": []string{}})
			}
		}))
	})

	AfterEach(func() {
		server.Close()
	})

	connect := func(level string) *vault.Vault {
		l, err := logging.New(logging.Config{Level: level, Writer: buf})
		Expect(err).ToNot(HaveOccurred())
		v, err := vault.NewVault(vault.VaultConfig{URL: server.URL, Token: token, Logger: l})
		Expect(err).ToNot(HaveOccurred())
		return v
	}

	It("logs method, URL and status at trace without token or body", func() {
		v := connect("trace")
		s, err := v.Read("secret/app")
		Expect(err).ToNot(HaveOccurred())
		Expect(s.Get("password")).To(Equal(secretValue))

		out := buf.String()
		Expect(out).To(ContainSubstring(`level=TRACE msg="vault http request" method=GET`))
		Expect(out).To(ContainSubstring("/v1/secret/app"))
		Expect(out).To(ContainSubstring("status=200"))
		Expect(out).ToNot(ContainSubstring(token))
		Expect(out).ToNot(ContainSubstring(secretValue))
		Expect(out).ToNot(ContainSubstring("X-Vault-Token"))
	})

	It("logs operations at debug without HTTP details or values", func() {
		v := connect("debug")
		_, err := v.Read("secret/app")
		Expect(err).ToNot(HaveOccurred())

		s := vault.NewSecret()
		Expect(s.Set("password", secretValue, false)).To(Succeed())
		_ = v.Write("secret/other", s)

		out := buf.String()
		Expect(out).To(ContainSubstring(`msg="reading secret" path=secret/app`))
		Expect(out).To(ContainSubstring(`msg="writing secret" path=secret/other count=1`))
		Expect(out).ToNot(ContainSubstring("vault http request"))
		Expect(out).ToNot(ContainSubstring(token))
		Expect(out).ToNot(ContainSubstring(secretValue))
	})

	It("logs nothing at info", func() {
		v := connect("info")
		_, err := v.Read("secret/app")
		Expect(err).ToNot(HaveOccurred())
		Expect(buf.String()).To(BeEmpty())
	})

	It("logs failed requests at trace", func() {
		v := connect("trace")
		server.Close()
		_, err := v.Read("secret/app")
		Expect(err).To(HaveOccurred())
		Expect(buf.String()).To(ContainSubstring(`msg="vault http request failed"`))
		Expect(buf.String()).ToNot(ContainSubstring(token))
	})
})
