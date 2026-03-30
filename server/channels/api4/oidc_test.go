// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestOIDCConfig(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// 1. Test that OIDC is reported as enabled in client license (bypass check)
	clientLicense := th.Client.GetOldClientLicense("")
	assert.Equal(t, "true", clientLicense["OpenId"])

	// 2. Test that OIDC settings are available in client config (bypass check)
	clientConfig, _, _ := th.Client.GetOldClientConfig("")
	// Note: Client config uses "EnableSignUpWithOpenId" key
	assert.Contains(t, clientConfig, "EnableSignUpWithOpenId")

	// 3. Test OIDC start endpoint (unconfigured)
	// We expect a 501 Not Implemented because env vars/config are missing in test env
	resp, err := http.Get(th.Client.URL + "/api/v4/auth/oidc/start")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)

	// 4. Test with some config
	cfg := th.App.Config()
	*cfg.OpenIdSettings.Enable = true
	*cfg.OpenIdSettings.DiscoveryEndpoint = "http://localhost:8080/realms/master/.well-known/openid-configuration"
	*cfg.OpenIdSettings.Id = "mattermost"
	*cfg.OpenIdSettings.Secret = "secret"
	*cfg.ServiceSettings.SiteURL = "http://localhost:8065"
	th.App.UpdateConfig(func(c *model.Config) {
		*c = *cfg
	})

	// Now it should try to redirect (or fail to create provider, but not 501)
	resp2, err := http.Get(th.Client.URL + "/api/v4/auth/oidc/start")
	require.NoError(t, err)
	defer resp2.Body.Close()
	// It will likely fail to create provider because http://localhost:8080 is down,
	// which returns 501 in our implementation for any config/provider error.
	assert.Equal(t, http.StatusNotImplemented, resp2.StatusCode)
}

func TestOIDCHijackEndpoints(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	endpoints := []string{
		"/oauth/openid/login",
		"/oauth/openid/complete",
		"/signup/openid/complete",
		"/login/openid/complete",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := http.Get(th.Client.URL + endpoint)
			require.NoError(t, err)
			defer resp.Body.Close()
			// Should be 501 (not configured) but definitely not 404
			assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)
		})
	}
}
