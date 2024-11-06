package main

type CloudServicesLoader interface {
	LoadAllCloudServices() ([]CloudService, error)
}
