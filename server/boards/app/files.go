// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	mm_model "github.com/mattermost/mattermost-server/v6/model"

	"github.com/mattermost/mattermost-server/v6/server/boards/model"
	"github.com/mattermost/mattermost-server/v6/server/boards/utils"
	"github.com/mattermost/mattermost-server/v6/server/platform/shared/filestore"
	"github.com/mattermost/mattermost-server/v6/server/platform/shared/mlog"
)

const emptyString = "empty"

var errEmptyFilename = errors.New("IsFileArchived: empty filename not allowed")
var ErrFileNotFound = errors.New("file not found")

func (a *App) SaveFile(reader io.Reader, teamID, rootID, filename string) (string, error) {
	// NOTE: File extension includes the dot
	fileExtension := strings.ToLower(filepath.Ext(filename))
	if fileExtension == ".jpeg" {
		fileExtension = ".jpg"
	}

	createdFilename := utils.NewID(utils.IDTypeNone)
	fullFilename := fmt.Sprintf(`%s%s`, createdFilename, fileExtension)
	filePath := filepath.Join(utils.GetBaseFilePath(), fullFilename)

	fileSize, appErr := a.filesBackend.WriteFile(reader, filePath)
	if appErr != nil {
		return "", fmt.Errorf("unable to store the file in the files storage: %w", appErr)
	}

	now := utils.GetMillis()

	fileInfo := &mm_model.FileInfo{
		Id:              createdFilename[1:],
		CreatorId:       "boards",
		PostId:          emptyString,
		ChannelId:       emptyString,
		CreateAt:        now,
		UpdateAt:        now,
		DeleteAt:        0,
		Path:            filePath,
		ThumbnailPath:   emptyString,
		PreviewPath:     emptyString,
		Name:            filename,
		Extension:       fileExtension,
		Size:            fileSize,
		MimeType:        emptyString,
		Width:           0,
		Height:          0,
		HasPreviewImage: false,
		MiniPreview:     nil,
		Content:         "",
		RemoteId:        nil,
	}
	err := a.store.SaveFileInfo(fileInfo)
	if err != nil {
		return "", err
	}

	return fullFilename, nil
}

func (a *App) GetFileInfo(filename string) (*mm_model.FileInfo, error) {
	if filename == "" {
		return nil, errEmptyFilename
	}

	// filename is in the format 7<some-alphanumeric-string>.<extension>
	// we want to extract the <some-alphanumeric-string> part of this as this
	// will be the fileinfo id.
	parts := strings.Split(filename, ".")
	fileInfoID := parts[0][1:]
	fileInfo, err := a.store.GetFileInfo(fileInfoID)
	if err != nil {
		return nil, err
	}

	return fileInfo, nil
}

func (a *App) GetFile(teamID, rootID, fileName string) (*mm_model.FileInfo, filestore.ReadCloseSeeker, error) {
	fileInfo, err := a.GetFileInfo(fileName)
	if err != nil && !model.IsErrNotFound(err) {
		a.logger.Error("111")
		return nil, nil, err
	}

	var filePath string

	if fileInfo != nil && fileInfo.Path != "" {
		filePath = fileInfo.Path
	} else {
		filePath = filepath.Join(teamID, rootID, fileName)
	}

	exists, err := a.filesBackend.FileExists(filePath)
	if err != nil {
		a.logger.Error(fmt.Sprintf("GetFile: Failed to check if file exists as path. Path: %s, error: %e", filePath, err))
		return nil, nil, err
	}

	if !exists {
		return nil, nil, ErrFileNotFound
	}

	reader, err := a.filesBackend.Reader(filePath)
	if err != nil {
		a.logger.Error(fmt.Sprintf("GetFile: Failed to get file reader of existing file at path: %s, error: %e", filePath, err))
		return nil, nil, err
	}
	return fileInfo, reader, nil
}

func (a *App) GetFileReader(teamID, rootID, filename string) (filestore.ReadCloseSeeker, error) {
	filePath := filepath.Join(teamID, rootID, filename)
	exists, err := a.filesBackend.FileExists(filePath)
	if err != nil {
		return nil, err
	}
	// FIXUP: Check the deprecated old location
	if teamID == "0" && !exists {
		oldExists, err2 := a.filesBackend.FileExists(filename)
		if err2 != nil {
			return nil, err2
		}
		if oldExists {
			err2 := a.filesBackend.MoveFile(filename, filePath)
			if err2 != nil {
				a.logger.Error("ERROR moving file",
					mlog.String("old", filename),
					mlog.String("new", filePath),
					mlog.Err(err2),
				)
			} else {
				a.logger.Debug("Moved file",
					mlog.String("old", filename),
					mlog.String("new", filePath),
				)
			}
		}
	} else if !exists {
		return nil, ErrFileNotFound
	}

	reader, err := a.filesBackend.Reader(filePath)
	if err != nil {
		return nil, err
	}

	return reader, nil
}

func (a *App) MoveFile(channelID, teamID, boardID, filename string) error {
	oldPath := filepath.Join(channelID, boardID, filename)
	newPath := filepath.Join(teamID, boardID, filename)
	err := a.filesBackend.MoveFile(oldPath, newPath)
	if err != nil {
		a.logger.Error("ERROR moving file",
			mlog.String("old", oldPath),
			mlog.String("new", newPath),
			mlog.Err(err),
		)
		return err
	}
	return nil
}
