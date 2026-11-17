//go:build android

package tsnetbridge

import (
	"net"
	"net/netip"
	"time"

	"tailscale.com/net/netmon"
)

func init() {
	// Android 11+ (API 30+) SELinux blocks NETLINK_ROUTE sockets (netlinkrib: permission denied).
	// Register a userspace fallback so tsnet netmon initializes cleanly without VpnService or root.
	netmon.RegisterInterfaceGetter(androidInterfaceGetter)
}

func androidInterfaceGetter() ([]netmon.Interface, error) {
	if ifs, err := net.Interfaces(); err == nil && len(ifs) > 0 {
		ret := make([]netmon.Interface, len(ifs))
		for i := range ifs {
			ret[i].Interface = &ifs[i]
		}
		return ret, nil
	}
	return syntheticAndroidInterfaces(), nil
}

func syntheticAndroidInterfaces() []netmon.Interface {
	var addrs []net.Addr
	if ip, ok := outboundSourceIP("udp4", "8.8.8.8:53"); ok {
		addrs = append(addrs, &net.IPNet{IP: ip.AsSlice(), Mask: net.CIDRMask(32, 32)})
	}
	if ip, ok := outboundSourceIP("udp6", "[2001:4860:4860::8888]:53"); ok {
		addrs = append(addrs, &net.IPNet{IP: ip.AsSlice(), Mask: net.CIDRMask(128, 128)})
	}
	if len(addrs) == 0 {
		addrs = append(addrs, &net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)})
		return []netmon.Interface{{
			Interface: &net.Interface{
				Index: 1,
				MTU:   65536,
				Name:  "lo",
				Flags: net.FlagUp | net.FlagLoopback | net.FlagRunning,
			},
			AltAddrs: addrs,
		}}
	}
	return []netmon.Interface{{
		Interface: &net.Interface{
			Index: 1,
			MTU:   1500,
			Name:  "android",
			Flags: net.FlagUp | net.FlagRunning | net.FlagBroadcast | net.FlagMulticast,
		},
		AltAddrs: addrs,
	}}
}

func outboundSourceIP(network, target string) (netip.Addr, bool) {
	d := net.Dialer{Timeout: 500 * time.Millisecond}
	c, err := d.Dial(network, target)
	if err != nil {
		return netip.Addr{}, false
	}
	defer c.Close()
	ua, ok := c.LocalAddr().(*net.UDPAddr)
	if !ok {
		return netip.Addr{}, false
	}
	addr, ok := netip.AddrFromSlice(ua.IP)
	if !ok {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}
