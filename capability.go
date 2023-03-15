package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Capability struct {
	StoreAndRead `json:"StoreAndRead"`
}

type StoreAndRead struct {
	StoreBookmarks          bool
	StoreReadSystemID       bool
	StoreUserIDKey          bool
	StoreSignedVideo        bool
	StoreGNSSTrackRecording bool
}

func writeCapabilities(fpath, filename string) error {
	data := Capability{
		StoreAndRead{
			StoreBookmarks:          true,
			StoreReadSystemID:       true,
			StoreUserIDKey:          true,
			StoreSignedVideo:        true,
			StoreGNSSTrackRecording: true,
		},
	}

	file, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		if err := os.Mkdir(fpath, 0777); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(fpath, filename), file, 0777)
}
