package containers

// TODO: must implement a full container manager module based on our requirements
type ContainerManager interface {
	CreateContainer(name string) error
	StopContainer(name string) error
	RemoveContainer(name string) error
	ListContainers() ([]string, error)
}
