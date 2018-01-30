package dockerutils

import (
	"errors"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

type Client struct {
	*client.Client
}

func NewClient() (*Client, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return &Client{
		cli,
	}, nil
}

func (cli *Client) GetContainerIDByMountedDevice(devicePathInContainer string) (string, error) {
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return "", err
	}

	for _, container := range containers {
		config, err := cli.ContainerInspect(context.Background(), container.ID)
		if err != nil {
			return "", err
		}

		devices := config.HostConfig.Devices
		for _, device := range devices {
			if device.PathInContainer == devicePathInContainer {
				return container.ID, nil
			}
		}
	}

	return "", errors.New("Container not found")
}

func (cli *Client) GetPidByContainerID(containerID string) (int, error) {
	config, err := cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return 0, err
	}

	return config.State.Pid, nil
}
