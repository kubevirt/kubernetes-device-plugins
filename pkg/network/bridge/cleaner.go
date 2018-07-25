package bridge

import (
	"sync"

	"github.com/golang/glog"
	monitor "github.com/kubevirt/kubernetes-device-plugins/pkg/netlink"
	"github.com/vishvananda/netlink"
)

// interfaceAndPid is used to tranfer user provided data between Register function
// and Run loop.
type interfaceAndPID struct {
	InterfaceName string
	PID           int
}

// cleaner is to be instantiated by DP and used to clean up orphan veth pair sides
// staying in host network namespaces.
type cleaner struct {
	interfaceByPID      map[int]string
	interfaceByPIDMutex *sync.Mutex
	interfaceAndPIDChan chan interfaceAndPID
}

func newCleaner() *cleaner {
	return &cleaner{
		interfaceByPID:      make(map[int]string),
		interfaceByPIDMutex: new(sync.Mutex),
		interfaceAndPIDChan: make(chan interfaceAndPID),
	}
}

// Run is a blocking function keeping track of created veth pair interfaces staying
// in host network namespace and PID of network namespace of the other veth ends.
// It monitors running processes' PIDs, once is the PID gone, it removes respective
// interface in host network namespace.
func (c *cleaner) Run() {
	glog.V(3).Info("Unused interface cleaner was initialized")

	// Listen for newly registered host interface - PID pairs, save them into internal map.
	// This is done in two steps, channel and map update, so user is not blocked on Register call.
	go func() {
		for {
			select {
			case interfaceAndPID := <-c.interfaceAndPIDChan:
				glog.V(3).Infof("Registering new host interface - PID pair: %s [%d]", interfaceAndPID.InterfaceName, interfaceAndPID.PID)
				c.interfaceByPIDMutex.Lock()
				c.interfaceByPID[interfaceAndPID.PID] = interfaceAndPID.InterfaceName
				c.interfaceByPIDMutex.Unlock()
			}
		}
	}()

	// Monitor terminated processes. Register callback that will remove host interface associated
	// with monitored PIDs.
	monitor := monitor.NewMonitor(monitor.ProcessTerminationCallback(func(terminatedPID int) {
		c.interfaceByPIDMutex.Lock()
		if interfaceName, found := c.interfaceByPID[terminatedPID]; found {
			glog.V(3).Infof("Monitored process with PID [%d] was terminated, starting host interface '%s' cleanup", terminatedPID, interfaceName)
			peer, err := netlink.LinkByName(interfaceName)
			if err != nil {
				glog.V(3).Infof("Interface '%s' was already removed", interfaceName)
			} else {
				netlink.LinkDel(peer)
				glog.V(3).Infof("Interface '%s' was removed from host", interfaceName)
			}
			delete(c.interfaceByPID, terminatedPID)
		}
		c.interfaceByPIDMutex.Unlock()
	}))
	err := monitor.Run()
	if err != nil {
		panic(err)
	}
}

// Register function is called by user once is an veth interface created and one of its
// sides moved to container's network namespace.
func (c *cleaner) Register(interfaceName string, pid int) {
	// Send interface PID pair to running cleaner instance
	c.interfaceAndPIDChan <- interfaceAndPID{
		InterfaceName: interfaceName,
		PID:           pid,
	}
}
