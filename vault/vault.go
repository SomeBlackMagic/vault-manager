package vault

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"github.com/cloudfoundry-community/vaultkv"

	"github.com/SomeBlackMagic/vault-manager/logging"
)

type Vault struct {
	client *vaultkv.KV
	log    *slog.Logger
}

type VaultConfig struct {
	URL        string
	Token      string
	Namespace  string
	CACerts    *x509.CertPool
	SkipVerify bool
	// Logger receives diagnostic messages; nil disables logging.
	Logger *slog.Logger
}

// NewVault creates a new Vault object.  If an empty token is specified,
// the current user's token is read from ~/.vault-token.
func NewVault(conf VaultConfig) (*Vault, error) {
	var err error
	if conf.CACerts == nil {
		// x509.SystemCertPool is not implemented for windows currently.
		// If nil is supplied for RootCAs, the system will verify the certs as per
		// https://golang.org/src/crypto/x509/verify.go (Line 741)
		conf.CACerts, err = x509.SystemCertPool()
		if err != nil && runtime.GOOS != "windows" {
			return nil, fmt.Errorf("unable to retrieve system root certificate authorities: %w", err)
		}
	}
	vaultURL, err := url.Parse(strings.TrimSuffix(conf.URL, "/"))
	if err != nil {
		return nil, fmt.Errorf("could not parse Vault URL: %w", err)
	}

	//The default port for Vault is typically 8200 (which is the VaultKV default),
	// but safe has historically ignored that and used the default http or https
	// port, depending on which was specified as the scheme
	if vaultURL.Port() == "" {
		port := ":80"
		if strings.ToLower(vaultURL.Scheme) == "https" {
			port = ":443"
		}
		vaultURL.Host = vaultURL.Host + port
	}

	proxyRouter, err := NewProxyRouter()
	if err != nil {
		return nil, fmt.Errorf("Error setting up proxy: %w", err)
	}

	log := logging.OrDiscard(conf.Logger)
	log.Debug("vault client configured",
		"url", RedactURL(vaultURL),
		"namespace", conf.Namespace,
		"skip_verify", conf.SkipVerify)

	return &Vault{
		client: (&vaultkv.Client{
			VaultURL:  vaultURL,
			AuthToken: conf.Token,
			Namespace: conf.Namespace,
			Client: &http.Client{
				Transport: newTracingTransport(&http.Transport{
					Proxy: proxyRouter.Proxy,
					TLSClientConfig: &tls.Config{
						RootCAs:            conf.CACerts,
						InsecureSkipVerify: conf.SkipVerify,
					},
					MaxIdleConnsPerHost: 100,
				}, log),
			},
		}).NewKV(),
		log: log,
	}, nil
}

// Log returns the logger used by this Vault client, never nil.
func (v *Vault) Log() *slog.Logger {
	return logging.OrDiscard(v.log)
}

func (v *Vault) Client() *vaultkv.KV {
	return v.client
}

func (v *Vault) MountVersion(path string) (uint, error) {
	path = Canonicalize(path)
	return v.client.MountVersion(path)
}

func (v *Vault) Versions(path string) ([]vaultkv.KVVersion, error) {
	path = Canonicalize(path)
	ret, err := v.client.Versions(path)
	if vaultkv.IsNotFound(err) {
		return nil, NewSecretNotFoundError(path)
	}

	return ret, err
}

func (v *Vault) Curl(method string, path string, body []byte) (*http.Response, error) {
	path = Canonicalize(path)
	u, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("Could not parse input path: %w", err)
	}

	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("could not parse query: %w", err)
	}

	return v.client.Client.Curl(method, u.Path, query, bytes.NewBuffer(body))
}

// errIfFolder returns an error with your provided message if the given path is a folder.
// Can also throw an error if contacting the backend failed, in which case that error
// is returned.
func (v *Vault) errIfFolder(path, message string, args ...interface{}) error {
	path = Canonicalize(path)
	if _, err := v.List(path); err == nil {
		//We don't want the folder error to be ignored because of the -f flag to rm,
		// so we explicitly don't make this a secretNotFound error
		return fmt.Errorf(message, args...)
	} else if err != nil && !IsNotFound(err) {
		return err
	}
	return nil
}

type mountpoint struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Config      map[string]interface{} `json:"config"`
}

func convertMountpoint(o interface{}) (mountpoint, bool) {
	mount := mountpoint{}
	if m, ok := o.(map[string]interface{}); ok {
		if t, ok := m["type"].(string); ok {
			mount.Type = t
		} else {
			return mount, false
		}
		if d, ok := m["description"].(string); ok {
			mount.Description = d
		} else {
			return mount, false
		}
		if c, ok := m["config"].(map[string]interface{}); ok {
			mount.Config = c
		} else {
			return mount, false
		}
		return mount, true
	}
	return mount, false
}

func (v *Vault) Mounts(typ string) ([]string, error) {
	mounts, err := v.client.Client.ListMounts()
	if err != nil {
		return nil, err
	}

	ret := []string{}

	for name, mountInfo := range mounts {
		if mountInfo.Type == typ {
			ret = append(ret, strings.TrimSuffix(name, "/")+"/")
		}
	}

	return ret, nil
}

func (v *Vault) IsMounted(typ, path string) (bool, error) {
	mounts, err := v.Mounts(typ)
	if err != nil {
		return false, err
	}

	for _, at := range mounts {
		if at == path || at == path+"/" {
			return true, nil
		}
	}
	return false, nil
}

func (v *Vault) Mount(typ, path string, params map[string]interface{}) error {
	mounted, err := v.IsMounted(typ, path)
	if err != nil {
		return err
	}

	if !mounted {
		p := mountpoint{
			Type:        typ,
			Description: "(managed by safe)",
			Config:      params,
		}
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}

		res, err := v.Curl("POST", fmt.Sprintf("sys/mounts/%s", path), data)
		if err != nil {
			return err
		}

		if res.StatusCode != 204 {
			body, err := io.ReadAll(res.Body)
			if err != nil {
				return err
			}
			return DecodeErrorResponse(body)
		}

	} else {
		data, err := json.Marshal(params)
		if err != nil {
			return err
		}

		res, err := v.Curl("POST", fmt.Sprintf("sys/mounts/%s/tune", path), data)
		if err != nil {
			return err
		}

		if res.StatusCode != 204 {
			body, err := io.ReadAll(res.Body)
			if err != nil {
				return err
			}
			return DecodeErrorResponse(body)
		}
	}

	return nil
}

func DecodeErrorResponse(body []byte) error {
	var raw map[string]interface{}

	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("Received non-200 with non-JSON payload:\n%s\n", body)
	}

	if rawErrors, ok := raw["errors"]; ok {
		var errors []string
		if elems, ok := rawErrors.([]interface{}); ok {
			for _, elem := range elems {
				if err, ok := elem.(string); ok {
					errors = append(errors, err)
				}
			}
			return fmt.Errorf("%s", strings.Join(errors, "\n"))
		} else {
			return fmt.Errorf("Received unexpected format of Vault error messages:\n%v\n", errors)
		}
	} else {
		return fmt.Errorf("Received non-200 with no error messagess:\n%v\n", raw)
	}
}

func (v *Vault) SetURL(u string) error {
	vaultURL, err := url.Parse(strings.TrimSuffix(u, "/"))
	if err != nil {
		return fmt.Errorf("could not parse Vault URL: %w", err)
	}

	//The default port for Vault is typically 8200 (which is the VaultKV default),
	// but safe has historically ignored that and used the default http or https
	// port, depending on which was specified as the scheme
	if vaultURL.Port() == "" {
		port := ":80"
		if strings.ToLower(vaultURL.Scheme) == "https" {
			port = ":443"
		}
		vaultURL.Host = vaultURL.Host + port
	}
	v.client.Client.VaultURL = vaultURL
	return nil
}
