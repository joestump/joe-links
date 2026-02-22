// Governing: SPEC-0001 REQ "CLI Entrypoint", ADR-0004
package main

import (
	"log"

	"github.com/joestump/joe-links/internal/config"
	"github.com/joestump/joe-links/internal/db"
	"github.com/spf13/cobra"
)

func newMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			database, err := db.New(cfg.DB.Driver, cfg.DB.DSN)
			if err != nil {
				return err
			}
			defer func() { _ = database.Close() }()

			if err := db.Migrate(database, cfg.DB.Driver); err != nil {
				return err
			}

			log.Println("migrations complete")
			return nil
		},
	}
}
