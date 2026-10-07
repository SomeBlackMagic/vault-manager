package app

import (
	"crypto/x509"
	"log/slog"
	"os"

	fmt "github.com/jhunt/go-ansi"
	"github.com/SomeBlackMagic/vault-manager/vault"
)

// Connect builds a Vault client for the current target. logger receives
// diagnostic messages and may be nil.
func Connect(auth bool, logger *slog.Logger) *vault.Vault {
	var caCertPool *x509.CertPool
	if os.Getenv("VAULT_CACERT") != "" {
		contents, err := os.ReadFile(os.Getenv("VAULT_CACERT"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "@R{!! Could not read CA certificates: %s}", err.Error())
		}

		caCertPool = x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(contents)
	}

	shouldSkipVerify := func() bool {
		skipVerifyVal := os.Getenv("VAULT_SKIP_VERIFY")
		if skipVerifyVal != "" && skipVerifyVal != "false" {
			return true
		}
		return false
	}

	conf := vault.VaultConfig{
		URL:        getVaultURL(),
		Token:      os.Getenv("VAULT_TOKEN"),
		Namespace:  os.Getenv("VAULT_NAMESPACE"),
		SkipVerify: shouldSkipVerify(),
		CACerts:    caCertPool,
		Logger:     logger,
	}

	if auth && conf.Token == "" {
		fmt.Fprintf(os.Stderr, "@R{You are not authenticated to a Vault.}\n")
		fmt.Fprintf(os.Stderr, "Try @C{safe auth ldap}\n")
		fmt.Fprintf(os.Stderr, " or @C{safe auth github}\n")
		fmt.Fprintf(os.Stderr, " or @C{safe auth okta}\n")
		fmt.Fprintf(os.Stderr, " or @C{safe auth token}\n")
		fmt.Fprintf(os.Stderr, " or @C{safe auth userpass}\n")
		fmt.Fprintf(os.Stderr, " or @C{safe auth approle}\n")
		os.Exit(1)
	}

	v, err := vault.NewVault(conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "@R{!! %s}\n", err)
		os.Exit(1)
	}
	return v
}

// getVaultURL exits program with error if no Vault targeted
func getVaultURL() string {
	ret := os.Getenv("VAULT_ADDR")
	if ret == "" {
		fmt.Fprintf(os.Stderr, "@R{You are not targeting a Vault.}\n")
		fmt.Fprintf(os.Stderr, "Try @C{safe target https://your-vault alias}\n")
		fmt.Fprintf(os.Stderr, " or @C{safe target alias}\n")
		os.Exit(1)
	}
	return ret
}
