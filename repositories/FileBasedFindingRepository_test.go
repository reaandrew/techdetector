package repositories

import (
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

func TestStoreWritesMatchesToFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "prefix")
	if err != nil {
		assert.Nil(t, err)
	}
	defer os.RemoveAll(dir)

	repository := FileBasedFindingRepository{
		path: dir,
	}

	err = repository.Store([]core.Finding{
		core.Finding{},
	})
	assert.Nil(t, err)
	count, err := utils.CountFiles(dir)
	assert.Nil(t, err)
	assert.Equal(t, 1, count)
}

func TestClearRemovesAllFiles(t *testing.T) {
	dir, err := os.MkdirTemp("", "prefix")
	if err != nil {
		assert.Nil(t, err)
	}
	defer os.RemoveAll(dir)

	repository := FileBasedFindingRepository{
		path: dir,
	}

	err = repository.Store([]core.Finding{
		core.Finding{},
	})
	assert.Nil(t, err)
	err = repository.Clear()
	assert.Nil(t, err)
	count, err := utils.CountFiles(dir)
	assert.Nil(t, err)
	assert.Equal(t, 0, count)
}

func TestClearOnlyDeletesFilesItCreated(t *testing.T) {
	dir, err := os.MkdirTemp("", "prefix")
	if err != nil {
		assert.Nil(t, err)
	}
	defer os.RemoveAll(dir)

	repository := FileBasedFindingRepository{
		path: dir,
	}
	otherFile := path.Join(dir, utils.GenerateRandomFilename("other"))
	err = os.WriteFile(otherFile, []byte("something"), 0644)
	assert.Nil(t, err)
	err = repository.Store([]core.Finding{
		core.Finding{},
	})
	assert.Nil(t, err)
	count_before, err := utils.CountFiles(dir)
	assert.Nil(t, err)
	assert.Equal(t, 2, count_before)
	err = repository.Clear()
	assert.Nil(t, err)
	count_after, err := utils.CountFiles(dir)
	assert.Nil(t, err)
	assert.Equal(t, 1, count_after)
}

func TestIterator(t *testing.T) {
	dir, err := os.MkdirTemp("", "prefix")
	if err != nil {
		assert.Nil(t, err)
	}
	defer os.RemoveAll(dir)

	repository := FileBasedFindingRepository{
		path: dir,
	}

	err = repository.Store([]core.Finding{
		{
			Name: "match 1",
		},
		{
			Name: "match 2",
		},
	})
	assert.Nil(t, err)
	err = repository.Store([]core.Finding{
		{
			Name: "match 3",
		},
		{
			Name: "match 4",
		},
	})
	assert.Nil(t, err)

	count := 0
	names := []string{}
	matchIterator := repository.NewIterator()
	for matchIterator.HasNext() {
		matchSet, err := matchIterator.Next()
		assert.Nil(t, err)
		for _, match := range matchSet.Matches {
			names = append(names, match.Name)
			count++
		}
	}

	assert.Equal(t, 4, count)
	assert.Equal(t, []string{"match 1", "match 2", "match 3", "match 4"}, names)
}
