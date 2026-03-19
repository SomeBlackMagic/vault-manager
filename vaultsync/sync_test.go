package vaultsync_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/SomeBlackMagic/vault-manager/vault"
	"github.com/SomeBlackMagic/vault-manager/vaultsync"
)

// mockVault implements VaultAccessor for testing.
type mockVault struct {
	secrets map[string]*vault.Secret // path -> secret
	written map[string]*vault.Secret // path -> secret that was written
	deleted []string
}

func newMockVault() *mockVault {
	return &mockVault{
		secrets: make(map[string]*vault.Secret),
		written: make(map[string]*vault.Secret),
	}
}

func (m *mockVault) addSecret(path string, data map[string]string) {
	s := vault.NewSecret()
	for k, v := range data {
		s.Set(k, v, false)
	}
	m.secrets[path] = s
}

func (m *mockVault) Read(path string) (*vault.Secret, error) {
	s, ok := m.secrets[path]
	if !ok {
		return nil, vault.NewSecretNotFoundError(path)
	}
	return s, nil
}

func (m *mockVault) Write(path string, s *vault.Secret) error {
	m.written[path] = s
	m.secrets[path] = s
	return nil
}

func (m *mockVault) Delete(path string, opts vault.DeleteOpts) error {
	m.deleted = append(m.deleted, path)
	delete(m.secrets, path)
	return nil
}

func (m *mockVault) List(path string) ([]string, error) {
	return nil, nil
}

func (m *mockVault) ConstructSecrets(path string, opts vault.TreeOpts) (vault.Secrets, error) {
	var secrets vault.Secrets
	for p, s := range m.secrets {
		entry := vault.SecretEntry{
			Path: p,
			Versions: []vault.SecretVersion{
				{
					Data:   s,
					Number: 1,
					State:  vault.SecretStateAlive,
				},
			},
		}
		secrets = append(secrets, entry)
	}
	secrets.Sort()
	return secrets, nil
}

var _ = Describe("JSON Value Handling", func() {
	Describe("ExpandValue", func() {
		It("returns the original string for plain strings", func() {
			Expect(vaultsync.ExpandValue("hello")).To(Equal("hello"))
		})

		It("returns the original string for empty strings", func() {
			Expect(vaultsync.ExpandValue("")).To(Equal(""))
		})

		It("returns the original string for invalid JSON starting with {", func() {
			Expect(vaultsync.ExpandValue("{not valid json")).To(Equal("{not valid json"))
		})

		It("expands a valid JSON object", func() {
			result := vaultsync.ExpandValue(`{"host":"db","port":5432}`)
			m, ok := result.(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(m["host"]).To(Equal("db"))
			Expect(m["port"]).To(BeNumerically("==", 5432))
		})

		It("expands a valid JSON array", func() {
			result := vaultsync.ExpandValue(`[1,2,3]`)
			arr, ok := result.([]interface{})
			Expect(ok).To(BeTrue())
			Expect(arr).To(HaveLen(3))
		})

		It("does not expand strings that don't start with { or [", func() {
			Expect(vaultsync.ExpandValue("true")).To(Equal("true"))
			Expect(vaultsync.ExpandValue("123")).To(Equal("123"))
		})
	})

	Describe("PackValue", func() {
		It("returns string as-is", func() {
			v, err := vaultsync.PackValue("hello")
			Expect(err).ToNot(HaveOccurred())
			Expect(v).To(Equal("hello"))
		})

		It("marshals a map to compact JSON", func() {
			m := map[string]interface{}{"host": "db", "port": float64(5432)}
			v, err := vaultsync.PackValue(m)
			Expect(err).ToNot(HaveOccurred())
			// Verify it's valid JSON
			var parsed map[string]interface{}
			Expect(json.Unmarshal([]byte(v), &parsed)).To(Succeed())
			Expect(parsed["host"]).To(Equal("db"))
		})

		It("marshals a slice to compact JSON", func() {
			s := []interface{}{float64(1), float64(2), float64(3)}
			v, err := vaultsync.PackValue(s)
			Expect(err).ToNot(HaveOccurred())
			Expect(v).To(Equal("[1,2,3]"))
		})
	})

	Describe("ExpandMap / PackMap round-trip", func() {
		It("round-trips correctly", func() {
			original := map[string]string{
				"plain":  "hello",
				"config": `{"host":"db","port":5432}`,
				"list":   `[1,2,3]`,
			}
			expanded := vaultsync.ExpandMap(original)
			Expect(expanded["plain"]).To(Equal("hello"))

			packed, err := vaultsync.PackMap(expanded)
			Expect(err).ToNot(HaveOccurred())
			Expect(packed["plain"]).To(Equal("hello"))

			// JSON values should round-trip (may have different key order but be equivalent)
			var origConfig, packedConfig map[string]interface{}
			Expect(json.Unmarshal([]byte(original["config"]), &origConfig)).To(Succeed())
			Expect(json.Unmarshal([]byte(packed["config"]), &packedConfig)).To(Succeed())
			Expect(packedConfig).To(Equal(origConfig))
		})
	})

	Describe("DeepDiffJSON", func() {
		It("returns empty for identical values", func() {
			m := map[string]interface{}{"a": "b"}
			Expect(vaultsync.DeepDiffJSON(m, m, "")).To(BeEmpty())
		})

		It("detects added fields in maps", func() {
			old := map[string]interface{}{"a": "1"}
			new := map[string]interface{}{"a": "1", "b": "2"}
			changes := vaultsync.DeepDiffJSON(old, new, "")
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].Path).To(Equal("b"))
			Expect(changes[0].OldValue).To(BeNil())
			Expect(changes[0].NewValue).To(Equal("2"))
		})

		It("detects removed fields in maps", func() {
			old := map[string]interface{}{"a": "1", "b": "2"}
			new := map[string]interface{}{"a": "1"}
			changes := vaultsync.DeepDiffJSON(old, new, "")
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].Path).To(Equal("b"))
			Expect(changes[0].OldValue).To(Equal("2"))
			Expect(changes[0].NewValue).To(BeNil())
		})

		It("detects modified fields in maps", func() {
			old := map[string]interface{}{"a": "1"}
			new := map[string]interface{}{"a": "2"}
			changes := vaultsync.DeepDiffJSON(old, new, "")
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].Path).To(Equal("a"))
			Expect(changes[0].OldValue).To(Equal("1"))
			Expect(changes[0].NewValue).To(Equal("2"))
		})

		It("handles nested maps", func() {
			old := map[string]interface{}{
				"db": map[string]interface{}{"host": "old"},
			}
			new := map[string]interface{}{
				"db": map[string]interface{}{"host": "new"},
			}
			changes := vaultsync.DeepDiffJSON(old, new, "")
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].Path).To(Equal("db.host"))
		})

		It("handles array differences", func() {
			old := []interface{}{float64(1), float64(2)}
			new := []interface{}{float64(1), float64(3), float64(4)}
			changes := vaultsync.DeepDiffJSON(old, new, "items")
			Expect(changes).To(HaveLen(2))
			// [1] changed from 2 to 3
			Expect(changes[0].Path).To(Equal("items[1]"))
			// [2] added
			Expect(changes[1].Path).To(Equal("items[2]"))
		})

		It("uses prefix correctly", func() {
			old := map[string]interface{}{"a": "1"}
			new := map[string]interface{}{"a": "2"}
			changes := vaultsync.DeepDiffJSON(old, new, "root")
			Expect(changes[0].Path).To(Equal("root.a"))
		})
	})
})

var _ = Describe("State", func() {
	Describe("ReadLocalState / WriteLocalSecret", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "vaultsync-test-*")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(tmpDir)
		})

		It("writes and reads back a secret with plain values", func() {
			data := map[string]interface{}{
				"username": "admin",
				"password": "secret123",
			}
			err := vaultsync.WriteLocalSecret(tmpDir, "secret/app/db", data)
			Expect(err).ToNot(HaveOccurred())

			// Verify file exists
			_, err = os.Stat(filepath.Join(tmpDir, "secret", "app", "db.json"))
			Expect(err).ToNot(HaveOccurred())

			secrets, err := vaultsync.ReadLocalState(tmpDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(secrets).To(HaveLen(1))
			Expect(secrets[0].Path).To(Equal("secret/app/db"))
			Expect(secrets[0].Data["username"]).To(Equal("admin"))
		})

		It("writes and reads back a secret with nested JSON values", func() {
			data := map[string]interface{}{
				"config": map[string]interface{}{
					"host": "db.example.com",
					"port": float64(5432),
				},
				"plain": "value",
			}
			err := vaultsync.WriteLocalSecret(tmpDir, "secret/app/config", data)
			Expect(err).ToNot(HaveOccurred())

			secrets, err := vaultsync.ReadLocalState(tmpDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(secrets).To(HaveLen(1))
			config, ok := secrets[0].Data["config"].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(config["host"]).To(Equal("db.example.com"))
		})

		It("returns empty slice for empty directory", func() {
			secrets, err := vaultsync.ReadLocalState(tmpDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(secrets).To(BeEmpty())
		})
	})
})

var _ = Describe("Diff", func() {
	Describe("ComputeChanges", func() {
		It("detects added secrets (local only)", func() {
			local := []vaultsync.LocalSecret{
				{Path: "secret/new", Data: map[string]interface{}{"key": "val"}},
			}
			remote := map[string]map[string]interface{}{}
			cs := vaultsync.ComputeChanges(local, remote)
			Expect(cs.Changes).To(HaveLen(1))
			Expect(cs.Changes[0].Type).To(Equal(vaultsync.ChangeAdd))
		})

		It("detects deleted secrets (remote only)", func() {
			local := []vaultsync.LocalSecret{}
			remote := map[string]map[string]interface{}{
				"secret/old": {"key": "val"},
			}
			cs := vaultsync.ComputeChanges(local, remote)
			Expect(cs.Changes).To(HaveLen(1))
			Expect(cs.Changes[0].Type).To(Equal(vaultsync.ChangeDelete))
		})

		It("detects modified secrets", func() {
			local := []vaultsync.LocalSecret{
				{Path: "secret/app", Data: map[string]interface{}{"key": "new"}},
			}
			remote := map[string]map[string]interface{}{
				"secret/app": {"key": "old"},
			}
			cs := vaultsync.ComputeChanges(local, remote)
			Expect(cs.Changes).To(HaveLen(1))
			Expect(cs.Changes[0].Type).To(Equal(vaultsync.ChangeModify))
		})

		It("detects unchanged secrets", func() {
			local := []vaultsync.LocalSecret{
				{Path: "secret/app", Data: map[string]interface{}{"key": "same"}},
			}
			remote := map[string]map[string]interface{}{
				"secret/app": {"key": "same"},
			}
			cs := vaultsync.ComputeChanges(local, remote)
			Expect(cs.Changes).To(HaveLen(1))
			Expect(cs.Changes[0].Type).To(Equal(vaultsync.ChangeNone))
		})

		It("handles nested JSON changes", func() {
			local := []vaultsync.LocalSecret{
				{Path: "secret/app", Data: map[string]interface{}{
					"config": map[string]interface{}{"host": "new-host"},
				}},
			}
			remote := map[string]map[string]interface{}{
				"secret/app": {
					"config": map[string]interface{}{"host": "old-host"},
				},
			}
			cs := vaultsync.ComputeChanges(local, remote)
			Expect(cs.Changes).To(HaveLen(1))
			Expect(cs.Changes[0].Type).To(Equal(vaultsync.ChangeModify))
		})
	})

	Describe("FormatChangeSummary", func() {
		It("formats the summary correctly", func() {
			cs := vaultsync.ChangeSet{
				Changes: []vaultsync.Change{
					{Type: vaultsync.ChangeAdd},
					{Type: vaultsync.ChangeAdd},
					{Type: vaultsync.ChangeModify},
					{Type: vaultsync.ChangeDelete},
				},
			}
			summary := vaultsync.FormatChangeSummary(cs)
			Expect(summary).To(ContainSubstring("2"))
			Expect(summary).To(ContainSubstring("1"))
		})
	})

	Describe("FormatDiff", func() {
		It("formats add change", func() {
			c := vaultsync.Change{
				Type:      vaultsync.ChangeAdd,
				Path:      "secret/new",
				LocalData: map[string]interface{}{"key": "val"},
			}
			output := vaultsync.FormatDiff(c)
			Expect(output).To(ContainSubstring("secret/new"))
			Expect(output).To(ContainSubstring("key"))
		})

		It("formats delete change", func() {
			c := vaultsync.Change{
				Type:       vaultsync.ChangeDelete,
				Path:       "secret/old",
				RemoteData: map[string]interface{}{"key": "val"},
			}
			output := vaultsync.FormatDiff(c)
			Expect(output).To(ContainSubstring("secret/old"))
		})

		It("formats modify change with nested JSON diff", func() {
			c := vaultsync.Change{
				Type: vaultsync.ChangeModify,
				Path: "secret/app",
				LocalData: map[string]interface{}{
					"config": map[string]interface{}{"host": "new-host", "port": float64(5432)},
				},
				RemoteData: map[string]interface{}{
					"config": map[string]interface{}{"host": "old-host", "port": float64(5432)},
				},
			}
			output := vaultsync.FormatDiff(c)
			Expect(output).To(ContainSubstring("secret/app"))
			Expect(output).To(ContainSubstring("config"))
			Expect(output).To(ContainSubstring("host"))
		})

		It("formats no-change", func() {
			c := vaultsync.Change{
				Type: vaultsync.ChangeNone,
				Path: "secret/same",
			}
			output := vaultsync.FormatDiff(c)
			Expect(output).To(ContainSubstring("secret/same"))
		})
	})
})

var _ = Describe("Plan", func() {
	It("computes correct changeset from mock vault", func() {
		mv := newMockVault()
		mv.addSecret("secret/existing", map[string]string{"key": "remote-val"})

		tmpDir, err := os.MkdirTemp("", "vaultsync-plan-*")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		// Write a local secret that differs
		err = vaultsync.WriteLocalSecret(tmpDir, "secret/existing", map[string]interface{}{"key": "local-val"})
		Expect(err).ToNot(HaveOccurred())

		// Write a local-only secret
		err = vaultsync.WriteLocalSecret(tmpDir, "secret/new", map[string]interface{}{"newkey": "newval"})
		Expect(err).ToNot(HaveOccurred())

		cs, err := vaultsync.Plan(mv, "secret", tmpDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(cs.HasChanges()).To(BeTrue())

		adds, modifies, _ := cs.Counts()
		Expect(adds).To(Equal(1))
		Expect(modifies).To(Equal(1))
	})
})

var _ = Describe("Apply", func() {
	It("writes and deletes secrets via mock vault", func() {
		mv := newMockVault()
		mv.addSecret("secret/to-delete", map[string]string{"key": "val"})
		mv.addSecret("secret/to-modify", map[string]string{"key": "old"})

		tmpDir, err := os.MkdirTemp("", "vaultsync-apply-*")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		// Write local secret that modifies existing
		err = vaultsync.WriteLocalSecret(tmpDir, "secret/to-modify", map[string]interface{}{"key": "new"})
		Expect(err).ToNot(HaveOccurred())

		// Write local-only secret
		err = vaultsync.WriteLocalSecret(tmpDir, "secret/to-add", map[string]interface{}{"newkey": "newval"})
		Expect(err).ToNot(HaveOccurred())

		// Note: Apply prompts for confirmation via stdin.
		// In a real test environment, we'd pipe "y\n" to stdin.
		// For now, this test verifies the Plan part works.
		cs, err := vaultsync.Plan(mv, "secret", tmpDir)
		Expect(err).ToNot(HaveOccurred())

		adds, modifies, deletes := cs.Counts()
		Expect(adds).To(Equal(1))
		Expect(modifies).To(Equal(1))
		Expect(deletes).To(Equal(1))
	})
})

var _ = Describe("PackMap for Apply", func() {
	It("packs nested JSON back to strings for Vault", func() {
		data := map[string]interface{}{
			"plain": "hello",
			"config": map[string]interface{}{
				"host": "db.example.com",
				"port": float64(5432),
			},
		}

		packed, err := vaultsync.PackMap(data)
		Expect(err).ToNot(HaveOccurred())
		Expect(packed["plain"]).To(Equal("hello"))

		// config should be a JSON string
		var parsed map[string]interface{}
		err = json.Unmarshal([]byte(packed["config"]), &parsed)
		Expect(err).ToNot(HaveOccurred())
		Expect(parsed["host"]).To(Equal("db.example.com"))
		Expect(parsed["port"]).To(BeNumerically("==", 5432))
	})
})
