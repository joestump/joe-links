// Governing: SPEC-0001 REQ "OIDC-Only Authentication", ADR-0003
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/joestump/joe-links/internal/config"
)

// Provider wraps an OIDC provider with OAuth2 configuration and token verification.
type Provider struct {
	verifier     *gooidc.IDTokenVerifier
	oauth2Config oauth2.Config
}

// NewProvider performs OIDC discovery and returns a configured Provider.
func NewProvider(ctx context.Context, cfg *config.Config) (*Provider, error) {
	provider, err := gooidc.NewProvider(ctx, cfg.OIDC.Issuer)
	if err != nil {
		return nil, fmt.Errorf("OIDC provider discovery failed for %s: %w", cfg.OIDC.Issuer, err)
	}

	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.OIDC.ClientID,
		ClientSecret: cfg.OIDC.ClientSecret,
		RedirectURL:  cfg.OIDC.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{gooidc.ScopeOpenID, "profile", "email"},
	}

	verifier := provider.Verifier(&gooidc.Config{ClientID: cfg.OIDC.ClientID})

	return &Provider{
		verifier:     verifier,
		oauth2Config: oauth2Cfg,
	}, nil
}

// AuthCodeURL generates the authorization URL with PKCE and state.
func (p *Provider) AuthCodeURL(state, codeChallenge string) string {
	return p.oauth2Config.AuthCodeURL(state,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

// Exchange trades an authorization code for tokens and returns the verified ID token.
func (p *Provider) Exchange(ctx context.Context, code, codeVerifier string) (*gooidc.IDToken, error) {
	token, err := p.oauth2Config.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("id_token verification: %w", err)
	}

	return idToken, nil
}

// GenerateState returns a cryptographically random state string.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GeneratePKCE returns a PKCE verifier and its S256 challenge.
func GeneratePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 64)
	if _, err = rand.Read(b); err != nil {
		return
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return
}
