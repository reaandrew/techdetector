package reporters

type Reporter interface {
	Report(repository FindingRepository) error
}
