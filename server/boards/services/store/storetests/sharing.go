// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v6/server/boards/model"
	"github.com/mattermost/mattermost-server/v6/server/boards/services/store"
)

func StoreTestSharingStore(t *testing.T, runStoreTests func(*testing.T, func(*testing.T, store.Store))) {
	t.Run("UpsertSharingAndGetSharing", func(t *testing.T) {
		runStoreTests(t, testUpsertSharingAndGetSharing)
	})
}

func testUpsertSharingAndGetSharing(t *testing.T, store store.Store) {
	t.Run("Insert first sharing and get it", func(t *testing.T) {
		sharing := model.Sharing{
			ID:         "sharing-id",
			Enabled:    true,
			Token:      "token",
			ModifiedBy: testUserID,
		}

		err := store.UpsertSharing(sharing)
		require.NoError(t, err)
		newSharing, err := store.GetSharing("sharing-id")
		require.NoError(t, err)
		newSharing.UpdateAt = 0
		require.Equal(t, sharing, *newSharing)
	})
	t.Run("Upsert the inserted sharing and get it", func(t *testing.T) {
		sharing := model.Sharing{
			ID:         "sharing-id",
			Enabled:    true,
			Token:      "token2",
			ModifiedBy: "user-id2",
		}

		newSharing, err := store.GetSharing("sharing-id")
		require.NoError(t, err)
		newSharing.UpdateAt = 0
		require.NotEqual(t, sharing, *newSharing)

		err = store.UpsertSharing(sharing)
		require.NoError(t, err)
		newSharing, err = store.GetSharing("sharing-id")
		require.NoError(t, err)
		newSharing.UpdateAt = 0
		require.Equal(t, sharing, *newSharing)
	})
	t.Run("Get not existing sharing", func(t *testing.T) {
		_, err := store.GetSharing("not-existing")
		require.Error(t, err)
		require.True(t, model.IsErrNotFound(err))
	})
}
