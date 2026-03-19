package vault_test

import (
	"encoding/base64"
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/SomeBlackMagic/vault-manager/vault"
)

var _ = Describe("Secret", func() {
	Describe("NewSecret", func() {
		It("creates a non-nil secret", func() {
			s := vault.NewSecret()
			Expect(s).ToNot(BeNil())
		})

		It("creates an empty secret", func() {
			s := vault.NewSecret()
			Expect(s.Empty()).To(BeTrue())
		})
	})

	Describe("Has", func() {
		It("returns false for a key that does not exist", func() {
			s := vault.NewSecret()
			Expect(s.Has("missing")).To(BeFalse())
		})

		It("returns true for a key that exists", func() {
			s := vault.NewSecret()
			s.Set("exists", "val", false)
			Expect(s.Has("exists")).To(BeTrue())
		})
	})

	Describe("Get", func() {
		It("returns empty string for a missing key", func() {
			s := vault.NewSecret()
			Expect(s.Get("missing")).To(Equal(""))
		})

		It("returns the value for an existing key", func() {
			s := vault.NewSecret()
			s.Set("key", "value", false)
			Expect(s.Get("key")).To(Equal("value"))
		})
	})

	Describe("Set", func() {
		It("sets a new key-value pair", func() {
			s := vault.NewSecret()
			err := s.Set("key", "value", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(s.Get("key")).To(Equal("value"))
		})

		It("overwrites an existing key when skipIfExists is false", func() {
			s := vault.NewSecret()
			s.Set("key", "old", false)
			err := s.Set("key", "new", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(s.Get("key")).To(Equal("new"))
		})

		It("returns an error when skipIfExists is true and key exists", func() {
			s := vault.NewSecret()
			s.Set("key", "old", false)
			err := s.Set("key", "new", true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already existed"))
		})

		It("sets normally when skipIfExists is true and key does not exist", func() {
			s := vault.NewSecret()
			err := s.Set("key", "value", true)
			Expect(err).ToNot(HaveOccurred())
			Expect(s.Get("key")).To(Equal("value"))
		})
	})

	Describe("Delete", func() {
		It("returns true and removes an existing key", func() {
			s := vault.NewSecret()
			s.Set("key", "value", false)
			Expect(s.Delete("key")).To(BeTrue())
			Expect(s.Has("key")).To(BeFalse())
		})

		It("returns false for a key that does not exist", func() {
			s := vault.NewSecret()
			Expect(s.Delete("missing")).To(BeFalse())
		})
	})

	Describe("Keys", func() {
		It("returns an empty slice for an empty secret", func() {
			s := vault.NewSecret()
			Expect(s.Keys()).To(BeEmpty())
		})

		It("returns all keys sorted alphabetically", func() {
			s := vault.NewSecret()
			s.Set("banana", "b", false)
			s.Set("apple", "a", false)
			s.Set("cherry", "c", false)
			Expect(s.Keys()).To(Equal([]string{"apple", "banana", "cherry"}))
		})
	})

	Describe("Empty", func() {
		It("returns true for a new secret", func() {
			s := vault.NewSecret()
			Expect(s.Empty()).To(BeTrue())
		})

		It("returns false after setting a key", func() {
			s := vault.NewSecret()
			s.Set("key", "value", false)
			Expect(s.Empty()).To(BeFalse())
		})

		It("returns true after deleting the only key", func() {
			s := vault.NewSecret()
			s.Set("key", "value", false)
			s.Delete("key")
			Expect(s.Empty()).To(BeTrue())
		})
	})

	Describe("JSON", func() {
		It("returns valid JSON for an empty secret", func() {
			s := vault.NewSecret()
			j := s.JSON()
			Expect(j).To(Equal("{}"))
		})

		It("returns valid JSON with key-value pairs", func() {
			s := vault.NewSecret()
			s.Set("key", "value", false)
			j := s.JSON()
			var m map[string]string
			err := json.Unmarshal([]byte(j), &m)
			Expect(err).ToNot(HaveOccurred())
			Expect(m["key"]).To(Equal("value"))
		})
	})

	Describe("YAML", func() {
		It("returns valid YAML for an empty secret", func() {
			s := vault.NewSecret()
			y := s.YAML()
			Expect(y).To(Equal("{}\n"))
		})

		It("returns valid YAML with key-value pairs", func() {
			s := vault.NewSecret()
			s.Set("key", "value", false)
			y := s.YAML()
			Expect(y).To(ContainSubstring("key: value"))
		})
	})

	Describe("MarshalJSON / UnmarshalJSON", func() {
		It("round-trips through JSON marshal/unmarshal", func() {
			s := vault.NewSecret()
			s.Set("alpha", "one", false)
			s.Set("beta", "two", false)
			b, err := json.Marshal(s)
			Expect(err).ToNot(HaveOccurred())

			s2 := vault.NewSecret()
			err = json.Unmarshal(b, s2)
			Expect(err).ToNot(HaveOccurred())
			Expect(s2.Get("alpha")).To(Equal("one"))
			Expect(s2.Get("beta")).To(Equal("two"))
		})

		It("unmarshals from a raw JSON string", func() {
			s := vault.NewSecret()
			err := json.Unmarshal([]byte(`{"foo":"bar"}`), s)
			Expect(err).ToNot(HaveOccurred())
			Expect(s.Get("foo")).To(Equal("bar"))
		})
	})

	Describe("SingleValue", func() {
		It("returns the value when the secret has exactly one key", func() {
			s := vault.NewSecret()
			s.Set("only", "thevalue", false)
			val, err := s.SingleValue()
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal("thevalue"))
		})

		It("returns an error when the secret is empty", func() {
			s := vault.NewSecret()
			_, err := s.SingleValue()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("0 results"))
		})

		It("returns an error when the secret has more than one key", func() {
			s := vault.NewSecret()
			s.Set("a", "1", false)
			s.Set("b", "2", false)
			_, err := s.SingleValue()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("2 results"))
		})
	})

	Describe("Format", func() {
		Context("with crypt-md5", func() {
			It("produces output starting with $1$", func() {
				s := vault.NewSecret()
				s.Set("password", "test123", false)
				err := s.Format("password", "password-md5", "crypt-md5", false)
				Expect(err).ToNot(HaveOccurred())
				Expect(s.Get("password-md5")).To(HavePrefix("$1$"))
			})
		})

		Context("with crypt-sha256", func() {
			It("produces output starting with $5$", func() {
				s := vault.NewSecret()
				s.Set("password", "test123", false)
				err := s.Format("password", "password-sha256", "crypt-sha256", false)
				Expect(err).ToNot(HaveOccurred())
				Expect(s.Get("password-sha256")).To(HavePrefix("$5$"))
			})
		})

		Context("with crypt-sha512", func() {
			It("produces output starting with $6$", func() {
				s := vault.NewSecret()
				s.Set("password", "test123", false)
				err := s.Format("password", "password-sha512", "crypt-sha512", false)
				Expect(err).ToNot(HaveOccurred())
				Expect(s.Get("password-sha512")).To(HavePrefix("$6$"))
			})
		})

		Context("with bcrypt", func() {
			It("produces output starting with $2a$", func() {
				s := vault.NewSecret()
				s.Set("password", "test123", false)
				err := s.Format("password", "password-bcrypt", "bcrypt", false)
				Expect(err).ToNot(HaveOccurred())
				Expect(s.Get("password-bcrypt")).To(HavePrefix("$2a$"))
			})
		})

		Context("with base64", func() {
			It("produces correct base64 encoding", func() {
				s := vault.NewSecret()
				s.Set("data", "hello world", false)
				err := s.Format("data", "data-b64", "base64", false)
				Expect(err).ToNot(HaveOccurred())
				expected := base64.StdEncoding.EncodeToString([]byte("hello world"))
				Expect(s.Get("data-b64")).To(Equal(expected))
			})
		})

		Context("with an invalid format type", func() {
			It("returns an error", func() {
				s := vault.NewSecret()
				s.Set("key", "val", false)
				err := s.Format("key", "key2", "invalid-fmt", false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not a valid encoding"))
			})
		})

		Context("when the source key does not exist", func() {
			It("returns a secretNotFound error", func() {
				s := vault.NewSecret()
				err := s.Format("nonexistent", "out", "base64", false)
				Expect(err).To(HaveOccurred())
				Expect(vault.IsSecretNotFound(err)).To(BeTrue())
			})
		})

		Context("with skipIfExists true and target key exists", func() {
			It("returns an error and does not overwrite", func() {
				s := vault.NewSecret()
				s.Set("in", "val", false)
				s.Set("out", "existing", false)
				err := s.Format("in", "out", "base64", true)
				Expect(err).To(HaveOccurred())
				Expect(s.Get("out")).To(Equal("existing"))
			})
		})
	})

	Describe("Password", func() {
		It("generates a password of the requested length", func() {
			s := vault.NewSecret()
			err := s.Password("pass", 32, "a-zA-Z0-9", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(s.Get("pass"))).To(Equal(32))
		})

		It("generates a password matching the given policy", func() {
			s := vault.NewSecret()
			err := s.Password("pass", 100, "a-z", false)
			Expect(err).ToNot(HaveOccurred())
			val := s.Get("pass")
			Expect(val).To(MatchRegexp("^[a-z]+$"))
		})

		It("generates a password of length 1", func() {
			s := vault.NewSecret()
			err := s.Password("pass", 1, "a-zA-Z0-9", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(s.Get("pass"))).To(Equal(1))
		})

		It("respects skipIfExists when key already exists", func() {
			s := vault.NewSecret()
			s.Set("pass", "original", false)
			err := s.Password("pass", 16, "a-z", true)
			Expect(err).To(HaveOccurred())
			Expect(s.Get("pass")).To(Equal("original"))
		})

		It("overwrites when skipIfExists is false", func() {
			s := vault.NewSecret()
			s.Set("pass", "original", false)
			err := s.Password("pass", 16, "a-z", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(s.Get("pass")).ToNot(Equal("original"))
		})
	})
})
