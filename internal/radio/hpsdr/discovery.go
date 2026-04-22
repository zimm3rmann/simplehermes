package hpsdr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"syscall"
	"time"

	"simplehermes/internal/radio"
)

const discoveryPort = 1024

func discover(ctx context.Context) ([]radio.Device, error) {
	ifaces, err := eligibleInterfaces()
	if err != nil {
		return nil, err
	}

	devicesByID := map[string]radio.Device{}
	var firstErr error

	for _, iface := range ifaces {
		devices, err := discoverOnInterface(ctx, iface)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		for _, device := range devices {
			devicesByID[device.ID] = device
		}
	}

	result := make([]radio.Device, 0, len(devicesByID))
	for _, device := range devicesByID {
		result = append(result, device)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].Address < result[j].Address
		}
		return result[i].Name < result[j].Name
	})

	if len(result) > 0 {
		return result, nil
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return result, nil
}

type ifaceInfo struct {
	Name string
	IP   net.IP
	Mask net.IPMask
}

func eligibleInterfaces() ([]ifaceInfo, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var out []ifaceInfo
	for _, iface := range all {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil || ipNet.Mask == nil {
				continue
			}

			ip4 := ipNet.IP.To4()
			if ip4 == nil {
				continue
			}

			out = append(out, ifaceInfo{
				Name: iface.Name,
				IP:   ip4,
				Mask: ipNet.Mask,
			})
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no active IPv4 network interfaces found")
	}

	return out, nil
}

func discoverOnInterface(ctx context.Context, iface ifaceInfo) ([]radio.Device, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: iface.IP, Port: 0})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := enableBroadcast(conn); err != nil {
		return nil, err
	}

	broadcast := interfaceBroadcast(iface.IP, iface.Mask)
	target := &net.UDPAddr{IP: broadcast, Port: discoveryPort}

	deadline := time.Now().Add(1200 * time.Millisecond)
	if readDeadline, ok := ctx.Deadline(); ok && readDeadline.Before(deadline) {
		deadline = readDeadline
	}

	if err := conn.SetReadDeadline(deadline); err != nil {
		return nil, err
	}

	if _, err := conn.WriteToUDP(protocol1Request(), target); err != nil {
		return nil, err
	}
	if _, err := conn.WriteToUDP(protocol2Request(), target); err != nil {
		return nil, err
	}

	buf := make([]byte, 2048)
	devicesByID := map[string]radio.Device{}

	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				break
			}
			return nil, err
		}

		device, ok := parseResponse(buf[:n], addr, iface.Name)
		if ok {
			devicesByID[device.ID] = device
		}
	}

	result := make([]radio.Device, 0, len(devicesByID))
	for _, device := range devicesByID {
		result = append(result, device)
	}

	return result, nil
}

func enableBroadcast(conn *net.UDPConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}

	var controlErr error
	if err := raw.Control(func(fd uintptr) {
		controlErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	}); err != nil {
		return err
	}

	return controlErr
}

func interfaceBroadcast(ip net.IP, mask net.IPMask) net.IP {
	out := make(net.IP, len(ip))
	for i := range ip {
		out[i] = ip[i] | ^mask[i]
	}
	return out
}

func protocol1Request() []byte {
	payload := make([]byte, 63)
	payload[0] = 0xEF
	payload[1] = 0xFE
	payload[2] = 0x02
	return payload
}

func protocol2Request() []byte {
	payload := make([]byte, 60)
	payload[4] = 0x02
	return payload
}

func parseResponse(payload []byte, addr *net.UDPAddr, interfaceName string) (radio.Device, bool) {
	if device, ok := parseProtocol1(payload, addr, interfaceName); ok {
		return device, true
	}
	if device, ok := parseProtocol2(payload, addr, interfaceName); ok {
		return device, true
	}
	return radio.Device{}, false
}

func parseProtocol1(payload []byte, addr *net.UDPAddr, interfaceName string) (radio.Device, bool) {
	if len(payload) < 19 || payload[0] != 0xEF || payload[1] != 0xFE {
		return radio.Device{}, false
	}

	status := payload[2]
	if status != 2 && status != 3 {
		return radio.Device{}, false
	}

	deviceCode := payload[10]
	version := int(payload[9])
	if deviceCode != 6 {
		return radio.Device{}, false
	}

	model := "Hermes Lite"
	name := "Hermes Lite"
	if version >= 42 {
		model = "Hermes Lite 2"
		name = "Hermes Lite 2"
	}

	supportedReceivers := 2
	if version >= 42 && len(payload) > 0x13 {
		supportedReceivers = int(payload[0x13])
	}

	return radio.Device{
		ID:                 "p1-" + addr.IP.String() + "-" + strconv.Itoa(addr.Port),
		Name:               name,
		Model:              model,
		Address:            addr.IP.String(),
		InterfaceName:      interfaceName,
		Protocol:           "protocol1",
		SoftwareVersion:    strconv.Itoa(version),
		Status:             deviceStatus(status),
		SupportedReceivers: supportedReceivers,
	}, true
}

func parseProtocol2(payload []byte, addr *net.UDPAddr, interfaceName string) (radio.Device, bool) {
	if len(payload) < 21 {
		return radio.Device{}, false
	}

	if payload[0] != 0x00 || payload[1] != 0x00 || payload[2] != 0x00 || payload[3] != 0x00 {
		return radio.Device{}, false
	}

	status := payload[4]
	if status != 2 && status != 3 {
		return radio.Device{}, false
	}

	if payload[11] != 6 {
		return radio.Device{}, false
	}

	return radio.Device{
		ID:                 "p2-" + addr.IP.String() + "-" + strconv.Itoa(addr.Port),
		Name:               "Hermes Lite",
		Model:              "Hermes Lite",
		Address:            addr.IP.String(),
		InterfaceName:      interfaceName,
		Protocol:           "protocol2",
		SoftwareVersion:    strconv.Itoa(int(payload[13])),
		Status:             deviceStatus(status),
		SupportedReceivers: int(payload[20]),
	}, true
}

func deviceStatus(status byte) string {
	switch status {
	case 2:
		return "available"
	case 3:
		return "streaming"
	default:
		return "unknown"
	}
}
