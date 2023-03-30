// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package imports

import (
	// Needed to ensure the init() method in the Playbooks product is run.
	// This file is copied to the mmserver imports package via makefile.
	_ "github.com/mattermost/mattermost-server/v6/server/playbooks/product"
)
