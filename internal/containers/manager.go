package containers

type ContainerManager interface {
	CreateContainer(name string) error
	StopContainer(name string) error
	RemoveContainer(name string) error
	ListContainers() ([]string, error)
}
