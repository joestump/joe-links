// Governing: SPEC-0001 REQ "CLI Entrypoint", ADR-0004
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/config"
	"github.com/joestump/joe-links/internal/db"
	"github.com/joestump/joe-links/internal/handler"
	"github.com/joestump/joe-links/internal/store"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			database, err := db.New(cfg.DB.Driver, cfg.DB.DSN)
			if err != nil {
				return err
			}
			defer database.Close()

			if err := db.Migrate(database, cfg.DB.Driver); err != nil {
				return err
			}

			sessionManager := auth.NewSessionManager(database, cfg.DB.Driver, cfg.SessionLifetime)

			ctx := context.Background()
			oidcProvider, err := auth.NewProvider(ctx, cfg)
			if err != nil {
				return err
			}

			userStore := store.NewUserStore(database)
			linkStore := store.NewLinkStore(database)

			authHandlers := auth.NewHandlers(oidcProvider, sessionManager, userStore, cfg.AdminEmail)
			authMiddleware := auth.NewMiddleware(sessionManager, userStore)

			router := handler.NewRouter(handler.Deps{
				SessionManager: sessionManager,
				AuthHandlers:   authHandlers,
				AuthMiddleware: authMiddleware,
				LinkStore:      linkStore,
			})

			log.Printf("listening on %s", cfg.HTTP.Addr)
			return http.ListenAndServe(cfg.HTTP.Addr, router)
		},
	}
}
