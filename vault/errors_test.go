package vault_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/SomeBlackMagic/vault-manager/vault"
)

var _ = Describe("Errors", func() {
	Describe("NewSecretNotFoundError", func() {
		It("returns an error with the path in the message", func() {
			err := vault.NewSecretNotFoundError("secret/my/path")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("secret/my/path"))
			Expect(err.Error()).To(ContainSubstring("no secret exists at path"))
		})

		It("satisfies the error interface", func() {
			var err error = vault.NewSecretNotFoundError("test")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("NewKeyNotFoundError", func() {
		It("returns an error with the path and key in the message", func() {
			err := vault.NewKeyNotFoundError("secret/path", "mykey")
			Expect(err.Error()).To(ContainSubstring("mykey"))
			Expect(err.Error()).To(ContainSubstring("secret/path"))
		})

		It("formats the error message correctly", func() {
			err := vault.NewKeyNotFoundError("secret/path", "mykey")
			Expect(err.Error()).To(Equal("no key `mykey` exists in secret `secret/path`"))
		})
	})

	Describe("IsSecretNotFound", func() {
		It("returns true for a secretNotFound error", func() {
			err := vault.NewSecretNotFoundError("p")
			Expect(vault.IsSecretNotFound(err)).To(BeTrue())
		})

		It("returns false for a keyNotFound error", func() {
			err := vault.NewKeyNotFoundError("p", "k")
			Expect(vault.IsSecretNotFound(err)).To(BeFalse())
		})

		It("returns false for a generic error", func() {
			err := fmt.Errorf("some error")
			Expect(vault.IsSecretNotFound(err)).To(BeFalse())
		})

		It("returns false for nil", func() {
			Expect(vault.IsSecretNotFound(nil)).To(BeFalse())
		})
	})

	Describe("IsKeyNotFound", func() {
		It("returns true for a keyNotFound error", func() {
			err := vault.NewKeyNotFoundError("p", "k")
			Expect(vault.IsKeyNotFound(err)).To(BeTrue())
		})

		It("returns false for a secretNotFound error", func() {
			err := vault.NewSecretNotFoundError("p")
			Expect(vault.IsKeyNotFound(err)).To(BeFalse())
		})

		It("returns false for a generic error", func() {
			err := fmt.Errorf("some error")
			Expect(vault.IsKeyNotFound(err)).To(BeFalse())
		})

		It("returns false for nil", func() {
			Expect(vault.IsKeyNotFound(nil)).To(BeFalse())
		})
	})

	Describe("IsNotFound", func() {
		It("returns true for a secretNotFound error", func() {
			Expect(vault.IsNotFound(vault.NewSecretNotFoundError("p"))).To(BeTrue())
		})

		It("returns true for a keyNotFound error", func() {
			Expect(vault.IsNotFound(vault.NewKeyNotFoundError("p", "k"))).To(BeTrue())
		})

		It("returns false for a generic error", func() {
			Expect(vault.IsNotFound(fmt.Errorf("nope"))).To(BeFalse())
		})

		It("returns false for nil", func() {
			Expect(vault.IsNotFound(nil)).To(BeFalse())
		})
	})
})
