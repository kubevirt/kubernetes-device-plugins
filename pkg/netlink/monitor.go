package netlink

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"syscall"

	"github.com/golang/glog"
	"golang.org/x/sys/unix"
)

// #include <linux/connector.h>
// #include <linux/cn_proc.h>
import "C"

// CbID corresponds to cb_id in connector.h
type CbID struct {
	Idx uint32
	Val uint32
}

// CnMsg corresponds to cn_msg in connector.h
type CnMsg struct {
	ID    CbID
	Seq   uint32
	Ack   uint32
	Len   uint16
	Flags uint16
}

// ProcEventHeader corresponds to proc_event in cn_proc.h
type ProcEventHeader struct {
	What      uint32
	CPU       uint32
	Timestamp uint64
}

// ExitProcEvent corresponds to exit_proc_event in cn_proc.h
type ExitProcEvent struct {
	ProcessPid  uint32
	ProcessTgid uint32
	ExitCode    uint32
	ExitSignal  uint32
}

type Monitor struct {
	messageCallback func(syscall.NetlinkMessage)
}

func NewMonitor(messageCallback func(syscall.NetlinkMessage)) *Monitor {
	return &Monitor{
		messageCallback: messageCallback,
	}
}

func (m *Monitor) Run() error {
	sock, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM, unix.NETLINK_CONNECTOR)
	if err != nil {
		return fmt.Errorf("failed to create unix socket: %s", err)
	}

	addr := &unix.SockaddrNetlink{Family: unix.AF_NETLINK, Groups: C.CN_IDX_PROC, Pid: uint32(os.Getpid())}
	err = unix.Bind(sock, addr)
	if err != nil {
		return fmt.Errorf("failed to bind unix socket: %s", err)
	}

	defer func() {
		send(sock, C.PROC_CN_MCAST_IGNORE)
		unix.Close(sock)
	}()

	err = send(sock, C.PROC_CN_MCAST_LISTEN)
	if err != nil {
		return fmt.Errorf("failed to subscribe monitor for listening: %s", err)
	}

	for {
		p := make([]byte, 1024)

		nlmessages, err := recvData(p, sock)

		if err != nil {
			glog.V(3).Infof("Monitor failed to read new messages: %s", err)
			continue
		}

		for _, message := range nlmessages {
			m.messageCallback(message)
		}
	}
}

func recvData(p []byte, sock int) ([]syscall.NetlinkMessage, error) {
	nr, from, err := unix.Recvfrom(sock, p, 0)

	if sockaddrNl, ok := from.(*unix.SockaddrNetlink); !ok || sockaddrNl.Pid != 0 {
		return nil, fmt.Errorf("sender was not kernel")
	}

	if err != nil {
		return nil, err
	}

	if nr < unix.NLMSG_HDRLEN {
		return nil, fmt.Errorf("received %d bytes", nr)
	}

	nlmessages, err := syscall.ParseNetlinkMessage(p[:nr])

	if err != nil {
		return nil, err
	}

	return nlmessages, nil
}

func send(sock int, msg uint32) error {
	cnMsg := CnMsg{}
	destAddr := &unix.SockaddrNetlink{Family: unix.AF_NETLINK, Groups: C.CN_IDX_PROC, Pid: 0}
	header := unix.NlMsghdr{
		Len:   unix.NLMSG_HDRLEN + uint32(binary.Size(cnMsg)+binary.Size(msg)),
		Type:  uint16(unix.NLMSG_DONE),
		Flags: 0,
		Seq:   1,
		Pid:   uint32(os.Getpid()),
	}
	cnMsg.ID = CbID{Idx: C.CN_IDX_PROC, Val: C.CN_VAL_PROC}
	cnMsg.Len = uint16(binary.Size(msg))
	cnMsg.Ack = 0
	cnMsg.Seq = 1
	buf := bytes.NewBuffer(make([]byte, 0, header.Len))
	binary.Write(buf, binary.LittleEndian, header)
	binary.Write(buf, binary.LittleEndian, cnMsg)
	binary.Write(buf, binary.LittleEndian, msg)

	return unix.Sendto(sock, buf.Bytes(), 0, destAddr)
}

func ProcessTerminationCallback(callback func(pid int)) func(m syscall.NetlinkMessage) {
	return func(m syscall.NetlinkMessage) {
		if m.Header.Type == unix.NLMSG_DONE {
			buf := bytes.NewBuffer(m.Data)
			msg := &CnMsg{}
			hdr := &ProcEventHeader{}
			binary.Read(buf, binary.LittleEndian, msg)
			binary.Read(buf, binary.LittleEndian, hdr)

			if hdr.What == C.PROC_EVENT_EXIT {
				event := &ExitProcEvent{}
				binary.Read(buf, binary.LittleEndian, event)
				pid := int(event.ProcessTgid)
				callback(pid)
			}
		}
	}
}
