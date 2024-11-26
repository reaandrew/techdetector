package reporters

import "github.com/reaandrew/techdetector/repositories"

type Reporter interface {
	Report(repository repositories.FindingRepository) error
}
