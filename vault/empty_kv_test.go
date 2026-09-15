package vault_test

import (
	"encoding/json"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"

	"github.com/SomeBlackMagic/vault-manager/vault"
)

var _ = Describe("ConstructSecrets with empty KV engine", func() {
	var (
		server *httptest.Server
		v      *vault.Vault
	)

	// fakeVaultHandler simulates a Vault server with a KV v2 mount named "kv"
	// that has no secrets.
	fakeVaultHandler := func() http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			// Mount discovery: returns "kv/" as a KV v2 mount.
			case r.URL.Path == "/v1/sys/internal/ui/mounts":
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"secret": map[string]interface{}{
							"kv/": map[string]interface{}{
								"type":        "kv",
								"description": "test kv",
								"options": map[string]interface{}{
									"version": "2",
								},
							},
						},
					},
				})

			// Token validation
			case r.URL.Path == "/v1/auth/token/lookup-self":
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"id": "test-token",
					},
				})

			default:
				// Everything else returns 404 — simulates empty KV engine
				// where neither metadata nor list calls find anything.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(404)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{},
				})
			}
		})
	}

	BeforeEach(func() {
		server = httptest.NewServer(fakeVaultHandler())
		var err error
		v, err = vault.NewVault(vault.VaultConfig{
			URL:   server.URL,
			Token: "test-token",
		})
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		server.Close()
	})

	It("returns empty secrets without error when KV engine has no secrets", func() {
		secrets, err := v.ConstructSecrets("kv", vault.TreeOpts{FetchKeys: true})
		Expect(err).ToNot(HaveOccurred())
		Expect(secrets).To(BeEmpty())
	})

	It("returns empty secrets for a subpath of an empty KV engine", func() {
		secrets, err := v.ConstructSecrets("kv/some/path", vault.TreeOpts{FetchKeys: true})
		Expect(err).ToNot(HaveOccurred())
		Expect(secrets).To(BeEmpty())
	})

	It("MountVersion returns 2 for the KV v2 mount", func() {
		version, err := v.MountVersion("kv")
		Expect(err).ToNot(HaveOccurred())
		Expect(version).To(Equal(uint(2)))
	})

	It("List returns not-found error for empty KV", func() {
		_, err := v.List("kv")
		Expect(err).To(HaveOccurred())
		Expect(vault.IsNotFound(err)).To(BeTrue())
	})
})

var _ = Describe("ConstructSecrets with empty KV v1 engine", func() {
	var (
		server *httptest.Server
		v      *vault.Vault
	)

	fakeVaultV1Handler := func() http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/v1/sys/internal/ui/mounts":
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"secret": map[string]interface{}{
							"kv/": map[string]interface{}{
								"type":        "kv",
								"description": "test kv v1",
								"options": map[string]interface{}{
									"version": "1",
								},
							},
						},
					},
				})

			case r.URL.Path == "/v1/auth/token/lookup-self":
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"id": "test-token",
					},
				})

			default:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(404)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{},
				})
			}
		})
	}

	BeforeEach(func() {
		server = httptest.NewServer(fakeVaultV1Handler())
		var err error
		v, err = vault.NewVault(vault.VaultConfig{
			URL:   server.URL,
			Token: "test-token",
		})
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		server.Close()
	})

	It("returns empty secrets without error when KV v1 engine has no secrets", func() {
		secrets, err := v.ConstructSecrets("kv", vault.TreeOpts{FetchKeys: true})
		Expect(err).ToNot(HaveOccurred())
		Expect(secrets).To(BeEmpty())
	})
})
