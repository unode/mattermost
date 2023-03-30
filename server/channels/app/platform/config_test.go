// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package platform

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/server/channels/einterfaces/mocks"
	smocks "github.com/mattermost/mattermost-server/v6/server/channels/store/storetest/mocks"
)

func TestConfigListener(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	originalSiteName := th.Service.Config().TeamSettings.SiteName

	listenerCalled := false
	listener := func(oldConfig *model.Config, newConfig *model.Config) {
		assert.False(t, listenerCalled, "listener called twice")

		assert.Equal(t, *originalSiteName, *oldConfig.TeamSettings.SiteName, "old config contains incorrect site name")
		assert.Equal(t, "test123", *newConfig.TeamSettings.SiteName, "new config contains incorrect site name")

		listenerCalled = true
	}
	listenerId := th.Service.AddConfigListener(listener)
	defer th.Service.RemoveConfigListener(listenerId)

	listener2Called := false
	listener2 := func(oldConfig *model.Config, newConfig *model.Config) {
		assert.False(t, listener2Called, "listener2 called twice")

		listener2Called = true
	}
	listener2Id := th.Service.AddConfigListener(listener2)
	defer th.Service.RemoveConfigListener(listener2Id)

	th.Service.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.SiteName = "test123"
	})

	assert.True(t, listenerCalled, "listener should've been called")
	assert.True(t, listener2Called, "listener 2 should've been called")
}

func TestConfigSave(t *testing.T) {
	cm := &mocks.ClusterInterface{}
	cm.On("SendClusterMessage", mock.AnythingOfType("*model.ClusterMessage")).Return(nil)
	th := SetupWithCluster(t, cm)
	defer th.TearDown()

	t.Run("trigger a config changed event for the cluster", func(t *testing.T) {
		oldCfg := th.Service.Config()
		newCfg := oldCfg.Clone()
		newCfg.ServiceSettings.SiteURL = model.NewString("http://newhost.me")

		sanitizedOldCfg := th.Service.configStore.RemoveEnvironmentOverrides(oldCfg)
		sanitizedNewCfg := th.Service.configStore.RemoveEnvironmentOverrides(newCfg)
		cm.On("ConfigChanged", sanitizedOldCfg, sanitizedNewCfg, true).Return(nil)

		_, _, appErr := th.Service.SaveConfig(newCfg, true)
		require.Nil(t, appErr)

		updatedCfg := th.Service.Config()
		assert.Equal(t, "http://newhost.me", *updatedCfg.ServiceSettings.SiteURL)
	})

	t.Run("do not restart the metrics server on a different type of config change", func(t *testing.T) {
		th := Setup(t, StartMetrics())
		defer th.TearDown()

		metricsMock := &mocks.MetricsInterface{}
		metricsMock.On("IncrementWebsocketEvent", mock.AnythingOfType("string")).Return()
		metricsMock.On("IncrementWebSocketBroadcastBufferSize", mock.AnythingOfType("string"), mock.AnythingOfType("float64")).Return()
		metricsMock.On("DecrementWebSocketBroadcastBufferSize", mock.AnythingOfType("string"), mock.AnythingOfType("float64")).Return()
		metricsMock.On("Register").Return()
		th.Service.metricsIFace = metricsMock

		// Change a random config setting
		cfg := th.Service.Config().Clone()
		cfg.ThemeSettings.EnableThemeSelection = model.NewBool(!*cfg.ThemeSettings.EnableThemeSelection)
		th.Service.SaveConfig(cfg, false)
		metricsMock.AssertNumberOfCalls(t, "Register", 0)

		// Disable metrics
		cfg.MetricsSettings.Enable = model.NewBool(false)
		th.Service.SaveConfig(cfg, false)

		// Change the metrics setting
		cfg.MetricsSettings.Enable = model.NewBool(true)
		th.Service.SaveConfig(cfg, false)
		metricsMock.AssertNumberOfCalls(t, "Register", 1)
	})
}

func TestIsFirstUserAccount(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()
	storeMock := th.Service.Store.(*smocks.Store)
	userStoreMock := &smocks.UserStore{}
	storeMock.On("User").Return(userStoreMock)

	type test struct {
		name   string
		count  int64
		err    error
		result bool
	}

	tests := []test{
		{"success no users", 0, nil, true},
		{"success one user", 1, nil, false},
		{"success multiple users", 42, nil, false},
		{"success negative users", -100, nil, true},
		{"failed request", 0, errors.New("error"), false},
	}

	for _, te := range tests {
		t.Run(te.name, func(t *testing.T) {
			*userStoreMock = smocks.UserStore{}

			userStoreMock.On("Count", model.UserCountOptions{IncludeDeleted: true}).Return(te.count, te.err)
			require.Equal(t, te.result, th.Service.IsFirstUserAccount())
		})
	}

	// create a session, this should not affect IsFirstUserAccount
	th.Service.sessionCache.Set("mock_session", 1)

	for _, te := range tests {
		t.Run(te.name+" with session", func(t *testing.T) {
			*userStoreMock = smocks.UserStore{}

			userStoreMock.On("Count", model.UserCountOptions{IncludeDeleted: true}).Return(te.count, te.err)
			require.Equal(t, te.result, th.Service.IsFirstUserAccount())
		})
	}
}
