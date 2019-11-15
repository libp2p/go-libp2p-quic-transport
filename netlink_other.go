// +build !linux
// +build !windows

package libp2pquic

import "github.com/vishvananda/netlink/nl"

// SupportedNlFamilies is the default netlink families used by the netlink package
var SupportedNlFamilies = nl.SupportedNlFamilies
