package core

type Reporter interface {
	Report(repository FindingRepository) error
}
