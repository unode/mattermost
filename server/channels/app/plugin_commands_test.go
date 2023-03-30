// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/mattermost/mattermost-server/v6/server/channels/app/request"
	"github.com/mattermost/mattermost-server/v6/server/channels/product"
	"github.com/mattermost/mattermost-server/v6/server/platform/shared/i18n"
)

func TestPluginCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	args := &model.CommandArgs{}
	args.TeamId = th.BasicTeam.Id
	args.ChannelId = th.BasicChannel.Id
	args.UserId = th.BasicUser.Id
	args.Command = "/plugin"

	t.Run("error before plugin command registered", func(t *testing.T) {
		_, err := th.App.ExecuteCommand(th.Context, args)
		require.NotNil(t, err)
	})

	t.Run("command handled by plugin", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.PluginSettings.Plugins["testloadpluginconfig"] = map[string]any{
				"TeamId": args.TeamId,
			}
		})

		tearDown, pluginIDs, activationErrors := SetAppEnvironmentWithPlugins(t, []string{`
			package main

			import (
				"github.com/mattermost/mattermost-server/v6/plugin"
				"github.com/mattermost/mattermost-server/v6/model"
			)

			type configuration struct {
				TeamId string
			}

			type MyPlugin struct {
				plugin.MattermostPlugin

				configuration configuration
			}

			func (p *MyPlugin) OnConfigurationChange() error {
				if err := p.API.LoadPluginConfiguration(&p.configuration); err != nil {
					return err
				}

				return nil
			}

			func (p *MyPlugin) OnActivate() error {
				err := p.API.RegisterCommand(&model.Command{
					TeamId: p.configuration.TeamId,
					Trigger: "plugin",
					DisplayName: "Plugin Command",
					AutoComplete: true,
					AutoCompleteDesc: "autocomplete",
				})
				if err != nil {
					p.API.LogError("error", "err", err)
				}

				return err
			}

			func (p *MyPlugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
				return &model.CommandResponse{
					ResponseType: model.CommandResponseTypeEphemeral,
					Text: "text",
				}, nil
			}

			func main() {
				plugin.ClientMain(&MyPlugin{})
			}
		`}, th.App, th.NewPluginAPI)
		defer tearDown()
		require.Len(t, activationErrors, 1)
		require.Nil(t, nil, activationErrors[0])

		resp, err := th.App.ExecuteCommand(th.Context, args)
		require.Nil(t, err)
		require.Equal(t, model.CommandResponseTypeEphemeral, resp.ResponseType)
		require.Equal(t, "text", resp.Text)

		err2 := th.App.DisablePlugin(pluginIDs[0])
		require.Nil(t, err2)

		commands, err3 := th.App.ListAutocompleteCommands(args.TeamId, i18n.T)
		require.Nil(t, err3)

		for _, commands := range commands {
			require.NotEqual(t, "plugin", commands.Trigger)
		}

		th.App.ch.RemovePlugin(pluginIDs[0])
	})

	t.Run("re-entrant command registration on config change", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.PluginSettings.Plugins["testloadpluginconfig"] = map[string]any{
				"TeamId": args.TeamId,
			}
		})

		tearDown, pluginIDs, activationErrors := SetAppEnvironmentWithPlugins(t, []string{`
			package main

			import (
				"github.com/mattermost/mattermost-server/v6/plugin"
				"github.com/mattermost/mattermost-server/v6/model"
			)

			type configuration struct {
				TeamId string
			}

			type MyPlugin struct {
				plugin.MattermostPlugin

				configuration configuration
			}

			func (p *MyPlugin) OnConfigurationChange() error {
				p.API.LogInfo("OnConfigurationChange")
				err := p.API.LoadPluginConfiguration(&p.configuration);
				if err != nil {
					return err
				}

				p.API.LogInfo("About to register")
				err = p.API.RegisterCommand(&model.Command{
					TeamId: p.configuration.TeamId,
					Trigger: "plugin",
					DisplayName: "Plugin Command",
					AutoComplete: true,
					AutoCompleteDesc: "autocomplete",
				})
				if err != nil {
					p.API.LogInfo("Registered, with error", err, err.Error())
					return err
				}
				p.API.LogInfo("Registered, without error")
				return nil
			}

			func (p *MyPlugin) ExecuteCommand(c *plugin.Context, commandArgs *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
				p.API.LogInfo("ExecuteCommand")
				// Saving the plugin config eventually results in a call to
				// OnConfigurationChange. This used to deadlock on account of
				// effectively acquiring a RWLock reentrantly.
				err := p.API.SavePluginConfig(map[string]any{
					"TeamId": p.configuration.TeamId,
				})
				if err != nil {
					p.API.LogError("Failed to save plugin config", err, err.Error())
					return nil, err
				}
				p.API.LogInfo("ExecuteCommand, saved plugin config")

				return &model.CommandResponse{
					ResponseType: model.CommandResponseTypeEphemeral,
					Text: "text",
				}, nil
			}

			func main() {
				plugin.ClientMain(&MyPlugin{})
			}
		`}, th.App, th.NewPluginAPI)
		defer tearDown()

		require.Len(t, activationErrors, 1)
		require.Nil(t, nil, activationErrors[0])

		wait := make(chan bool)
		killed := false
		go func() {
			defer close(wait)

			resp, err := th.App.ExecuteCommand(th.Context, args)

			// Ignore if we kill below.
			if !killed {
				require.Nil(t, err)
				require.Equal(t, model.CommandResponseTypeEphemeral, resp.ResponseType)
				require.Equal(t, "text", resp.Text)
			}
		}()

		select {
		case <-wait:
		case <-time.After(10 * time.Second):
			killed = true
		}

		th.App.ch.RemovePlugin(pluginIDs[0])
		require.False(t, killed, "execute command appears to have deadlocked")
	})

	t.Run("error after plugin command unregistered", func(t *testing.T) {
		_, err := th.App.ExecuteCommand(th.Context, args)
		require.NotNil(t, err)
	})

	t.Run("plugins can override built-in commands", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.PluginSettings.Plugins["testloadpluginconfig"] = map[string]any{
				"TeamId": args.TeamId,
			}
		})

		tearDown, pluginIDs, activationErrors := SetAppEnvironmentWithPlugins(t, []string{`
			package main

			import (
				"github.com/mattermost/mattermost-server/v6/plugin"
				"github.com/mattermost/mattermost-server/v6/model"
			)

			type configuration struct {
				TeamId string
			}

			type MyPlugin struct {
				plugin.MattermostPlugin

				configuration configuration
			}

			func (p *MyPlugin) OnConfigurationChange() error {
				if err := p.API.LoadPluginConfiguration(&p.configuration); err != nil {
					return err
				}

				return nil
			}

			func (p *MyPlugin) OnActivate() error {
				err := p.API.RegisterCommand(&model.Command{
					TeamId: p.configuration.TeamId,
					Trigger: "code",
					DisplayName: "Plugin Command",
					AutoComplete: true,
					AutoCompleteDesc: "autocomplete",
				})
				if err != nil {
					p.API.LogError("error", "err", err)
				}

				return err
			}

			func (p *MyPlugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
				return &model.CommandResponse{
					ResponseType: model.CommandResponseTypeEphemeral,
					Text: "text",
				}, nil
			}

			func main() {
				plugin.ClientMain(&MyPlugin{})
			}
		`}, th.App, th.NewPluginAPI)
		defer tearDown()
		require.Len(t, activationErrors, 1)
		require.Nil(t, nil, activationErrors[0])

		args.Command = "/code"
		resp, err := th.App.ExecuteCommand(th.Context, args)
		require.Nil(t, err)
		require.Equal(t, model.CommandResponseTypeEphemeral, resp.ResponseType)
		require.Equal(t, "text", resp.Text)

		th.App.ch.RemovePlugin(pluginIDs[0])
	})
	t.Run("plugin has crashed before execution of command", func(t *testing.T) {
		tearDown, pluginIDs, activationErrors := SetAppEnvironmentWithPlugins(t, []string{`
			package main

			import (
				"github.com/mattermost/mattermost-server/v6/plugin"
				"github.com/mattermost/mattermost-server/v6/model"
			)

			type MyPlugin struct {
				plugin.MattermostPlugin

			}

			func (p *MyPlugin) OnActivate() error {
				err := p.API.RegisterCommand(&model.Command{
					Trigger: "code",
				})
				if err != nil {
					p.API.LogError("error", "err", err)
				}
				panic("Uncaught Error")

				return err
			}

			func (p *MyPlugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
				return &model.CommandResponse{}, nil
			}

			func main() {
				plugin.ClientMain(&MyPlugin{})
			}
		`}, th.App, th.NewPluginAPI)
		defer tearDown()
		require.Len(t, activationErrors, 1)
		require.Nil(t, nil, activationErrors[0])
		args.Command = "/code"
		resp, err := th.App.ExecuteCommand(th.Context, args)
		require.Nil(t, resp)
		require.NotNil(t, err)
		require.Equal(t, err.Id, "model.plugin_command_error.error.app_error")
		th.App.ch.RemovePlugin(pluginIDs[0])
	})

	t.Run("plugin has crashed due to the execution of the command", func(t *testing.T) {
		tearDown, pluginIDs, activationErrors := SetAppEnvironmentWithPlugins(t, []string{`
			package main

			import (
				"github.com/mattermost/mattermost-server/v6/plugin"
				"github.com/mattermost/mattermost-server/v6/model"
			)

			type MyPlugin struct {
				plugin.MattermostPlugin

			}

			func (p *MyPlugin) OnActivate() error {
				err := p.API.RegisterCommand(&model.Command{
					Trigger: "code",
				})
				if err != nil {
					p.API.LogError("error", "err", err)
				}

				return err
			}

			func (p *MyPlugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
				panic("Uncaught Error")
				return &model.CommandResponse{}, nil
			}

			func main() {
				plugin.ClientMain(&MyPlugin{})
			}
		`}, th.App, th.NewPluginAPI)
		defer tearDown()
		require.Len(t, activationErrors, 1)
		require.Nil(t, nil, activationErrors[0])
		args.Command = "/code"
		resp, err := th.App.ExecuteCommand(th.Context, args)
		require.Nil(t, resp)
		require.NotNil(t, err)
		require.Equal(t, err.Id, "model.plugin_command_crash.error.app_error")
		th.App.ch.RemovePlugin(pluginIDs[0])
	})

	t.Run("plugin returning status code 0", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.PluginSettings.Plugins["testloadpluginconfig"] = map[string]any{
				"TeamId": args.TeamId,
			}
		})

		tearDown, _, activationErrors := SetAppEnvironmentWithPlugins(t, []string{`
			package main

			import (
				"github.com/mattermost/mattermost-server/v6/plugin"
				"github.com/mattermost/mattermost-server/v6/model"
			)

			type configuration struct {
				TeamId string
			}

			type MyPlugin struct {
				plugin.MattermostPlugin

				configuration configuration
			}

			func (p *MyPlugin) OnActivate() error {
				err := p.API.RegisterCommand(&model.Command{
					TeamId: p.configuration.TeamId,
					Trigger: "plugin",
					DisplayName: "Plugin Command",
					AutoComplete: true,
					AutoCompleteDesc: "autocomplete",
				})
				if err != nil {
					p.API.LogError("error", "err", err)
				}

				return err
			}

			func (p *MyPlugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
				return nil, &model.AppError{
					Message: "error",
					StatusCode: 0,
				}
			}

			func main() {
				plugin.ClientMain(&MyPlugin{})
			}
		`}, th.App, th.NewPluginAPI)
		defer tearDown()
		require.Len(t, activationErrors, 1)
		require.Nil(t, nil, activationErrors[0])

		args.Command = "/plugin"
		_, err := th.App.ExecuteCommand(th.Context, args)
		require.NotNil(t, err)
		require.Equal(t, 500, err.StatusCode)
	})

}

// Test Product with the minimum code needed to handle
// hooksmanager and slash commands
type TProduct struct {
	hooksService product.HooksService
}

func newTProduct(m map[product.ServiceKey]any) (product.Product, error) {
	return &TProduct{
		hooksService: m[product.HooksKey].(product.HooksService),
	}, nil
}
func (p *TProduct) Start() error {
	p.hooksService.RegisterHooks("productT", p)
	return nil
}
func (p *TProduct) Stop() error { return nil }
func (p *TProduct) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	return &model.CommandResponse{Text: "product slash command called"}, nil
}

func TestProductCommands(t *testing.T) {

	products := map[string]product.Manifest{
		"productT": {
			Initializer:  newTProduct,
			Dependencies: map[product.ServiceKey]struct{}{},
		},
	}

	t.Run("Execute product command", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		// Server hijack.
		// This must be done in a cleaner way.
		th.Server.initializeProducts(products, th.Server.services)
		th.Server.products["productT"].Start()
		require.Len(t, th.Server.products, 2) // 1 product + channels

		err := th.App.RegisterProductCommand("productT", &model.Command{
			TeamId:           th.BasicTeam.Id,
			Trigger:          "product",
			DisplayName:      "Product Command",
			AutoComplete:     true,
			AutoCompleteDesc: "autocomplete",
		})
		require.NoError(t, err)

		ctx := request.EmptyContext(th.TestLogger)
		resp, err2 := th.App.ExecuteCommand(ctx, &model.CommandArgs{
			TeamId:    th.BasicTeam.Id,
			ChannelId: th.BasicChannel.Id,
			UserId:    th.BasicUser.Id,
			Command:   "/product",
		})
		require.Nil(t, err2)
		require.NotNil(t, resp)
		assert.Equal(t, "product slash command called", resp.Text)
	})

	t.Run("Product commands can override builtin commands", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		// Server hijack.
		// This must be done in a cleaner way.
		th.Server.initializeProducts(products, th.Server.services)
		th.Server.products["productT"].Start()
		require.Len(t, th.Server.products, 2) // 1 product + channels

		err := th.App.RegisterProductCommand("productT", &model.Command{
			TeamId:           th.BasicTeam.Id,
			Trigger:          "away",
			DisplayName:      "Product Command",
			AutoComplete:     true,
			AutoCompleteDesc: "autocomplete",
		})
		require.NoError(t, err)

		ctx := request.EmptyContext(th.TestLogger)
		resp, err2 := th.App.ExecuteCommand(ctx, &model.CommandArgs{
			TeamId:    th.BasicTeam.Id,
			ChannelId: th.BasicChannel.Id,
			UserId:    th.BasicUser.Id,
			Command:   "/away",
		})
		require.Nil(t, err2)
		require.NotNil(t, resp)
		assert.Equal(t, "product slash command called", resp.Text)
	})

	t.Run("Plugin commands can override product commands", func(t *testing.T) {

		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.PluginSettings.Plugins["testloadpluginconfig"] = map[string]any{
				"TeamId": th.BasicTeam.Id,
			}
		})

		tearDown, _, activationErrors := SetAppEnvironmentWithPlugins(t, []string{`
			package main

			import (
				"github.com/mattermost/mattermost-server/v6/plugin"
				"github.com/mattermost/mattermost-server/v6/model"
			)

			type configuration struct {
				TeamId string
			}

			type MyPlugin struct {
				plugin.MattermostPlugin

				configuration configuration
			}

			func (p *MyPlugin) OnConfigurationChange() error {
				if err := p.API.LoadPluginConfiguration(&p.configuration); err != nil {
					return err
				}

				return nil
			}

			func (p *MyPlugin) OnActivate() error {
				err := p.API.RegisterCommand(&model.Command{
					TeamId: p.configuration.TeamId,
					Trigger: "triggername",
					DisplayName: "Plugin Command",
					AutoComplete: true,
					AutoCompleteDesc: "autocomplete",
				})
				if err != nil {
					p.API.LogError("error", "err", err)
				}

				return err
			}

			func (p *MyPlugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
				return &model.CommandResponse{
					Text: "plugin slash command called",
				}, nil
			}

			func main() {
				plugin.ClientMain(&MyPlugin{})
			}
		`}, th.App, th.NewPluginAPI)
		defer tearDown()
		require.Len(t, activationErrors, 1)
		require.Nil(t, nil, activationErrors[0])

		// Server hijack.
		// This must be done in a cleaner way.
		th.Server.initializeProducts(products, th.Server.services)
		th.Server.products["productT"].Start()
		require.Len(t, th.Server.products, 2) // 1 product + channels

		err := th.App.RegisterProductCommand("productT", &model.Command{
			TeamId:           th.BasicTeam.Id,
			Trigger:          "triggername",
			DisplayName:      "Product Command",
			AutoComplete:     true,
			AutoCompleteDesc: "autocomplete",
		})
		require.NoError(t, err)

		ctx := request.EmptyContext(th.TestLogger)
		resp, err2 := th.App.ExecuteCommand(ctx, &model.CommandArgs{
			TeamId:    th.BasicTeam.Id,
			ChannelId: th.BasicChannel.Id,
			UserId:    th.BasicUser.Id,
			Command:   "/triggername",
		})
		require.Nil(t, err2)
		require.NotNil(t, resp)
		assert.Equal(t, "plugin slash command called", resp.Text)

	})
}
