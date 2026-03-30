// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// oidcStateEntry tracks state for the OAuth2 authorization code flow.
type oidcStateEntry struct {
	DesktopToken string
	CreatedAt    time.Time
}

// oidcStateStore is a thread-safe store for OAuth2 state parameters.
type oidcStateStore struct {
	mu      sync.Mutex
	entries map[string]oidcStateEntry
}

func newOIDCStateStore() *oidcStateStore {
	store := &oidcStateStore{
		entries: make(map[string]oidcStateEntry),
	}
	// Clean up expired entries every 5 minutes.
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			store.cleanup()
		}
	}()
	return store
}

func (s *oidcStateStore) Set(state string, entry oidcStateEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[state] = entry
}

func (s *oidcStateStore) GetAndDelete(state string) (oidcStateEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[state]
	if ok {
		delete(s.entries, state)
	}
	return entry, ok
}

func (s *oidcStateStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-10 * time.Minute)
	for state, entry := range s.entries {
		if entry.CreatedAt.Before(cutoff) {
			delete(s.entries, state)
		}
	}
}

var stateStore = newOIDCStateStore()

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func getOIDCConfig() (*oauth2.Config, *oidc.Provider, error) {
	issuer := os.Getenv("OIDC_ISSUER")
	clientID := os.Getenv("OIDC_CLIENT_ID")
	clientSecret := os.Getenv("OIDC_CLIENT_SECRET")
	redirectURL := os.Getenv("OIDC_REDIRECT_URL")

	if issuer == "" || clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil, nil, fmt.Errorf(
			"OIDC not configured: set OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET, OIDC_REDIRECT_URL",
		)
	}

	provider, err := oidc.NewProvider(context.Background(), issuer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	return config, provider, nil
}

func (api *API) InitOIDC() {
	api.BaseRoutes.Root.HandleFunc(
		"/api/v4/auth/oidc/start",
		api.APIHandler(oidcLoginStart),
	).Methods("GET")

	api.BaseRoutes.Root.HandleFunc(
		"/api/v4/auth/oidc/complete",
		api.APIHandler(oidcLoginComplete),
	).Methods("GET")
}

func (api *API) InitOIDCLocal() {
	// No local-mode OIDC endpoints needed.
}

func oidcLoginStart(c *Context, w http.ResponseWriter, r *http.Request) {
	config, _, err := getOIDCConfig()
	if err != nil {
		c.Err = model.NewAppError(
			"oidcLoginStart", "api.oidc.not_configured.app_error",
			nil, err.Error(), http.StatusNotImplemented,
		)
		return
	}

	state, err := generateState()
	if err != nil {
		c.Err = model.NewAppError(
			"oidcLoginStart", "api.oidc.state_error.app_error",
			nil, err.Error(), http.StatusInternalServerError,
		)
		return
	}

	desktopToken := r.URL.Query().Get("desktop_token")
	stateStore.Set(state, oidcStateEntry{
		DesktopToken: desktopToken,
		CreatedAt:    time.Now(),
	})

	http.Redirect(w, r, config.AuthCodeURL(state), http.StatusFound)
}

func oidcLoginComplete(c *Context, w http.ResponseWriter, r *http.Request) {
	config, provider, err := getOIDCConfig()
	if err != nil {
		c.Err = model.NewAppError(
			"oidcLoginComplete", "api.oidc.not_configured.app_error",
			nil, err.Error(), http.StatusNotImplemented,
		)
		return
	}

	// Validate state parameter.
	state := r.URL.Query().Get("state")
	stateEntry, ok := stateStore.GetAndDelete(state)
	if !ok {
		c.Err = model.NewAppError(
			"oidcLoginComplete", "api.oidc.invalid_state.app_error",
			nil, "invalid or expired state parameter", http.StatusBadRequest,
		)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		c.Err = model.NewAppError(
			"oidcLoginComplete", "api.oidc.missing_code.app_error",
			nil, "authorization code not provided", http.StatusBadRequest,
		)
		return
	}

	// Exchange authorization code for tokens.
	token, err := config.Exchange(r.Context(), code)
	if err != nil {
		c.Err = model.NewAppError(
			"oidcLoginComplete", "api.oidc.token_exchange.app_error",
			nil, err.Error(), http.StatusInternalServerError,
		)
		return
	}

	// Verify ID token.
	verifier := provider.Verifier(&oidc.Config{ClientID: config.ClientID})

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		c.Err = model.NewAppError(
			"oidcLoginComplete", "api.oidc.missing_id_token.app_error",
			nil, "no id_token in response", http.StatusInternalServerError,
		)
		return
	}

	idToken, err := verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		c.Err = model.NewAppError(
			"oidcLoginComplete", "api.oidc.token_verify.app_error",
			nil, err.Error(), http.StatusUnauthorized,
		)
		return
	}

	// Extract claims.
	var claims struct {
		Email             string `json:"email"`
		PreferredUsername  string `json:"preferred_username"`
		Name              string `json:"name"`
		GivenName         string `json:"given_name"`
		FamilyName        string `json:"family_name"`
		Position          string `json:"position"`
	}
	if err := idToken.Claims(&claims); err != nil {
		c.Err = model.NewAppError(
			"oidcLoginComplete", "api.oidc.claims_parse.app_error",
			nil, err.Error(), http.StatusInternalServerError,
		)
		return
	}

	if claims.Email == "" {
		c.Err = model.NewAppError(
			"oidcLoginComplete", "api.oidc.missing_email.app_error",
			nil, "email claim is required", http.StatusBadRequest,
		)
		return
	}

	// Find or create user.
	user, appErr := c.App.GetUserByEmail(claims.Email)
	if appErr != nil {
		// User doesn't exist — create one.
		username := claims.PreferredUsername
		if username == "" {
			username = strings.Split(claims.Email, "@")[0]
		}

		firstName := claims.GivenName
		lastName := claims.FamilyName
		if firstName == "" && lastName == "" && claims.Name != "" {
			parts := strings.SplitN(claims.Name, " ", 2)
			firstName = parts[0]
			if len(parts) > 1 {
				lastName = parts[1]
			}
		}

		newUser := &model.User{
			Email:     claims.Email,
			Username:  username,
			FirstName: firstName,
			LastName:  lastName,
			Position:  claims.Position,
			AuthService: "oidc",
			AuthData:    model.NewPointer(claims.Email),
		}

		var createErr *model.AppError
		user, createErr = c.App.CreateUserAsAdmin(c.AppContext, newUser, "")
		if createErr != nil {
			c.Err = createErr
			return
		}

		mlog.Info("Created new user via OIDC",
			mlog.String("email", claims.Email),
			mlog.String("username", username),
		)
	}

	// Create session.
	session := &model.Session{
		UserId:    user.Id,
		DeviceId:  "",
		Roles:     user.GetRawRoles(),
		IsOAuth:   true,
		ExpiredNotify: true,
	}

	session, appErr = c.App.CreateSession(c.AppContext, session)
	if appErr != nil {
		c.Err = appErr
		return
	}

	// Handle desktop app token.
	if stateEntry.DesktopToken != "" {
		desktopToken, appErr := c.App.GenerateAndSaveDesktopToken(
			time.Now().Add(
				time.Duration(model.SessionUserAccessTokenExpiryHours)*time.Hour,
			),
			user,
		)
		if appErr != nil {
			c.Err = appErr
			return
		}

		http.Redirect(w, r,
			fmt.Sprintf("/login/desktop?desktop_token=%s", desktopToken.Token),
			http.StatusFound,
		)
		return
	}

	// Set session cookie and redirect.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Set-Cookie", fmt.Sprintf(
		"%s=%s; Path=/; HttpOnly; Secure; SameSite=Lax",
		model.SessionCookieToken, session.Token,
	))

	response := map[string]string{
		"redirect": "/",
	}

	http.Redirect(w, r, "/", http.StatusFound)

	_ = json.NewEncoder(w).Encode(response)
}
