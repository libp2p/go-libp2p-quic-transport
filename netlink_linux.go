// +build linux

package libp2pquic

import "golang.org/x/sys/unix"

// We just need netlink_route here.
// note: We should avoid the use of netlink_xfrm or netlink_netfilter has it is
// not allowed by Android in his base policy.
var SupportedNlFamilies = []int{unix.NETLINK_ROUTE}
