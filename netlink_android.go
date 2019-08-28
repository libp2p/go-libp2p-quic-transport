// +build android

package libp2pquic

import "golang.org/x/sys/unix"

// Android doesn't allow netlink_xfrm and netlink_netfilter in his base policy
var SupportedNlFamilies = []int{unix.NETLINK_ROUTE}
