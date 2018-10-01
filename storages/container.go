package storages

type Container struct {
	ID              string
	Ecs             bool
	Container       string
	Service         string
	Address         string
	Revisions       string
	MemoryStatsPath []string
	CPUStatsPath    []string
}
