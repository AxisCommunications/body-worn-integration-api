package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func writeCategories(fpath, filename string) error {
	// Base categories, user can manually edit the file to change them.
	// The file is always created, but the user will have to enable FullStoreAndReadSupport
	// or ReadCategories capability to make the Body Worn system fetch the categories.
	categories := []struct {
		Name string
		Id   string
	}{
		{Id: "1", Name: "Testimony"},
		{Id: "2", Name: "Disorderly Conduct"},
		{Id: "3", Name: "Assault"},
		{Id: "4", Name: "Homicide"},
		{Id: "5", Name: "Traffic"},
		{Id: "6", Name: "Category A"},
		{Id: "7", Name: "Category B"},
		{Id: "8", Name: "Category C"},
		{Id: "9", Name: "Category D"},
		{Id: "10", Name: "Category E"},
	}

	categoriesJson, err := json.Marshal(categories)
	if err != nil {
		return fmt.Errorf("failed to marshal base categories: %v", err)
	}

	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		if err := os.Mkdir(fpath, 0777); err != nil {
			return err
		}
	}

	return os.WriteFile(filepath.Join(fpath, filename), categoriesJson, 0777)
}
