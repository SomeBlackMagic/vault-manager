package vault_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/SomeBlackMagic/vault-manager/vault"
)

var _ = Describe("Utils", func() {
	Describe("ParsePath", func() {
		var inPath, inKey, inVersion string
		var outPath, outKey string
		var outVersion uint64

		var expPath, expKey string
		var expVersion uint64

		JustBeforeEach(func() {
			var fullInPath string = inPath
			if inKey != "" {
				fullInPath = fullInPath + ":" + inKey
			}
			if inVersion != "" {
				fullInPath = fullInPath + "^" + inVersion
			}
			outPath, outKey, outVersion = vault.ParsePath(fullInPath)
		})

		AfterEach(func() {
			inPath, inKey, inVersion = "", "", ""
			outPath, outKey = "", ""
			outVersion = 0
			expPath, expKey = "", ""
			expVersion = 0
		})

		assertPathValues := func() {
			It("should have the expected values", func() {
				By("having the correct path value")
				Expect(outPath).To(Equal(expPath))

				By("having the correct key value")
				Expect(outKey).To(Equal(expKey))

				By("having the correct version value")
				Expect(outVersion).To(Equal(expVersion))
			})
		}

		type ioStruct struct{ in, out, desc string }

		paths := []ioStruct{
			{"secret/foo", "secret/foo", "that is basic"},
			{`secret/f\:oo`, "secret/f:oo", "that has an escaped colon"},
			{`secret/f\^oo`, "secret/f^oo", "that has an escaped caret"},
		}

		keys := []ioStruct{
			{"bar", "bar", "that is basic"},
			{`b\:ar`, "b:ar", "that has an escaped colon"},
			{`b\^ar`, "b^ar", "that has an escaped caret"},
		}

		Context("with a path", func() {
			for i := range paths {
				path := paths[i]
				Context(path.desc, func() {
					BeforeEach(func() {
						inPath, expPath = path.in, path.out
					})

					assertPathValues()

					Context("with a key", func() {
						for j := range keys {
							key := keys[j]
							Context(key.desc, func() {
								BeforeEach(func() {
									inKey, expKey = key.in, key.out
								})

								assertPathValues()

								Context("with a version", func() {
									Context("that is zero", func() {
										BeforeEach(func() {
											inVersion, expVersion = "0", 0
										})

										assertPathValues()
									})

									Context("that is positive", func() {
										BeforeEach(func() {
											inVersion, expVersion = "21", 21
										})

										assertPathValues()
									})
								})
							})
						}
					})
				})
			}
		})

		Context("with a path that has an unescaped colon and a key", func() {
			BeforeEach(func() {
				inPath, inKey = "secret:foo", "bar"
				expPath, expKey = "secret:foo", "bar"
			})

			assertPathValues()
		})

		Context("with a path that has an unescaped caret and a version", func() {
			BeforeEach(func() {
				inPath, inVersion = "secret^foo", "2"
				expPath, expVersion = "secret^foo", 2
			})
		})
	})

	Describe("EscapePathSegment", func() {
		It("escapes colons", func() {
			Expect(vault.EscapePathSegment("foo:bar")).To(Equal(`foo\:bar`))
		})

		It("escapes carets", func() {
			Expect(vault.EscapePathSegment("foo^bar")).To(Equal(`foo\^bar`))
		})

		It("escapes both colons and carets", func() {
			Expect(vault.EscapePathSegment("a:b^c")).To(Equal(`a\:b\^c`))
		})

		It("returns the same string if no special characters", func() {
			Expect(vault.EscapePathSegment("secret/foo/bar")).To(Equal("secret/foo/bar"))
		})

		It("handles empty string", func() {
			Expect(vault.EscapePathSegment("")).To(Equal(""))
		})

		It("handles multiple consecutive colons", func() {
			Expect(vault.EscapePathSegment("a::b")).To(Equal(`a\:\:b`))
		})
	})

	Describe("EncodePath", func() {
		It("encodes path only with no key or version", func() {
			Expect(vault.EncodePath("secret/foo", "", 0)).To(Equal(`secret/foo`))
		})

		It("encodes path with key", func() {
			Expect(vault.EncodePath("secret/foo", "bar", 0)).To(Equal(`secret/foo:bar`))
		})

		It("encodes path with version", func() {
			Expect(vault.EncodePath("secret/foo", "", 5)).To(Equal(`secret/foo^5`))
		})

		It("encodes path with key and version", func() {
			Expect(vault.EncodePath("secret/foo", "bar", 5)).To(Equal(`secret/foo:bar^5`))
		})

		It("escapes colons in the path segment", func() {
			Expect(vault.EncodePath("secret/f:oo", "bar", 0)).To(Equal(`secret/f\:oo:bar`))
		})

		It("escapes carets in the key segment", func() {
			Expect(vault.EncodePath("secret/foo", "b^ar", 0)).To(Equal(`secret/foo:b\^ar`))
		})

		It("round-trips through ParsePath", func() {
			encoded := vault.EncodePath("secret/f:oo", "b^ar", 21)
			path, key, version := vault.ParsePath(encoded)
			Expect(path).To(Equal("secret/f:oo"))
			Expect(key).To(Equal("b^ar"))
			Expect(version).To(Equal(uint64(21)))
		})
	})

	Describe("PathHasKey", func() {
		It("returns true when path contains a key", func() {
			Expect(vault.PathHasKey("secret/foo:bar")).To(BeTrue())
		})

		It("returns false when path has no key", func() {
			Expect(vault.PathHasKey("secret/foo")).To(BeFalse())
		})

		It("returns false for an empty string", func() {
			Expect(vault.PathHasKey("")).To(BeFalse())
		})
	})

	Describe("PathHasVersion", func() {
		It("returns true when path contains a version", func() {
			Expect(vault.PathHasVersion("secret/foo:bar^3")).To(BeTrue())
		})

		It("returns true when path has version without key", func() {
			Expect(vault.PathHasVersion("secret/foo^3")).To(BeTrue())
		})

		It("returns false when path has no version", func() {
			Expect(vault.PathHasVersion("secret/foo:bar")).To(BeFalse())
		})

		It("returns false when version is zero", func() {
			Expect(vault.PathHasVersion("secret/foo^0")).To(BeFalse())
		})
	})

	Describe("Canonicalize", func() {
		It("trims a trailing slash", func() {
			Expect(vault.Canonicalize("secret/foo/")).To(Equal("secret/foo"))
		})

		It("trims a leading slash", func() {
			Expect(vault.Canonicalize("/secret/foo")).To(Equal("secret/foo"))
		})

		It("trims both leading and trailing slashes", func() {
			Expect(vault.Canonicalize("/secret/foo/")).To(Equal("secret/foo"))
		})

		It("collapses multiple consecutive slashes", func() {
			Expect(vault.Canonicalize("secret//foo///bar")).To(Equal("secret/foo/bar"))
		})

		It("handles a single slash", func() {
			Expect(vault.Canonicalize("/")).To(Equal(""))
		})

		It("handles an empty string", func() {
			Expect(vault.Canonicalize("")).To(Equal(""))
		})

		It("handles a path with no issues", func() {
			Expect(vault.Canonicalize("secret/foo/bar")).To(Equal("secret/foo/bar"))
		})
	})
})
