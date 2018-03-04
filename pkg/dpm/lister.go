package dpm

type PluginNamesList []string

type ListerInterface interface {
	GetResourceName() string
	Discover(chan PluginNamesList)
	NewPlugin(string) PluginInterface
}
