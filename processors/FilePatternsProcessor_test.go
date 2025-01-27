package processors

import (
	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSomething(t *testing.T) {
	patterns := []Pattern{
		{
			Name:     "Something",
			Type:     "Cloud Service",
			Category: "Something",
			Filenames: []string{
				"no_extension",
			},
		},
	}
	processor := FilePatternsProcessor{Patterns: patterns}

	matches, err := processor.Process("/no_extension", "some-repo", "content")
	assert.Nil(t, err)
	assert.Len(t, matches, 1)
}

func TestAsteriskInPattern(t *testing.T) {
	patterns := []Pattern{
		{
			Name:     "Something",
			Type:     "Cloud Service",
			Category: "Something",
			Filenames: []string{
				".*ora",
			},
		},
	}
	processor := FilePatternsProcessor{Patterns: patterns}
	processor.CompilePatterns()

	matches, err := processor.Process("/something.ora", "some-repo", "content")
	assert.Nil(t, err)
	assert.Len(t, matches, 1)
}

func TestFileExtension(t *testing.T) {
	patterns := []Pattern{
		{
			Name:     "Something",
			Type:     "Cloud Service",
			Category: "Something",
			FileExtensions: []string{
				"py",
			},
		},
	}
	processor := FilePatternsProcessor{Patterns: patterns}
	processor.CompilePatterns()

	matches, err := processor.Process("/something.py", "some-repo", "content")
	assert.Nil(t, err)
	assert.Len(t, matches, 1)
}

func TestContentPatternWithFileExtensionCriteria(t *testing.T) {
	patterns := []Pattern{
		{
			Name:     "Something",
			Type:     "Cloud Service",
			Category: "Something",
			FileExtensions: []string{
				"py",
			},
			ContentPatterns: []string{
				"BABALOO",
			},
		},
	}
	processor := FilePatternsProcessor{Patterns: patterns}
	processor.CompilePatterns()

	content := `
Something BABALOO
`
	matches, err := processor.Process("/something.py", "some-repo", content)
	assert.Nil(t, err)
	assert.Len(t, matches, 1)
}

func TestContentPatternWithFileNamesCriteria(t *testing.T) {
	patterns := []Pattern{
		{
			Name:     "Something",
			Type:     "Cloud Service",
			Category: "Something",
			Filenames: []string{
				".*ora",
			},
			ContentPatterns: []string{
				"BABALOO",
			},
		},
	}
	processor := FilePatternsProcessor{Patterns: patterns}
	processor.CompilePatterns()

	content := `
Something BABALOO
`
	matches, err := processor.Process("/something.ora", "some-repo", content)
	assert.Nil(t, err)
	assert.Len(t, matches, 1)
}

func TestContentPatternFailsIfFileNamesCriteriaDoesNotMatch(t *testing.T) {
	patterns := []Pattern{
		{
			Name:     "Something",
			Type:     "Cloud Service",
			Category: "Something",
			Filenames: []string{
				".*ora",
			},
			ContentPatterns: []string{
				"BABALOO",
			},
		},
	}
	processor := FilePatternsProcessor{Patterns: patterns}
	processor.CompilePatterns()

	content := `
Something BABALOO
`
	matches, err := processor.Process("/something.abc", "some-repo", content)
	assert.Nil(t, err)
	assert.Len(t, matches, 0)
}

func TestContentPatternsContainingPeriodPasses(t *testing.T) {
	patterns := []Pattern{
		{
			Name:     "Something",
			Type:     "Cloud Service",
			Category: "Something",
			Filenames: []string{
				".*ora",
			},
			ContentPatterns: []string{
				"Amazon.ACMPCA",
			},
		},
	}
	processor := FilePatternsProcessor{Patterns: patterns}
	processor.CompilePatterns()

	content := `
using Amazon.ACMPCA;
`
	matches, err := processor.Process("/something.ora", "some-repo", content)
	assert.Nil(t, err)
	assert.Len(t, matches, 1)
}

func TestMatchPath(t *testing.T) {
	p := Pattern{
		Name:      "Something",
		Type:      "Some Type",
		Category:  "Some Category",
		Filenames: nil,
		PathPatterns: []string{
			"**/.circleci/config.yml",
		},
		FileExtensions:       nil,
		ContentPatterns:      nil,
		FilenameRegexs:       nil,
		ContentPatternRegexs: nil,
		PathPatternGlobs: []glob.Glob{
			glob.MustCompile("**/.circleci/config.yml"),
		},
		Properties: nil,
	}

	result := matchPath(p, ".circleci/config.yml")

	assert.True(t, result)
}

func TestMatchPathReturnsFalse(t *testing.T) {
	p := Pattern{
		Name:      "Something",
		Type:      "Some Type",
		Category:  "Some Category",
		Filenames: nil,
		PathPatterns: []string{
			"**/.circleci/config.yml",
		},
		FileExtensions:       nil,
		ContentPatterns:      nil,
		FilenameRegexs:       nil,
		ContentPatternRegexs: nil,
		PathPatternGlobs: []glob.Glob{
			glob.MustCompile("**/.circleci/config.yml"),
		},
		Properties: nil,
	}

	result := matchPath(p, "/tmp/techdetector/Fennel/test/bad/all.sh")

	assert.False(t, result)
}
