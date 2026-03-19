package cmd

type Options struct {
	Insecure     bool `cli:"-k, --insecure"`
	Version      bool `cli:"-v, --version"`
	Help         bool `cli:"-h, --help"`
	Clobber      bool `cli:"--clobber, --no-clobber"`
	SkipIfExists bool
	Quiet        bool `cli:"--quiet"`

	// Behavour of -T must chain through -- separated commands.  There is code
	// that relies on this.  Will default to $VAULT_MANAGER_TARGET if it exists, or
	// the current vault-manager target otherwise.
	UseTarget string `cli:"-T, --target" env:"VAULT_MANAGER_TARGET"`

	HelpCommand    struct{} `cli:"help"`
	VersionCommand struct{} `cli:"version"`

	Envvars struct{} `cli:"envvars"`
	Targets struct {
		JSON bool `cli:"--json"`
	} `cli:"targets"`

	Status struct {
		ErrorIfSealed bool `cli:"-e, --err-sealed"`
	} `cli:"status"`

	Unseal struct{} `cli:"unseal"`
	Seal   struct{} `cli:"seal"`
	Env    struct {
		ForBash bool `cli:"--bash"`
		ForFish bool `cli:"--fish"`
		ForJSON bool `cli:"--json"`
	} `cli:"env"`

	Auth struct {
		Path string `cli:"-p, --path"`
		JSON bool   `cli:"--json"`
	} `cli:"auth, login"`

	Logout struct{} `cli:"logout"`
	Renew  struct{} `cli:"renew"`
	Ask    struct{} `cli:"ask"`
	Set    struct{} `cli:"set, write"`
	Paste  struct{} `cli:"paste"`
	Exists struct{} `cli:"exists, check"`

	Local struct {
		As     string `cli:"--as"`
		File   string `cli:"-f, --file"`
		Memory bool   `cli:"-m, --memory"`
		Port   int    `cli:"-p, --port"`
	} `cli:"local"`

	Init struct {
		Single    bool `cli:"-s, --single"`
		NKeys     int  `cli:"--keys"`
		Threshold int  `cli:"--threshold"`
		JSON      bool `cli:"--json"`
		Sealed    bool `cli:"--sealed"`
		NoMount   bool `cli:"--no-mount"`
		Persist   bool `cli:"--persist, --no-persist"`
	} `cli:"init"`

	Rekey struct {
		NKeys     int      `cli:"--keys, --num-unseal-keys"`
		Threshold int      `cli:"--threshold, --keys-to-unseal"`
		GPG       []string `cli:"--gpg"`
		Persist   bool     `cli:"--persist, --no-persist"`
	} `cli:"rekey"`

	Get struct {
		KeysOnly bool `cli:"--keys"`
		Yaml     bool `cli:"--yaml"`
	} `cli:"get, read, cat"`

	Versions struct{} `cli:"versions,revisions"`

	List struct {
		Single bool `cli:"-1"`
		Quick  bool `cli:"-q, --quick"`
	} `cli:"ls"`

	Paths struct {
		ShowKeys bool `cli:"--keys"`
		Quick    bool `cli:"-q, --quick"`
	} `cli:"paths"`

	Tree struct {
		ShowKeys   bool `cli:"--keys"`
		HideLeaves bool `cli:"-d, --hide-leaves"`
		Quick      bool `cli:"-q, --quick"`
	} `cli:"tree"`

	Target struct {
		JSON        bool     `cli:"--json"`
		Interactive bool     `cli:"-i, --interactive"`
		Strongbox   bool     `cli:"-s, --strongbox, --no-strongbox"`
		CACerts     []string `cli:"--ca-cert"`
		Namespace   string   `cli:"-n, --namespace"`

		Delete struct{} `cli:"delete, rm"`
	} `cli:"target"`

	Delete struct {
		Recurse bool `cli:"-R, -r, --recurse"`
		Force   bool `cli:"-f, --force"`
		Destroy bool `cli:"-D, -d, --destroy"`
		All     bool `cli:"-a, --all"`
	} `cli:"delete, rm"`

	Undelete struct {
		All bool `cli:"-a, --all"`
	} `cli:"undelete, unrm, urm"`

	Revert struct {
		Deleted bool `cli:"-d, --deleted"`
	} `cli:"revert"`

	Export struct {
		All     bool `cli:"-a, --all"`
		Deleted bool `cli:"-d, --deleted"`
		//These do nothing but are kept for backwards-compat
		OnlyAlive bool `cli:"-o, --only-alive"`
		Shallow   bool `cli:"-s, --shallow"`
	} `cli:"export"`

	Import struct {
		IgnoreDestroyed bool `cli:"-I, --ignore-destroyed"`
		IgnoreDeleted   bool `cli:"-i, --ignore-deleted"`
		Shallow         bool `cli:"-s, --shallow"`
	} `cli:"import"`

	Move struct {
		Recurse bool `cli:"-R, -r, --recurse"`
		Force   bool `cli:"-f, --force"`
		Deep    bool `cli:"-d, --deep"`
	} `cli:"move, rename, mv"`

	Copy struct {
		Recurse bool `cli:"-R, -r, --recurse"`
		Force   bool `cli:"-f, --force"`
		Deep    bool `cli:"-d, --deep"`
	} `cli:"copy, cp"`

	Gen struct {
		Policy string `cli:"-p, --policy"`
		Length int    `cli:"-l, --length"`
	} `cli:"gen, auto, generate"`

	SSH     struct{} `cli:"ssh"`
	RSA     struct{} `cli:"rsa"`
	DHParam struct{} `cli:"dhparam, dhparams, dh"`
	Prompt  struct{} `cli:"prompt"`
	Vault   struct{} `cli:"vault!"`
	Fmt     struct{} `cli:"fmt"`

	Curl struct {
		DataOnly bool `cli:"--data-only"`
	} `cli:"curl"`

	UUID   struct{} `cli:"uuid"`
	Option struct{} `cli:"option"`

	Sync struct {
		Pull  struct{} `cli:"pull"`
		Plan  struct{} `cli:"plan"`
		Apply struct{} `cli:"apply"`
	} `cli:"sync"`

	X509 struct {
		Validate struct {
			CA         bool     `cli:"-A, --ca"`
			SignedBy   string   `cli:"-i, --signed-by"`
			NotRevoked bool     `cli:"-R, --not-revoked"`
			Revoked    bool     `cli:"-r, --revoked"`
			NotExpired bool     `cli:"-E, --not-expired"`
			Expired    bool     `cli:"-e, --expired"`
			Name       []string `cli:"-n, --for"`
			Bits       []int    `cli:"-b, --bits"`
		} `cli:"validate, check"`

		Issue struct {
			CA           bool     `cli:"-A, --ca"`
			Subject      string   `cli:"-s, --subj, --subject"`
			Bits         int      `cli:"-b, --bits"`
			SignedBy     string   `cli:"-i, --signed-by"`
			Name         []string `cli:"-n, --name"`
			TTL          string   `cli:"-t, --ttl"`
			KeyUsage     []string `cli:"-u, --key-usage"`
			SigAlgorithm string   `cli:"-l, --sig-algorithm"`
		} `cli:"issue"`

		Revoke struct {
			SignedBy string `cli:"-i, --signed-by"`
		} `cli:"revoke"`

		Renew struct {
			Subject      string   `cli:"-s, --subj, --subject"`
			Name         []string `cli:"-n, --name"`
			SignedBy     string   `cli:"-i, --signed-by"`
			TTL          string   `cli:"-t, --ttl"`
			KeyUsage     []string `cli:"-u, --key-usage"`
			SigAlgorithm string   `cli:"-l, --sig-algorithm"`
		} `cli:"renew"`

		Reissue struct {
			Subject      string   `cli:"-s, --subj, --subject"`
			Name         []string `cli:"-n, --name"`
			Bits         int      `cli:"-b, --bits"`
			SignedBy     string   `cli:"-i, --signed-by"`
			TTL          string   `cli:"-t, --ttl"`
			KeyUsage     []string `cli:"-u, --key-usage"`
			SigAlgorithm string   `cli:"-l, --sig-algorithm"`
		} `cli:"reissue"`

		Show struct {
		} `cli:"show"`

		CRL struct {
			Renew bool `cli:"--renew"`
		} `cli:"crl"`
	} `cli:"x509"`
}

func NewOptions() *Options {
	opt := &Options{}
	opt.Gen.Policy = "a-zA-Z0-9"
	opt.Clobber = true
	opt.X509.Issue.Bits = 4096
	opt.Init.Persist = true
	opt.Rekey.Persist = true
	opt.Target.Strongbox = true
	return opt
}
