package cmd

import "github.com/SomeBlackMagic/vault-manager/app"

// RegisterAll registers all CLI commands with the runner.
func RegisterAll(r *app.Runner, opt *Options, version string, revision string) {
	registerHelpCommands(r, opt, version, revision)
	registerTargetCommands(r, opt)
	registerAuthCommands(r, opt)
	registerSecretCommands(r, opt)
	registerTreeCommands(r, opt)
	registerMigrationCommands(r, opt)
	registerGenerateCommands(r, opt)
	registerUtilsCommands(r, opt)
	registerX509Commands(r, opt)
	registerAdminCommands(r, opt)
	registerSyncCommands(r, opt)
}
