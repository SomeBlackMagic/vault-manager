package vault_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/SomeBlackMagic/vault-manager/vault"
)

var _ = Describe("Tree", func() {
	Describe("PathLessThan", func() {
		It("returns true when left is alphabetically before right at same depth", func() {
			Expect(vault.PathLessThan("secret/a", "secret/b")).To(BeTrue())
		})

		It("returns false when left is alphabetically after right", func() {
			Expect(vault.PathLessThan("secret/b", "secret/a")).To(BeFalse())
		})

		It("returns true for identical non-trailing-slash paths", func() {
			// Implementation detail: when len(left)==len(right) and all segments match,
			// returns !strings.HasSuffix(left, "/"), so "secret/a" → true
			Expect(vault.PathLessThan("secret/a", "secret/a")).To(BeTrue())
		})

		It("returns false for identical trailing-slash paths", func() {
			Expect(vault.PathLessThan("secret/a/", "secret/a/")).To(BeFalse())
		})

		It("compares segment by segment, not lexicographically", func() {
			Expect(vault.PathLessThan("secret/a/z", "secret/b/a")).To(BeTrue())
		})

		It("shorter path is less than longer path when prefix matches", func() {
			Expect(vault.PathLessThan("secret/a", "secret/a/b")).To(BeTrue())
		})

		It("handles paths with trailing slashes via Canonicalize", func() {
			Expect(vault.PathLessThan("secret/a/", "secret/b")).To(BeTrue())
		})

		It("non-trailing-slash path is less than trailing-slash path of same length", func() {
			Expect(vault.PathLessThan("secret/a", "secret/a/")).To(BeTrue())
		})
	})

	Describe("Secrets.Sort", func() {
		It("sorts secrets by path using PathLessThan", func() {
			s := vault.Secrets{
				{Path: "secret/c"},
				{Path: "secret/a"},
				{Path: "secret/b"},
			}
			s.Sort()
			Expect(s[0].Path).To(Equal("secret/a"))
			Expect(s[1].Path).To(Equal("secret/b"))
			Expect(s[2].Path).To(Equal("secret/c"))
		})

		It("sorts an already sorted list without change", func() {
			s := vault.Secrets{
				{Path: "secret/a"},
				{Path: "secret/b"},
			}
			s.Sort()
			Expect(s[0].Path).To(Equal("secret/a"))
			Expect(s[1].Path).To(Equal("secret/b"))
		})

		It("handles an empty secrets list", func() {
			s := vault.Secrets{}
			s.Sort()
			Expect(s).To(BeEmpty())
		})

		It("handles a single-element list", func() {
			s := vault.Secrets{{Path: "secret/only"}}
			s.Sort()
			Expect(s[0].Path).To(Equal("secret/only"))
		})
	})

	Describe("Secrets.Merge", func() {
		It("merges two non-overlapping sorted secrets lists", func() {
			s1 := vault.Secrets{{Path: "secret/a"}, {Path: "secret/c"}}
			s2 := vault.Secrets{{Path: "secret/b"}, {Path: "secret/d"}}
			merged := s1.Merge(s2)
			Expect(len(merged)).To(Equal(4))
			Expect(merged[0].Path).To(Equal("secret/a"))
			Expect(merged[1].Path).To(Equal("secret/b"))
			Expect(merged[2].Path).To(Equal("secret/c"))
			Expect(merged[3].Path).To(Equal("secret/d"))
		})

		It("skips duplicates based on path", func() {
			s1 := vault.Secrets{{Path: "secret/a"}, {Path: "secret/b"}}
			s2 := vault.Secrets{{Path: "secret/a"}, {Path: "secret/c"}}
			merged := s1.Merge(s2)
			Expect(len(merged)).To(Equal(3))
			Expect(merged[0].Path).To(Equal("secret/a"))
			Expect(merged[1].Path).To(Equal("secret/b"))
			Expect(merged[2].Path).To(Equal("secret/c"))
		})

		It("returns a copy of s1 when s2 is empty", func() {
			s1 := vault.Secrets{{Path: "secret/a"}}
			s2 := vault.Secrets{}
			merged := s1.Merge(s2)
			Expect(len(merged)).To(Equal(1))
			Expect(merged[0].Path).To(Equal("secret/a"))
		})

		It("returns s2 contents when s1 is empty", func() {
			s1 := vault.Secrets{}
			s2 := vault.Secrets{{Path: "secret/b"}}
			merged := s1.Merge(s2)
			Expect(len(merged)).To(Equal(1))
			Expect(merged[0].Path).To(Equal("secret/b"))
		})
	})

	Describe("Secrets.Paths", func() {
		Context("when secrets have versions with keys", func() {
			It("returns paths with keys from the latest version", func() {
				sec := vault.NewSecret()
				sec.Set("pass", "secret", false)
				sec.Set("user", "admin", false)
				s := vault.Secrets{
					{Path: "secret/app", Versions: []vault.SecretVersion{
						{Data: sec, Number: 1, State: vault.SecretStateAlive},
					}},
				}
				paths := s.Paths()
				Expect(paths).To(HaveLen(2))
				Expect(paths[0]).To(Equal("secret/app:pass"))
				Expect(paths[1]).To(Equal("secret/app:user"))
			})
		})

		Context("when secrets have no versions", func() {
			It("returns just the path", func() {
				s := vault.Secrets{{Path: "secret/app"}}
				paths := s.Paths()
				Expect(paths).To(HaveLen(1))
				Expect(paths[0]).To(Equal("secret/app"))
			})
		})

		Context("when path contains special characters", func() {
			It("escapes colons and carets in paths and keys", func() {
				sec := vault.NewSecret()
				sec.Set("my:key", "val", false)
				s := vault.Secrets{
					{Path: "secret/a:b", Versions: []vault.SecretVersion{
						{Data: sec, Number: 1, State: vault.SecretStateAlive},
					}},
				}
				paths := s.Paths()
				Expect(paths[0]).To(Equal(`secret/a\:b:my\:key`))
			})
		})
	})

	Describe("SecretEntry.Basename", func() {
		It("returns the last path segment", func() {
			e := vault.SecretEntry{Path: "secret/foo/bar"}
			Expect(e.Basename()).To(Equal("bar"))
		})

		It("returns the only segment for a single-segment path", func() {
			e := vault.SecretEntry{Path: "mysecret"}
			Expect(e.Basename()).To(Equal("mysecret"))
		})

		It("returns empty string for an empty path", func() {
			e := vault.SecretEntry{Path: ""}
			Expect(e.Basename()).To(Equal(""))
		})
	})
})
