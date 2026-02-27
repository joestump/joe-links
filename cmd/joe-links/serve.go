// Governing: SPEC-0001 REQ "CLI Entrypoint", "Go HTTP Server", ADR-0004
// Governing: SPEC-0016 REQ "Click Recording", ADR-0016
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
			defer func() { _ = database.Close() }()

			if err := db.Migrate(database, cfg.DB.Driver); err != nil {
				return err
			}

			sessionManager := auth.NewSessionManager(database, cfg.DB.Driver, cfg.SessionLifetime, !cfg.InsecureCookies)

			ctx := context.Background()
			oidcProvider, err := auth.NewProvider(ctx, cfg)
			if err != nil {
				return err
			}

			userStore := store.NewUserStore(database)
			ownershipStore := store.NewOwnershipStore(database)
			tagStore := store.NewTagStore(database)
			linkStore := store.NewLinkStore(database, ownershipStore, tagStore)
			tokenStore := auth.NewSQLTokenStore(database)
			keywordStore := store.NewKeywordStore(database)

			// Governing: SPEC-0016 REQ "Click Recording", ADR-0016
			clickCh := make(chan store.ClickEvent, 256)
			clickStore := store.NewClickStore(database)
			go runClickWriter(ctx, clickCh, clickStore)

			authHandlers := auth.NewHandlers(oidcProvider, sessionManager, userStore, cfg.AdminEmail, cfg.AdminGroups, cfg.GroupsClaim, !cfg.InsecureCookies)
			authMiddleware := auth.NewMiddleware(sessionManager, userStore)

			router := handler.NewRouter(handler.Deps{
				SessionManager: sessionManager,
				AuthHandlers:   authHandlers,
				AuthMiddleware: authMiddleware,
				LinkStore:      linkStore,
				OwnershipStore: ownershipStore,
				TagStore:       tagStore,
				UserStore:      userStore,
				TokenStore:     tokenStore,
				KeywordStore:   keywordStore,
				ClickStore:     clickStore,
				ClickCh:        clickCh,
				ShortKeyword:   cfg.ShortKeyword,
			})

			log.Printf("listening on %s", cfg.HTTP.Addr)
			return http.ListenAndServe(cfg.HTTP.Addr, router)
		},
	}
}

// runClickWriter reads click events from the channel and persists them.
// On context cancellation it drains remaining events before returning.
// Governing: SPEC-0016 REQ "Click Recording", ADR-0016
func runClickWriter(ctx context.Context, ch <-chan store.ClickEvent, cs *store.ClickStore) {
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return
			}
			if err := cs.RecordClick(ctx, e); err != nil {
				log.Printf("click write error: %v", err)
			}
		case <-ctx.Done():
			// Drain remaining events.
			for {
				select {
				case e, ok := <-ch:
					if !ok {
						return
					}
					if err := cs.RecordClick(context.Background(), e); err != nil {
						log.Printf("click drain error: %v", err)
					}
				default:
					return
				}
			}
		}
	}
}
