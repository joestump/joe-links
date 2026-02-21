// Governing: SPEC-0007 REQ "Main Swagger Annotation Block", ADR-0010

// @title           joe-links API
// @version         1.0
// @description     Self-hosted go-links service. Authenticate with a Personal Access Token.
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerToken
// @in              header
// @name            Authorization
// @description     Type "Bearer" followed by a space and your API token. Example: "Bearer jl_xxx"
package api
