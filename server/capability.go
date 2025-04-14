package server

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Capability struct {
	Read         `json:"Read"`
	Store        `json:"Store"`
	StoreAndRead `json:"StoreAndRead"`
}

type Read struct {
	ReadCategories bool
}

type Store struct {
	StoreBookmarks          bool
	StoreRejectedContent    bool
	StoreUserIDKey          bool
	StoreSignedVideo        bool
	StoreGNSSTrackRecording bool
}

type StoreAndRead struct {
	StoreReadSystemID bool
}

func writeCapabilities(fpath, filename string) error {
	data := Capability{
		Read{
			ReadCategories: true,
		},
		Store{
			StoreBookmarks:          true,
			StoreRejectedContent:    true,
			StoreUserIDKey:          true,
			StoreSignedVideo:        true,
			StoreGNSSTrackRecording: true,
		},
		StoreAndRead{
			StoreReadSystemID: true,
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
